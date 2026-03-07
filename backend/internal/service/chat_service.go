package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
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
	pool          *pgxpool.Pool
	userRepo      *repository.UserRepository
	runtimeConfig *RuntimeConfigService
	mu            sync.Mutex
	lastSent      map[uuid.UUID]time.Time
	minGap        time.Duration
	wordMu        sync.RWMutex
	wordBlock     []string
	wordTTL       time.Duration
	wordAt        time.Time
}

func NewChatService(pool *pgxpool.Pool, userRepo *repository.UserRepository, runtimeConfig *RuntimeConfigService) *ChatService {
	s := &ChatService{
		pool:          pool,
		userRepo:      userRepo,
		runtimeConfig: runtimeConfig,
		lastSent:      make(map[uuid.UUID]time.Time),
		minGap:        3 * time.Second,
		wordBlock:     cloneChatWords(chatDefaultWordBlock),
		wordTTL:       chatWordBlockTTL,
	}
	loadCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_ = s.refreshBlockedWords(loadCtx)
	return s
}

type ChatMessage struct {
	ID                  int64     `json:"id"`
	Channel             string    `json:"channel"`
	SenderUserID        string    `json:"senderUserId,omitempty"`
	SenderLinuxDoUserID string    `json:"senderLinuxDoUserId,omitempty"`
	SenderName          string    `json:"senderName"`
	Content             string    `json:"content"`
	CreatedAt           time.Time `json:"createdAt"`
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

type ChatReportRecord struct {
	ID                         int64      `json:"id"`
	MessageID                  int64      `json:"messageId"`
	ReporterLinuxDoUserID      string     `json:"reporterLinuxDoUserId"`
	ReporterName               string     `json:"reporterName"`
	Reason                     string     `json:"reason"`
	MessageContent             string     `json:"messageContent"`
	MessageSenderName          string     `json:"messageSenderName"`
	MessageSenderLinuxDoUserID string     `json:"messageSenderLinuxDoUserId"`
	ReviewStatus               string     `json:"reviewStatus"`
	ReviewNote                 string     `json:"reviewNote"`
	ReviewedByLinuxDoUserID    string     `json:"reviewedByLinuxDoUserId"`
	ReviewedAt                 *time.Time `json:"reviewedAt,omitempty"`
	CreatedAt                  time.Time  `json:"createdAt"`
}

type ChatReportListResult struct {
	Reports []ChatReportRecord `json:"reports"`
}

type ChatCleanupResult struct {
	DeletedExpired  int64 `json:"deletedExpired"`
	DeletedOverflow int64 `json:"deletedOverflow"`
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

type ChatReportNotFoundError struct {
	ReportID int64
}

func (e *ChatReportNotFoundError) Error() string {
	return fmt.Sprintf("chat report not found: %d", e.ReportID)
}

type InvalidChatReportReviewStatusError struct {
	Status string
}

func (e *InvalidChatReportReviewStatusError) Error() string {
	return fmt.Sprintf("invalid chat report review status: %s", e.Status)
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
			cm.id,
			cm.channel,
			COALESCE(cm.sender_user_id::text, ''),
			COALESCE(u.linux_do_user_id, ''),
			cm.sender_name,
			cm.content,
			cm.created_at
		FROM chat_messages cm
		LEFT JOIN users u ON u.id = cm.sender_user_id
		WHERE cm.channel = $1
		ORDER BY cm.created_at DESC
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
		if err := rows.Scan(
			&message.ID,
			&message.Channel,
			&message.SenderUserID,
			&message.SenderLinuxDoUserID,
			&message.SenderName,
			&message.Content,
			&message.CreatedAt,
		); err != nil {
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
	maxContentRune := s.chatMaxContentRune(ctx)
	if utf8.RuneCountInString(content) > maxContentRune {
		return nil, &InvalidChatContentError{Reason: "too_long"}
	}
	if err := s.checkRateLimit(ctx, userID); err != nil {
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
		Channel:             channel,
		SenderUserID:        userID.String(),
		SenderLinuxDoUserID: strings.TrimSpace(user.LinuxDoUserID),
		SenderName:          user.LinuxDoUsername,
		Content:             filtered,
	}
	if err := s.pool.QueryRow(ctx, insertSQL, channel, userID, message.SenderName, filtered).Scan(&message.ID, &message.CreatedAt); err != nil {
		return nil, fmt.Errorf("insert chat message: %w", err)
	}
	return message, nil
}

func (s *ChatService) Cleanup(ctx context.Context, retentionTTL time.Duration, maxMessages int) (*ChatCleanupResult, error) {
	if retentionTTL <= 0 {
		retentionTTL = 10 * time.Minute
	}
	if maxMessages <= 0 {
		maxMessages = 500
	}

	cutoff := time.Now().UTC().Add(-retentionTTL)

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin chat cleanup transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	const deleteExpiredSQL = `
		DELETE FROM chat_messages
		WHERE created_at < $1
	`
	tag, err := tx.Exec(ctx, deleteExpiredSQL, cutoff)
	if err != nil {
		return nil, fmt.Errorf("delete expired chat messages: %w", err)
	}
	deletedExpired := tag.RowsAffected()

	const deleteOverflowSQL = `
		WITH overflow AS (
			SELECT id
			FROM chat_messages
			ORDER BY created_at DESC, id DESC
			OFFSET $1
		)
		DELETE FROM chat_messages cm
		USING overflow
		WHERE cm.id = overflow.id
	`
	tag, err = tx.Exec(ctx, deleteOverflowSQL, maxMessages)
	if err != nil {
		return nil, fmt.Errorf("delete overflow chat messages: %w", err)
	}
	deletedOverflow := tag.RowsAffected()

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit chat cleanup transaction: %w", err)
	}

	return &ChatCleanupResult{
		DeletedExpired:  deletedExpired,
		DeletedOverflow: deletedOverflow,
	}, nil
}

func (s *ChatService) Report(ctx context.Context, reporterID uuid.UUID, messageID int64, reason string) error {
	if messageID <= 0 {
		return &ChatMessageNotFoundError{MessageID: messageID}
	}
	reason = strings.TrimSpace(reason)
	if reason == "" {
		reason = "no_reason"
	}

	const findSQL = `
		SELECT
			COALESCE(cm.sender_user_id::text, ''),
			COALESCE(u.linux_do_user_id, ''),
			COALESCE(u.linux_do_username, ''),
			COALESCE(cm.sender_name, '')
		FROM chat_messages cm
		LEFT JOIN users u ON u.id = cm.sender_user_id
		WHERE cm.id = $1
	`
	var (
		targetUserIDRaw  string
		targetLinuxDoID  string
		targetLinuxName  string
		targetSenderName string
	)
	if err := s.pool.QueryRow(ctx, findSQL, messageID).Scan(
		&targetUserIDRaw,
		&targetLinuxDoID,
		&targetLinuxName,
		&targetSenderName,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return &ChatMessageNotFoundError{MessageID: messageID}
		}
		return fmt.Errorf("query chat message for report: %w", err)
	}

	detail := map[string]any{
		"messageId":  messageID,
		"reporterId": reporterID.String(),
		"reason":     reason,
	}
	if parsedTargetUserID, err := uuid.Parse(strings.TrimSpace(targetUserIDRaw)); err == nil {
		detail["targetUserId"] = parsedTargetUserID.String()
	}
	if strings.TrimSpace(targetLinuxDoID) != "" {
		detail["targetLinuxDoUserId"] = strings.TrimSpace(targetLinuxDoID)
	}
	targetName := strings.TrimSpace(targetLinuxName)
	if targetName == "" {
		targetName = strings.TrimSpace(targetSenderName)
	}
	if targetName != "" {
		detail["targetName"] = targetName
	}

	detailJSON, err := json.Marshal(detail)
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
	maxMuteMinutes := s.chatAdminMaxMuteMinutes(ctx)
	if durationMinutes <= 0 || durationMinutes > maxMuteMinutes {
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

func (s *ChatService) ListReports(ctx context.Context, status string, limit int) (*ChatReportListResult, error) {
	status = normalizeChatReportFilterStatus(status)
	if limit <= 0 {
		limit = 100
	}
	if limit > 300 {
		limit = 300
	}

	const query = `
		SELECT
			re.id,
			COALESCE(msg.message_id, 0),
			COALESCE(ru.linux_do_user_id, ''),
			COALESCE(ru.linux_do_username, ''),
			COALESCE(re.detail->>'reason', ''),
			COALESCE(cm.content, ''),
			COALESCE(cm.sender_name, ''),
			COALESCE(su.linux_do_user_id, ''),
			COALESCE(re.detail->>'reviewStatus', 'pending'),
			COALESCE(re.detail->>'reviewNote', ''),
			COALESCE(re.detail->>'reviewedByLinuxDoUserId', ''),
			CASE
				WHEN COALESCE(re.detail->>'reviewedAt', '') = '' THEN NULL
				ELSE (re.detail->>'reviewedAt')::timestamptz
			END AS reviewed_at,
			re.created_at
		FROM risk_events re
		LEFT JOIN users ru ON ru.id = re.user_id
		LEFT JOIN LATERAL (
			SELECT CASE
				WHEN COALESCE(re.detail->>'messageId', '') ~ '^[0-9]+$' THEN (re.detail->>'messageId')::bigint
				ELSE NULL
			END AS message_id
		) msg ON true
		LEFT JOIN chat_messages cm ON cm.id = msg.message_id
		LEFT JOIN users su ON su.id = cm.sender_user_id
		WHERE re.event_type = 'chat_report'
			AND (
				$1 = 'all'
				OR COALESCE(re.detail->>'reviewStatus', 'pending') = $1
			)
		ORDER BY re.created_at DESC, re.id DESC
		LIMIT $2
	`

	rows, err := s.pool.Query(ctx, query, status, limit)
	if err != nil {
		return nil, fmt.Errorf("query chat reports: %w", err)
	}
	defer rows.Close()

	records := make([]ChatReportRecord, 0, limit)
	for rows.Next() {
		record := ChatReportRecord{}
		if err := rows.Scan(
			&record.ID,
			&record.MessageID,
			&record.ReporterLinuxDoUserID,
			&record.ReporterName,
			&record.Reason,
			&record.MessageContent,
			&record.MessageSenderName,
			&record.MessageSenderLinuxDoUserID,
			&record.ReviewStatus,
			&record.ReviewNote,
			&record.ReviewedByLinuxDoUserID,
			&record.ReviewedAt,
			&record.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan chat report row: %w", err)
		}
		record.ReviewStatus = normalizeChatReportState(record.ReviewStatus)
		records = append(records, record)
	}
	if rows.Err() != nil {
		return nil, fmt.Errorf("iterate chat report rows: %w", rows.Err())
	}

	return &ChatReportListResult{Reports: records}, nil
}

func (s *ChatService) ReviewReport(
	ctx context.Context,
	reportID int64,
	reviewStatus string,
	reviewNote string,
	operatorUserID uuid.UUID,
) error {
	if reportID <= 0 {
		return &ChatReportNotFoundError{ReportID: reportID}
	}
	rawReviewStatus := strings.TrimSpace(reviewStatus)
	reviewStatus = normalizeChatReportReviewStatus(reviewStatus)
	if reviewStatus == "" {
		return &InvalidChatReportReviewStatusError{Status: rawReviewStatus}
	}
	reviewNote = strings.TrimSpace(reviewNote)
	if len(reviewNote) > 500 {
		reviewNote = reviewNote[:500]
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin review chat report transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	const findSQL = `
		SELECT detail
		FROM risk_events
		WHERE id = $1
		  AND event_type = 'chat_report'
		FOR UPDATE
	`

	var detailRaw []byte
	if err := tx.QueryRow(ctx, findSQL, reportID).Scan(&detailRaw); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return &ChatReportNotFoundError{ReportID: reportID}
		}
		return fmt.Errorf("query chat report for review: %w", err)
	}

	detail := map[string]any{}
	if len(detailRaw) > 0 {
		_ = json.Unmarshal(detailRaw, &detail)
	}
	if detail == nil {
		detail = map[string]any{}
	}

	reviewedAt := time.Now().UTC()
	reviewerLinuxDoUserID := ""
	const reviewerSQL = `SELECT COALESCE(linux_do_user_id, '') FROM users WHERE id = $1`
	if err := tx.QueryRow(ctx, reviewerSQL, operatorUserID).Scan(&reviewerLinuxDoUserID); err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return fmt.Errorf("query chat report reviewer: %w", err)
	}

	previousReviewStatus := normalizeChatReportState(chatReportDetailString(detail, "reviewStatus"))
	penaltyAlreadyApplied := chatReportDetailBool(detail, "penaltyApplied")
	shouldApplyPenalty := reviewStatus == "approved" && previousReviewStatus != "approved" && !penaltyAlreadyApplied

	autoMuteApplied := false
	autoMuteSkipReason := ""
	autoMuteStrike := 0
	autoMuteDurationMinutes := 0
	var autoMuteMutedUntil *time.Time

	if shouldApplyPenalty {
		targetUser, err := s.resolveChatReportTargetUserTx(ctx, tx, detail)
		if err != nil {
			return err
		}
		if targetUser == nil {
			autoMuteSkipReason = "target_user_not_found"
		} else {
			detail["targetUserId"] = targetUser.ID.String()
			if targetUser.LinuxDoUserID != "" {
				detail["targetLinuxDoUserId"] = targetUser.LinuxDoUserID
			}
			if targetUser.LinuxDoUsername != "" {
				detail["targetName"] = targetUser.LinuxDoUsername
			}

			priorApprovedCount, err := s.countApprovedReportOffensesTx(ctx, tx, targetUser.ID, reportID)
			if err != nil {
				return err
			}
			autoMuteStrike = priorApprovedCount + 1
			autoMuteDurationMinutes = chatReportMuteDurationMinutesByStrike(autoMuteStrike)
			mutedUntil := reviewedAt.Add(time.Duration(autoMuteDurationMinutes) * time.Minute)
			muteReason := fmt.Sprintf("举报审核通过（第%d次）", autoMuteStrike)

			const insertMuteSQL = `
				INSERT INTO chat_mutes (user_id, reason, muted_until, created_by_user_id, created_at)
				VALUES ($1, $2, $3, $4, now())
			`
			if _, err := tx.Exec(ctx, insertMuteSQL, targetUser.ID, muteReason, mutedUntil, operatorUserID); err != nil {
				return fmt.Errorf("insert chat mute from report review: %w", err)
			}

			autoMuteApplied = true
			autoMuteMutedUntil = &mutedUntil
			detail["penaltyApplied"] = true
			detail["penaltyStrike"] = autoMuteStrike
			detail["penaltyDurationMinutes"] = autoMuteDurationMinutes
			detail["penaltyMutedUntil"] = mutedUntil.Format(time.RFC3339)
			detail["penaltyReason"] = muteReason
			detail["penaltyAppliedAt"] = reviewedAt.Format(time.RFC3339)
			delete(detail, "penaltySkipReason")
		}
	}

	detail["reviewStatus"] = reviewStatus
	detail["reviewNote"] = reviewNote
	detail["reviewedByUserId"] = operatorUserID.String()
	detail["reviewedByLinuxDoUserId"] = strings.TrimSpace(reviewerLinuxDoUserID)
	detail["reviewedAt"] = reviewedAt.Format(time.RFC3339)
	if shouldApplyPenalty && !autoMuteApplied {
		detail["penaltyApplied"] = false
		detail["penaltySkipReason"] = autoMuteSkipReason
	}

	detailJSON, err := json.Marshal(detail)
	if err != nil {
		return fmt.Errorf("marshal reviewed chat report detail: %w", err)
	}

	const updateSQL = `UPDATE risk_events SET detail = $2::jsonb WHERE id = $1`
	if _, err := tx.Exec(ctx, updateSQL, reportID, string(detailJSON)); err != nil {
		return fmt.Errorf("update reviewed chat report: %w", err)
	}

	auditDetail := map[string]any{
		"reportId":     reportID,
		"reviewStatus": reviewStatus,
		"reviewNote":   reviewNote,
	}
	if shouldApplyPenalty {
		auditDetail["autoMuteApplied"] = autoMuteApplied
		auditDetail["autoMuteStrike"] = autoMuteStrike
		auditDetail["autoMuteDurationMinutes"] = autoMuteDurationMinutes
		if autoMuteMutedUntil != nil {
			auditDetail["autoMuteMutedUntil"] = autoMuteMutedUntil.Format(time.RFC3339)
		}
		if autoMuteSkipReason != "" {
			auditDetail["autoMuteSkipReason"] = autoMuteSkipReason
		}
	}
	if err := s.insertChatAdminRiskEventTx(ctx, tx, operatorUserID, "review_report", auditDetail); err != nil {
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit review chat report transaction: %w", err)
	}
	return nil
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

func (s *ChatService) findUserByIDTx(ctx context.Context, tx pgx.Tx, userID uuid.UUID) (*chatUserRef, error) {
	const query = `
		SELECT id, COALESCE(linux_do_user_id, ''), COALESCE(linux_do_username, '')
		FROM users
		WHERE id = $1
	`

	record := &chatUserRef{}
	if err := tx.QueryRow(ctx, query, userID).Scan(&record.ID, &record.LinuxDoUserID, &record.LinuxDoUsername); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("query user by id in tx: %w", err)
	}
	return record, nil
}

func (s *ChatService) resolveChatReportTargetUserTx(ctx context.Context, tx pgx.Tx, detail map[string]any) (*chatUserRef, error) {
	targetUserIDRaw := strings.TrimSpace(chatReportDetailString(detail, "targetUserId"))
	if targetUserIDRaw != "" {
		targetUserID, err := uuid.Parse(targetUserIDRaw)
		if err == nil {
			targetUser, err := s.findUserByIDTx(ctx, tx, targetUserID)
			if err != nil {
				return nil, err
			}
			if targetUser != nil {
				if targetUser.LinuxDoUsername == "" {
					targetUser.LinuxDoUsername = strings.TrimSpace(chatReportDetailString(detail, "targetName"))
				}
				if targetUser.LinuxDoUserID == "" {
					targetUser.LinuxDoUserID = strings.TrimSpace(chatReportDetailString(detail, "targetLinuxDoUserId"))
				}
				return targetUser, nil
			}
		}
	}

	messageID, ok := chatReportDetailInt64(detail, "messageId")
	if !ok || messageID <= 0 {
		return nil, nil
	}

	const query = `
		SELECT
			COALESCE(cm.sender_user_id::text, ''),
			COALESCE(u.linux_do_user_id, ''),
			COALESCE(u.linux_do_username, ''),
			COALESCE(cm.sender_name, '')
		FROM chat_messages cm
		LEFT JOIN users u ON u.id = cm.sender_user_id
		WHERE cm.id = $1
	`

	var (
		senderUserIDRaw string
		linuxDoUserID   string
		linuxDoUsername string
		senderName      string
	)
	if err := tx.QueryRow(ctx, query, messageID).Scan(
		&senderUserIDRaw,
		&linuxDoUserID,
		&linuxDoUsername,
		&senderName,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("query chat report target from message: %w", err)
	}
	senderUserIDRaw = strings.TrimSpace(senderUserIDRaw)
	if senderUserIDRaw == "" {
		return nil, nil
	}
	senderUserID, err := uuid.Parse(senderUserIDRaw)
	if err != nil {
		return nil, nil
	}
	targetName := strings.TrimSpace(linuxDoUsername)
	if targetName == "" {
		targetName = strings.TrimSpace(senderName)
	}
	return &chatUserRef{
		ID:              senderUserID,
		LinuxDoUserID:   strings.TrimSpace(linuxDoUserID),
		LinuxDoUsername: targetName,
	}, nil
}

func (s *ChatService) countApprovedReportOffensesTx(ctx context.Context, tx pgx.Tx, targetUserID uuid.UUID, excludeReportID int64) (int, error) {
	const query = `
		SELECT COUNT(*)
		FROM risk_events re
		LEFT JOIN LATERAL (
			SELECT CASE
				WHEN COALESCE(re.detail->>'messageId', '') ~ '^[0-9]+$' THEN (re.detail->>'messageId')::bigint
				ELSE NULL
			END AS message_id
		) msg ON true
		LEFT JOIN chat_messages cm ON cm.id = msg.message_id
		WHERE re.event_type = 'chat_report'
		  AND re.id <> $2
		  AND COALESCE(re.detail->>'reviewStatus', 'pending') = 'approved'
		  AND (
			COALESCE(re.detail->>'targetUserId', '') = $1::text
			OR (
				COALESCE(re.detail->>'targetUserId', '') = ''
				AND cm.sender_user_id = $1
			)
		  )
	`

	count := 0
	if err := tx.QueryRow(ctx, query, targetUserID, excludeReportID).Scan(&count); err != nil {
		return 0, fmt.Errorf("count approved chat report offenses: %w", err)
	}
	return count, nil
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

func chatReportMuteDurationMinutesByStrike(strike int) int {
	switch {
	case strike <= 1:
		return 5
	case strike == 2:
		return 30
	case strike == 3:
		return 60
	case strike == 4:
		return 3 * 60
	case strike == 5:
		return 12 * 60
	default:
		return 24 * 60
	}
}

func chatReportDetailString(detail map[string]any, key string) string {
	if detail == nil {
		return ""
	}
	value, ok := detail[key]
	if !ok || value == nil {
		return ""
	}
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	case fmt.Stringer:
		return strings.TrimSpace(typed.String())
	default:
		return strings.TrimSpace(fmt.Sprint(value))
	}
}

func chatReportDetailInt64(detail map[string]any, key string) (int64, bool) {
	if detail == nil {
		return 0, false
	}
	value, ok := detail[key]
	if !ok || value == nil {
		return 0, false
	}
	switch typed := value.(type) {
	case int:
		return int64(typed), true
	case int8:
		return int64(typed), true
	case int16:
		return int64(typed), true
	case int32:
		return int64(typed), true
	case int64:
		return typed, true
	case uint:
		return int64(typed), true
	case uint8:
		return int64(typed), true
	case uint16:
		return int64(typed), true
	case uint32:
		return int64(typed), true
	case uint64:
		return int64(typed), true
	case float32:
		return int64(typed), true
	case float64:
		return int64(typed), true
	case json.Number:
		parsed, err := typed.Int64()
		if err != nil {
			return 0, false
		}
		return parsed, true
	case string:
		parsed, err := strconv.ParseInt(strings.TrimSpace(typed), 10, 64)
		if err != nil {
			return 0, false
		}
		return parsed, true
	default:
		return 0, false
	}
}

func chatReportDetailBool(detail map[string]any, key string) bool {
	if detail == nil {
		return false
	}
	value, ok := detail[key]
	if !ok || value == nil {
		return false
	}
	switch typed := value.(type) {
	case bool:
		return typed
	case string:
		normalized := strings.ToLower(strings.TrimSpace(typed))
		return normalized == "true" || normalized == "1" || normalized == "yes"
	case float64:
		return typed != 0
	case int:
		return typed != 0
	default:
		return false
	}
}

func normalizeChatReportFilterStatus(value string) string {
	switch normalizeChatReportState(value) {
	case "approved":
		return "approved"
	case "rejected":
		return "rejected"
	case "all":
		return "all"
	default:
		return "pending"
	}
}

func normalizeChatReportReviewStatus(value string) string {
	switch normalizeChatReportState(value) {
	case "approved":
		return "approved"
	case "rejected":
		return "rejected"
	default:
		return ""
	}
}

func normalizeChatReportState(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "approved":
		return "approved"
	case "rejected":
		return "rejected"
	case "all":
		return "all"
	default:
		return "pending"
	}
}

func (s *ChatService) maybeRefreshBlockedWords(ctx context.Context) {
	s.syncWordTTLFromRuntimeConfig(ctx)

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

func (s *ChatService) checkRateLimit(ctx context.Context, userID uuid.UUID) error {
	s.syncMinGapFromRuntimeConfig(ctx)

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

func (s *ChatService) chatMaxContentRune(ctx context.Context) int {
	if s.runtimeConfig == nil {
		return chatMaxContentRune
	}
	return s.runtimeConfig.GetInt(ctx, RuntimeConfigKeyChatMessageMaxRunes, chatMaxContentRune, 20, 2000)
}

func (s *ChatService) chatAdminMaxMuteMinutes(ctx context.Context) int {
	if s.runtimeConfig == nil {
		return 7 * 24 * 60
	}
	return s.runtimeConfig.GetInt(ctx, RuntimeConfigKeyChatAdminMaxMuteMinutes, 7*24*60, 1, 365*24*60)
}

func (s *ChatService) syncMinGapFromRuntimeConfig(ctx context.Context) {
	if s.runtimeConfig == nil {
		return
	}
	minGapMS := s.runtimeConfig.GetInt(ctx, RuntimeConfigKeyChatSendMinGapMS, 3000, 200, 60000)
	minGap := time.Duration(minGapMS) * time.Millisecond
	s.mu.Lock()
	s.minGap = minGap
	s.mu.Unlock()
}

func (s *ChatService) syncWordTTLFromRuntimeConfig(ctx context.Context) {
	if s.runtimeConfig == nil {
		return
	}
	ttlSeconds := s.runtimeConfig.GetInt(ctx, RuntimeConfigKeyChatWordCacheTTLSec, int(chatWordBlockTTL/time.Second), 5, 3600)
	wordTTL := time.Duration(ttlSeconds) * time.Second
	s.wordMu.Lock()
	s.wordTTL = wordTTL
	s.wordMu.Unlock()
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
