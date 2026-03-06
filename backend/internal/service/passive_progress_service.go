package service

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	// 后端自动推进刷图步长：1 秒结算一次，避免前端轮询依赖。
	passiveHuntingEncounterInterval = time.Second
)

type PassiveProgressService struct {
	pool           *pgxpool.Pool
	runtimeConfig  *RuntimeConfigService
	realtimeBroker *GameRealtimeBroker
}

func NewPassiveProgressService(pool *pgxpool.Pool, runtimeConfig *RuntimeConfigService, realtimeBroker *GameRealtimeBroker) *PassiveProgressService {
	return &PassiveProgressService{
		pool:           pool,
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

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit passive progress transaction: %w", err)
	}
	if s.realtimeBroker != nil {
		s.realtimeBroker.Publish(userID, GameRealtimeTopicAll)
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
			SELECT user_id, MAX(updated_at) AS updated_at
			FROM (
				SELECT user_id, updated_at
				FROM player_hunting_runs
				WHERE is_active = TRUE

				UNION ALL

				SELECT user_id, updated_at
				FROM player_meditation_runs
				WHERE is_active = TRUE
			) active_sources
			GROUP BY user_id
		) active_runs
		ORDER BY updated_at ASC
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
		if err := s.Advance(ctx, userID); err != nil {
			return processed, fmt.Errorf("advance active hunting run user %s: %w", userID, err)
		}
		processed++
	}
	return processed, nil
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
	player := buildHuntingPlayerEntity(state, effectBonus)
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

		mapBaseGain := fixedHuntingMapBaseGain(targetMap)
		spiritCost := fixedHuntingMapSpiritCost(targetMap)
		if state.Spirit < float64(spiritCost) {
			runActive = false
			runState = huntingRunStateExhausted
			state.LastState = huntingRunStateExhausted
			state.ReviveUntil = time.Time{}
			state.RunUpdatedAt = nextEncounterAt
			setHuntingRunLog(state, "灵力耗尽，刷怪暂停")
			break
		}

		state.Spirit -= float64(spiritCost)
		state.TotalSpiritCost += spiritCost
		stateDirty = true
		state.RunUpdatedAt = nextEncounterAt

		enemy, tier := generateHuntingEnemy(state.Level, state.KillCount, targetMap, rng)
		battleState := runHuntingAutoBattle(player, enemy, rng)
		if battleState == huntingBattleStateTimeout {
			state.CurrentHP = player.CurrentHealth
			state.MaxHP = player.Stats.MaxHealth
			state.LastState = huntingRunStateRunning
			state.ReviveUntil = time.Time{}
			setHuntingRunLog(state, fmt.Sprintf("与%s缠斗未分胜负，继续周旋", enemy.Name))
			continue
		}
		if battleState != dungeonBattleStateVictory {
			player.CurrentHealth = 0
			state.CurrentHP = 0
			state.MaxHP = player.Stats.MaxHealth
			state.LastState = huntingRunStateReviving
			reviveDuration := resolveHuntingReviveDuration(
				huntingReviveDuration(state.Level, targetMap),
				reviveMultiplier,
			)
			state.ReviveUntil = nextEncounterAt.Add(reviveDuration)
			reviveSeconds := int64(reviveDuration / time.Second)
			if reviveSeconds < 1 {
				reviveSeconds = 1
			}
			setHuntingRunLog(state, fmt.Sprintf("你已战死，%d秒后自动复活", reviveSeconds))
			continue
		}

		cultivationGain := int64(math.Ceil(float64(mapBaseGain) * huntingGainMultiplier * tier.GainMultiplier))
		if cultivationGain <= 0 {
			cultivationGain = 1
		}

		if shouldDoubleGain(state.Luck) {
			cultivationGain *= 2
		}

		effectiveCultivationRate := state.CultivationRate * (1 + effectBonus.CultivationRateBonus)
		cultivationGain = applyCultivationRate(cultivationGain, effectiveCultivationRate)

		state.Cultivation += cultivationGain
		state.TotalCultivationGain += cultivationGain
		state.KillCount += 1
		cultivationTimes += 1

		if state.Cultivation >= state.MaxCultivation {
			cultivationStateView := buildCultivationStateFromHunting(state)
			if applyBreakthrough(cultivationStateView) {
				breakthroughTimes += 1
			}
			applyCultivationStateToHunting(state, cultivationStateView)
		}

		totalRecoverRate := resolveHuntingHealRate(healBaseRate, healCapRate, effectBonus.AutoHealRate)
		if totalRecoverRate > 0 {
			dungeonHeal(player, player.Stats.MaxHealth*totalRecoverRate)
		}

		spiritRefund := rollHuntingSpiritRefund(spiritCost, refundChance, refundMinRatio, refundMaxRatio, rng)
		if spiritRefund > 0 {
			state.Spirit += float64(spiritRefund)
		}

		if dropped, ok := maybeHuntingDropEquipment(state.Level, targetMap, tier, rng); ok {
			items = append(items, dropped)
			itemsChanged = true
		}
		herbDropped := herbItem{}
		hasHerbDrop := false
		if droppedHerb, ok := maybeHuntingDropHerb(targetMap, tier, rng); ok {
			herbs = append(herbs, droppedHerb)
			herbsChanged = true
			herbDropped = droppedHerb
			hasHerbDrop = true
		}

		state.CurrentHP = player.CurrentHealth
		state.MaxHP = player.Stats.MaxHealth
		state.LastState = huntingRunStateRunning
		state.ReviveUntil = time.Time{}
		logMessage := fmt.Sprintf("你击杀了%s，获得%d修为", enemy.Name, cultivationGain)
		if spiritRefund > 0 {
			logMessage = fmt.Sprintf("%s，返还%d灵力", logMessage, spiritRefund)
		}
		if hasHerbDrop {
			logMessage = fmt.Sprintf("%s，并采得灵草%s", logMessage, herbDropped.Name)
		}
		setHuntingRunLog(state, logMessage)
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
	state.CurrentHP = player.CurrentHealth
	state.MaxHP = player.Stats.MaxHealth
	if runActive {
		return updateActiveHuntingRunTx(ctx, tx, userID, state)
	}

	return stopHuntingRunWithStateTx(ctx, tx, userID, state, runState, now)
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
			updated_at = $11
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
			updated_at = $11
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
