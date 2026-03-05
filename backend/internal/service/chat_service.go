package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/kowming/vue-idle-xiuxian/backend/internal/repository"
)

const (
	chatDefaultChannel = "world"
	chatMaxContentRune = 200
	chatWordBlockTTL   = time.Minute
)

var chatAllowedChannels = map[string]struct{}{
	"world":  {},
	"system": {},
}

var chatDefaultWordBlock = []string{
	"傻逼",
	"fuck",
	"shit",
	"nmsl",
}

type ChatService struct {
	pool      *pgxpool.Pool
	userRepo  *repository.UserRepository
	mu        sync.Mutex
	lastSent  map[uuid.UUID]time.Time
	minGap    time.Duration
	wordMu    sync.RWMutex
	wordBlock []string
	wordTTL   time.Duration
	wordAt    time.Time
}

func NewChatService(pool *pgxpool.Pool, userRepo *repository.UserRepository) *ChatService {
	s := &ChatService{
		pool:      pool,
		userRepo:  userRepo,
		lastSent:  make(map[uuid.UUID]time.Time),
		minGap:    3 * time.Second,
		wordBlock: cloneChatWords(chatDefaultWordBlock),
		wordTTL:   chatWordBlockTTL,
	}
	loadCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_ = s.refreshBlockedWords(loadCtx)
	return s
}

type ChatMessage struct {
	ID           int64     `json:"id"`
	Channel      string    `json:"channel"`
	SenderUserID string    `json:"senderUserId,omitempty"`
	SenderName   string    `json:"senderName"`
	Content      string    `json:"content"`
	CreatedAt    time.Time `json:"createdAt"`
}

type ChatHistoryResult struct {
	Channel  string        `json:"channel"`
	Messages []ChatMessage `json:"messages"`
}

type ChatMuteStatus struct {
	Muted      bool       `json:"muted"`
	MutedUntil *time.Time `json:"mutedUntil,omitempty"`
	Reason     string     `json:"reason,omitempty"`
}

type ChatMuteRecord struct {
	ID                  int64     `json:"id"`
	TargetLinuxDoUserID string    `json:"targetLinuxDoUserId"`
	TargetName          string    `json:"targetName"`
	Reason              string    `json:"reason"`
	MutedUntil          time.Time `json:"mutedUntil"`
	CreatedAt           time.Time `json:"createdAt"`
	CreatedByLinuxDoID  string    `json:"createdByLinuxDoUserId,omitempty"`
}

type ChatMuteListResult struct {
	Mutes []ChatMuteRecord `json:"mutes"`
}

type ChatBlockedWord struct {
	Word      string    `json:"word"`
	Enabled   bool      `json:"enabled"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type ChatBlockedWordListResult struct {
	Words []ChatBlockedWord `json:"words"`
}

type InvalidChatChannelError struct {
	Channel string
}

func (e *InvalidChatChannelError) Error() string {
	return fmt.Sprintf("invalid chat channel: %s", e.Channel)
}

type InvalidChatContentError struct {
	Reason string
}

func (e *InvalidChatContentError) Error() string {
	return fmt.Sprintf("invalid chat content: %s", e.Reason)
}

type ChatRateLimitedError struct {
	RetryAfter time.Duration
}

func (e *ChatRateLimitedError) Error() string {
	return fmt.Sprintf("chat rate limited: retry after %s", e.RetryAfter)
}

type ChatMutedError struct {
	MutedUntil time.Time
}

func (e *ChatMutedError) Error() string {
	return fmt.Sprintf("chat muted until: %s", e.MutedUntil.UTC().Format(time.RFC3339))
}

type ChatMessageNotFoundError struct {
	MessageID int64
}

func (e *ChatMessageNotFoundError) Error() string {
	return fmt.Sprintf("chat message not found: %d", e.MessageID)
}

type ChatTargetUserNotFoundError struct {
	LinuxDoUserID string
}

func (e *ChatTargetUserNotFoundError) Error() string {
	return fmt.Sprintf("chat target user not found: %s", e.LinuxDoUserID)
}

type InvalidChatMuteDurationError struct {
	DurationMinutes int
}

func (e *InvalidChatMuteDurationError) Error() string {
	return fmt.Sprintf("invalid chat mute duration minutes: %d", e.DurationMinutes)
}

type InvalidChatBlockedWordError struct {
	Reason string
}

func (e *InvalidChatBlockedWordError) Error() string {
	return fmt.Sprintf("invalid chat blocked word: %s", e.Reason)
}

func (s *ChatService) History(ctx context.Context, channel string, limit int) (*ChatHistoryResult, error) {
	channel = normalizeChannel(channel)
	if err := validateChannel(channel); err != nil {
		return nil, err
	}
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}

	const query = `
		SELECT
			id,
			channel,
			COALESCE(sender_user_id::text, ''),
			sender_name,
			content,
			created_at
		FROM chat_messages
		WHERE channel = $1
		ORDER BY created_at DESC
		LIMIT $2
	`

	rows, err := s.pool.Query(ctx, query, channel, limit)
	if err != nil {
		return nil, fmt.Errorf("query chat history: %w", err)
	}
	defer rows.Close()

	messages := make([]ChatMessage, 0, limit)
	for rows.Next() {
		var message ChatMessage
		if err := rows.Scan(&message.ID, &message.Channel, &message.SenderUserID, &message.SenderName, &message.Content, &message.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan chat history row: %w", err)
		}
		messages = append(messages, message)
	}
	if rows.Err() != nil {
		return nil, fmt.Errorf("iterate chat history rows: %w", rows.Err())
	}

	for left, right := 0, len(messages)-1; left < right; left, right = left+1, right-1 {
		messages[left], messages[right] = messages[right], messages[left]
	}

	return &ChatHistoryResult{
		Channel:  channel,
		Messages: messages,
	}, nil
}

func (s *ChatService) Send(ctx context.Context, userID uuid.UUID, channel string, content string) (*ChatMessage, error) {
	channel = normalizeChannel(channel)
	if err := validateChannel(channel); err != nil {
		return nil, err
	}

	content = strings.TrimSpace(content)
	if content == "" {
		return nil, &InvalidChatContentError{Reason: "empty"}
	}
	if utf8.RuneCountInString(content) > chatMaxContentRune {
		return nil, &InvalidChatContentError{Reason: "too_long"}
	}
	if err := s.checkRateLimit(userID); err != nil {
		return nil, err
	}
	if err := s.checkMuted(ctx, userID); err != nil {
		return nil, err
	}

	user, err := s.userRepo.FindByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, fmt.Errorf("chat sender user not found")
	}

	s.maybeRefreshBlockedWords(ctx)
	filtered := s.filterContent(content)

	const insertSQL = `
		INSERT INTO chat_messages (channel, sender_user_id, sender_name, content, created_at)
		VALUES ($1, $2, $3, $4, now())
		RETURNING id, created_at
	`

	message := &ChatMessage{
		Channel:      channel,
		SenderUserID: userID.String(),
		SenderName:   user.LinuxDoUsername,
		Content:      filtered,
	}
	if err := s.pool.QueryRow(ctx, insertSQL, channel, userID, message.SenderName, filtered).Scan(&message.ID, &message.CreatedAt); err != nil {
		return nil, fmt.Errorf("insert chat message: %w", err)
	}
	return message, nil
}

func (s *ChatService) Report(ctx context.Context, reporterID uuid.UUID, messageID int64, reason string) error {
	if messageID <= 0 {
		return &ChatMessageNotFoundError{MessageID: messageID}
	}
	reason = strings.TrimSpace(reason)
	if reason == "" {
		reason = "no_reason"
	}

	const findSQL = `SELECT EXISTS(SELECT 1 FROM chat_messages WHERE id = $1)`
	var exists bool
	if err := s.pool.QueryRow(ctx, findSQL, messageID).Scan(&exists); err != nil {
		return fmt.Errorf("check chat message exists: %w", err)
	}
	if !exists {
		return &ChatMessageNotFoundError{MessageID: messageID}
	}

	detailJSON, err := json.Marshal(map[string]any{
		"messageId":  messageID,
		"reporterId": reporterID.String(),
		"reason":     reason,
	})
	if err != nil {
		return fmt.Errorf("marshal chat report detail: %w", err)
	}

	const insertSQL = `
		INSERT INTO risk_events (user_id, event_type, severity, detail, created_at)
		VALUES ($1, 'chat_report', 'medium', $2::jsonb, now())
	`
	if _, err := s.pool.Exec(ctx, insertSQL, reporterID, string(detailJSON)); err != nil {
		return fmt.Errorf("insert chat report risk event: %w", err)
	}
	return nil
}

func (s *ChatService) MuteStatus(ctx context.Context, userID uuid.UUID) (*ChatMuteStatus, error) {
	const query = `
		SELECT muted_until, COALESCE(reason, '')
		FROM chat_mutes
		WHERE user_id = $1
		  AND muted_until > now()
		ORDER BY muted_until DESC
		LIMIT 1
	`

	var (
		mutedUntil time.Time
		reason     string
	)
	if err := s.pool.QueryRow(ctx, query, userID).Scan(&mutedUntil, &reason); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return &ChatMuteStatus{Muted: false}, nil
		}
		return nil, fmt.Errorf("query chat mute status: %w", err)
	}
	ts := mutedUntil.UTC()
	return &ChatMuteStatus{
		Muted:      true,
		MutedUntil: &ts,
		Reason:     reason,
	}, nil
}

func (s *ChatService) MuteByLinuxDoUserID(
	ctx context.Context,
	targetLinuxDoUserID string,
	durationMinutes int,
	reason string,
	operatorUserID uuid.UUID,
) (*ChatMuteStatus, error) {
	targetLinuxDoUserID = strings.TrimSpace(targetLinuxDoUserID)
	if targetLinuxDoUserID == "" {
		return nil, &ChatTargetUserNotFoundError{LinuxDoUserID: targetLinuxDoUserID}
	}
	if durationMinutes <= 0 || durationMinutes > 7*24*60 {
		return nil, &InvalidChatMuteDurationError{DurationMinutes: durationMinutes}
	}
	reason = strings.TrimSpace(reason)
	if reason == "" {
		reason = "admin_mute"
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin chat admin mute transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	targetUser, err := s.findUserByLinuxDoUserIDTx(ctx, tx, targetLinuxDoUserID)
	if err != nil {
		return nil, err
	}
	if targetUser == nil {
		return nil, &ChatTargetUserNotFoundError{LinuxDoUserID: targetLinuxDoUserID}
	}

	mutedUntil := time.Now().UTC().Add(time.Duration(durationMinutes) * time.Minute)
	const insertSQL = `
		INSERT INTO chat_mutes (user_id, reason, muted_until, created_by_user_id, created_at)
		VALUES ($1, $2, $3, $4, now())
	`
	if _, err := tx.Exec(ctx, insertSQL, targetUser.ID, reason, mutedUntil, operatorUserID); err != nil {
		return nil, fmt.Errorf("insert chat mute: %w", err)
	}

	if err := s.insertChatAdminRiskEventTx(ctx, tx, operatorUserID, "mute", map[string]any{
		"targetLinuxDoUserId": targetUser.LinuxDoUserID,
		"targetUserId":        targetUser.ID.String(),
		"durationMinutes":     durationMinutes,
		"reason":              reason,
		"mutedUntil":          mutedUntil,
	}); err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit chat admin mute transaction: %w", err)
	}

	return &ChatMuteStatus{
		Muted:      true,
		MutedUntil: &mutedUntil,
		Reason:     reason,
	}, nil
}

func (s *ChatService) UnmuteByLinuxDoUserID(ctx context.Context, targetLinuxDoUserID string, operatorUserID uuid.UUID) (bool, error) {
	targetLinuxDoUserID = strings.TrimSpace(targetLinuxDoUserID)
	if targetLinuxDoUserID == "" {
		return false, &ChatTargetUserNotFoundError{LinuxDoUserID: targetLinuxDoUserID}
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return false, fmt.Errorf("begin chat admin unmute transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	targetUser, err := s.findUserByLinuxDoUserIDTx(ctx, tx, targetLinuxDoUserID)
	if err != nil {
		return false, err
	}
	if targetUser == nil {
		return false, &ChatTargetUserNotFoundError{LinuxDoUserID: targetLinuxDoUserID}
	}

	const updateSQL = `
		UPDATE chat_mutes
		SET muted_until = now()
		WHERE user_id = $1
		  AND muted_until > now()
	`
	tag, err := tx.Exec(ctx, updateSQL, targetUser.ID)
	if err != nil {
		return false, fmt.Errorf("update chat mutes to unmute: %w", err)
	}
	updated := tag.RowsAffected() > 0

	if err := s.insertChatAdminRiskEventTx(ctx, tx, operatorUserID, "unmute", map[string]any{
		"targetLinuxDoUserId": targetUser.LinuxDoUserID,
		"targetUserId":        targetUser.ID.String(),
		"updated":             updated,
	}); err != nil {
		return false, err
	}

	if err := tx.Commit(ctx); err != nil {
		return false, fmt.Errorf("commit chat admin unmute transaction: %w", err)
	}
	return updated, nil
}

func (s *ChatService) ListActiveMutes(ctx context.Context, targetLinuxDoUserID string, limit int) (*ChatMuteListResult, error) {
	targetLinuxDoUserID = strings.TrimSpace(targetLinuxDoUserID)
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}

	const query = `
		SELECT
			cm.id,
			tu.linux_do_user_id,
			COALESCE(tu.linux_do_username, ''),
			COALESCE(cm.reason, ''),
			cm.muted_until,
			cm.created_at,
			COALESCE(ou.linux_do_user_id, '')
		FROM chat_mutes cm
		JOIN users tu ON tu.id = cm.user_id
		LEFT JOIN users ou ON ou.id = cm.created_by_user_id
		WHERE cm.muted_until > now()
		  AND ($1 = '' OR tu.linux_do_user_id = $1)
		ORDER BY cm.muted_until DESC, cm.id DESC
		LIMIT $2
	`

	rows, err := s.pool.Query(ctx, query, targetLinuxDoUserID, limit)
	if err != nil {
		return nil, fmt.Errorf("query active chat mutes: %w", err)
	}
	defer rows.Close()

	records := make([]ChatMuteRecord, 0, limit)
	for rows.Next() {
		record := ChatMuteRecord{}
		if err := rows.Scan(
			&record.ID,
			&record.TargetLinuxDoUserID,
			&record.TargetName,
			&record.Reason,
			&record.MutedUntil,
			&record.CreatedAt,
			&record.CreatedByLinuxDoID,
		); err != nil {
			return nil, fmt.Errorf("scan active chat mute row: %w", err)
		}
		records = append(records, record)
	}
	if rows.Err() != nil {
		return nil, fmt.Errorf("iterate active chat mutes rows: %w", rows.Err())
	}

	return &ChatMuteListResult{Mutes: records}, nil
}

func (s *ChatService) ListBlockedWords(ctx context.Context, includeDisabled bool, limit int) (*ChatBlockedWordListResult, error) {
	if limit <= 0 {
		limit = 200
	}
	if limit > 500 {
		limit = 500
	}

	const query = `
		SELECT word, enabled, updated_at
		FROM chat_block_words
		WHERE ($1 OR enabled = true)
		ORDER BY updated_at DESC, word ASC
		LIMIT $2
	`

	rows, err := s.pool.Query(ctx, query, includeDisabled, limit)
	if err != nil {
		return nil, fmt.Errorf("query chat blocked words: %w", err)
	}
	defer rows.Close()

	words := make([]ChatBlockedWord, 0, limit)
	for rows.Next() {
		record := ChatBlockedWord{}
		if err := rows.Scan(&record.Word, &record.Enabled, &record.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan chat blocked word row: %w", err)
		}
		words = append(words, record)
	}
	if rows.Err() != nil {
		return nil, fmt.Errorf("iterate chat blocked word rows: %w", rows.Err())
	}

	return &ChatBlockedWordListResult{Words: words}, nil
}

func (s *ChatService) UpsertBlockedWord(ctx context.Context, word string, enabled bool, operatorUserID uuid.UUID) (*ChatBlockedWord, error) {
	normalized, err := normalizeBlockedWord(word)
	if err != nil {
		return nil, err
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin upsert chat blocked word transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	const query = `
		INSERT INTO chat_block_words (word, enabled, created_at, updated_at)
		VALUES ($1, $2, now(), now())
		ON CONFLICT (word)
		DO UPDATE SET enabled = EXCLUDED.enabled, updated_at = now()
		RETURNING word, enabled, updated_at
	`

	record := &ChatBlockedWord{}
	if err := tx.QueryRow(ctx, query, normalized, enabled).Scan(&record.Word, &record.Enabled, &record.UpdatedAt); err != nil {
		return nil, fmt.Errorf("upsert chat blocked word: %w", err)
	}

	if err := s.insertChatAdminRiskEventTx(ctx, tx, operatorUserID, "upsert_block_word", map[string]any{
		"word":    record.Word,
		"enabled": record.Enabled,
	}); err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit upsert chat blocked word transaction: %w", err)
	}
	_ = s.refreshBlockedWords(ctx)
	return record, nil
}

func (s *ChatService) DeleteBlockedWord(ctx context.Context, word string, operatorUserID uuid.UUID) (bool, error) {
	normalized, err := normalizeBlockedWord(word)
	if err != nil {
		return false, err
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return false, fmt.Errorf("begin delete chat blocked word transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	const query = `DELETE FROM chat_block_words WHERE word = $1`
	tag, err := tx.Exec(ctx, query, normalized)
	if err != nil {
		return false, fmt.Errorf("delete chat blocked word: %w", err)
	}
	removed := tag.RowsAffected() > 0

	if err := s.insertChatAdminRiskEventTx(ctx, tx, operatorUserID, "delete_block_word", map[string]any{
		"word":    normalized,
		"removed": removed,
	}); err != nil {
		return false, err
	}

	if err := tx.Commit(ctx); err != nil {
		return false, fmt.Errorf("commit delete chat blocked word transaction: %w", err)
	}
	_ = s.refreshBlockedWords(ctx)
	return removed, nil
}

type chatUserRef struct {
	ID              uuid.UUID
	LinuxDoUserID   string
	LinuxDoUsername string
}

func (s *ChatService) findUserByLinuxDoUserIDTx(ctx context.Context, tx pgx.Tx, linuxDoUserID string) (*chatUserRef, error) {
	const query = `
		SELECT id, linux_do_user_id, linux_do_username
		FROM users
		WHERE linux_do_user_id = $1
	`

	record := &chatUserRef{}
	if err := tx.QueryRow(ctx, query, linuxDoUserID).Scan(&record.ID, &record.LinuxDoUserID, &record.LinuxDoUsername); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("query user by linux do user id in tx: %w", err)
	}
	return record, nil
}

func (s *ChatService) insertChatAdminRiskEventTx(ctx context.Context, tx pgx.Tx, operatorUserID uuid.UUID, action string, detail map[string]any) error {
	if detail == nil {
		detail = map[string]any{}
	}
	detail["action"] = action
	detail["operatorUserId"] = operatorUserID.String()
	detail["occurredAt"] = time.Now().UTC()

	detailJSON, err := json.Marshal(detail)
	if err != nil {
		return fmt.Errorf("marshal chat admin risk event detail: %w", err)
	}

	const query = `
		INSERT INTO risk_events (user_id, event_type, severity, detail, created_at)
		VALUES ($1, 'chat_admin_action', 'low', $2::jsonb, now())
	`
	if _, err := tx.Exec(ctx, query, operatorUserID, string(detailJSON)); err != nil {
		return fmt.Errorf("insert chat admin risk event: %w", err)
	}
	return nil
}

func (s *ChatService) maybeRefreshBlockedWords(ctx context.Context) {
	s.wordMu.RLock()
	shouldRefresh := s.wordAt.IsZero() || time.Since(s.wordAt) >= s.wordTTL
	s.wordMu.RUnlock()
	if !shouldRefresh {
		return
	}
	_ = s.refreshBlockedWords(ctx)
}

func (s *ChatService) refreshBlockedWords(ctx context.Context) error {
	const query = `
		SELECT word
		FROM chat_block_words
		WHERE enabled = true
		ORDER BY char_length(word) DESC, word ASC
	`

	rows, err := s.pool.Query(ctx, query)
	if err != nil {
		return fmt.Errorf("query enabled chat blocked words: %w", err)
	}
	defer rows.Close()

	words := make([]string, 0, 32)
	for rows.Next() {
		var word string
		if err := rows.Scan(&word); err != nil {
			return fmt.Errorf("scan enabled chat blocked word row: %w", err)
		}
		word = strings.TrimSpace(word)
		if word == "" {
			continue
		}
		words = append(words, word)
	}
	if rows.Err() != nil {
		return fmt.Errorf("iterate enabled chat blocked words: %w", rows.Err())
	}
	if len(words) == 0 {
		words = cloneChatWords(chatDefaultWordBlock)
	}

	s.wordMu.Lock()
	s.wordBlock = words
	s.wordAt = time.Now().UTC()
	s.wordMu.Unlock()
	return nil
}

func (s *ChatService) checkMuted(ctx context.Context, userID uuid.UUID) error {
	status, err := s.MuteStatus(ctx, userID)
	if err != nil {
		return err
	}
	if status == nil || !status.Muted || status.MutedUntil == nil {
		return nil
	}
	return &ChatMutedError{MutedUntil: *status.MutedUntil}
}

func (s *ChatService) checkRateLimit(userID uuid.UUID) error {
	now := time.Now()
	s.mu.Lock()
	defer s.mu.Unlock()

	if last, ok := s.lastSent[userID]; ok {
		diff := now.Sub(last)
		if diff < s.minGap {
			return &ChatRateLimitedError{RetryAfter: s.minGap - diff}
		}
	}
	s.lastSent[userID] = now
	return nil
}

func (s *ChatService) filterContent(content string) string {
	words := s.snapshotBlockedWords()
	filtered := content
	for _, word := range words {
		word = strings.TrimSpace(word)
		if word == "" {
			continue
		}
		masked := strings.Repeat("*", utf8.RuneCountInString(word))
		filtered = strings.ReplaceAll(filtered, word, masked)
		filtered = strings.ReplaceAll(filtered, strings.ToUpper(word), masked)
		filtered = strings.ReplaceAll(filtered, strings.ToLower(word), masked)
	}
	return filtered
}

func (s *ChatService) snapshotBlockedWords() []string {
	s.wordMu.RLock()
	defer s.wordMu.RUnlock()
	return cloneChatWords(s.wordBlock)
}

func normalizeChannel(channel string) string {
	channel = strings.TrimSpace(strings.ToLower(channel))
	if channel == "" {
		return chatDefaultChannel
	}
	return channel
}

func validateChannel(channel string) error {
	if _, ok := chatAllowedChannels[channel]; !ok {
		return &InvalidChatChannelError{Channel: channel}
	}
	return nil
}

func normalizeBlockedWord(word string) (string, error) {
	normalized := strings.TrimSpace(strings.ToLower(word))
	if normalized == "" {
		return "", &InvalidChatBlockedWordError{Reason: "empty"}
	}
	if utf8.RuneCountInString(normalized) > 32 {
		return "", &InvalidChatBlockedWordError{Reason: "too_long"}
	}
	return normalized, nil
}

func cloneChatWords(words []string) []string {
	if len(words) == 0 {
		return []string{}
	}
	out := make([]string, len(words))
	copy(out, words)
	return out
}
