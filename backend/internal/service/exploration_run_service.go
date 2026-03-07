package service

import (
	"context"
	"fmt"
	"log"
	"math"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/kowming/vue-idle-xiuxian/backend/internal/repository"
)

const (
	explorationRunStateRunning            = "running"
	explorationRunStateStopped            = "stopped"
	explorationRunStateInsufficientSpirit = "insufficient_spirit"
	explorationRunStateOffline            = "offline_timeout"
	explorationRunStateInvalidLocation    = "invalid_location"
	explorationRunStateSystem             = "system_stopped"

	defaultExplorationRunInterval   = 3 * time.Second
	defaultExplorationRunOfflineCap = 12 * time.Hour
)

type ExplorationRunStatusResult struct {
	IsActive            bool    `json:"isActive"`
	State               string  `json:"state"`
	LocationID          string  `json:"locationId"`
	LocationName        string  `json:"locationName"`
	TotalRuns           int64   `json:"totalRuns"`
	TotalSpiritCost     int64   `json:"totalSpiritCost"`
	LastLogSeq          int64   `json:"lastLogSeq"`
	LastLogMessage      string  `json:"lastLogMessage"`
	ProgressPercent     float64 `json:"progressPercent"`
	ProgressRemainingMs int64   `json:"progressRemainingMs"`
	ProgressLabel       string  `json:"progressLabel"`
	StartedAt           int64   `json:"startedAt,omitempty"`
}

type ExplorationRunActionResult struct {
	Message  string                      `json:"message"`
	State    string                      `json:"state"`
	Run      *ExplorationRunStatusResult `json:"run"`
	Snapshot *repository.PlayerSnapshot  `json:"snapshot,omitempty"`
}

type explorationRunState struct {
	RunActive       bool
	LocationID      string
	LocationName    string
	TotalRuns       int64
	TotalSpiritCost int64
	LastState       string
	LastLogSeq      int64
	LastLogMessage  string
	StartedAt       time.Time
	EndedAt         time.Time
	RunUpdatedAt    time.Time
}

func (s *ExplorationService) ExplorationStatus(ctx context.Context, userID uuid.UUID) (*ExplorationRunStatusResult, error) {
	const query = `
		SELECT
			COALESCE(per.is_active, FALSE),
			COALESCE(per.location_id, ''),
			COALESCE(per.location_name, ''),
			COALESCE(per.total_runs, 0),
			COALESCE(per.total_spirit_cost, 0),
			COALESCE(per.last_state, ''),
			COALESCE(per.last_log_seq, 0),
			COALESCE(per.last_log_message, ''),
			COALESCE(per.started_at, to_timestamp(0)),
			COALESCE(per.ended_at, to_timestamp(0)),
			COALESCE(per.updated_at, now())
		FROM player_profiles pp
		LEFT JOIN player_exploration_runs per ON per.user_id = pp.user_id
		WHERE pp.user_id = $1
	`

	runState := &explorationRunState{}
	if err := s.pool.QueryRow(ctx, query, userID).Scan(
		&runState.RunActive,
		&runState.LocationID,
		&runState.LocationName,
		&runState.TotalRuns,
		&runState.TotalSpiritCost,
		&runState.LastState,
		&runState.LastLogSeq,
		&runState.LastLogMessage,
		&runState.StartedAt,
		&runState.EndedAt,
		&runState.RunUpdatedAt,
	); err != nil {
		if err == pgx.ErrNoRows {
			return &ExplorationRunStatusResult{
				IsActive: false,
				State:    explorationRunStateStopped,
			}, nil
		}
		return nil, fmt.Errorf("query exploration status: %w", err)
	}

	status := buildExplorationRunStatus(runState, time.Now())
	if status.State == "" {
		status.State = explorationRunStateStopped
	}
	return status, nil
}

func (s *ExplorationService) ExplorationAutoStart(
	ctx context.Context,
	userID uuid.UUID,
	locationID string,
) (*ExplorationRunActionResult, error) {
	locationID = strings.TrimSpace(locationID)
	location, ok := explorationLocationByID(locationID)
	if !ok {
		return nil, &InvalidLocationError{LocationID: locationID}
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin exploration auto start transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if err := ensureNoActiveDungeonRunTx(ctx, tx, userID); err != nil {
		return nil, err
	}
	if err := ensureHuntingRunRow(ctx, tx, userID); err != nil {
		return nil, err
	}
	if err := stopHuntingForConflictTx(ctx, tx, userID, "进行探索，刷怪已自动结束"); err != nil {
		return nil, err
	}
	if err := ensureMeditationRunRow(ctx, tx, userID); err != nil {
		return nil, err
	}
	if err := stopMeditationForConflictTx(ctx, tx, userID, "进行探索，打坐已自动结束"); err != nil {
		return nil, err
	}
	if err := ensureExplorationRows(ctx, tx, userID); err != nil {
		return nil, err
	}
	if err := ensureExplorationRunRow(ctx, tx, userID); err != nil {
		return nil, err
	}

	runState, err := loadExplorationRunStateForUpdate(ctx, tx, userID)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	if runState.RunActive && runState.LocationID == location.ID {
		status := buildExplorationRunStatus(runState, now)
		if err := tx.Commit(ctx); err != nil {
			return nil, fmt.Errorf("commit exploration auto start noop transaction: %w", err)
		}
		s.notifyRealtime(userID, GameRealtimeTopicExploration, GameRealtimeTopicSnapshot, GameRealtimeTopicMeditation, GameRealtimeTopicHunting)
		return &ExplorationRunActionResult{
			Message: "当前已在自动探索中",
			State:   status.State,
			Run:     status,
		}, nil
	}

	exploreState, err := loadExplorationState(ctx, tx, userID)
	if err != nil {
		return nil, err
	}
	if exploreState.Level < location.MinLevel {
		return nil, &LocationLockedError{RequiredLevel: location.MinLevel, CurrentLevel: exploreState.Level}
	}
	if exploreState.Spirit < float64(location.SpiritCost) {
		return nil, &InsufficientSpiritError{Required: float64(location.SpiritCost), Current: exploreState.Spirit}
	}

	runState.RunActive = true
	runState.LocationID = location.ID
	runState.LocationName = location.Name
	runState.TotalRuns = 0
	runState.TotalSpiritCost = 0
	runState.LastState = explorationRunStateRunning
	runState.StartedAt = now
	runState.EndedAt = time.Time{}
	runState.RunUpdatedAt = now
	setExplorationRunLog(runState, fmt.Sprintf("已开始在%s自动探索", location.Name))

	if err := updateExplorationRunTx(ctx, tx, userID, runState); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit exploration auto start transaction: %w", err)
	}
	s.notifyRealtime(userID, GameRealtimeTopicExploration, GameRealtimeTopicSnapshot, GameRealtimeTopicMeditation, GameRealtimeTopicHunting)

	snapshot, err := s.userRepo.GetSnapshot(ctx, userID)
	if err != nil {
		return nil, err
	}

	return &ExplorationRunActionResult{
		Message:  fmt.Sprintf("已开始在%s自动探索", location.Name),
		State:    explorationRunStateRunning,
		Run:      buildExplorationRunStatus(runState, now),
		Snapshot: snapshot,
	}, nil
}

func (s *ExplorationService) ExplorationAutoStop(ctx context.Context, userID uuid.UUID) (*ExplorationRunActionResult, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin exploration auto stop transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if err := ensureExplorationRunRow(ctx, tx, userID); err != nil {
		return nil, err
	}
	runState, err := loadExplorationRunStateForUpdate(ctx, tx, userID)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	message := "当前未在自动探索"
	if runState.RunActive {
		runState.RunActive = false
		runState.LastState = explorationRunStateStopped
		runState.EndedAt = now
		runState.RunUpdatedAt = now
		setExplorationRunLog(runState, "已停止自动探索")
		message = "已停止自动探索"
	}

	if err := updateExplorationRunTx(ctx, tx, userID, runState); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit exploration auto stop transaction: %w", err)
	}
	s.notifyRealtime(userID, GameRealtimeTopicExploration, GameRealtimeTopicSnapshot)

	snapshot, err := s.userRepo.GetSnapshot(ctx, userID)
	if err != nil {
		return nil, err
	}
	return &ExplorationRunActionResult{
		Message:  message,
		State:    runState.LastState,
		Run:      buildExplorationRunStatus(runState, now),
		Snapshot: snapshot,
	}, nil
}

func (s *ExplorationService) AdvanceActiveRuns(ctx context.Context, limit int) (int, error) {
	if limit <= 0 {
		limit = 200
	}

	const query = `
		SELECT user_id
		FROM player_exploration_runs
		WHERE is_active = TRUE
		ORDER BY last_processed_at ASC, updated_at ASC
		LIMIT $1
	`
	rows, err := s.pool.Query(ctx, query, limit)
	if err != nil {
		return 0, fmt.Errorf("list active exploration runs: %w", err)
	}
	defer rows.Close()

	userIDs := make([]uuid.UUID, 0, limit)
	for rows.Next() {
		var userID uuid.UUID
		if err := rows.Scan(&userID); err != nil {
			return 0, fmt.Errorf("scan active exploration run user id: %w", err)
		}
		userIDs = append(userIDs, userID)
	}
	if err := rows.Err(); err != nil {
		return 0, fmt.Errorf("iterate active exploration run users: %w", err)
	}

	processed := 0
	for _, userID := range userIDs {
		advanceCtx, cancel := context.WithTimeout(ctx, sweepPerUserAdvanceTimeout)
		err := wrapSweepUserAdvance(func() error {
			return s.AdvanceUserRun(advanceCtx, userID)
		})
		cancel()
		if err != nil {
			if ctx.Err() != nil {
				return processed, ctx.Err()
			}
			if isSweepSkippableError(err) {
				log.Printf("exploration sweep skip user=%s err=%v", userID.String(), err)
				_ = s.markAdvanceFailure(context.Background(), userID, err)
				continue
			}
			log.Printf("exploration sweep isolate user=%s err=%v", userID.String(), err)
			_ = s.markAdvanceFailure(context.Background(), userID, err)
			continue
		}
		processed++
	}
	return processed, nil
}

func (s *ExplorationService) markAdvanceFailure(ctx context.Context, userID uuid.UUID, advanceErr error) error {
	if s == nil || s.pool == nil {
		return nil
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	message := trimSweepError(advanceErr)
	if message == "" {
		message = "unknown sweep error"
	}
	logMessage := fmt.Sprintf("系统保护性暂停：%s", message)

	const query = `
		UPDATE player_exploration_runs
		SET
			failure_count = failure_count + 1,
			last_error = $2,
			last_processed_at = now(),
			last_log_seq = CASE WHEN failure_count + 1 >= $3 AND is_active THEN COALESCE(last_log_seq, 0) + 1 ELSE last_log_seq END,
			last_log_message = CASE WHEN failure_count + 1 >= $3 AND is_active THEN $4 ELSE last_log_message END,
			last_state = CASE WHEN failure_count + 1 >= $3 AND is_active THEN $5 ELSE last_state END,
			is_active = CASE WHEN failure_count + 1 >= $3 THEN FALSE ELSE is_active END,
			ended_at = CASE WHEN failure_count + 1 >= $3 THEN now() ELSE ended_at END,
			updated_at = now()
		WHERE user_id = $1
		  AND is_active = TRUE
	`
	if _, err := tx.Exec(ctx, query, userID, message, sweepFailureStopThreshold, logMessage, explorationRunStateSystem); err != nil {
		return fmt.Errorf("mark exploration advance failure: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit exploration advance failure: %w", err)
	}
	if s.realtimeBroker != nil {
		s.realtimeBroker.Publish(userID, GameRealtimeTopicExploration)
		s.realtimeBroker.Publish(userID, GameRealtimeTopicSnapshot)
	}
	return nil
}

func (s *ExplorationService) AdvanceUserRun(ctx context.Context, userID uuid.UUID) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin exploration advance transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if err := ensureExplorationRows(ctx, tx, userID); err != nil {
		return err
	}
	if err := ensureExplorationRunRow(ctx, tx, userID); err != nil {
		return err
	}
	runState, err := loadExplorationRunStateForUpdate(ctx, tx, userID)
	if err != nil {
		return err
	}
	if !runState.RunActive {
		if err := tx.Commit(ctx); err != nil {
			return fmt.Errorf("commit exploration advance inactive transaction: %w", err)
		}
		return nil
	}

	now := time.Now()
	elapsed := now.Sub(runState.RunUpdatedAt)
	if elapsed <= 0 {
		if err := touchExplorationRunProcessedAtTx(ctx, tx, userID); err != nil {
			return err
		}
		if err := tx.Commit(ctx); err != nil {
			return fmt.Errorf("commit exploration advance idle transaction: %w", err)
		}
		return nil
	}
	if elapsed > defaultExplorationRunOfflineCap {
		runState.RunActive = false
		runState.LastState = explorationRunStateOffline
		runState.EndedAt = now
		runState.RunUpdatedAt = now
		setExplorationRunLog(runState, "离线超过12小时，自动探索已结束")
		if err := updateExplorationRunTx(ctx, tx, userID, runState); err != nil {
			return err
		}
		if err := tx.Commit(ctx); err != nil {
			return fmt.Errorf("commit exploration advance offline transaction: %w", err)
		}
		s.notifyRealtime(userID, GameRealtimeTopicExploration, GameRealtimeTopicSnapshot)
		return nil
	}

	steps := int(elapsed / defaultExplorationRunInterval)
	if steps <= 0 {
		if err := touchExplorationRunProcessedAtTx(ctx, tx, userID); err != nil {
			return err
		}
		if err := tx.Commit(ctx); err != nil {
			return fmt.Errorf("commit exploration advance waiting transaction: %w", err)
		}
		return nil
	}
	if steps > 20 {
		steps = 20
	}

	location, ok := explorationLocationByID(runState.LocationID)
	if !ok {
		runState.RunActive = false
		runState.LastState = explorationRunStateInvalidLocation
		runState.EndedAt = now
		runState.RunUpdatedAt = now
		setExplorationRunLog(runState, "探索地点不存在，自动探索已停止")
		if err := updateExplorationRunTx(ctx, tx, userID, runState); err != nil {
			return err
		}
		if err := tx.Commit(ctx); err != nil {
			return fmt.Errorf("commit exploration advance invalid location transaction: %w", err)
		}
		s.notifyRealtime(userID, GameRealtimeTopicExploration, GameRealtimeTopicSnapshot)
		return nil
	}

	exploreState, err := loadExplorationState(ctx, tx, userID)
	if err != nil {
		return err
	}
	previousLevel := exploreState.Level

	runChanged := false
	stateChanged := false
	for i := 0; i < steps; i++ {
		if exploreState.Level < location.MinLevel {
			runState.RunActive = false
			runState.LastState = explorationRunStateStopped
			runState.EndedAt = now
			runState.RunUpdatedAt = now
			setExplorationRunLog(runState, "境界不足，自动探索已停止")
			runChanged = true
			break
		}
		if exploreState.Spirit < float64(location.SpiritCost) {
			runState.RunActive = false
			runState.LastState = explorationRunStateInsufficientSpirit
			runState.EndedAt = now
			runState.RunUpdatedAt = now
			setExplorationRunLog(runState, "灵力不足，自动探索已停止")
			runChanged = true
			break
		}

		roundResult := applyExplorationRound(exploreState, location)
		stateChanged = true
		runChanged = true
		runState.TotalRuns++
		runState.TotalSpiritCost += location.SpiritCost
		runState.LastState = explorationRunStateRunning
		runState.EndedAt = time.Time{}
		runState.RunUpdatedAt = runState.RunUpdatedAt.Add(defaultExplorationRunInterval)

		logMessage := fmt.Sprintf("在%s探索完成", location.Name)
		if count := len(roundResult.Messages); count > 0 {
			logMessage = roundResult.Messages[count-1]
		}
		setExplorationRunLog(runState, logMessage)
	}

	if stateChanged {
		if err := persistExplorationState(ctx, tx, userID, exploreState); err != nil {
			return err
		}
	}
	if runChanged {
		if runState.RunUpdatedAt.After(now.Add(defaultExplorationRunInterval)) {
			runState.RunUpdatedAt = now
		}
		if err := updateExplorationRunTx(ctx, tx, userID, runState); err != nil {
			return err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit exploration advance transaction: %w", err)
	}
	if stateChanged && exploreState.Level > previousLevel && s.userRepo != nil && s.realtimeBroker != nil {
		snapshot, snapErr := s.userRepo.GetSnapshot(ctx, userID)
		if snapErr == nil && snapshot != nil {
			for _, realmName := range majorRealmTransitionsBetween(previousLevel, exploreState.Level) {
				publishWorldAnnouncement(ctx, s.realtimeBroker, buildMajorRealmBreakthroughAnnouncement(snapshot.Name, realmName))
			}
		}
	}

	if stateChanged || runChanged {
		s.notifyRealtime(userID, GameRealtimeTopicExploration, GameRealtimeTopicSnapshot)
	}
	return nil
}

func ensureExplorationRunRow(ctx context.Context, tx pgx.Tx, userID uuid.UUID) error {
	const query = `
		INSERT INTO player_exploration_runs (
			user_id, is_active, location_id, location_name, total_runs, total_spirit_cost, last_state
		)
		VALUES ($1, FALSE, '', '', 0, 0, 'stopped')
		ON CONFLICT (user_id) DO NOTHING
	`
	if _, err := tx.Exec(ctx, query, userID); err != nil {
		return fmt.Errorf("ensure exploration run row: %w", err)
	}
	return nil
}

func loadExplorationRunStateForUpdate(ctx context.Context, tx pgx.Tx, userID uuid.UUID) (*explorationRunState, error) {
	const query = `
		SELECT
			per.is_active,
			COALESCE(per.location_id, ''),
			COALESCE(per.location_name, ''),
			COALESCE(per.total_runs, 0),
			COALESCE(per.total_spirit_cost, 0),
			COALESCE(per.last_state, ''),
			COALESCE(per.last_log_seq, 0),
			COALESCE(per.last_log_message, ''),
			COALESCE(per.started_at, to_timestamp(0)),
			COALESCE(per.ended_at, to_timestamp(0)),
			per.updated_at
		FROM player_exploration_runs per
		WHERE per.user_id = $1
		FOR UPDATE
	`

	state := &explorationRunState{}
	if err := tx.QueryRow(ctx, query, userID).Scan(
		&state.RunActive,
		&state.LocationID,
		&state.LocationName,
		&state.TotalRuns,
		&state.TotalSpiritCost,
		&state.LastState,
		&state.LastLogSeq,
		&state.LastLogMessage,
		&state.StartedAt,
		&state.EndedAt,
		&state.RunUpdatedAt,
	); err != nil {
		return nil, fmt.Errorf("load exploration run state: %w", err)
	}
	return state, nil
}

func updateExplorationRunTx(ctx context.Context, tx pgx.Tx, userID uuid.UUID, state *explorationRunState) error {
	const query = `
		UPDATE player_exploration_runs
		SET
			is_active = $2,
			location_id = $3,
			location_name = $4,
			total_runs = $5,
			total_spirit_cost = $6,
			last_state = $7,
			last_log_seq = $8,
			last_log_message = $9,
			started_at = $10,
			ended_at = $11,
			updated_at = $12,
			last_processed_at = $12,
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
		state.LocationID,
		state.LocationName,
		state.TotalRuns,
		state.TotalSpiritCost,
		state.LastState,
		state.LastLogSeq,
		state.LastLogMessage,
		startedAt,
		endedAt,
		updatedAt,
	); err != nil {
		return fmt.Errorf("update exploration run: %w", err)
	}
	return nil
}

func touchExplorationRunProcessedAtTx(ctx context.Context, tx pgx.Tx, userID uuid.UUID) error {
	const query = `
		UPDATE player_exploration_runs
		SET last_processed_at = now()
		WHERE user_id = $1
		  AND is_active = TRUE
	`
	if _, err := tx.Exec(ctx, query, userID); err != nil {
		return fmt.Errorf("touch exploration last processed at: %w", err)
	}
	return nil
}

func buildExplorationRunStatus(state *explorationRunState, now time.Time) *ExplorationRunStatusResult {
	result := &ExplorationRunStatusResult{
		IsActive:        state.RunActive,
		State:           state.LastState,
		LocationID:      state.LocationID,
		LocationName:    state.LocationName,
		TotalRuns:       state.TotalRuns,
		TotalSpiritCost: state.TotalSpiritCost,
		LastLogSeq:      state.LastLogSeq,
		LastLogMessage:  state.LastLogMessage,
		ProgressLabel:   "探索进度",
	}
	if result.State == "" {
		result.State = explorationRunStateStopped
	}
	if !state.StartedAt.IsZero() && state.StartedAt.Unix() > 0 {
		result.StartedAt = state.StartedAt.UnixMilli()
	}

	if !state.RunActive {
		return result
	}

	intervalMs := int64(defaultExplorationRunInterval / time.Millisecond)
	if intervalMs <= 0 {
		intervalMs = 3000
	}
	elapsedMs := now.Sub(state.RunUpdatedAt).Milliseconds()
	if elapsedMs < 0 {
		elapsedMs = 0
	}
	if elapsedMs > intervalMs {
		elapsedMs = intervalMs
	}
	result.ProgressPercent = math.Round((float64(elapsedMs) * 10000 / float64(intervalMs))) / 100
	result.ProgressRemainingMs = intervalMs - elapsedMs
	return result
}

func setExplorationRunLog(state *explorationRunState, message string) {
	state.LastLogSeq++
	state.LastLogMessage = strings.TrimSpace(message)
}
