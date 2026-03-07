package service

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/kowming/vue-idle-xiuxian/backend/internal/repository"
)

const (
	meditationRunStateRunning = "running"
	meditationRunStateStopped = "stopped"
	meditationRunStateFull    = "full"
	meditationRunStateOffline = "offline_timeout"
	meditationRunStateSystem  = "system_stopped"
)

type MeditationStatusResult struct {
	IsActive        bool    `json:"isActive"`
	State           string  `json:"state"`
	CurrentSpirit   float64 `json:"currentSpirit"`
	SpiritCap       float64 `json:"spiritCap"`
	CurrentRate     float64 `json:"currentRate"`
	TotalSpiritGain float64 `json:"totalSpiritGain"`
	StartedAt       int64   `json:"startedAt,omitempty"`
	LastLogSeq      int64   `json:"lastLogSeq"`
	LastLogMessage  string  `json:"lastLogMessage"`
}

type MeditationActionResult struct {
	Message  string                     `json:"message"`
	State    string                     `json:"state"`
	Run      *MeditationStatusResult    `json:"run"`
	Snapshot *repository.PlayerSnapshot `json:"snapshot,omitempty"`
}

type MeditationSpiritFullError struct{}

func (e *MeditationSpiritFullError) Error() string {
	return "spirit already full"
}

type MeditationConflictError struct {
	Conflict string
}

func (e *MeditationConflictError) Error() string {
	return fmt.Sprintf("meditation conflict: %s", e.Conflict)
}

type meditationRunState struct {
	RunActive        bool
	TotalSpiritGain  float64
	LastState        string
	LastLogSeq       int64
	LastLogMessage   string
	StartedAt        time.Time
	EndedAt          time.Time
	RunUpdatedAt     time.Time
	Level            int
	Spirit           float64
	SpiritRate       float64
	ActiveEffectsRaw []byte
}

func (s *GameService) MeditationStatus(ctx context.Context, userID uuid.UUID) (*MeditationStatusResult, error) {
	const query = `
		SELECT
			COALESCE(pmr.is_active, FALSE),
			COALESCE(pmr.total_spirit_gain, 0),
			COALESCE(pmr.last_state, ''),
			COALESCE(pmr.last_log_seq, 0),
			COALESCE(pmr.last_log_message, ''),
			COALESCE(pmr.started_at, to_timestamp(0)),
			COALESCE(pmr.ended_at, to_timestamp(0)),
			COALESCE(pmr.updated_at, now()),
			pp.level,
			COALESCE(pr.spirit, 0),
			COALESCE(pr.spirit_rate, 0),
			COALESCE(pis.active_effects, '[]'::jsonb)
		FROM player_profiles pp
		LEFT JOIN player_meditation_runs pmr ON pmr.user_id = pp.user_id
		LEFT JOIN player_resources pr ON pr.user_id = pp.user_id
		LEFT JOIN player_inventory_state pis ON pis.user_id = pp.user_id
		WHERE pp.user_id = $1
	`

	state := &meditationRunState{}
	if err := s.pool.QueryRow(ctx, query, userID).Scan(
		&state.RunActive,
		&state.TotalSpiritGain,
		&state.LastState,
		&state.LastLogSeq,
		&state.LastLogMessage,
		&state.StartedAt,
		&state.EndedAt,
		&state.RunUpdatedAt,
		&state.Level,
		&state.Spirit,
		&state.SpiritRate,
		&state.ActiveEffectsRaw,
	); err != nil {
		if err == pgx.ErrNoRows {
			return &MeditationStatusResult{
				IsActive: false,
				State:    meditationRunStateStopped,
			}, nil
		}
		return nil, fmt.Errorf("query meditation status: %w", err)
	}

	status := buildMeditationStatus(state, time.Now())
	if status.State == "" {
		status.State = meditationRunStateStopped
	}
	return status, nil
}

func (s *GameService) MeditationStart(ctx context.Context, userID uuid.UUID) (*MeditationActionResult, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin meditation start transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if err := ensureNoActiveDungeonRunTx(ctx, tx, userID); err != nil {
		return nil, err
	}

	if err := ensureHuntingRunRow(ctx, tx, userID); err != nil {
		return nil, err
	}
	huntingActive, err := loadHuntingRunActiveForUpdate(ctx, tx, userID)
	if err != nil {
		return nil, err
	}
	if huntingActive {
		return nil, &MeditationConflictError{Conflict: "hunting"}
	}
	if err := ensureExplorationRunRow(ctx, tx, userID); err != nil {
		return nil, err
	}
	if err := stopExplorationForConflictTx(ctx, tx, userID, "开始打坐，自动探索已结束"); err != nil {
		return nil, err
	}

	if err := ensureMeditationRunRow(ctx, tx, userID); err != nil {
		return nil, err
	}
	state, err := loadMeditationRunStateForUpdate(ctx, tx, userID)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	nowMilli := now.UnixMilli()
	_, bonus := decodeMeditationActiveEffects(state.ActiveEffectsRaw, nowMilli)
	spiritCap := resolveMeditationSpiritCap(state.Level, bonus)
	currentSpirit := math.Max(0, state.Spirit)
	if currentSpirit >= spiritCap {
		return nil, &MeditationSpiritFullError{}
	}

	if state.RunActive {
		status := buildMeditationStatus(state, now)
		if err := tx.Commit(ctx); err != nil {
			return nil, fmt.Errorf("commit meditation start transaction: %w", err)
		}
		s.notifyRealtime(userID, GameRealtimeTopicMeditation, GameRealtimeTopicSnapshot)
		return &MeditationActionResult{
			Message: "当前已在打坐中",
			State:   status.State,
			Run:     status,
		}, nil
	}

	state.RunActive = true
	state.TotalSpiritGain = 0
	state.LastState = meditationRunStateRunning
	state.StartedAt = now
	state.EndedAt = time.Time{}
	state.RunUpdatedAt = now
	setMeditationRunLog(state, "已开始打坐，灵力恢复中")

	if err := updateMeditationRunTx(ctx, tx, userID, state); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit meditation start transaction: %w", err)
	}
	s.notifyRealtime(userID, GameRealtimeTopicMeditation, GameRealtimeTopicSnapshot)

	snapshot, err := s.userRepo.GetSnapshot(ctx, userID)
	if err != nil {
		return nil, err
	}

	return &MeditationActionResult{
		Message:  "已开始打坐",
		State:    state.LastState,
		Run:      buildMeditationStatus(state, now),
		Snapshot: snapshot,
	}, nil
}

func (s *GameService) MeditationStop(ctx context.Context, userID uuid.UUID) (*MeditationActionResult, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin meditation stop transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if err := ensureMeditationRunRow(ctx, tx, userID); err != nil {
		return nil, err
	}
	state, err := loadMeditationRunStateForUpdate(ctx, tx, userID)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	message := "当前未在打坐"
	if state.RunActive {
		state.RunActive = false
		state.LastState = meditationRunStateStopped
		state.EndedAt = now
		state.RunUpdatedAt = now
		setMeditationRunLog(state, "已停止打坐")
		message = "已停止打坐"
		if err := updateMeditationRunTx(ctx, tx, userID, state); err != nil {
			return nil, err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit meditation stop transaction: %w", err)
	}
	s.notifyRealtime(userID, GameRealtimeTopicMeditation, GameRealtimeTopicSnapshot)

	snapshot, err := s.userRepo.GetSnapshot(ctx, userID)
	if err != nil {
		return nil, err
	}

	return &MeditationActionResult{
		Message:  message,
		State:    state.LastState,
		Run:      buildMeditationStatus(state, now),
		Snapshot: snapshot,
	}, nil
}

func ensureMeditationRunRow(ctx context.Context, tx pgx.Tx, userID uuid.UUID) error {
	const query = `
		INSERT INTO player_meditation_runs (user_id)
		VALUES ($1)
		ON CONFLICT (user_id) DO NOTHING
	`
	if _, err := tx.Exec(ctx, query, userID); err != nil {
		return fmt.Errorf("ensure meditation run row: %w", err)
	}
	return nil
}

func loadHuntingRunActiveForUpdate(ctx context.Context, tx pgx.Tx, userID uuid.UUID) (bool, error) {
	const query = `
		SELECT is_active
		FROM player_hunting_runs
		WHERE user_id = $1
		FOR UPDATE
	`
	var active bool
	if err := tx.QueryRow(ctx, query, userID).Scan(&active); err != nil {
		return false, fmt.Errorf("load hunting active state: %w", err)
	}
	return active, nil
}

func loadMeditationRunStateForUpdate(ctx context.Context, tx pgx.Tx, userID uuid.UUID) (*meditationRunState, error) {
	const query = `
		SELECT
			pmr.is_active,
			COALESCE(pmr.total_spirit_gain, 0),
			COALESCE(pmr.last_state, ''),
			COALESCE(pmr.last_log_seq, 0),
			COALESCE(pmr.last_log_message, ''),
			COALESCE(pmr.started_at, to_timestamp(0)),
			COALESCE(pmr.ended_at, to_timestamp(0)),
			pmr.updated_at,
			pp.level,
			pr.spirit,
			pr.spirit_rate,
			COALESCE(pis.active_effects, '[]'::jsonb)
		FROM player_meditation_runs pmr
		JOIN player_profiles pp ON pp.user_id = pmr.user_id
		JOIN player_resources pr ON pr.user_id = pmr.user_id
		JOIN player_inventory_state pis ON pis.user_id = pmr.user_id
		WHERE pmr.user_id = $1
		FOR UPDATE OF pmr, pp, pr, pis
	`

	state := &meditationRunState{}
	if err := tx.QueryRow(ctx, query, userID).Scan(
		&state.RunActive,
		&state.TotalSpiritGain,
		&state.LastState,
		&state.LastLogSeq,
		&state.LastLogMessage,
		&state.StartedAt,
		&state.EndedAt,
		&state.RunUpdatedAt,
		&state.Level,
		&state.Spirit,
		&state.SpiritRate,
		&state.ActiveEffectsRaw,
	); err != nil {
		return nil, fmt.Errorf("load meditation run state: %w", err)
	}
	return state, nil
}

func buildMeditationStatus(state *meditationRunState, now time.Time) *MeditationStatusResult {
	nowMilli := now.UnixMilli()
	_, bonus := decodeMeditationActiveEffects(state.ActiveEffectsRaw, nowMilli)
	currentRate := resolveMeditationSpiritRegen(state.Level, state.SpiritRate, bonus)
	spiritCap := resolveMeditationSpiritCap(state.Level, bonus)

	status := &MeditationStatusResult{
		IsActive:        state.RunActive,
		State:           state.LastState,
		CurrentSpirit:   math.Max(0, state.Spirit),
		SpiritCap:       spiritCap,
		CurrentRate:     currentRate,
		TotalSpiritGain: math.Max(0, state.TotalSpiritGain),
		LastLogSeq:      state.LastLogSeq,
		LastLogMessage:  state.LastLogMessage,
	}
	if status.State == "" {
		status.State = meditationRunStateStopped
	}
	if !state.StartedAt.IsZero() && state.StartedAt.Unix() > 0 {
		status.StartedAt = state.StartedAt.UnixMilli()
	}
	return status
}

func setMeditationRunLog(state *meditationRunState, message string) {
	state.LastLogSeq++
	state.LastLogMessage = message
}

func advanceMeditationRunByElapsedTx(
	ctx context.Context,
	tx pgx.Tx,
	userID uuid.UUID,
	state *meditationRunState,
	now time.Time,
	offlineCapDuration time.Duration,
) error {
	if state == nil || !state.RunActive {
		return nil
	}

	elapsed := now.Sub(state.RunUpdatedAt)
	if elapsed <= 0 {
		return nil
	}

	processDuration := elapsed
	forceStopByOffline := false
	if processDuration > offlineCapDuration {
		processDuration = offlineCapDuration
		forceStopByOffline = true
	}

	nowMilli := now.UnixMilli()
	_, bonus := decodeMeditationActiveEffects(state.ActiveEffectsRaw, nowMilli)
	currentRate := resolveMeditationSpiritRegen(state.Level, state.SpiritRate, bonus)
	spiritCap := resolveMeditationSpiritCap(state.Level, bonus)
	targetCap := recoverableSpiritCap(math.Max(0, state.Spirit), spiritCap)

	if state.Spirit < 0 {
		state.Spirit = 0
	}

	gainedSpirit := currentRate * processDuration.Seconds()
	if gainedSpirit < 0 {
		gainedSpirit = 0
	}
	remainingSpirit := targetCap - state.Spirit
	if remainingSpirit < 0 {
		remainingSpirit = 0
	}
	if gainedSpirit > remainingSpirit {
		gainedSpirit = remainingSpirit
	}

	if gainedSpirit > 0 {
		state.Spirit += gainedSpirit
		state.TotalSpiritGain += gainedSpirit
	}
	state.RunUpdatedAt = now

	switch {
	case state.Spirit >= targetCap-0.000001:
		state.RunActive = false
		state.LastState = meditationRunStateFull
		state.EndedAt = now
		setMeditationRunLog(state, "灵力已恢复至上限，打坐结束")
	case forceStopByOffline:
		state.RunActive = false
		state.LastState = meditationRunStateOffline
		state.EndedAt = now
		setMeditationRunLog(state, "离线超过12小时，打坐自动结束")
	default:
		state.LastState = meditationRunStateRunning
		state.EndedAt = time.Time{}
	}

	if err := persistMeditationSpiritStateTx(ctx, tx, userID, state.Spirit); err != nil {
		return err
	}
	return updateMeditationRunTx(ctx, tx, userID, state)
}

func persistMeditationSpiritStateTx(ctx context.Context, tx pgx.Tx, userID uuid.UUID, spirit float64) error {
	const query = `
		UPDATE player_resources
		SET spirit = $2, updated_at = now()
		WHERE user_id = $1
	`
	if _, err := tx.Exec(ctx, query, userID, spirit); err != nil {
		return fmt.Errorf("update meditation spirit state: %w", err)
	}
	return nil
}

func updateMeditationRunTx(ctx context.Context, tx pgx.Tx, userID uuid.UUID, state *meditationRunState) error {
	const query = `
		UPDATE player_meditation_runs
		SET
			is_active = $2,
			total_spirit_gain = $3,
			last_state = $4,
			last_log_seq = $5,
			last_log_message = $6,
			started_at = $7,
			ended_at = $8,
			updated_at = $9,
			last_processed_at = $9,
			failure_count = 0,
			last_error = ''
		WHERE user_id = $1
	`

	var startedAt any
	if !state.StartedAt.IsZero() && state.StartedAt.Unix() > 0 {
		startedAt = state.StartedAt
	}

	var endedAt any
	if !state.EndedAt.IsZero() && state.EndedAt.Unix() > 0 {
		endedAt = state.EndedAt
	}

	updatedAt := state.RunUpdatedAt
	if updatedAt.IsZero() {
		updatedAt = time.Now()
	}

	if _, err := tx.Exec(
		ctx,
		query,
		userID,
		state.RunActive,
		state.TotalSpiritGain,
		state.LastState,
		state.LastLogSeq,
		state.LastLogMessage,
		startedAt,
		endedAt,
		updatedAt,
	); err != nil {
		return fmt.Errorf("update meditation run: %w", err)
	}
	return nil
}

func stopMeditationForConflictTx(ctx context.Context, tx pgx.Tx, userID uuid.UUID, message string) error {
	const query = `
		UPDATE player_meditation_runs
		SET
			is_active = FALSE,
			last_state = $2,
			last_log_seq = last_log_seq + 1,
			last_log_message = $3,
			ended_at = now(),
			updated_at = now(),
			last_processed_at = now(),
			failure_count = 0,
			last_error = ''
		WHERE user_id = $1
		  AND is_active = TRUE
	`
	if _, err := tx.Exec(ctx, query, userID, meditationRunStateStopped, message); err != nil {
		return fmt.Errorf("stop meditation for conflict: %w", err)
	}
	return nil
}
