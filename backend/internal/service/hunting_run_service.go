package service

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/kowming/vue-idle-xiuxian/backend/internal/repository"
)

const (
	huntingRunStateRunning    = "running"
	huntingRunStateReviving   = "reviving"
	huntingRunStateDefeat     = "defeat"
	huntingRunStateExhausted  = "exhausted"
	huntingRunStateStopped    = "stopped"
	huntingRunStateOffline    = "offline_timeout"
	huntingBattleStateTimeout = "timeout"

	// 刷图定位为日常主升级玩法：单位灵力收益默认约为打坐的 2 倍。
	defaultHuntingWinGainMultiplier = 2.0
)

type HuntingRunStatusResult struct {
	IsActive             bool    `json:"isActive"`
	State                string  `json:"state"`
	MapID                string  `json:"mapId"`
	MapName              string  `json:"mapName"`
	CurrentHP            float64 `json:"currentHp"`
	MaxHP                float64 `json:"maxHp"`
	KillCount            int64   `json:"killCount"`
	TotalSpiritCost      int64   `json:"totalSpiritCost"`
	TotalCultivationGain int64   `json:"totalCultivationGain"`
	ProgressPercent      float64 `json:"progressPercent"`
	ProgressLabel        string  `json:"progressLabel"`
	ProgressRemainingMs  int64   `json:"progressRemainingMs"`
	LastLogSeq           int64   `json:"lastLogSeq"`
	LastLogMessage       string  `json:"lastLogMessage"`
	ReviveUntil          int64   `json:"reviveUntil,omitempty"`
	MinLevel             int     `json:"minLevel,omitempty"`
	RewardFactor         float64 `json:"rewardFactor,omitempty"`
}

type HuntingRunStartResult struct {
	Message  string                     `json:"message"`
	State    string                     `json:"state"`
	Run      *HuntingRunStatusResult    `json:"run"`
	Snapshot *repository.PlayerSnapshot `json:"snapshot"`
}

type HuntingDroppedEquipment struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Type    string `json:"type"`
	Quality string `json:"quality"`
}

type HuntingRunTickResult struct {
	Message           string                     `json:"message"`
	State             string                     `json:"state"`
	MapID             string                     `json:"mapId"`
	MapName           string                     `json:"mapName"`
	MonsterName       string                     `json:"monsterName"`
	EnemyTier         string                     `json:"enemyTier"`
	SpiritCost        int64                      `json:"spiritCost"`
	SpiritRefund      int64                      `json:"spiritRefund"`
	CultivationGain   int64                      `json:"cultivationGain"`
	DoubleGain        bool                       `json:"doubleGain"`
	DoubleGainTimes   int                        `json:"doubleGainTimes"`
	Breakthrough      bool                       `json:"breakthrough"`
	DroppedEquipments []HuntingDroppedEquipment  `json:"droppedEquipments,omitempty"`
	Logs              []string                   `json:"logs"`
	Run               *HuntingRunStatusResult    `json:"run"`
	Snapshot          *repository.PlayerSnapshot `json:"snapshot"`
}

type HuntingRunStopResult struct {
	Message  string                     `json:"message"`
	State    string                     `json:"state"`
	Run      *HuntingRunStatusResult    `json:"run"`
	Snapshot *repository.PlayerSnapshot `json:"snapshot"`
}

type HuntingRunNotActiveError struct{}

func (e *HuntingRunNotActiveError) Error() string {
	return "hunting run not active"
}

type huntingRunState struct {
	RunActive            bool
	MapID                string
	MapName              string
	CurrentHP            float64
	MaxHP                float64
	KillCount            int64
	TotalSpiritCost      int64
	TotalCultivationGain int64
	LastState            string
	RunUpdatedAt         time.Time
	ReviveUntil          time.Time
	LastLogSeq           int64
	LastLogMessage       string

	Level           int
	Realm           string
	Cultivation     int64
	MaxCultivation  int64
	Spirit          float64
	SpiritRate      float64
	Luck            float64
	CultivationRate float64

	BaseAttributesRaw   []byte
	CombatAttributesRaw []byte
	CombatResistRaw     []byte
	SpecialAttrsRaw     []byte
	HerbsRaw            []byte
	ItemsRaw            []byte
	ActiveEffectsRaw    []byte
}

type huntingEffectBonus struct {
	CultivationRateBonus float64
	CombatBoostBonus     float64
	AllAttributesBonus   float64
	AutoHealRate         float64
}

type huntingEnemyTier struct {
	ID             string
	DisplayName    string
	HealthMult     float64
	DamageMult     float64
	DefenseMult    float64
	SpeedMult      float64
	GainMultiplier float64
	DropMultiplier float64
}

func (s *GameService) HuntingStatus(ctx context.Context, userID uuid.UUID) (*HuntingRunStatusResult, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin hunting status transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if err := ensureHuntingRunRow(ctx, tx, userID); err != nil {
		return nil, err
	}

	state, err := loadHuntingRunStateForUpdate(ctx, tx, userID)
	if err != nil {
		return nil, err
	}
	status := buildHuntingRunStatus(state, time.Now())

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit hunting status transaction: %w", err)
	}
	return status, nil
}

func (s *GameService) HuntingStart(ctx context.Context, userID uuid.UUID, mapID string) (*HuntingRunStartResult, error) {
	mapID = strings.TrimSpace(mapID)
	targetMap, ok := findHuntingMapByID(mapID)
	if !ok {
		return nil, &InvalidHuntingMapError{MapID: mapID}
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin hunting start transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if err := ensureNoActiveDungeonRunTx(ctx, tx, userID); err != nil {
		return nil, err
	}

	if err := ensureHuntingRunRow(ctx, tx, userID); err != nil {
		return nil, err
	}

	state, err := loadHuntingRunStateForUpdate(ctx, tx, userID)
	if err != nil {
		return nil, err
	}
	if state.Level < targetMap.MinLevel {
		return nil, &HuntingMapLockedError{
			MapID:         targetMap.ID,
			RequiredLevel: targetMap.MinLevel,
			CurrentLevel:  state.Level,
		}
	}
	if err := ensureMeditationRunRow(ctx, tx, userID); err != nil {
		return nil, err
	}
	if err := stopMeditationForConflictTx(ctx, tx, userID, "进入刷怪地图，打坐已自动结束"); err != nil {
		return nil, err
	}

	nowMilli := time.Now().UnixMilli()
	activeEffects, bonus := huntingDecodeActiveEffects(state.ActiveEffectsRaw, nowMilli)
	player := buildHuntingPlayerEntity(state, bonus)

	const updateRunSQL = `
		UPDATE player_hunting_runs
		SET
			is_active = TRUE,
			map_id = $2,
			map_name = $3,
			current_hp = $4,
			max_hp = $5,
			kill_count = 0,
			total_spirit_cost = 0,
			total_cultivation_gain = 0,
			last_state = $6,
			revive_until = NULL,
			last_log_seq = 0,
			last_log_message = '',
			started_at = now(),
			ended_at = NULL,
			updated_at = now()
		WHERE user_id = $1
	`
	if _, err := tx.Exec(
		ctx,
		updateRunSQL,
		userID,
		targetMap.ID,
		targetMap.Name,
		player.CurrentHealth,
		player.Stats.MaxHealth,
		huntingRunStateRunning,
	); err != nil {
		return nil, fmt.Errorf("update hunting run on start: %w", err)
	}

	if err := updateHuntingActiveEffectsTx(ctx, tx, userID, activeEffects); err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit hunting start transaction: %w", err)
	}
	s.notifyRealtime(userID, GameRealtimeTopicHunting, GameRealtimeTopicMeditation, GameRealtimeTopicSnapshot)

	snapshot, err := s.userRepo.GetSnapshot(ctx, userID)
	if err != nil {
		return nil, err
	}

	return &HuntingRunStartResult{
		Message: fmt.Sprintf("已进入%s，开始刷怪历练", targetMap.Name),
		State:   huntingRunStateRunning,
		Run: &HuntingRunStatusResult{
			IsActive:             true,
			State:                huntingRunStateRunning,
			MapID:                targetMap.ID,
			MapName:              targetMap.Name,
			CurrentHP:            player.CurrentHealth,
			MaxHP:                player.Stats.MaxHealth,
			KillCount:            0,
			TotalSpiritCost:      0,
			TotalCultivationGain: 0,
			ProgressPercent:      0,
			ProgressLabel:        "击杀进度",
			ProgressRemainingMs:  int64(passiveHuntingEncounterInterval / time.Millisecond),
			LastLogSeq:           0,
			LastLogMessage:       "",
			MinLevel:             targetMap.MinLevel,
			RewardFactor:         targetMap.RewardFactor,
		},
		Snapshot: snapshot,
	}, nil
}

func (s *GameService) HuntingTick(ctx context.Context, userID uuid.UUID) (*HuntingRunTickResult, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin hunting tick transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if err := ensureHuntingRunRow(ctx, tx, userID); err != nil {
		return nil, err
	}

	state, err := loadHuntingRunStateForUpdate(ctx, tx, userID)
	if err != nil {
		return nil, err
	}
	if !state.RunActive {
		return nil, &HuntingRunNotActiveError{}
	}

	targetMap, ok := findHuntingMapByID(state.MapID)
	if !ok {
		return nil, &InvalidHuntingMapError{MapID: state.MapID}
	}
	huntingGainMultiplier := s.getHuntingWinGainMultiplier(ctx)
	refundChance, refundMinRatio, refundMaxRatio := s.getHuntingSpiritRefundConfig(ctx)

	now := time.Now()
	nowMilli := now.UnixMilli()
	activeEffects, effectBonus := huntingDecodeActiveEffects(state.ActiveEffectsRaw, nowMilli)

	if state.LastState == huntingRunStateReviving && !state.ReviveUntil.IsZero() && state.ReviveUntil.Unix() > 0 {
		if now.Before(state.ReviveUntil) {
			remainingSeconds := int64(math.Ceil(state.ReviveUntil.Sub(now).Seconds()))
			if remainingSeconds < 1 {
				remainingSeconds = 1
			}
			if err := updateHuntingActiveEffectsTx(ctx, tx, userID, activeEffects); err != nil {
				return nil, err
			}
			if err := tx.Commit(ctx); err != nil {
				return nil, fmt.Errorf("commit hunting reviving transaction: %w", err)
			}
			s.notifyRealtime(userID, GameRealtimeTopicHunting, GameRealtimeTopicSnapshot)
			snapshot, err := s.userRepo.GetSnapshot(ctx, userID)
			if err != nil {
				return nil, err
			}
			runStatus := buildHuntingRunStatus(state, now)
			message := fmt.Sprintf("你已战死，%d秒后自动复活", remainingSeconds)
			return &HuntingRunTickResult{
				Message:    message,
				State:      huntingRunStateReviving,
				MapID:      targetMap.ID,
				MapName:    targetMap.Name,
				SpiritCost: 0,
				Logs:       []string{message},
				Run:        runStatus,
				Snapshot:   snapshot,
			}, nil
		}

		player := buildHuntingPlayerEntity(state, effectBonus)
		state.CurrentHP = player.Stats.MaxHealth
		state.MaxHP = player.Stats.MaxHealth
		state.LastState = huntingRunStateRunning
		state.ReviveUntil = time.Time{}
		state.RunUpdatedAt = now
		setHuntingRunLog(state, "你已复活，继续战斗")

		const reviveSQL = `
			UPDATE player_hunting_runs
			SET
				is_active = TRUE,
				current_hp = $2,
				max_hp = $3,
				last_state = $4,
				revive_until = NULL,
				last_log_seq = $5,
				last_log_message = $6,
				ended_at = NULL,
				updated_at = now()
			WHERE user_id = $1
		`
		if _, err := tx.Exec(
			ctx,
			reviveSQL,
			userID,
			state.CurrentHP,
			state.MaxHP,
			state.LastState,
			state.LastLogSeq,
			state.LastLogMessage,
		); err != nil {
			return nil, fmt.Errorf("update hunting run on revive: %w", err)
		}
		if err := updateHuntingActiveEffectsTx(ctx, tx, userID, activeEffects); err != nil {
			return nil, err
		}
		if err := tx.Commit(ctx); err != nil {
			return nil, fmt.Errorf("commit hunting revive transaction: %w", err)
		}
		s.notifyRealtime(userID, GameRealtimeTopicHunting, GameRealtimeTopicSnapshot)
		snapshot, err := s.userRepo.GetSnapshot(ctx, userID)
		if err != nil {
			return nil, err
		}
		runStatus := buildHuntingRunStatus(state, now)
		return &HuntingRunTickResult{
			Message:    state.LastLogMessage,
			State:      huntingRunStateRunning,
			MapID:      targetMap.ID,
			MapName:    targetMap.Name,
			SpiritCost: 0,
			Logs:       []string{state.LastLogMessage},
			Run:        runStatus,
			Snapshot:   snapshot,
		}, nil
	}

	mapBaseGain := fixedHuntingMapBaseGain(targetMap)
	spiritCost := fixedHuntingMapSpiritCost(targetMap)

	if state.Spirit < float64(spiritCost) {
		state.RunActive = false
		state.LastState = huntingRunStateExhausted
		state.ReviveUntil = time.Time{}
		state.RunUpdatedAt = now
		setHuntingRunLog(state, "灵力耗尽，刷怪暂停")

		const stopRunSQL = `
			UPDATE player_hunting_runs
			SET
				is_active = $2,
				current_hp = $3,
				max_hp = $4,
				kill_count = $5,
				total_spirit_cost = $6,
					total_cultivation_gain = $7,
					last_state = $8,
					revive_until = NULL,
					last_log_seq = $9,
					last_log_message = $10,
					ended_at = now(),
					updated_at = now()
				WHERE user_id = $1
			`
		if _, err := tx.Exec(
			ctx,
			stopRunSQL,
			userID,
			false,
			state.CurrentHP,
			state.MaxHP,
			state.KillCount,
			state.TotalSpiritCost,
			state.TotalCultivationGain,
			state.LastState,
			state.LastLogSeq,
			state.LastLogMessage,
		); err != nil {
			return nil, fmt.Errorf("stop hunting run by spirit exhaustion: %w", err)
		}
		if err := updateHuntingActiveEffectsTx(ctx, tx, userID, activeEffects); err != nil {
			return nil, err
		}
		if err := tx.Commit(ctx); err != nil {
			return nil, fmt.Errorf("commit hunting exhaustion transaction: %w", err)
		}
		s.notifyRealtime(userID, GameRealtimeTopicHunting, GameRealtimeTopicSnapshot)

		snapshot, err := s.userRepo.GetSnapshot(ctx, userID)
		if err != nil {
			return nil, err
		}
		runStatus := buildHuntingRunStatus(state, now)
		return &HuntingRunTickResult{
			Message:    state.LastLogMessage,
			State:      huntingRunStateExhausted,
			MapID:      targetMap.ID,
			MapName:    targetMap.Name,
			SpiritCost: 0,
			Logs:       []string{state.LastLogMessage},
			Run:        runStatus,
			Snapshot:   snapshot,
		}, nil
	}

	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	player := buildHuntingPlayerEntity(state, effectBonus)
	enemy, tier := generateHuntingEnemy(state.Level, state.KillCount, targetMap, rng)

	state.Spirit -= float64(spiritCost)
	battleState := runHuntingAutoBattle(player, enemy, rng)

	result := &HuntingRunTickResult{
		State:       huntingRunStateRunning,
		MapID:       targetMap.ID,
		MapName:     targetMap.Name,
		MonsterName: enemy.Name,
		EnemyTier:   tier.DisplayName,
		SpiritCost:  spiritCost,
		Logs:        []string{},
	}

	droppedEquipments := make([]HuntingDroppedEquipment, 0, 1)
	breakthrough := false
	cultivationGain := int64(0)
	doubleGain := false
	doubleGainTimes := 0
	spiritRefund := int64(0)

	if battleState == dungeonBattleStateVictory {
		cultivationGain = int64(math.Ceil(float64(mapBaseGain) * huntingGainMultiplier * tier.GainMultiplier))
		if cultivationGain <= 0 {
			cultivationGain = 1
		}

		if shouldDoubleGain(state.Luck) {
			doubleGain = true
			doubleGainTimes = 1
			cultivationGain *= 2
		}

		effectiveCultivationRate := state.CultivationRate * (1 + effectBonus.CultivationRateBonus)
		cultivationGain = applyCultivationRate(cultivationGain, effectiveCultivationRate)

		state.Cultivation += cultivationGain
		if state.Cultivation >= state.MaxCultivation {
			cultivationStateView := buildCultivationStateFromHunting(state)
			breakthrough = applyBreakthrough(cultivationStateView)
			applyCultivationStateToHunting(state, cultivationStateView)
		}

		// 胜利后恢复一定生命，保证刷图可持续性。
		baseRecoverRate, capRecoverRate := s.getHuntingAutoHealRates(ctx)
		totalRecoverRate := resolveHuntingHealRate(baseRecoverRate, capRecoverRate, effectBonus.AutoHealRate)
		if totalRecoverRate > 0 {
			dungeonHeal(player, player.Stats.MaxHealth*totalRecoverRate)
		}

		spiritRefund = rollHuntingSpiritRefund(spiritCost, refundChance, refundMinRatio, refundMaxRatio, rng)
		if spiritRefund > 0 {
			state.Spirit += float64(spiritRefund)
		}

		items, err := huntingDecodeItems(state.ItemsRaw)
		if err != nil {
			return nil, err
		}
		herbs, err := huntingDecodeHerbs(state.HerbsRaw)
		if err != nil {
			return nil, err
		}
		herbDropped := herbItem{}
		hasHerbDrop := false

		if dropped, ok := maybeHuntingDropEquipment(state.Level, targetMap, tier, rng); ok {
			items = append(items, dropped)
			droppedEquipments = append(droppedEquipments, huntingBuildDroppedEquipmentView(dropped))
		}
		if droppedHerb, ok := maybeHuntingDropHerb(targetMap, tier, rng); ok {
			herbs = append(herbs, droppedHerb)
			herbDropped = droppedHerb
			hasHerbDrop = true
		}

		if err := persistCultivationState(ctx, tx, userID, &cultivationState{
			Level:           state.Level,
			Realm:           state.Realm,
			Cultivation:     state.Cultivation,
			MaxCultivation:  state.MaxCultivation,
			Spirit:          state.Spirit,
			SpiritRate:      state.SpiritRate,
			Luck:            state.Luck,
			CultivationRate: state.CultivationRate,
		}, 1, breakthrough); err != nil {
			return nil, err
		}

		if err := updateHuntingInventoryTx(ctx, tx, userID, items, herbs, activeEffects); err != nil {
			return nil, err
		}

		nextKillCount := state.KillCount + 1
		nextTotalSpirit := state.TotalSpiritCost + spiritCost
		nextTotalCultivation := state.TotalCultivationGain + cultivationGain
		state.KillCount = nextKillCount
		state.TotalSpiritCost = nextTotalSpirit
		state.TotalCultivationGain = nextTotalCultivation
		state.CurrentHP = player.CurrentHealth
		state.MaxHP = player.Stats.MaxHealth
		state.LastState = huntingRunStateRunning
		state.ReviveUntil = time.Time{}
		state.RunUpdatedAt = now
		logMessage := fmt.Sprintf("你击杀了%s，获得%d修为", enemy.Name, cultivationGain)
		if spiritRefund > 0 {
			logMessage = fmt.Sprintf("%s，返还%d灵力", logMessage, spiritRefund)
		}
		if hasHerbDrop {
			logMessage = fmt.Sprintf("%s，并采得灵草%s", logMessage, herbDropped.Name)
		}
		setHuntingRunLog(state, logMessage)

		const updateRunSQL = `
			UPDATE player_hunting_runs
			SET
				is_active = TRUE,
				current_hp = $2,
				max_hp = $3,
				kill_count = $4,
				total_spirit_cost = $5,
				total_cultivation_gain = $6,
				last_state = $7,
				revive_until = NULL,
				last_log_seq = $8,
				last_log_message = $9,
				updated_at = now()
			WHERE user_id = $1
		`
		if _, err := tx.Exec(
			ctx,
			updateRunSQL,
			userID,
			state.CurrentHP,
			state.MaxHP,
			nextKillCount,
			nextTotalSpirit,
			nextTotalCultivation,
			state.LastState,
			state.LastLogSeq,
			state.LastLogMessage,
		); err != nil {
			return nil, fmt.Errorf("update hunting run on victory: %w", err)
		}

		result.Message = state.LastLogMessage
		result.State = huntingRunStateRunning
		result.CultivationGain = cultivationGain
		result.SpiritRefund = spiritRefund
		result.DoubleGain = doubleGain
		result.DoubleGainTimes = doubleGainTimes
		result.Breakthrough = breakthrough
		result.DroppedEquipments = droppedEquipments
		result.Logs = []string{state.LastLogMessage}
		result.Run = buildHuntingRunStatus(state, now)
	} else if battleState == huntingBattleStateTimeout {
		if err := persistCultivationState(ctx, tx, userID, &cultivationState{
			Level:           state.Level,
			Realm:           state.Realm,
			Cultivation:     state.Cultivation,
			MaxCultivation:  state.MaxCultivation,
			Spirit:          state.Spirit,
			SpiritRate:      state.SpiritRate,
			Luck:            state.Luck,
			CultivationRate: state.CultivationRate,
		}, 0, false); err != nil {
			return nil, err
		}
		if err := updateHuntingActiveEffectsTx(ctx, tx, userID, activeEffects); err != nil {
			return nil, err
		}

		nextTotalSpirit := state.TotalSpiritCost + spiritCost
		state.TotalSpiritCost = nextTotalSpirit
		state.CurrentHP = player.CurrentHealth
		state.MaxHP = player.Stats.MaxHealth
		state.LastState = huntingRunStateRunning
		state.ReviveUntil = time.Time{}
		state.RunUpdatedAt = now
		setHuntingRunLog(state, fmt.Sprintf("与%s缠斗未分胜负，继续周旋", enemy.Name))

		const updateRunSQL = `
			UPDATE player_hunting_runs
			SET
				is_active = TRUE,
				current_hp = $2,
				max_hp = $3,
				total_spirit_cost = $4,
				last_state = $5,
				revive_until = NULL,
				last_log_seq = $6,
				last_log_message = $7,
				ended_at = NULL,
				updated_at = now()
			WHERE user_id = $1
		`
		if _, err := tx.Exec(
			ctx,
			updateRunSQL,
			userID,
			state.CurrentHP,
			state.MaxHP,
			nextTotalSpirit,
			state.LastState,
			state.LastLogSeq,
			state.LastLogMessage,
		); err != nil {
			return nil, fmt.Errorf("update hunting run on timeout: %w", err)
		}

		result.Message = state.LastLogMessage
		result.State = huntingRunStateRunning
		result.CultivationGain = 0
		result.Logs = []string{state.LastLogMessage}
		result.Run = buildHuntingRunStatus(state, now)
	} else {
		if err := persistCultivationState(ctx, tx, userID, &cultivationState{
			Level:           state.Level,
			Realm:           state.Realm,
			Cultivation:     state.Cultivation,
			MaxCultivation:  state.MaxCultivation,
			Spirit:          state.Spirit,
			SpiritRate:      state.SpiritRate,
			Luck:            state.Luck,
			CultivationRate: state.CultivationRate,
		}, 0, false); err != nil {
			return nil, err
		}
		if err := updateHuntingActiveEffectsTx(ctx, tx, userID, activeEffects); err != nil {
			return nil, err
		}

		nextTotalSpirit := state.TotalSpiritCost + spiritCost
		reviveDuration := resolveHuntingReviveDuration(
			huntingReviveDuration(state.Level, targetMap),
			s.getHuntingReviveMultiplier(ctx),
		)
		reviveSeconds := int64(reviveDuration / time.Second)
		if reviveSeconds < 1 {
			reviveSeconds = 1
		}
		state.TotalSpiritCost = nextTotalSpirit
		state.CurrentHP = 0
		state.MaxHP = player.Stats.MaxHealth
		state.LastState = huntingRunStateReviving
		state.ReviveUntil = now.Add(reviveDuration)
		state.RunUpdatedAt = now
		setHuntingRunLog(state, fmt.Sprintf("你已战死，%d秒后自动复活", reviveSeconds))

		const updateRunSQL = `
			UPDATE player_hunting_runs
			SET
				is_active = TRUE,
				current_hp = 0,
				max_hp = $2,
				total_spirit_cost = $3,
				last_state = $4,
				revive_until = $5,
				last_log_seq = $6,
				last_log_message = $7,
				ended_at = NULL,
				updated_at = now()
			WHERE user_id = $1
		`
		if _, err := tx.Exec(
			ctx,
			updateRunSQL,
			userID,
			state.MaxHP,
			nextTotalSpirit,
			state.LastState,
			state.ReviveUntil,
			state.LastLogSeq,
			state.LastLogMessage,
		); err != nil {
			return nil, fmt.Errorf("update hunting run on defeat: %w", err)
		}

		result.Message = state.LastLogMessage
		result.State = huntingRunStateReviving
		result.CultivationGain = 0
		result.Logs = []string{state.LastLogMessage}
		result.Run = buildHuntingRunStatus(state, now)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit hunting tick transaction: %w", err)
	}
	s.notifyRealtime(userID, GameRealtimeTopicHunting, GameRealtimeTopicSnapshot)

	snapshot, err := s.userRepo.GetSnapshot(ctx, userID)
	if err != nil {
		return nil, err
	}
	result.Snapshot = snapshot
	return result, nil
}

func (s *GameService) HuntingStop(ctx context.Context, userID uuid.UUID) (*HuntingRunStopResult, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin hunting stop transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if err := ensureHuntingRunRow(ctx, tx, userID); err != nil {
		return nil, err
	}

	state, err := loadHuntingRunStateForUpdate(ctx, tx, userID)
	if err != nil {
		return nil, err
	}

	const stopRunSQL = `
			UPDATE player_hunting_runs
			SET
				is_active = FALSE,
				last_state = $2,
				revive_until = NULL,
				ended_at = now(),
				updated_at = now()
			WHERE user_id = $1
		`
	if _, err := tx.Exec(ctx, stopRunSQL, userID, huntingRunStateStopped); err != nil {
		return nil, fmt.Errorf("stop hunting run: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit hunting stop transaction: %w", err)
	}
	s.notifyRealtime(userID, GameRealtimeTopicHunting, GameRealtimeTopicSnapshot)

	snapshot, err := s.userRepo.GetSnapshot(ctx, userID)
	if err != nil {
		return nil, err
	}

	return &HuntingRunStopResult{
		Message: "已退出刷怪地图",
		State:   huntingRunStateStopped,
		Run: &HuntingRunStatusResult{
			IsActive:             false,
			State:                huntingRunStateStopped,
			MapID:                state.MapID,
			MapName:              state.MapName,
			CurrentHP:            state.CurrentHP,
			MaxHP:                state.MaxHP,
			KillCount:            state.KillCount,
			TotalSpiritCost:      state.TotalSpiritCost,
			TotalCultivationGain: state.TotalCultivationGain,
			ProgressPercent:      0,
			ProgressLabel:        "",
			ProgressRemainingMs:  0,
			LastLogSeq:           state.LastLogSeq,
			LastLogMessage:       state.LastLogMessage,
		},
		Snapshot: snapshot,
	}, nil
}

func ensureHuntingRunRow(ctx context.Context, tx pgx.Tx, userID uuid.UUID) error {
	const query = `
		INSERT INTO player_hunting_runs (
			user_id, is_active, map_id, map_name, current_hp, max_hp, kill_count, total_spirit_cost, total_cultivation_gain, last_state
		)
		VALUES ($1, FALSE, '', '', 0, 0, 0, 0, 0, '')
		ON CONFLICT (user_id) DO NOTHING
	`
	if _, err := tx.Exec(ctx, query, userID); err != nil {
		return fmt.Errorf("ensure hunting run row: %w", err)
	}
	return nil
}

func loadHuntingRunStateForUpdate(ctx context.Context, tx pgx.Tx, userID uuid.UUID) (*huntingRunState, error) {
	const query = `
		SELECT
			phr.is_active,
			COALESCE(phr.map_id, ''),
			COALESCE(phr.map_name, ''),
			COALESCE(phr.current_hp, 0),
			COALESCE(phr.max_hp, 0),
			COALESCE(phr.kill_count, 0),
			COALESCE(phr.total_spirit_cost, 0),
			COALESCE(phr.total_cultivation_gain, 0),
			COALESCE(phr.last_state, ''),
			COALESCE(phr.revive_until, to_timestamp(0)),
			COALESCE(phr.last_log_seq, 0),
			COALESCE(phr.last_log_message, ''),
			phr.updated_at,
			pp.level,
			pp.realm,
			pp.cultivation,
			pp.max_cultivation,
			pr.spirit,
			pr.spirit_rate,
			pr.luck,
			pr.cultivation_rate,
			pa.base_attributes,
			pa.combat_attributes,
			pa.combat_resistance,
			pa.special_attributes,
			COALESCE(pis.herbs, '[]'::jsonb),
			COALESCE(pis.items, '[]'::jsonb),
			COALESCE(pis.active_effects, '[]'::jsonb)
		FROM player_hunting_runs phr
		JOIN player_profiles pp ON pp.user_id = phr.user_id
		JOIN player_resources pr ON pr.user_id = phr.user_id
		JOIN player_attributes pa ON pa.user_id = phr.user_id
		JOIN player_inventory_state pis ON pis.user_id = phr.user_id
		WHERE phr.user_id = $1
		FOR UPDATE OF phr, pp, pr, pa, pis
	`

	state := &huntingRunState{}
	if err := tx.QueryRow(ctx, query, userID).Scan(
		&state.RunActive,
		&state.MapID,
		&state.MapName,
		&state.CurrentHP,
		&state.MaxHP,
		&state.KillCount,
		&state.TotalSpiritCost,
		&state.TotalCultivationGain,
		&state.LastState,
		&state.ReviveUntil,
		&state.LastLogSeq,
		&state.LastLogMessage,
		&state.RunUpdatedAt,
		&state.Level,
		&state.Realm,
		&state.Cultivation,
		&state.MaxCultivation,
		&state.Spirit,
		&state.SpiritRate,
		&state.Luck,
		&state.CultivationRate,
		&state.BaseAttributesRaw,
		&state.CombatAttributesRaw,
		&state.CombatResistRaw,
		&state.SpecialAttrsRaw,
		&state.HerbsRaw,
		&state.ItemsRaw,
		&state.ActiveEffectsRaw,
	); err != nil {
		return nil, fmt.Errorf("load hunting run state: %w", err)
	}
	return state, nil
}

func buildHuntingRunStatus(state *huntingRunState, now time.Time) *HuntingRunStatusResult {
	status := &HuntingRunStatusResult{
		IsActive:             state.RunActive,
		State:                state.LastState,
		MapID:                state.MapID,
		MapName:              state.MapName,
		CurrentHP:            state.CurrentHP,
		MaxHP:                state.MaxHP,
		KillCount:            state.KillCount,
		TotalSpiritCost:      state.TotalSpiritCost,
		TotalCultivationGain: state.TotalCultivationGain,
		LastLogSeq:           state.LastLogSeq,
		LastLogMessage:       state.LastLogMessage,
	}
	if state.RunActive && status.State == "" {
		status.State = huntingRunStateRunning
	}
	if !state.ReviveUntil.IsZero() && state.ReviveUntil.Unix() > 0 {
		status.ReviveUntil = state.ReviveUntil.UnixMilli()
	}

	progressPercent, progressLabel, progressRemaining := calculateHuntingProgress(state, now)
	status.ProgressPercent = progressPercent
	status.ProgressLabel = progressLabel
	status.ProgressRemainingMs = progressRemaining

	if cfg, ok := findHuntingMapByID(state.MapID); ok {
		status.MinLevel = cfg.MinLevel
		status.RewardFactor = cfg.RewardFactor
	}
	return status
}

func calculateHuntingProgress(state *huntingRunState, now time.Time) (float64, string, int64) {
	if !state.RunActive {
		return 0, "", 0
	}

	if state.LastState == huntingRunStateReviving && !state.ReviveUntil.IsZero() && state.ReviveUntil.Unix() > 0 {
		totalMs := state.ReviveUntil.Sub(state.RunUpdatedAt).Milliseconds()
		if totalMs <= 0 {
			totalMs = int64((5 * time.Second) / time.Millisecond)
		}
		remainingMs := state.ReviveUntil.Sub(now).Milliseconds()
		if remainingMs < 0 {
			remainingMs = 0
		}
		elapsedMs := totalMs - remainingMs
		if elapsedMs < 0 {
			elapsedMs = 0
		}
		if elapsedMs > totalMs {
			elapsedMs = totalMs
		}
		return float64(elapsedMs) * 100 / float64(totalMs), "复活倒计时", remainingMs
	}

	durationMs := int64(passiveHuntingEncounterInterval / time.Millisecond)
	if durationMs <= 0 {
		durationMs = 1000
	}
	elapsedMs := now.Sub(state.RunUpdatedAt).Milliseconds()
	if elapsedMs < 0 {
		elapsedMs = 0
	}
	if elapsedMs > durationMs {
		elapsedMs = durationMs
	}
	remainingMs := durationMs - elapsedMs
	if remainingMs < 0 {
		remainingMs = 0
	}
	return float64(elapsedMs) * 100 / float64(durationMs), "击杀进度", remainingMs
}

func setHuntingRunLog(state *huntingRunState, message string) {
	text := strings.TrimSpace(message)
	if text == "" {
		return
	}
	state.LastLogSeq += 1
	state.LastLogMessage = text
}

func huntingReviveDuration(level int, mapCfg huntingMapConfig) time.Duration {
	baseSeconds := 6 + mapCfg.MinLevel/8
	if baseSeconds < 4 {
		baseSeconds = 4
	}
	levelAdvance := maxInt(0, level-mapCfg.MinLevel)
	reduceSeconds := levelAdvance / 15
	seconds := baseSeconds - reduceSeconds
	if seconds < 3 {
		seconds = 3
	}
	if seconds > 30 {
		seconds = 30
	}
	return time.Duration(seconds) * time.Second
}

func buildCultivationStateFromHunting(state *huntingRunState) *cultivationState {
	return &cultivationState{
		Level:           state.Level,
		Realm:           state.Realm,
		Cultivation:     state.Cultivation,
		MaxCultivation:  state.MaxCultivation,
		Spirit:          state.Spirit,
		SpiritRate:      state.SpiritRate,
		Luck:            state.Luck,
		CultivationRate: state.CultivationRate,
	}
}

func applyCultivationStateToHunting(target *huntingRunState, state *cultivationState) {
	target.Level = state.Level
	target.Realm = state.Realm
	target.Cultivation = state.Cultivation
	target.MaxCultivation = state.MaxCultivation
	target.Spirit = state.Spirit
	target.SpiritRate = state.SpiritRate
	target.Luck = state.Luck
	target.CultivationRate = state.CultivationRate
}

func buildHuntingPlayerEntity(state *huntingRunState, bonus huntingEffectBonus) *dungeonEntity {
	base := dungeonDecodeFloatMap(state.BaseAttributesRaw)
	combat := dungeonDecodeFloatMap(state.CombatAttributesRaw)
	resist := dungeonDecodeFloatMap(state.CombatResistRaw)
	special := dungeonDecodeFloatMap(state.SpecialAttrsRaw)

	stats := dungeonCombatStats{
		Health:      dungeonReadWithDefault(base, "health", 100),
		MaxHealth:   dungeonReadWithDefault(base, "health", 100),
		Damage:      dungeonReadWithDefault(base, "attack", 10),
		Defense:     dungeonReadWithDefault(base, "defense", 5),
		Speed:       dungeonReadWithDefault(base, "speed", 10),
		CritRate:    dungeonReadWithDefault(combat, "critRate", 0),
		ComboRate:   dungeonReadWithDefault(combat, "comboRate", 0),
		CounterRate: dungeonReadWithDefault(combat, "counterRate", 0),
		StunRate:    dungeonReadWithDefault(combat, "stunRate", 0),
		DodgeRate:   dungeonReadWithDefault(combat, "dodgeRate", 0),
		VampireRate: dungeonReadWithDefault(combat, "vampireRate", 0),

		CritResist:    dungeonReadWithDefault(resist, "critResist", 0),
		ComboResist:   dungeonReadWithDefault(resist, "comboResist", 0),
		CounterResist: dungeonReadWithDefault(resist, "counterResist", 0),
		StunResist:    dungeonReadWithDefault(resist, "stunResist", 0),
		DodgeResist:   dungeonReadWithDefault(resist, "dodgeResist", 0),
		VampireResist: dungeonReadWithDefault(resist, "vampireResist", 0),

		HealBoost:         dungeonReadWithDefault(special, "healBoost", 0),
		CritDamageBoost:   dungeonReadWithDefault(special, "critDamageBoost", 0),
		CritDamageReduce:  dungeonReadWithDefault(special, "critDamageReduce", 0),
		FinalDamageBoost:  dungeonReadWithDefault(special, "finalDamageBoost", 0),
		FinalDamageReduce: dungeonReadWithDefault(special, "finalDamageReduce", 0),
		CombatBoost:       dungeonReadWithDefault(special, "combatBoost", 0),
		ResistanceBoost:   dungeonReadWithDefault(special, "resistanceBoost", 0),
	}

	if bonus.CombatBoostBonus > 0 {
		stats.CombatBoost += bonus.CombatBoostBonus
	}
	if bonus.AllAttributesBonus > 0 {
		boost := 1 + bonus.AllAttributesBonus
		stats.Damage *= boost
		stats.Defense *= boost
		stats.Speed *= 1 + bonus.AllAttributesBonus*0.6
		stats.MaxHealth *= boost
		stats.Health = stats.MaxHealth
	}

	if stats.MaxHealth <= 0 {
		stats.MaxHealth = 1
	}

	currentHP := state.CurrentHP
	if !state.RunActive || currentHP <= 0 || currentHP > stats.MaxHealth {
		currentHP = stats.MaxHealth
	}
	if currentHP < 1 {
		currentHP = 1
	}

	return &dungeonEntity{
		Name:          "修士",
		Stats:         stats,
		CurrentHealth: currentHP,
	}
}

func huntingDecodeItems(raw []byte) ([]map[string]any, error) {
	items := make([]map[string]any, 0)
	if len(raw) == 0 {
		return items, nil
	}
	if err := json.Unmarshal(raw, &items); err != nil {
		return nil, fmt.Errorf("decode hunting inventory items: %w", err)
	}
	if items == nil {
		items = []map[string]any{}
	}
	return items, nil
}

func huntingDecodeHerbs(raw []byte) ([]herbItem, error) {
	herbs := make([]herbItem, 0)
	if len(raw) == 0 {
		return herbs, nil
	}
	if err := json.Unmarshal(raw, &herbs); err != nil {
		return nil, fmt.Errorf("decode hunting herbs: %w", err)
	}
	if herbs == nil {
		herbs = []herbItem{}
	}
	return herbs, nil
}

func huntingDecodeActiveEffects(raw []byte, nowMilli int64) ([]map[string]any, huntingEffectBonus) {
	effects := make([]map[string]any, 0)
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &effects)
	}
	if effects == nil {
		effects = []map[string]any{}
	}

	filtered := make([]map[string]any, 0, len(effects))
	bonus := huntingEffectBonus{}
	for _, effect := range effects {
		if inventoryReadInt64(effect["endTime"], 0) <= nowMilli {
			continue
		}
		filtered = append(filtered, effect)

		effectType := inventoryReadString(effect["type"])
		effectValue := inventoryReadFloat(effect["value"], 0)
		switch effectType {
		case "combatBoost":
			bonus.CombatBoostBonus += effectValue
		case "allAttributes":
			bonus.AllAttributesBonus += effectValue
		case "autoHeal":
			bonus.AutoHealRate += effectValue
		case "cultivationRate", "cultivationEfficiency", "comprehension":
			bonus.CultivationRateBonus += effectValue
		}
	}

	if bonus.CultivationRateBonus > 3 {
		bonus.CultivationRateBonus = 3
	}
	if bonus.CombatBoostBonus > 2 {
		bonus.CombatBoostBonus = 2
	}
	if bonus.AllAttributesBonus > 1.5 {
		bonus.AllAttributesBonus = 1.5
	}
	if bonus.AutoHealRate > 0.4 {
		bonus.AutoHealRate = 0.4
	}

	return filtered, bonus
}

func updateHuntingActiveEffectsTx(ctx context.Context, tx pgx.Tx, userID uuid.UUID, activeEffects []map[string]any) error {
	activeEffectsJSON, err := json.Marshal(activeEffects)
	if err != nil {
		return fmt.Errorf("marshal hunting active effects: %w", err)
	}
	const query = `
		UPDATE player_inventory_state
		SET active_effects = $2::jsonb, updated_at = now()
		WHERE user_id = $1
	`
	if _, err := tx.Exec(ctx, query, userID, string(activeEffectsJSON)); err != nil {
		return fmt.Errorf("update hunting active effects: %w", err)
	}
	return nil
}

func updateHuntingInventoryTx(
	ctx context.Context,
	tx pgx.Tx,
	userID uuid.UUID,
	items []map[string]any,
	herbs []herbItem,
	activeEffects []map[string]any,
) error {
	itemsJSON, err := json.Marshal(items)
	if err != nil {
		return fmt.Errorf("marshal hunting inventory items: %w", err)
	}
	herbsJSON, err := json.Marshal(herbs)
	if err != nil {
		return fmt.Errorf("marshal hunting inventory herbs: %w", err)
	}
	activeEffectsJSON, err := json.Marshal(activeEffects)
	if err != nil {
		return fmt.Errorf("marshal hunting inventory active effects: %w", err)
	}

	const query = `
		UPDATE player_inventory_state
		SET herbs = $2::jsonb, items = $3::jsonb, active_effects = $4::jsonb, updated_at = now()
		WHERE user_id = $1
	`
	if _, err := tx.Exec(ctx, query, userID, string(herbsJSON), string(itemsJSON), string(activeEffectsJSON)); err != nil {
		return fmt.Errorf("update hunting inventory state: %w", err)
	}
	return nil
}

func generateHuntingEnemy(level int, killCount int64, mapCfg huntingMapConfig, rng *rand.Rand) (*dungeonEntity, huntingEnemyTier) {
	effectiveKillCount := huntingDifficultyKillCount(mapCfg, killCount)
	tier := chooseHuntingEnemyTier(effectiveKillCount, rng)

	progressScale := 1 + math.Min(80, float64(effectiveKillCount))*0.015
	levelScale := 1 + float64(maxInt(0, level-1))*0.06
	mapScale := 0.9 + mapCfg.RewardFactor*0.25
	scale := progressScale * levelScale * mapScale

	health := (40 + float64(level)*15) * scale * tier.HealthMult
	damage := (5 + float64(level)*1.4) * scale * tier.DamageMult
	defense := (2 + float64(level)*0.8) * scale * tier.DefenseMult
	speed := (6 + float64(level)*0.9) * scale * tier.SpeedMult

	critRate := math.Min(0.45, (0.02+mapCfg.RewardFactor*0.008+float64(effectiveKillCount)*0.0007)*tier.DamageMult)
	dodgeRate := math.Min(0.35, (0.015+mapCfg.RewardFactor*0.006+float64(effectiveKillCount)*0.0005)*tier.SpeedMult)
	vampireRate := math.Min(0.25, (0.008+mapCfg.RewardFactor*0.004+float64(effectiveKillCount)*0.0004)*tier.DamageMult)

	monsterBaseName := "妖兽"
	if len(mapCfg.Monsters) > 0 {
		monsterBaseName = mapCfg.Monsters[rng.Intn(len(mapCfg.Monsters))]
	}
	monsterName := monsterBaseName
	if tier.ID == "elite" {
		monsterName = "精英·" + monsterBaseName
	}
	if tier.ID == "boss" {
		monsterName = "领主·" + monsterBaseName
	}

	enemy := &dungeonEntity{
		Name: monsterName,
		Stats: dungeonCombatStats{
			Health:            health,
			MaxHealth:         health,
			Damage:            damage,
			Defense:           defense,
			Speed:             speed,
			CritRate:          critRate,
			ComboRate:         math.Min(0.45, critRate*0.7),
			CounterRate:       math.Min(0.35, critRate*0.5),
			StunRate:          math.Min(0.3, critRate*0.4),
			DodgeRate:         dodgeRate,
			VampireRate:       vampireRate,
			CritResist:        math.Min(0.4, dodgeRate*0.8),
			ComboResist:       math.Min(0.35, dodgeRate*0.7),
			CounterResist:     math.Min(0.35, dodgeRate*0.6),
			StunResist:        math.Min(0.3, dodgeRate*0.5),
			DodgeResist:       math.Min(0.3, dodgeRate*0.5),
			VampireResist:     math.Min(0.25, vampireRate*0.5),
			HealBoost:         0,
			CritDamageBoost:   0.2 + mapCfg.RewardFactor*0.06,
			CritDamageReduce:  0.05 + mapCfg.RewardFactor*0.03,
			FinalDamageBoost:  0.03 + mapCfg.RewardFactor*0.02,
			FinalDamageReduce: 0.02 + mapCfg.RewardFactor*0.015,
			CombatBoost:       0,
			ResistanceBoost:   0,
		},
		CurrentHealth: health,
	}

	return enemy, tier
}

func huntingDifficultyKillCount(mapCfg huntingMapConfig, actualKillCount int64) int64 {
	if actualKillCount < 0 {
		actualKillCount = 0
	}
	// 前五张地图固定强度，不再随累计击杀持续抬高难度。
	if mapCfg.MinLevel <= 40 {
		fixed := int64(math.Round(float64(mapCfg.MinLevel)*0.5 + 2))
		if fixed < 2 {
			fixed = 2
		}
		return fixed
	}
	// 后四张图保持动态成长玩法。
	return actualKillCount
}

func chooseHuntingEnemyTier(killCount int64, rng *rand.Rand) huntingEnemyTier {
	eliteChance := 0.06 + math.Min(0.08, float64(killCount)*0.0008)
	bossChance := 0.005 + math.Min(0.03, float64(killCount)*0.0005)
	roll := rng.Float64()
	if roll < bossChance {
		return huntingEnemyTier{
			ID:             "boss",
			DisplayName:    "首领",
			HealthMult:     2.2,
			DamageMult:     1.8,
			DefenseMult:    1.6,
			SpeedMult:      1.25,
			GainMultiplier: 1.8,
			DropMultiplier: 8.0,
		}
	}
	if roll < bossChance+eliteChance {
		return huntingEnemyTier{
			ID:             "elite",
			DisplayName:    "精英",
			HealthMult:     1.45,
			DamageMult:     1.35,
			DefenseMult:    1.2,
			SpeedMult:      1.1,
			GainMultiplier: 1.35,
			DropMultiplier: 3.0,
		}
	}
	return huntingEnemyTier{
		ID:             "normal",
		DisplayName:    "普通",
		HealthMult:     1.0,
		DamageMult:     1.0,
		DefenseMult:    1.0,
		SpeedMult:      1.0,
		GainMultiplier: 1.0,
		DropMultiplier: 1.0,
	}
}

func maybeHuntingDropEquipment(level int, mapCfg huntingMapConfig, tier huntingEnemyTier, rng *rand.Rand) (map[string]any, bool) {
	dropChance := (0.002 + mapCfg.RewardFactor*0.0015) * tier.DropMultiplier
	if dropChance > 0.08 {
		dropChance = 0.08
	}
	if rng.Float64() >= dropChance {
		return nil, false
	}

	quality := rollHuntingEquipmentQuality(mapCfg, tier, rng)
	itemLevel := level + rng.Intn(5) - 2
	if tier.ID == "elite" {
		itemLevel += 1
	}
	if tier.ID == "boss" {
		itemLevel += 3
	}
	if itemLevel < 1 {
		itemLevel = 1
	}
	if itemLevel > realmCount() {
		itemLevel = realmCount()
	}
	return gachaGenerateEquipment(itemLevel, quality), true
}

func maybeHuntingDropHerb(mapCfg huntingMapConfig, tier huntingEnemyTier, rng *rand.Rand) (herbItem, bool) {
	dropChance := (0.0012 + mapCfg.RewardFactor*0.0008) * tier.DropMultiplier
	if dropChance > 0.03 {
		dropChance = 0.03
	}
	if rng.Float64() >= dropChance {
		return herbItem{}, false
	}
	herb := randomHerbItem()
	if herb.ID == "" {
		return herbItem{}, false
	}
	return herb, true
}

func rollHuntingEquipmentQuality(mapCfg huntingMapConfig, tier huntingEnemyTier, rng *rand.Rand) string {
	roll := rng.Float64()
	mapBonus := math.Max(0, mapCfg.RewardFactor-1)
	epicBoost := mapBonus * 0.03
	legendaryBoost := mapBonus * 0.015
	mythicBoost := mapBonus * 0.004

	switch tier.ID {
	case "boss":
		switch {
		case roll < 0.02+mythicBoost:
			return "mythic"
		case roll < 0.08+legendaryBoost:
			return "legendary"
		case roll < 0.24+epicBoost:
			return "epic"
		case roll < 0.50:
			return "rare"
		case roll < 0.78:
			return "uncommon"
		default:
			return "common"
		}
	case "elite":
		switch {
		case roll < 0.004+mythicBoost*0.5:
			return "mythic"
		case roll < 0.02+legendaryBoost*0.8:
			return "legendary"
		case roll < 0.08+epicBoost:
			return "epic"
		case roll < 0.28:
			return "rare"
		case roll < 0.60:
			return "uncommon"
		default:
			return "common"
		}
	default:
		switch {
		case roll < 0.001+mythicBoost*0.3:
			return "mythic"
		case roll < 0.006+legendaryBoost*0.4:
			return "legendary"
		case roll < 0.03+epicBoost*0.6:
			return "epic"
		case roll < 0.12:
			return "rare"
		case roll < 0.42:
			return "uncommon"
		default:
			return "common"
		}
	}
}

func huntingBuildDroppedEquipmentView(item map[string]any) HuntingDroppedEquipment {
	return HuntingDroppedEquipment{
		ID:      inventoryReadString(item["id"]),
		Name:    inventoryReadString(item["name"]),
		Type:    inventoryReadString(item["type"]),
		Quality: inventoryReadString(item["quality"]),
	}
}
