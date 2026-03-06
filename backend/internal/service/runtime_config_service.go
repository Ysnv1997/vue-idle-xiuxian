package service

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	RuntimeConfigKeyHuntingWinGainMultiplier    = "gameplay.hunting.win_gain_multiplier"
	RuntimeConfigKeyHuntingOfflineCapSeconds    = "gameplay.hunting.offline_cap_seconds"
	RuntimeConfigKeyHuntingReviveMultiplier     = "gameplay.hunting.revive_multiplier"
	RuntimeConfigKeyHuntingAutoHealBaseRate     = "gameplay.hunting.auto_heal.base_rate"
	RuntimeConfigKeyHuntingAutoHealCapRate      = "gameplay.hunting.auto_heal.cap_rate"
	RuntimeConfigKeyHuntingSpiritRefundChance   = "gameplay.hunting.spirit_refund.chance"
	RuntimeConfigKeyHuntingSpiritRefundMinRatio = "gameplay.hunting.spirit_refund.min_ratio"
	RuntimeConfigKeyHuntingSpiritRefundMaxRatio = "gameplay.hunting.spirit_refund.max_ratio"
	RuntimeConfigKeyChatMessageMaxRunes         = "chat.message.max_runes"
	RuntimeConfigKeyChatSendMinGapMS            = "chat.send.min_gap_ms"
	RuntimeConfigKeyChatWordCacheTTLSec         = "chat.block_word_cache_ttl_seconds"
	RuntimeConfigKeyChatAdminMaxMuteMinutes     = "chat.admin.max_mute_minutes"
	RuntimeConfigKeyChatRetentionSeconds        = "chat.retention.seconds"
	RuntimeConfigKeyChatRetentionMaxMessages    = "chat.retention.max_messages"
)

type RuntimeConfigValueType string

const (
	RuntimeConfigTypeString RuntimeConfigValueType = "string"
	RuntimeConfigTypeInt    RuntimeConfigValueType = "int"
	RuntimeConfigTypeFloat  RuntimeConfigValueType = "float"
	RuntimeConfigTypeBool   RuntimeConfigValueType = "bool"
)

type RuntimeConfigDefinition struct {
	Key         string
	Default     string
	ValueType   RuntimeConfigValueType
	Category    string
	Description string
}

var defaultRuntimeConfigDefinitions = []RuntimeConfigDefinition{
	{
		Key:         RuntimeConfigKeyHuntingWinGainMultiplier,
		Default:     "2",
		ValueType:   RuntimeConfigTypeFloat,
		Category:    "gameplay",
		Description: "刷图胜利修为倍率（相对打坐基准）",
	},
	{
		Key:         RuntimeConfigKeyHuntingOfflineCapSeconds,
		Default:     "43200",
		ValueType:   RuntimeConfigTypeInt,
		Category:    "gameplay",
		Description: "离线刷图最大结算时长（秒）",
	},
	{
		Key:         RuntimeConfigKeyHuntingReviveMultiplier,
		Default:     "1",
		ValueType:   RuntimeConfigTypeFloat,
		Category:    "gameplay",
		Description: "刷图死亡复活时间倍率",
	},
	{
		Key:         RuntimeConfigKeyHuntingAutoHealBaseRate,
		Default:     "0.10",
		ValueType:   RuntimeConfigTypeFloat,
		Category:    "gameplay",
		Description: "刷图每次胜利基础回血比例",
	},
	{
		Key:         RuntimeConfigKeyHuntingAutoHealCapRate,
		Default:     "0.45",
		ValueType:   RuntimeConfigTypeFloat,
		Category:    "gameplay",
		Description: "刷图每次胜利回血比例上限",
	},
	{
		Key:         RuntimeConfigKeyHuntingSpiritRefundChance,
		Default:     "0.2",
		ValueType:   RuntimeConfigTypeFloat,
		Category:    "gameplay",
		Description: "刷图击杀返还灵力触发概率（0~1）",
	},
	{
		Key:         RuntimeConfigKeyHuntingSpiritRefundMinRatio,
		Default:     "0.1",
		ValueType:   RuntimeConfigTypeFloat,
		Category:    "gameplay",
		Description: "刷图击杀返还灵力最小比例（0~1）",
	},
	{
		Key:         RuntimeConfigKeyHuntingSpiritRefundMaxRatio,
		Default:     "0.35",
		ValueType:   RuntimeConfigTypeFloat,
		Category:    "gameplay",
		Description: "刷图击杀返还灵力最大比例（0~1）",
	},
	{
		Key:         RuntimeConfigKeyChatMessageMaxRunes,
		Default:     "200",
		ValueType:   RuntimeConfigTypeInt,
		Category:    "chat",
		Description: "聊天单条消息最大字符数",
	},
	{
		Key:         RuntimeConfigKeyChatSendMinGapMS,
		Default:     "3000",
		ValueType:   RuntimeConfigTypeInt,
		Category:    "chat",
		Description: "聊天发送最小间隔（毫秒）",
	},
	{
		Key:         RuntimeConfigKeyChatWordCacheTTLSec,
		Default:     "60",
		ValueType:   RuntimeConfigTypeInt,
		Category:    "chat",
		Description: "违禁词缓存刷新间隔（秒）",
	},
	{
		Key:         RuntimeConfigKeyChatAdminMaxMuteMinutes,
		Default:     "10080",
		ValueType:   RuntimeConfigTypeInt,
		Category:    "chat",
		Description: "聊天管理员最大禁言时长（分钟）",
	},
	{
		Key:         RuntimeConfigKeyChatRetentionSeconds,
		Default:     "600",
		ValueType:   RuntimeConfigTypeInt,
		Category:    "chat",
		Description: "聊天保留时长（秒）",
	},
	{
		Key:         RuntimeConfigKeyChatRetentionMaxMessages,
		Default:     "500",
		ValueType:   RuntimeConfigTypeInt,
		Category:    "chat",
		Description: "聊天最多保留消息数",
	},
}

type RuntimeConfig struct {
	Key             string     `json:"key"`
	Value           string     `json:"value"`
	ValueType       string     `json:"valueType"`
	Category        string     `json:"category"`
	Description     string     `json:"description"`
	UpdatedByUserID *uuid.UUID `json:"updatedByUserId,omitempty"`
	CreatedAt       time.Time  `json:"createdAt"`
	UpdatedAt       time.Time  `json:"updatedAt"`
}

type RuntimeConfigAuditLog struct {
	ID                  int64      `json:"id"`
	Key                 string     `json:"key"`
	Action              string     `json:"action"`
	OldValue            string     `json:"oldValue"`
	OldValueType        string     `json:"oldValueType"`
	OldCategory         string     `json:"oldCategory"`
	OldDescription      string     `json:"oldDescription"`
	NewValue            string     `json:"newValue"`
	NewValueType        string     `json:"newValueType"`
	NewCategory         string     `json:"newCategory"`
	NewDescription      string     `json:"newDescription"`
	OperatorUserID      *uuid.UUID `json:"operatorUserId,omitempty"`
	OperatorLinuxDoUser string     `json:"operatorLinuxDoUserId,omitempty"`
	OperatorUsername    string     `json:"operatorUsername,omitempty"`
	CreatedAt           time.Time  `json:"createdAt"`
}

type RuntimeConfigAuditLogListResult struct {
	Logs []RuntimeConfigAuditLog `json:"logs"`
}

type RuntimeConfigListResult struct {
	Configs []RuntimeConfig `json:"configs"`
}

type RuntimeConfigUpsertInput struct {
	Key             string
	Value           string
	ValueType       string
	Category        string
	Description     string
	UpdatedByUserID *uuid.UUID
}

type InvalidRuntimeConfigKeyError struct{}

func (e *InvalidRuntimeConfigKeyError) Error() string {
	return "invalid runtime config key"
}

type InvalidRuntimeConfigTypeError struct {
	ValueType string
}

func (e *InvalidRuntimeConfigTypeError) Error() string {
	return fmt.Sprintf("invalid runtime config type: %s", e.ValueType)
}

type InvalidRuntimeConfigValueError struct {
	Reason string
}

func (e *InvalidRuntimeConfigValueError) Error() string {
	return fmt.Sprintf("invalid runtime config value: %s", e.Reason)
}

type runtimeConfigCacheState struct {
	values    map[string]string
	loadedAt  time.Time
	expiresAt time.Time
}

type runtimeConfigRecordSnapshot struct {
	Key         string
	Value       string
	ValueType   string
	Category    string
	Description string
}

type RuntimeConfigService struct {
	pool     *pgxpool.Pool
	cacheTTL time.Duration

	cacheMu sync.RWMutex
	cache   runtimeConfigCacheState
}

func NewRuntimeConfigService(pool *pgxpool.Pool) *RuntimeConfigService {
	return &RuntimeConfigService{
		pool:     pool,
		cacheTTL: 5 * time.Second,
		cache: runtimeConfigCacheState{
			values: map[string]string{},
		},
	}
}

func (s *RuntimeConfigService) EnsureDefaults(ctx context.Context) error {
	const query = `
		INSERT INTO game_runtime_configs (
			key,
			value,
			value_type,
			category,
			description,
			updated_by_user_id,
			created_at,
			updated_at
		)
		VALUES ($1, $2, $3, $4, $5, NULL, now(), now())
		ON CONFLICT (key)
		DO UPDATE SET
			value_type = EXCLUDED.value_type,
			category = EXCLUDED.category,
			description = EXCLUDED.description
		WHERE
			game_runtime_configs.value_type IS DISTINCT FROM EXCLUDED.value_type
			OR game_runtime_configs.category IS DISTINCT FROM EXCLUDED.category
			OR game_runtime_configs.description IS DISTINCT FROM EXCLUDED.description
	`
	for _, def := range defaultRuntimeConfigDefinitions {
		if _, err := s.pool.Exec(
			ctx,
			query,
			def.Key,
			def.Default,
			string(def.ValueType),
			def.Category,
			def.Description,
		); err != nil {
			return fmt.Errorf("ensure runtime config default %s: %w", def.Key, err)
		}
	}
	s.InvalidateCache()
	return nil
}

func (s *RuntimeConfigService) List(ctx context.Context, category string, keyword string, limit int) (*RuntimeConfigListResult, error) {
	category = strings.TrimSpace(strings.ToLower(category))
	keyword = strings.TrimSpace(strings.ToLower(keyword))
	if limit <= 0 {
		limit = 300
	}
	if limit > 2000 {
		limit = 2000
	}

	const query = `
		SELECT
			key,
			value,
			value_type,
			category,
			COALESCE(description, ''),
			updated_by_user_id,
			created_at,
			updated_at
		FROM game_runtime_configs
		WHERE
			($1 = '' OR category = $1)
			AND (
				$2 = ''
				OR LOWER(key) LIKE '%' || $2 || '%'
				OR LOWER(COALESCE(description, '')) LIKE '%' || $2 || '%'
			)
		ORDER BY category ASC, key ASC
		LIMIT $3
	`
	rows, err := s.pool.Query(ctx, query, category, keyword, limit)
	if err != nil {
		return nil, fmt.Errorf("query runtime configs: %w", err)
	}
	defer rows.Close()

	configs := make([]RuntimeConfig, 0, limit)
	for rows.Next() {
		record := RuntimeConfig{}
		if err := rows.Scan(
			&record.Key,
			&record.Value,
			&record.ValueType,
			&record.Category,
			&record.Description,
			&record.UpdatedByUserID,
			&record.CreatedAt,
			&record.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan runtime config row: %w", err)
		}
		configs = append(configs, record)
	}
	if rows.Err() != nil {
		return nil, fmt.Errorf("iterate runtime config rows: %w", rows.Err())
	}
	return &RuntimeConfigListResult{Configs: configs}, nil
}

func (s *RuntimeConfigService) ListAudits(
	ctx context.Context,
	key string,
	category string,
	limit int,
) (*RuntimeConfigAuditLogListResult, error) {
	key = normalizeRuntimeConfigKey(key)
	category = normalizeRuntimeConfigCategory(category)
	if limit <= 0 {
		limit = 200
	}
	if limit > 2000 {
		limit = 2000
	}

	const query = `
		SELECT
			l.id,
			l.key,
			l.action,
			COALESCE(l.old_value, ''),
			COALESCE(l.old_value_type, ''),
			COALESCE(l.old_category, ''),
			COALESCE(l.old_description, ''),
			l.new_value,
			l.new_value_type,
			l.new_category,
			COALESCE(l.new_description, ''),
			l.operator_user_id,
			COALESCE(u.linux_do_user_id, ''),
			COALESCE(u.linux_do_username, ''),
			l.created_at
		FROM game_runtime_config_audit_logs l
		LEFT JOIN users u ON u.id = l.operator_user_id
		WHERE
			($1 = '' OR l.key = $1)
			AND ($2 = '' OR l.new_category = $2)
		ORDER BY l.id DESC
		LIMIT $3
	`
	rows, err := s.pool.Query(ctx, query, key, category, limit)
	if err != nil {
		return nil, fmt.Errorf("query runtime config audit logs: %w", err)
	}
	defer rows.Close()

	logs := make([]RuntimeConfigAuditLog, 0, limit)
	for rows.Next() {
		logItem := RuntimeConfigAuditLog{}
		if err := rows.Scan(
			&logItem.ID,
			&logItem.Key,
			&logItem.Action,
			&logItem.OldValue,
			&logItem.OldValueType,
			&logItem.OldCategory,
			&logItem.OldDescription,
			&logItem.NewValue,
			&logItem.NewValueType,
			&logItem.NewCategory,
			&logItem.NewDescription,
			&logItem.OperatorUserID,
			&logItem.OperatorLinuxDoUser,
			&logItem.OperatorUsername,
			&logItem.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan runtime config audit log row: %w", err)
		}
		logs = append(logs, logItem)
	}
	if rows.Err() != nil {
		return nil, fmt.Errorf("iterate runtime config audit log rows: %w", rows.Err())
	}

	return &RuntimeConfigAuditLogListResult{Logs: logs}, nil
}

func (s *RuntimeConfigService) Upsert(ctx context.Context, input RuntimeConfigUpsertInput) (*RuntimeConfig, error) {
	key := normalizeRuntimeConfigKey(input.Key)
	if key == "" {
		return nil, &InvalidRuntimeConfigKeyError{}
	}

	valueType := normalizeRuntimeConfigType(input.ValueType)
	if valueType == "" {
		valueType = string(RuntimeConfigTypeString)
	}
	if !isRuntimeConfigTypeAllowed(valueType) {
		return nil, &InvalidRuntimeConfigTypeError{ValueType: valueType}
	}

	category := normalizeRuntimeConfigCategory(input.Category)
	if category == "" {
		category = "general"
	}
	value := strings.TrimSpace(input.Value)
	if err := validateRuntimeConfigValue(valueType, value); err != nil {
		return nil, err
	}

	description := strings.TrimSpace(input.Description)
	if len(description) > 200 {
		description = description[:200]
	}

	const query = `
		INSERT INTO game_runtime_configs (
			key,
			value,
			value_type,
			category,
			description,
			updated_by_user_id,
			created_at,
			updated_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, now(), now())
		ON CONFLICT (key)
		DO UPDATE SET
			value = EXCLUDED.value,
			value_type = EXCLUDED.value_type,
			category = EXCLUDED.category,
			description = EXCLUDED.description,
			updated_by_user_id = EXCLUDED.updated_by_user_id,
			updated_at = now()
		RETURNING key, value, value_type, category, COALESCE(description, ''), updated_by_user_id, created_at, updated_at, (xmax = 0)
	`

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin runtime config upsert transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	oldSnapshot, existed, err := s.loadRuntimeConfigSnapshotForUpdate(ctx, tx, key)
	if err != nil {
		return nil, err
	}

	record := &RuntimeConfig{}
	inserted := false
	if err := tx.QueryRow(
		ctx,
		query,
		key,
		value,
		valueType,
		category,
		description,
		input.UpdatedByUserID,
	).Scan(
		&record.Key,
		&record.Value,
		&record.ValueType,
		&record.Category,
		&record.Description,
		&record.UpdatedByUserID,
		&record.CreatedAt,
		&record.UpdatedAt,
		&inserted,
	); err != nil {
		return nil, fmt.Errorf("upsert runtime config: %w", err)
	}

	changed := !existed ||
		oldSnapshot.Value != record.Value ||
		oldSnapshot.ValueType != record.ValueType ||
		oldSnapshot.Category != record.Category ||
		oldSnapshot.Description != record.Description
	if changed {
		action := "update"
		var oldSnapshotPtr *runtimeConfigRecordSnapshot
		if inserted {
			action = "create"
		} else {
			if existed {
				oldSnapshotCopy := oldSnapshot
				oldSnapshotPtr = &oldSnapshotCopy
			}
		}
		if err := s.insertRuntimeConfigAuditTx(
			ctx,
			tx,
			action,
			oldSnapshotPtr,
			record,
			input.UpdatedByUserID,
		); err != nil {
			return nil, err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit runtime config upsert transaction: %w", err)
	}

	s.InvalidateCache()
	return record, nil
}

func (s *RuntimeConfigService) loadRuntimeConfigSnapshotForUpdate(
	ctx context.Context,
	tx pgx.Tx,
	key string,
) (runtimeConfigRecordSnapshot, bool, error) {
	const query = `
		SELECT
			key,
			value,
			value_type,
			category,
			COALESCE(description, '')
		FROM game_runtime_configs
		WHERE key = $1
		FOR UPDATE
	`

	snapshot := runtimeConfigRecordSnapshot{}
	if err := tx.QueryRow(ctx, query, key).Scan(
		&snapshot.Key,
		&snapshot.Value,
		&snapshot.ValueType,
		&snapshot.Category,
		&snapshot.Description,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return runtimeConfigRecordSnapshot{}, false, nil
		}
		return runtimeConfigRecordSnapshot{}, false, fmt.Errorf("load runtime config snapshot: %w", err)
	}
	return snapshot, true, nil
}

func (s *RuntimeConfigService) insertRuntimeConfigAuditTx(
	ctx context.Context,
	tx pgx.Tx,
	action string,
	oldSnapshot *runtimeConfigRecordSnapshot,
	newConfig *RuntimeConfig,
	operatorUserID *uuid.UUID,
) error {
	var oldValue any
	var oldValueType any
	var oldCategory any
	var oldDescription any
	if oldSnapshot != nil {
		oldValue = oldSnapshot.Value
		oldValueType = oldSnapshot.ValueType
		oldCategory = oldSnapshot.Category
		oldDescription = oldSnapshot.Description
	}

	const query = `
		INSERT INTO game_runtime_config_audit_logs (
			key,
			action,
			old_value,
			old_value_type,
			old_category,
			old_description,
			new_value,
			new_value_type,
			new_category,
			new_description,
			operator_user_id,
			created_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, now())
	`
	if _, err := tx.Exec(
		ctx,
		query,
		newConfig.Key,
		action,
		oldValue,
		oldValueType,
		oldCategory,
		oldDescription,
		newConfig.Value,
		newConfig.ValueType,
		newConfig.Category,
		newConfig.Description,
		operatorUserID,
	); err != nil {
		return fmt.Errorf("insert runtime config audit log: %w", err)
	}
	return nil
}

func (s *RuntimeConfigService) InvalidateCache() {
	s.cacheMu.Lock()
	defer s.cacheMu.Unlock()
	s.cache.values = map[string]string{}
	s.cache.loadedAt = time.Time{}
	s.cache.expiresAt = time.Time{}
}

func (s *RuntimeConfigService) GetInt(ctx context.Context, key string, fallback int, min int, max int) int {
	value, ok, err := s.getRawValue(ctx, key)
	if err != nil || !ok {
		return clampInt(fallback, min, max)
	}
	parsed, parseErr := strconv.Atoi(strings.TrimSpace(value))
	if parseErr != nil {
		return clampInt(fallback, min, max)
	}
	return clampInt(parsed, min, max)
}

func (s *RuntimeConfigService) GetFloat64(ctx context.Context, key string, fallback float64, min float64, max float64) float64 {
	value, ok, err := s.getRawValue(ctx, key)
	if err != nil || !ok {
		return clampFloat64(fallback, min, max)
	}
	parsed, parseErr := strconv.ParseFloat(strings.TrimSpace(value), 64)
	if parseErr != nil {
		return clampFloat64(fallback, min, max)
	}
	return clampFloat64(parsed, min, max)
}

func (s *RuntimeConfigService) getRawValue(ctx context.Context, key string) (string, bool, error) {
	key = normalizeRuntimeConfigKey(key)
	if key == "" {
		return "", false, nil
	}

	if value, ok := s.getFromCache(key); ok {
		return value, true, nil
	}
	if err := s.refreshCache(ctx); err != nil {
		return "", false, err
	}
	value, ok := s.getFromCache(key)
	return value, ok, nil
}

func (s *RuntimeConfigService) getFromCache(key string) (string, bool) {
	s.cacheMu.RLock()
	defer s.cacheMu.RUnlock()
	if s.cache.expiresAt.IsZero() || time.Now().After(s.cache.expiresAt) {
		return "", false
	}
	value, ok := s.cache.values[key]
	return value, ok
}

func (s *RuntimeConfigService) refreshCache(ctx context.Context) error {
	const query = `
		SELECT key, value
		FROM game_runtime_configs
	`
	rows, err := s.pool.Query(ctx, query)
	if err != nil {
		return fmt.Errorf("query runtime config cache rows: %w", err)
	}
	defer rows.Close()

	values := make(map[string]string, 64)
	for rows.Next() {
		var key string
		var value string
		if err := rows.Scan(&key, &value); err != nil {
			return fmt.Errorf("scan runtime config cache row: %w", err)
		}
		key = normalizeRuntimeConfigKey(key)
		if key == "" {
			continue
		}
		values[key] = strings.TrimSpace(value)
	}
	if rows.Err() != nil {
		return fmt.Errorf("iterate runtime config cache rows: %w", rows.Err())
	}

	now := time.Now()
	s.cacheMu.Lock()
	s.cache.values = values
	s.cache.loadedAt = now
	s.cache.expiresAt = now.Add(s.cacheTTL)
	s.cacheMu.Unlock()
	return nil
}

func normalizeRuntimeConfigKey(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	if len(value) > 128 {
		value = value[:128]
	}
	return value
}

func normalizeRuntimeConfigType(value string) string {
	return strings.TrimSpace(strings.ToLower(value))
}

func normalizeRuntimeConfigCategory(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	if len(value) > 64 {
		value = value[:64]
	}
	return value
}

func isRuntimeConfigTypeAllowed(valueType string) bool {
	switch RuntimeConfigValueType(valueType) {
	case RuntimeConfigTypeString, RuntimeConfigTypeInt, RuntimeConfigTypeFloat, RuntimeConfigTypeBool:
		return true
	default:
		return false
	}
}

func validateRuntimeConfigValue(valueType string, value string) error {
	switch RuntimeConfigValueType(valueType) {
	case RuntimeConfigTypeString:
		if len(value) > 2000 {
			return &InvalidRuntimeConfigValueError{Reason: "string too long"}
		}
	case RuntimeConfigTypeInt:
		if _, err := strconv.Atoi(value); err != nil {
			return &InvalidRuntimeConfigValueError{Reason: "int parse failed"}
		}
	case RuntimeConfigTypeFloat:
		if _, err := strconv.ParseFloat(value, 64); err != nil {
			return &InvalidRuntimeConfigValueError{Reason: "float parse failed"}
		}
	case RuntimeConfigTypeBool:
		normalized := strings.ToLower(strings.TrimSpace(value))
		if normalized != "true" && normalized != "false" && normalized != "1" && normalized != "0" {
			return &InvalidRuntimeConfigValueError{Reason: "bool parse failed"}
		}
	default:
		return &InvalidRuntimeConfigTypeError{ValueType: valueType}
	}
	return nil
}

func clampInt(value int, min int, max int) int {
	if min <= max {
		if value < min {
			value = min
		}
		if value > max {
			value = max
		}
	}
	return value
}

func clampFloat64(value float64, min float64, max float64) float64 {
	if min <= max {
		if value < min {
			value = min
		}
		if value > max {
			value = max
		}
	}
	return value
}
