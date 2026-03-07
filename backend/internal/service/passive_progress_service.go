package service

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/kowming/vue-idle-xiuxian/backend/internal/repository"
)

const (
	// 后端自动推进刷图步长：1 秒结算一次，避免前端轮询依赖。
	passiveHuntingEncounterInterval = time.Second
)

type PassiveProgressService struct {
	pool           *pgxpool.Pool
	userRepo       *repository.UserRepository
	runtimeConfig  *RuntimeConfigService
	realtimeBroker *GameRealtimeBroker
}

func NewPassiveProgressService(pool *pgxpool.Pool, userRepo *repository.UserRepository, runtimeConfig *RuntimeConfigService, realtimeBroker *GameRealtimeBroker) *PassiveProgressService {
	return &PassiveProgressService{
		pool:           pool,
		userRepo:       userRepo,
		runtimeConfig:  runtimeConfig,
		realtimeBroker: realtimeBroker,
	}
}

func (s *PassiveProgressService) Advance(ctx context.Context, userID uuid.UUID) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin passive progress transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if err := ensureHuntingRunRow(ctx, tx, userID); err != nil {
		return err
	}

	huntingState, err := loadHuntingRunStateForUpdate(ctx, tx, userID)
	if err != nil {
		return err
	}
	previousHuntingLevel := huntingState.Level

	if huntingState.RunActive {
		huntingGainMultiplier := s.getHuntingWinGainMultiplier(ctx)
		offlineCapDuration := s.getHuntingOfflineCapDuration(ctx)
		reviveMultiplier := s.getHuntingReviveMultiplier(ctx)
		healBaseRate, healCapRate := s.getHuntingAutoHealRates(ctx)
		refundChance, refundMinRatio, refundMaxRatio := s.getHuntingSpiritRefundConfig(ctx)
		if err := advanceHuntingRunByElapsedTx(
			ctx,
			tx,
			userID,
			huntingState,
			time.Now(),
			huntingGainMultiplier,
			offlineCapDuration,
			reviveMultiplier,
			healBaseRate,
			healCapRate,
			refundChance,
			refundMinRatio,
			refundMaxRatio,
		); err != nil {
			return err
		}
	}

	if err := ensureMeditationRunRow(ctx, tx, userID); err != nil {
		return err
	}

	meditationState, err := loadMeditationRunStateForUpdate(ctx, tx, userID)
	if err != nil {
		return err
	}

	if huntingState.RunActive && meditationState.RunActive {
		now := time.Now()
		meditationState.RunActive = false
		meditationState.LastState = meditationRunStateStopped
		meditationState.EndedAt = now
		meditationState.RunUpdatedAt = now
		setMeditationRunLog(meditationState, "刷图进行中，打坐已自动结束")
		if err := updateMeditationRunTx(ctx, tx, userID, meditationState); err != nil {
			return err
		}
	} else if meditationState.RunActive {
		if err := advanceMeditationRunByElapsedTx(
			ctx,
			tx,
			userID,
			meditationState,
			time.Now(),
			defaultMeditationOfflineCap,
		); err != nil {
			return err
		}
	}

	if err := touchPassiveRunsProcessedAtTx(ctx, tx, userID); err != nil {
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit passive progress transaction: %w", err)
	}
	if s.userRepo != nil && huntingState.Level > previousHuntingLevel {
		snapshot, snapErr := s.userRepo.GetSnapshot(ctx, userID)
		if snapErr == nil && snapshot != nil && s.realtimeBroker != nil {
			for _, realmName := range majorRealmTransitionsBetween(previousHuntingLevel, huntingState.Level) {
				publishWorldAnnouncement(ctx, s.realtimeBroker, buildMajorRealmBreakthroughAnnouncement(snapshot.Name, realmName))
			}
		}
	}
	if s.realtimeBroker != nil {
		s.realtimeBroker.Publish(userID, GameRealtimeTopicAll)
	}
	return nil
}

func touchPassiveRunsProcessedAtTx(ctx context.Context, tx pgx.Tx, userID uuid.UUID) error {
	const huntingQuery = `
		UPDATE player_hunting_runs
		SET last_processed_at = now()
		WHERE user_id = $1
		  AND is_active = TRUE
	`
	if _, err := tx.Exec(ctx, huntingQuery, userID); err != nil {
		return fmt.Errorf("touch hunting last processed at: %w", err)
	}

	const meditationQuery = `
		UPDATE player_meditation_runs
		SET last_processed_at = now()
		WHERE user_id = $1
		  AND is_active = TRUE
	`
	if _, err := tx.Exec(ctx, meditationQuery, userID); err != nil {
		return fmt.Errorf("touch meditation last processed at: %w", err)
	}
	return nil
}

func (s *PassiveProgressService) TouchActivity(ctx context.Context, userID uuid.UUID) error {
	const query = `
		INSERT INTO player_activity (user_id, last_seen_at, updated_at)
		VALUES ($1, now(), now())
		ON CONFLICT (user_id)
		DO UPDATE SET
			last_seen_at = EXCLUDED.last_seen_at,
			updated_at = now()
	`
	if _, err := s.pool.Exec(ctx, query, userID); err != nil {
		return fmt.Errorf("touch player activity: %w", err)
	}
	return nil
}

// AdvanceActiveRuns 批量推进活跃中的玩家，返回本轮成功推进的用户数。
func (s *PassiveProgressService) AdvanceActiveRuns(ctx context.Context, limit int) (int, error) {
	if limit <= 0 {
		limit = 200
	}

	const query = `
		SELECT user_id
		FROM (
			SELECT user_id, MIN(last_processed_at) AS last_processed_at
			FROM (
				SELECT user_id, last_processed_at
				FROM player_hunting_runs
				WHERE is_active = TRUE

				UNION ALL

				SELECT user_id, last_processed_at
				FROM player_meditation_runs
				WHERE is_active = TRUE
			) active_sources
			GROUP BY user_id
		) active_runs
		ORDER BY last_processed_at ASC
		LIMIT $1
	`

	rows, err := s.pool.Query(ctx, query, limit)
	if err != nil {
		return 0, fmt.Errorf("list active hunting runs: %w", err)
	}
	defer rows.Close()

	userIDs := make([]uuid.UUID, 0, limit)
	for rows.Next() {
		var userID uuid.UUID
		if err := rows.Scan(&userID); err != nil {
			return 0, fmt.Errorf("scan active hunting run user id: %w", err)
		}
		userIDs = append(userIDs, userID)
	}
	if err := rows.Err(); err != nil {
		return 0, fmt.Errorf("iterate active hunting run users: %w", err)
	}

	processed := 0
	for _, userID := range userIDs {
		advanceCtx, cancel := context.WithTimeout(ctx, sweepPerUserAdvanceTimeout)
		err := wrapSweepUserAdvance(func() error {
			return s.Advance(advanceCtx, userID)
		})
		cancel()
		if err != nil {
			if ctx.Err() != nil {
				return processed, ctx.Err()
			}
			if isSweepSkippableError(err) {
				log.Printf("hunting sweep skip user=%s err=%v", userID.String(), err)
				_ = s.markAdvanceFailure(context.Background(), userID, err)
				continue
			}
			log.Printf("hunting sweep isolate user=%s err=%v", userID.String(), err)
			_ = s.markAdvanceFailure(context.Background(), userID, err)
			continue
		}
		processed++
	}
	return processed, nil
}

func (s *PassiveProgressService) markAdvanceFailure(ctx context.Context, userID uuid.UUID, advanceErr error) error {
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

	const updateHuntingSQL = `
		UPDATE player_hunting_runs
		SET
			failure_count = failure_count + 1,
			last_error = $2,
			last_processed_at = now(),
			last_log_seq = CASE WHEN failure_count + 1 >= $3 AND is_active THEN COALESCE(last_log_seq, 0) + 1 ELSE last_log_seq END,
			last_log_message = CASE WHEN failure_count + 1 >= $3 AND is_active THEN $4 ELSE last_log_message END,
			last_state = CASE WHEN failure_count + 1 >= $3 AND is_active THEN $5 ELSE last_state END,
			is_active = CASE WHEN failure_count + 1 >= $3 THEN FALSE ELSE is_active END,
			ended_at = CASE WHEN failure_count + 1 >= $3 THEN now() ELSE ended_at END,
			updated_at = now(),
			revive_until = CASE WHEN failure_count + 1 >= $3 THEN NULL ELSE revive_until END
		WHERE user_id = $1
		  AND is_active = TRUE
	`
	if _, err := tx.Exec(ctx, updateHuntingSQL, userID, message, sweepFailureStopThreshold, logMessage, huntingRunStateSystem); err != nil {
		return fmt.Errorf("mark hunting advance failure: %w", err)
	}

	const updateMeditationSQL = `
		UPDATE player_meditation_runs
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
	if _, err := tx.Exec(ctx, updateMeditationSQL, userID, message, sweepFailureStopThreshold, logMessage, meditationRunStateSystem); err != nil {
		return fmt.Errorf("mark meditation advance failure: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit advance failure marker: %w", err)
	}
	if s.realtimeBroker != nil {
		s.realtimeBroker.Publish(userID, GameRealtimeTopicAll)
	}
	return nil
}

func (s *PassiveProgressService) getHuntingWinGainMultiplier(ctx context.Context) float64 {
	if s.runtimeConfig == nil {
		return defaultHuntingWinGainMultiplier
	}
	return s.runtimeConfig.GetFloat64(ctx, RuntimeConfigKeyHuntingWinGainMultiplier, defaultHuntingWinGainMultiplier, 0.5, 10.0)
}

func (s *PassiveProgressService) getHuntingOfflineCapDuration(ctx context.Context) time.Duration {
	if s.runtimeConfig == nil {
		return defaultHuntingOfflineCap
	}
	seconds := s.runtimeConfig.GetInt(
		ctx,
		RuntimeConfigKeyHuntingOfflineCapSeconds,
		int(defaultHuntingOfflineCap/time.Second),
		600,
		7*24*60*60,
	)
	return time.Duration(seconds) * time.Second
}

func (s *PassiveProgressService) getHuntingReviveMultiplier(ctx context.Context) float64 {
	if s.runtimeConfig == nil {
		return defaultHuntingReviveMultiplier
	}
	return s.runtimeConfig.GetFloat64(ctx, RuntimeConfigKeyHuntingReviveMultiplier, defaultHuntingReviveMultiplier, 0.2, 5.0)
}

func (s *PassiveProgressService) getHuntingAutoHealRates(ctx context.Context) (float64, float64) {
	if s.runtimeConfig == nil {
		return defaultHuntingAutoHealBaseRate, defaultHuntingAutoHealCapRate
	}
	base := s.runtimeConfig.GetFloat64(
		ctx,
		RuntimeConfigKeyHuntingAutoHealBaseRate,
		defaultHuntingAutoHealBaseRate,
		0,
		1,
	)
	capRate := s.runtimeConfig.GetFloat64(
		ctx,
		RuntimeConfigKeyHuntingAutoHealCapRate,
		defaultHuntingAutoHealCapRate,
		0,
		1,
	)
	if capRate < base {
		capRate = base
	}
	return base, capRate
}

func (s *PassiveProgressService) getHuntingSpiritRefundConfig(ctx context.Context) (float64, float64, float64) {
	if s.runtimeConfig == nil {
		return defaultHuntingSpiritRefundChance, defaultHuntingSpiritRefundMinRatio, defaultHuntingSpiritRefundMaxRatio
	}
	chance := s.runtimeConfig.GetFloat64(
		ctx,
		RuntimeConfigKeyHuntingSpiritRefundChance,
		defaultHuntingSpiritRefundChance,
		0,
		1,
	)
	minRatio := s.runtimeConfig.GetFloat64(
		ctx,
		RuntimeConfigKeyHuntingSpiritRefundMinRatio,
		defaultHuntingSpiritRefundMinRatio,
		0,
		1,
	)
	maxRatio := s.runtimeConfig.GetFloat64(
		ctx,
		RuntimeConfigKeyHuntingSpiritRefundMaxRatio,
		defaultHuntingSpiritRefundMaxRatio,
		0,
		1,
	)
	return resolveHuntingSpiritRefundConfig(chance, minRatio, maxRatio)
}

func advanceHuntingRunByElapsedTx(
	ctx context.Context,
	tx pgx.Tx,
	userID uuid.UUID,
	state *huntingRunState,
	now time.Time,
	huntingGainMultiplier float64,
	offlineCapDuration time.Duration,
	reviveMultiplier float64,
	healBaseRate float64,
	healCapRate float64,
	refundChance float64,
	refundMinRatio float64,
	refundMaxRatio float64,
) error {
	targetMap, ok := findHuntingMapByID(state.MapID)
	if !ok {
		const invalidMapStopSQL = `
			UPDATE player_hunting_runs
			SET
				is_active = FALSE,
				last_state = $2,
				revive_until = NULL,
				ended_at = now(),
				updated_at = now()
			WHERE user_id = $1
		`
		if _, err := tx.Exec(ctx, invalidMapStopSQL, userID, huntingRunStateStopped); err != nil {
			return fmt.Errorf("stop hunting run for invalid map: %w", err)
		}
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
	if !forceStopByOffline {
		nextActionAt := state.RunUpdatedAt.Add(passiveHuntingEncounterInterval)
		if state.LastState == huntingRunStateReviving && !state.ReviveUntil.IsZero() && state.ReviveUntil.Unix() > 0 {
			nextActionAt = state.ReviveUntil
		}
		if now.Before(nextActionAt) {
			return nil
		}
	}

	nowMilli := now.UnixMilli()
	activeEffects, effectBonus := huntingDecodeActiveEffects(state.ActiveEffectsRaw, nowMilli)
	items, err := huntingDecodeItems(state.ItemsRaw)
	if err != nil {
		return err
	}
	herbs, err := huntingDecodeHerbs(state.HerbsRaw)
	if err != nil {
		return err
	}

	itemsChanged := false
	herbsChanged := false
	rng := rand.New(rand.NewSource(now.UnixNano() + state.KillCount + int64(state.Level)))

	runState := huntingRunStateRunning
	runActive := true
	cultivationTimes := int64(0)
	breakthroughTimes := int64(0)
	stateDirty := false
	processUntil := state.RunUpdatedAt.Add(processDuration)
	if processUntil.After(now) {
		processUntil = now
	}

	for runActive {
		if state.LastState == huntingRunStateReviving {
			reviveAt := state.ReviveUntil
			if reviveAt.IsZero() || reviveAt.Unix() <= 0 {
				reviveDuration := resolveHuntingReviveDuration(
					huntingReviveDuration(state.Level, targetMap),
					reviveMultiplier,
				)
				reviveAt = state.RunUpdatedAt.Add(reviveDuration)
				state.ReviveUntil = reviveAt
			}
			if reviveAt.After(processUntil) {
				break
			}

			player := buildHuntingPlayerEntity(state, effectBonus)
			player.CurrentHealth = player.Stats.MaxHealth
			state.CurrentHP = player.CurrentHealth
			state.MaxHP = player.Stats.MaxHealth
			state.LastState = huntingRunStateRunning
			state.ReviveUntil = time.Time{}
			state.RunUpdatedAt = reviveAt
			setHuntingRunLog(state, "你已复活，继续战斗")
			continue
		}

		nextEncounterAt := state.RunUpdatedAt.Add(passiveHuntingEncounterInterval)
		if nextEncounterAt.After(processUntil) {
			break
		}

		outcome, nextItems, nextHerbs, err := resolveHuntingEncounter(state, targetMap, items, herbs, huntingEncounterConfig{
			OccurredAt:       nextEncounterAt,
			GainMultiplier:   huntingGainMultiplier,
			ReviveMultiplier: reviveMultiplier,
			HealBaseRate:     healBaseRate,
			HealCapRate:      healCapRate,
			RefundChance:     refundChance,
			RefundMinRatio:   refundMinRatio,
			RefundMaxRatio:   refundMaxRatio,
			EffectBonus:      effectBonus,
			RNG:              rng,
		})
		if err != nil {
			return err
		}
		items = nextItems
		herbs = nextHerbs
		if outcome.PersistChanged {
			stateDirty = true
		}
		if outcome.CultivationGain > 0 {
			cultivationTimes++
		}
		if outcome.BreakthroughTimes > 0 {
			breakthroughTimes += outcome.BreakthroughTimes
		}
		if outcome.ItemsChanged {
			itemsChanged = true
		}
		if outcome.HerbsChanged {
			herbsChanged = true
		}
		if outcome.State == huntingRunStateExhausted {
			runActive = false
			runState = huntingRunStateExhausted
			break
		}
	}

	if runActive && forceStopByOffline {
		runActive = false
		runState = huntingRunStateOffline
		state.LastState = huntingRunStateOffline
		state.ReviveUntil = time.Time{}
		state.RunUpdatedAt = now
		setHuntingRunLog(state, "离线超过12小时，自动结束刷怪")
	}

	if stateDirty {
		if err := persistCultivationState(ctx, tx, userID, buildCultivationStateFromHunting(state), cultivationTimes, false); err != nil {
			return err
		}
		if breakthroughTimes > 0 {
			if err := incrementBreakthroughCountTx(ctx, tx, userID, breakthroughTimes); err != nil {
				return err
			}
		}
	}

	if itemsChanged || herbsChanged {
		if err := updateHuntingInventoryTx(ctx, tx, userID, items, herbs, activeEffects); err != nil {
			return err
		}
	} else {
		if err := updateHuntingActiveEffectsTx(ctx, tx, userID, activeEffects); err != nil {
			return err
		}
	}

	state.RunActive = runActive
	if runActive {
		if err := updateActiveHuntingRunTx(ctx, tx, userID, state); err != nil {
			return err
		}
	} else {
		if err := stopHuntingRunWithStateTx(ctx, tx, userID, state, runState, now); err != nil {
			return err
		}
	}

	return nil
}

func runHuntingAutoBattle(player *dungeonEntity, enemy *dungeonEntity, rng *rand.Rand) string {
	const maxRounds = 16

	for round := 1; round <= maxRounds; round++ {
		playerFirst := player.Stats.Speed > enemy.Stats.Speed
		if player.Stats.Speed == enemy.Stats.Speed {
			playerFirst = rng.Float64() < 0.5
		}

		first := player
		second := enemy
		playerFirstRound := true
		if !playerFirst {
			first = enemy
			second = player
			playerFirstRound = false
		}

		firstAttack := dungeonCalculateDamage(first, second, rng)
		firstOutcome := dungeonTakeDamage(second, first, firstAttack, rng)
		if !firstOutcome.Dodged && firstAttack.IsVampire {
			healAmount := firstOutcome.Damage * 0.3
			dungeonHeal(first, healAmount)
		}

		if firstOutcome.IsDead {
			if playerFirstRound {
				return dungeonBattleStateVictory
			}
			return dungeonBattleStateDefeat
		}

		if firstAttack.IsStun {
			continue
		}

		secondAttack := dungeonCalculateDamage(second, first, rng)
		secondOutcome := dungeonTakeDamage(first, second, secondAttack, rng)
		if !secondOutcome.Dodged && secondAttack.IsVampire {
			healAmount := secondOutcome.Damage * 0.3
			dungeonHeal(second, healAmount)
		}

		if secondOutcome.IsDead {
			if playerFirstRound {
				return dungeonBattleStateDefeat
			}
			return dungeonBattleStateVictory
		}
	}

	return huntingBattleStateTimeout
}

func updateActiveHuntingRunTx(
	ctx context.Context,
	tx pgx.Tx,
	userID uuid.UUID,
	state *huntingRunState,
) error {
	const query = `
		UPDATE player_hunting_runs
		SET
			is_active = TRUE,
			current_hp = $2,
			max_hp = $3,
			kill_count = $4,
			total_spirit_cost = $5,
			total_cultivation_gain = $6,
			last_state = $7,
			revive_until = $8,
			ended_at = NULL,
			last_log_seq = $9,
			last_log_message = $10,
			updated_at = $11,
			last_processed_at = $11,
			failure_count = 0,
			last_error = ''
		WHERE user_id = $1
	`
	var reviveUntil any
	if !state.ReviveUntil.IsZero() && state.ReviveUntil.Unix() > 0 {
		reviveUntil = state.ReviveUntil
	} else {
		reviveUntil = nil
	}
	updatedAt := state.RunUpdatedAt
	if updatedAt.IsZero() {
		updatedAt = time.Now()
	}
	if _, err := tx.Exec(
		ctx,
		query,
		userID,
		state.CurrentHP,
		state.MaxHP,
		state.KillCount,
		state.TotalSpiritCost,
		state.TotalCultivationGain,
		state.LastState,
		reviveUntil,
		state.LastLogSeq,
		state.LastLogMessage,
		updatedAt,
	); err != nil {
		return fmt.Errorf("update active hunting run by passive progress: %w", err)
	}
	return nil
}

func stopHuntingRunWithStateTx(
	ctx context.Context,
	tx pgx.Tx,
	userID uuid.UUID,
	state *huntingRunState,
	runState string,
	now time.Time,
) error {
	currentHP := state.CurrentHP
	if runState == huntingRunStateDefeat {
		currentHP = 0
	}
	updatedAt := state.RunUpdatedAt
	if updatedAt.IsZero() {
		updatedAt = now
	}

	const query = `
		UPDATE player_hunting_runs
		SET
			is_active = FALSE,
			current_hp = $2,
			max_hp = $3,
			kill_count = $4,
			total_spirit_cost = $5,
			total_cultivation_gain = $6,
			last_state = $7,
			revive_until = NULL,
			last_log_seq = $8,
			last_log_message = $9,
			ended_at = $10,
			updated_at = $11,
			last_processed_at = $11,
			failure_count = 0,
			last_error = ''
		WHERE user_id = $1
	`
	if _, err := tx.Exec(
		ctx,
		query,
		userID,
		currentHP,
		state.MaxHP,
		state.KillCount,
		state.TotalSpiritCost,
		state.TotalCultivationGain,
		runState,
		state.LastLogSeq,
		state.LastLogMessage,
		now,
		updatedAt,
	); err != nil {
		return fmt.Errorf("stop hunting run by passive progress: %w", err)
	}
	return nil
}

func incrementBreakthroughCountTx(ctx context.Context, tx pgx.Tx, userID uuid.UUID, count int64) error {
	if count <= 0 {
		return nil
	}

	const query = `
		INSERT INTO player_cultivation_stats (user_id, total_cultivation_time, breakthrough_count, updated_at)
		VALUES ($1, 0, $2, now())
		ON CONFLICT (user_id)
		DO UPDATE SET
			breakthrough_count = player_cultivation_stats.breakthrough_count + EXCLUDED.breakthrough_count,
			updated_at = now()
	`
	if _, err := tx.Exec(ctx, query, userID, count); err != nil {
		return fmt.Errorf("increment breakthrough count: %w", err)
	}
	return nil
}
