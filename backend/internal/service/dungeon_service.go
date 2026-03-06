package service

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/kowming/vue-idle-xiuxian/backend/internal/repository"
)

const (
	dungeonBattleStateVictory = "victory"
	dungeonBattleStateDefeat  = "defeat"
	dungeonBattleStateOption  = "option_required"
)

type DungeonService struct {
	pool     *pgxpool.Pool
	userRepo *repository.UserRepository
}

func NewDungeonService(pool *pgxpool.Pool, userRepo *repository.UserRepository) *DungeonService {
	return &DungeonService{pool: pool, userRepo: userRepo}
}

type DungeonStartResult struct {
	Message      string                     `json:"message"`
	State        string                     `json:"state,omitempty"`
	Difficulty   int                        `json:"difficulty"`
	CurrentFloor int                        `json:"currentFloor"`
	Floor        int                        `json:"floor,omitempty"`
	NeedsOption  bool                       `json:"needsOption,omitempty"`
	Options      []DungeonOption            `json:"options,omitempty"`
	RefreshCount int                        `json:"refreshCount,omitempty"`
	Snapshot     *repository.PlayerSnapshot `json:"snapshot"`
}

type DungeonTurnResult struct {
	Message                string                     `json:"message"`
	State                  string                     `json:"state"`
	Difficulty             int                        `json:"difficulty"`
	Floor                  int                        `json:"floor"`
	NeedsOption            bool                       `json:"needsOption,omitempty"`
	Options                []DungeonOption            `json:"options,omitempty"`
	RefreshCount           int                        `json:"refreshCount,omitempty"`
	RewardSpiritStones     int64                      `json:"rewardSpiritStones,omitempty"`
	RewardRefinementStones int64                      `json:"rewardRefinementStones,omitempty"`
	Logs                   []string                   `json:"logs"`
	Snapshot               *repository.PlayerSnapshot `json:"snapshot"`
}

type DungeonTurnInput struct {
	SelectedOptionID string
	RefreshOptions   bool
}

type DungeonOption struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Type        string `json:"type"`
}

type DungeonInvalidDifficultyError struct {
	Difficulty int
}

func (e *DungeonInvalidDifficultyError) Error() string {
	return fmt.Sprintf("invalid dungeon difficulty: %d", e.Difficulty)
}

type DungeonRunNotActiveError struct{}

func (e *DungeonRunNotActiveError) Error() string {
	return "dungeon run not active"
}

type DungeonInvalidOptionError struct {
	OptionID string
}

func (e *DungeonInvalidOptionError) Error() string {
	return fmt.Sprintf("invalid dungeon option: %s", e.OptionID)
}

type DungeonRefreshExhaustedError struct{}

func (e *DungeonRefreshExhaustedError) Error() string {
	return "dungeon refresh options exhausted"
}

func (s *DungeonService) Start(ctx context.Context, userID uuid.UUID, difficulty int) (*DungeonStartResult, error) {
	if !dungeonValidDifficulty(difficulty) {
		return nil, &DungeonInvalidDifficultyError{Difficulty: difficulty}
	}

	rng := rand.New(rand.NewSource(time.Now().UnixNano()))

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin dungeon start transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if err := lockDungeonRunTx(ctx, tx, userID); err != nil {
		return nil, err
	}
	if err := ensureHuntingRunRow(ctx, tx, userID); err != nil {
		return nil, err
	}
	if err := stopHuntingForConflictTx(ctx, tx, userID, "进入秘境，刷怪已自动结束"); err != nil {
		return nil, err
	}
	if err := ensureMeditationRunRow(ctx, tx, userID); err != nil {
		return nil, err
	}
	if err := stopMeditationForConflictTx(ctx, tx, userID, "进入秘境，打坐已自动结束"); err != nil {
		return nil, err
	}

	const progressQuery = `
		SELECT highest_floor, highest_floor_2x, highest_floor_5x, highest_floor_10x, highest_floor_100x
		FROM player_dungeon_progress
		WHERE user_id = $1
		FOR UPDATE
	`
	var highestFloor int
	var highestFloor2 int
	var highestFloor5 int
	var highestFloor10 int
	var highestFloor100 int
	if err := tx.QueryRow(ctx, progressQuery, userID).Scan(
		&highestFloor,
		&highestFloor2,
		&highestFloor5,
		&highestFloor10,
		&highestFloor100,
	); err != nil {
		return nil, fmt.Errorf("query dungeon progress for start: %w", err)
	}

	startFloor := dungeonHighestFloorByDifficulty(difficulty, highestFloor, highestFloor2, highestFloor5, highestFloor10, highestFloor100)
	targetFloor := startFloor + 1
	needsOption := dungeonNeedsOption(targetFloor)
	optionFloor := 0
	refreshCount := 0
	options := make([]DungeonOption, 0)
	optionIDsRaw := []byte("[]")
	if needsOption {
		optionFloor = targetFloor
		refreshCount = rng.Intn(3) + 1
		options = dungeonGenerateOptions(targetFloor, rng)
		optionIDsRaw, err = json.Marshal(dungeonOptionIDs(options))
		if err != nil {
			return nil, fmt.Errorf("marshal dungeon options on start: %w", err)
		}
	}

	const updateRunSQL = `
		UPDATE player_dungeon_runs
		SET
			is_active = TRUE,
			difficulty = $2,
			current_floor = $3,
			awaiting_option = $4,
			option_floor = $5,
			refresh_count = $6,
			current_options = $7,
			selected_buffs = '[]'::jsonb,
			updated_at = now()
		WHERE user_id = $1
	`
	if _, err := tx.Exec(
		ctx,
		updateRunSQL,
		userID,
		difficulty,
		startFloor,
		needsOption,
		optionFloor,
		refreshCount,
		optionIDsRaw,
	); err != nil {
		return nil, fmt.Errorf("update dungeon run on start: %w", err)
	}

	const updateProgressSQL = `
		UPDATE player_dungeon_progress
		SET total_runs = total_runs + 1, updated_at = now()
		WHERE user_id = $1
	`
	if _, err := tx.Exec(ctx, updateProgressSQL, userID); err != nil {
		return nil, fmt.Errorf("update dungeon runs count on start: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit dungeon start transaction: %w", err)
	}

	snapshot, err := s.userRepo.GetSnapshot(ctx, userID)
	if err != nil {
		return nil, err
	}

	result := &DungeonStartResult{
		Message:      "秘境探索开始",
		Difficulty:   difficulty,
		CurrentFloor: startFloor,
		Snapshot:     snapshot,
	}
	if needsOption {
		result.State = dungeonBattleStateOption
		result.NeedsOption = true
		result.Floor = targetFloor
		result.Options = options
		result.RefreshCount = refreshCount
		result.Message = fmt.Sprintf("第%d层可选择增益", targetFloor)
	}
	return result, nil
}

func (s *DungeonService) NextTurn(ctx context.Context, userID uuid.UUID, input DungeonTurnInput) (*DungeonTurnResult, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin dungeon turn transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if err := ensureDungeonRows(ctx, tx, userID); err != nil {
		return nil, err
	}

	state, err := loadDungeonState(ctx, tx, userID)
	if err != nil {
		return nil, err
	}
	if !state.RunActive {
		return nil, &DungeonRunNotActiveError{}
	}
	if !dungeonValidDifficulty(state.Difficulty) {
		return nil, &DungeonInvalidDifficultyError{Difficulty: state.Difficulty}
	}

	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	targetFloor := state.CurrentFloor + 1
	result := &DungeonTurnResult{
		State:      dungeonBattleStateDefeat,
		Difficulty: state.Difficulty,
		Floor:      targetFloor,
		Logs:       make([]string, 0, 16),
	}

	if state.AwaitingOption {
		targetFloor = state.OptionFloor
		result.Floor = targetFloor

		if input.RefreshOptions {
			if state.RefreshCount <= 0 {
				return nil, &DungeonRefreshExhaustedError{}
			}

			options := dungeonGenerateOptions(targetFloor, rng)
			optionIDsRaw, marshalErr := json.Marshal(dungeonOptionIDs(options))
			if marshalErr != nil {
				return nil, fmt.Errorf("marshal dungeon refreshed options: %w", marshalErr)
			}

			nextRefreshCount := state.RefreshCount - 1
			const refreshSQL = `
				UPDATE player_dungeon_runs
				SET refresh_count = $2, current_options = $3, updated_at = now()
				WHERE user_id = $1
			`
			if _, err := tx.Exec(ctx, refreshSQL, userID, nextRefreshCount, optionIDsRaw); err != nil {
				return nil, fmt.Errorf("refresh dungeon options: %w", err)
			}

			if err := tx.Commit(ctx); err != nil {
				return nil, fmt.Errorf("commit dungeon refresh options: %w", err)
			}

			snapshot, err := s.userRepo.GetSnapshot(ctx, userID)
			if err != nil {
				return nil, err
			}

			result.State = dungeonBattleStateOption
			result.NeedsOption = true
			result.Options = options
			result.RefreshCount = nextRefreshCount
			result.Message = fmt.Sprintf("第%d层增益已刷新", targetFloor)
			result.Snapshot = snapshot
			return result, nil
		}

		if input.SelectedOptionID == "" {
			result.State = dungeonBattleStateOption
			result.NeedsOption = true
			result.Options = dungeonOptionsFromIDs(state.CurrentOptionIDs)
			result.RefreshCount = state.RefreshCount
			result.Message = fmt.Sprintf("第%d层需先选择增益", targetFloor)
			return result, nil
		}

		if !dungeonOptionInList(input.SelectedOptionID, state.CurrentOptionIDs) {
			return nil, &DungeonInvalidOptionError{OptionID: input.SelectedOptionID}
		}

		selectedOption, ok := dungeonOptionDefinitionByID(input.SelectedOptionID)
		if !ok {
			return nil, &DungeonInvalidOptionError{OptionID: input.SelectedOptionID}
		}

		state.SelectedBuffIDs = dungeonAppendUniqueID(state.SelectedBuffIDs, input.SelectedOptionID)
		selectedBuffsRaw, marshalErr := json.Marshal(state.SelectedBuffIDs)
		if marshalErr != nil {
			return nil, fmt.Errorf("marshal selected dungeon buffs: %w", marshalErr)
		}

		const selectOptionSQL = `
			UPDATE player_dungeon_runs
			SET
				awaiting_option = FALSE,
				option_floor = 0,
				refresh_count = 0,
				current_options = '[]'::jsonb,
				selected_buffs = $2,
				updated_at = now()
			WHERE user_id = $1
		`
		if _, err := tx.Exec(ctx, selectOptionSQL, userID, selectedBuffsRaw); err != nil {
			return nil, fmt.Errorf("select dungeon option: %w", err)
		}
		result.Logs = append(result.Logs, fmt.Sprintf("选择增益：%s", selectedOption.Name))
	} else if dungeonNeedsOption(targetFloor) {
		refreshCount := rng.Intn(3) + 1
		options := dungeonGenerateOptions(targetFloor, rng)
		optionIDsRaw, marshalErr := json.Marshal(dungeonOptionIDs(options))
		if marshalErr != nil {
			return nil, fmt.Errorf("marshal dungeon options: %w", marshalErr)
		}

		const optionSQL = `
			UPDATE player_dungeon_runs
			SET
				awaiting_option = TRUE,
				option_floor = $2,
				refresh_count = $3,
				current_options = $4,
				updated_at = now()
			WHERE user_id = $1
		`
		if _, err := tx.Exec(ctx, optionSQL, userID, targetFloor, refreshCount, optionIDsRaw); err != nil {
			return nil, fmt.Errorf("prepare dungeon options: %w", err)
		}

		if err := tx.Commit(ctx); err != nil {
			return nil, fmt.Errorf("commit dungeon options: %w", err)
		}

		snapshot, err := s.userRepo.GetSnapshot(ctx, userID)
		if err != nil {
			return nil, err
		}

		result.State = dungeonBattleStateOption
		result.NeedsOption = true
		result.Options = options
		result.RefreshCount = refreshCount
		result.Message = fmt.Sprintf("第%d层可选择增益", targetFloor)
		result.Snapshot = snapshot
		return result, nil
	}

	player := buildDungeonPlayerEntity(state)
	enemy := generateDungeonEnemy(targetFloor, state.Difficulty)
	battleState, battleLogs := runDungeonBattle(player, enemy, rng)
	result.State = battleState
	result.Logs = append(result.Logs, battleLogs...)

	if battleState == dungeonBattleStateVictory {
		rewardSpiritStones := int64(10 * targetFloor * state.Difficulty)
		rewardRefinementStones := int64(0)
		bossKillsAdd := int64(0)
		eliteKillsAdd := int64(0)
		if targetFloor%10 == 0 {
			bossKillsAdd = 1
		} else if targetFloor%5 == 0 {
			eliteKillsAdd = 1
			rewardRefinementStones = int64(state.Difficulty)
		}

		const updateRunSQL = `
			UPDATE player_dungeon_runs
			SET
				is_active = TRUE,
				current_floor = $2,
				awaiting_option = FALSE,
				option_floor = 0,
				refresh_count = 0,
				current_options = '[]'::jsonb,
				updated_at = now()
			WHERE user_id = $1
		`
		if _, err := tx.Exec(ctx, updateRunSQL, userID, targetFloor); err != nil {
			return nil, fmt.Errorf("update dungeon run on victory: %w", err)
		}

		const updateResourcesSQL = `
			UPDATE player_resources
			SET
				spirit_stones = spirit_stones + $2,
				refinement_stones = refinement_stones + $3,
				updated_at = now()
			WHERE user_id = $1
		`
		if _, err := tx.Exec(ctx, updateResourcesSQL, userID, rewardSpiritStones, rewardRefinementStones); err != nil {
			return nil, fmt.Errorf("update dungeon rewards resources: %w", err)
		}

		const updateProgressSQL = `
			UPDATE player_dungeon_progress
			SET
				highest_floor = CASE WHEN $3 = 1 THEN GREATEST(highest_floor, $2) ELSE highest_floor END,
				highest_floor_2x = CASE WHEN $3 = 2 THEN GREATEST(highest_floor_2x, $2) ELSE highest_floor_2x END,
				highest_floor_5x = CASE WHEN $3 = 5 THEN GREATEST(highest_floor_5x, $2) ELSE highest_floor_5x END,
				highest_floor_10x = CASE WHEN $3 = 10 THEN GREATEST(highest_floor_10x, $2) ELSE highest_floor_10x END,
				highest_floor_100x = CASE WHEN $3 = 100 THEN GREATEST(highest_floor_100x, $2) ELSE highest_floor_100x END,
				total_kills = total_kills + 1,
				streak_kills = streak_kills + 1,
				boss_kills = boss_kills + $4,
				elite_kills = elite_kills + $5,
				total_rewards = total_rewards + 1,
				updated_at = now()
			WHERE user_id = $1
		`
		if _, err := tx.Exec(ctx, updateProgressSQL, userID, targetFloor, state.Difficulty, bossKillsAdd, eliteKillsAdd); err != nil {
			return nil, fmt.Errorf("update dungeon progress on victory: %w", err)
		}

		result.Message = fmt.Sprintf("击败了第%d层敌人", targetFloor)
		result.RewardSpiritStones = rewardSpiritStones
		result.RewardRefinementStones = rewardRefinementStones
		result.Logs = append(result.Logs, fmt.Sprintf("获得了%d灵石", rewardSpiritStones))
		if rewardRefinementStones > 0 {
			result.Logs = append(result.Logs, fmt.Sprintf("获得了%d颗洗练石", rewardRefinementStones))
		}
	} else {
		const stopRunSQL = `
			UPDATE player_dungeon_runs
			SET
				is_active = FALSE,
				current_floor = $2,
				awaiting_option = FALSE,
				option_floor = 0,
				refresh_count = 0,
				current_options = '[]'::jsonb,
				selected_buffs = '[]'::jsonb,
				updated_at = now()
			WHERE user_id = $1
		`
		if _, err := tx.Exec(ctx, stopRunSQL, userID, targetFloor); err != nil {
			return nil, fmt.Errorf("stop dungeon run on defeat: %w", err)
		}

		const updateProgressSQL = `
			UPDATE player_dungeon_progress
			SET death_count = death_count + 1, last_failed_floor = $2, streak_kills = 0, updated_at = now()
			WHERE user_id = $1
		`
		if _, err := tx.Exec(ctx, updateProgressSQL, userID, targetFloor); err != nil {
			return nil, fmt.Errorf("update dungeon progress on defeat: %w", err)
		}

		if state.Difficulty != 100 {
			lossRate := rng.Float64()*0.4 + 0.1
			cultivationLoss := int64(float64(state.Cultivation) * lossRate)
			nextCultivation := state.Cultivation - cultivationLoss
			if nextCultivation < 0 {
				nextCultivation = 0
			}

			const updateProfileSQL = `
				UPDATE player_profiles
				SET cultivation = $2, updated_at = now()
				WHERE user_id = $1
			`
			if _, err := tx.Exec(ctx, updateProfileSQL, userID, nextCultivation); err != nil {
				return nil, fmt.Errorf("update profile cultivation after dungeon defeat: %w", err)
			}
			result.Message = fmt.Sprintf("第%d层战斗失败，损失了%d点修为", targetFloor, cultivationLoss)
			result.Logs = append(result.Logs, fmt.Sprintf("战斗失败，损失%d点修为", cultivationLoss))
		} else {
			levelLoss := rng.Intn(3) + 1
			nextLevel := maxInt(1, state.Level-levelLoss)
			nextRealm := realmByLevel(nextLevel)

			const updateProfileSQL = `
				UPDATE player_profiles
				SET level = $2, realm = $3, cultivation = 0, max_cultivation = $4, updated_at = now()
				WHERE user_id = $1
			`
			if _, err := tx.Exec(ctx, updateProfileSQL, userID, nextLevel, nextRealm.Name, nextRealm.MaxCultivation); err != nil {
				return nil, fmt.Errorf("update profile level after dungeon defeat: %w", err)
			}
			result.Message = fmt.Sprintf("第%d层战斗失败，跌落了%d个境界", targetFloor, state.Level-nextLevel)
			result.Logs = append(result.Logs, fmt.Sprintf("战斗失败，境界降至%s", nextRealm.Name))
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit dungeon turn transaction: %w", err)
	}

	snapshot, err := s.userRepo.GetSnapshot(ctx, userID)
	if err != nil {
		return nil, err
	}
	result.Snapshot = snapshot
	return result, nil
}

type dungeonState struct {
	RunActive      bool
	Difficulty     int
	CurrentFloor   int
	AwaitingOption bool
	OptionFloor    int
	RefreshCount   int

	Level       int
	Cultivation int64

	BaseAttributesRaw   []byte
	CombatAttributesRaw []byte
	CombatResistRaw     []byte
	SpecialAttrsRaw     []byte

	CurrentOptionsRaw []byte
	SelectedBuffsRaw  []byte
	CurrentOptionIDs  []string
	SelectedBuffIDs   []string
}

func ensureDungeonRows(ctx context.Context, tx pgx.Tx, userID uuid.UUID) error {
	const progressSQL = `
		INSERT INTO player_dungeon_progress (
			user_id, highest_floor, highest_floor_2x, highest_floor_5x, highest_floor_10x, highest_floor_100x,
			last_failed_floor, total_runs, boss_kills, elite_kills, total_kills, death_count, total_rewards
		)
		VALUES ($1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0)
		ON CONFLICT (user_id) DO NOTHING
	`
	if _, err := tx.Exec(ctx, progressSQL, userID); err != nil {
		return fmt.Errorf("ensure dungeon progress row: %w", err)
	}

	const runSQL = `
		INSERT INTO player_dungeon_runs (user_id, is_active, difficulty, current_floor)
		VALUES ($1, FALSE, 1, 0)
		ON CONFLICT (user_id) DO NOTHING
	`
	if _, err := tx.Exec(ctx, runSQL, userID); err != nil {
		return fmt.Errorf("ensure dungeon run row: %w", err)
	}

	return nil
}

func loadDungeonState(ctx context.Context, tx pgx.Tx, userID uuid.UUID) (*dungeonState, error) {
	const query = `
		SELECT
			pdr.is_active,
			pdr.difficulty,
			pdr.current_floor,
			pdr.awaiting_option,
			pdr.option_floor,
			pdr.refresh_count,
			pp.level,
			pp.cultivation,
			pa.base_attributes,
			pa.combat_attributes,
			pa.combat_resistance,
			pa.special_attributes,
			COALESCE(pdr.current_options, '[]'::jsonb),
			COALESCE(pdr.selected_buffs, '[]'::jsonb)
		FROM player_dungeon_runs pdr
		JOIN player_profiles pp ON pp.user_id = pdr.user_id
		JOIN player_attributes pa ON pa.user_id = pdr.user_id
		JOIN player_dungeon_progress pdp ON pdp.user_id = pdr.user_id
		JOIN player_resources pr ON pr.user_id = pdr.user_id
		WHERE pdr.user_id = $1
		FOR UPDATE OF pdr, pp, pa, pdp, pr
	`

	state := &dungeonState{}
	if err := tx.QueryRow(ctx, query, userID).Scan(
		&state.RunActive,
		&state.Difficulty,
		&state.CurrentFloor,
		&state.AwaitingOption,
		&state.OptionFloor,
		&state.RefreshCount,
		&state.Level,
		&state.Cultivation,
		&state.BaseAttributesRaw,
		&state.CombatAttributesRaw,
		&state.CombatResistRaw,
		&state.SpecialAttrsRaw,
		&state.CurrentOptionsRaw,
		&state.SelectedBuffsRaw,
	); err != nil {
		return nil, fmt.Errorf("load dungeon state: %w", err)
	}
	state.CurrentOptionIDs = dungeonDecodeStringArray(state.CurrentOptionsRaw)
	state.SelectedBuffIDs = dungeonDecodeStringArray(state.SelectedBuffsRaw)
	return state, nil
}

type dungeonCombatStats struct {
	Health      float64
	MaxHealth   float64
	Damage      float64
	Defense     float64
	Speed       float64
	CritRate    float64
	ComboRate   float64
	CounterRate float64
	StunRate    float64
	DodgeRate   float64
	VampireRate float64

	CritResist    float64
	ComboResist   float64
	CounterResist float64
	StunResist    float64
	DodgeResist   float64
	VampireResist float64

	HealBoost         float64
	CritDamageBoost   float64
	CritDamageReduce  float64
	FinalDamageBoost  float64
	FinalDamageReduce float64
	CombatBoost       float64
	ResistanceBoost   float64
}

type dungeonEntity struct {
	Name          string
	Stats         dungeonCombatStats
	CurrentHealth float64
}

type dungeonAttackOutcome struct {
	Damage    float64
	IsCrit    bool
	IsCombo   bool
	IsVampire bool
	IsStun    bool
}

type dungeonDamageOutcome struct {
	Dodged        bool
	Damage        float64
	CurrentHealth float64
	IsDead        bool
	IsCounter     bool
}

func buildDungeonPlayerEntity(state *dungeonState) *dungeonEntity {
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
		CritRate:    dungeonReadWithDefault(combat, "critRate", 0.05),
		ComboRate:   dungeonReadWithDefault(combat, "comboRate", 0),
		CounterRate: dungeonReadWithDefault(combat, "counterRate", 0),
		StunRate:    dungeonReadWithDefault(combat, "stunRate", 0),
		DodgeRate:   dungeonReadWithDefault(combat, "dodgeRate", 0.05),
		VampireRate: dungeonReadWithDefault(combat, "vampireRate", 0),

		CritResist:    dungeonReadWithDefault(resist, "critResist", 0),
		ComboResist:   dungeonReadWithDefault(resist, "comboResist", 0),
		CounterResist: dungeonReadWithDefault(resist, "counterResist", 0),
		StunResist:    dungeonReadWithDefault(resist, "stunResist", 0),
		DodgeResist:   dungeonReadWithDefault(resist, "dodgeResist", 0),
		VampireResist: dungeonReadWithDefault(resist, "vampireResist", 0),

		HealBoost:         dungeonReadWithDefault(special, "healBoost", 0),
		CritDamageBoost:   dungeonReadWithDefault(special, "critDamageBoost", 0.5),
		CritDamageReduce:  dungeonReadWithDefault(special, "critDamageReduce", 0),
		FinalDamageBoost:  dungeonReadWithDefault(special, "finalDamageBoost", 0),
		FinalDamageReduce: dungeonReadWithDefault(special, "finalDamageReduce", 0),
		CombatBoost:       dungeonReadWithDefault(special, "combatBoost", 0),
		ResistanceBoost:   dungeonReadWithDefault(special, "resistanceBoost", 0),
	}

	dungeonApplyBuffs(&stats, state.SelectedBuffIDs)
	if stats.MaxHealth <= 0 {
		stats.MaxHealth = 1
	}
	if stats.Health <= 0 || stats.Health > stats.MaxHealth {
		stats.Health = stats.MaxHealth
	}

	return &dungeonEntity{
		Name:          "修士",
		Stats:         stats,
		CurrentHealth: stats.Health,
	}
}

func generateDungeonEnemy(floor int, difficulty int) *dungeonEntity {
	stats := dungeonCombatStats{
		Health:      100 + float64(difficulty*floor*200),
		MaxHealth:   100 + float64(difficulty*floor*200),
		Damage:      8 + float64(difficulty*floor*2),
		Defense:     3 + float64(difficulty*floor*2),
		Speed:       5 + float64(difficulty*floor*2),
		CritRate:    0.05 + float64(difficulty*floor)*0.02,
		ComboRate:   0.03 + float64(difficulty*floor)*0.02,
		CounterRate: 0.03 + float64(difficulty*floor)*0.02,
		StunRate:    0.02 + float64(difficulty*floor)*0.01,
		DodgeRate:   0.05 + float64(difficulty*floor)*0.02,
		VampireRate: 0.02 + float64(difficulty*floor)*0.01,

		CritResist:    0.02 + float64(difficulty*floor)*0.01,
		ComboResist:   0.02 + float64(difficulty*floor)*0.01,
		CounterResist: 0.02 + float64(difficulty*floor)*0.01,
		StunResist:    0.02 + float64(difficulty*floor)*0.01,
		DodgeResist:   0.02 + float64(difficulty*floor)*0.01,
		VampireResist: 0.02 + float64(difficulty*floor)*0.01,

		HealBoost:         0.05 + float64(difficulty*floor)*0.02,
		CritDamageBoost:   0.2 + float64(difficulty*floor)*0.1,
		CritDamageReduce:  0.1 + float64(difficulty*floor)*0.05,
		FinalDamageBoost:  0.05 + float64(difficulty*floor)*0.02,
		FinalDamageReduce: 0.05 + float64(difficulty*floor)*0.02,
		CombatBoost:       0.03 + float64(difficulty*floor)*0.02,
		ResistanceBoost:   0.03 + float64(difficulty*floor)*0.02,
	}

	enemyType := "normal"
	if floor%10 == 0 {
		enemyType = "boss"
	} else if floor%5 == 0 {
		enemyType = "elite"
	}

	scalePercent := func(v, mult, cap float64) float64 {
		next := v * mult
		if next > cap {
			return cap
		}
		return next
	}

	if enemyType == "elite" {
		stats.Health *= 1.5
		stats.MaxHealth *= 1.5
		stats.Damage *= 1.5
		stats.Defense *= 1.5
		stats.Speed *= 1.5

		stats.CritRate = scalePercent(stats.CritRate, 1.3, 0.8)
		stats.ComboRate = scalePercent(stats.ComboRate, 1.3, 0.8)
		stats.CounterRate = scalePercent(stats.CounterRate, 1.3, 0.8)
		stats.StunRate = scalePercent(stats.StunRate, 1.3, 0.8)
		stats.DodgeRate = scalePercent(stats.DodgeRate, 1.3, 0.8)
		stats.VampireRate = scalePercent(stats.VampireRate, 1.3, 0.8)
		stats.CritResist = scalePercent(stats.CritResist, 1.3, 0.8)
		stats.ComboResist = scalePercent(stats.ComboResist, 1.3, 0.8)
		stats.CounterResist = scalePercent(stats.CounterResist, 1.3, 0.8)
		stats.StunResist = scalePercent(stats.StunResist, 1.3, 0.8)
		stats.DodgeResist = scalePercent(stats.DodgeResist, 1.3, 0.8)
		stats.VampireResist = scalePercent(stats.VampireResist, 1.3, 0.8)
		stats.HealBoost = scalePercent(stats.HealBoost, 1.3, 0.8)
		stats.CritDamageBoost = scalePercent(stats.CritDamageBoost, 1.3, 0.8)
		stats.CritDamageReduce = scalePercent(stats.CritDamageReduce, 1.3, 0.8)
		stats.FinalDamageBoost = scalePercent(stats.FinalDamageBoost, 1.3, 0.8)
		stats.FinalDamageReduce = scalePercent(stats.FinalDamageReduce, 1.3, 0.8)
		stats.CombatBoost = scalePercent(stats.CombatBoost, 1.3, 0.8)
		stats.ResistanceBoost = scalePercent(stats.ResistanceBoost, 1.3, 0.8)
	} else if enemyType == "boss" {
		stats.Health *= 2
		stats.MaxHealth *= 2
		stats.Damage *= 2
		stats.Defense *= 2
		stats.Speed *= 2

		stats.CritRate = scalePercent(stats.CritRate, 1.5, 0.9)
		stats.ComboRate = scalePercent(stats.ComboRate, 1.5, 0.9)
		stats.CounterRate = scalePercent(stats.CounterRate, 1.5, 0.9)
		stats.StunRate = scalePercent(stats.StunRate, 1.5, 0.9)
		stats.DodgeRate = scalePercent(stats.DodgeRate, 1.5, 0.9)
		stats.VampireRate = scalePercent(stats.VampireRate, 1.5, 0.9)
		stats.CritResist = scalePercent(stats.CritResist, 1.5, 0.9)
		stats.ComboResist = scalePercent(stats.ComboResist, 1.5, 0.9)
		stats.CounterResist = scalePercent(stats.CounterResist, 1.5, 0.9)
		stats.StunResist = scalePercent(stats.StunResist, 1.5, 0.9)
		stats.DodgeResist = scalePercent(stats.DodgeResist, 1.5, 0.9)
		stats.VampireResist = scalePercent(stats.VampireResist, 1.5, 0.9)
		stats.HealBoost = scalePercent(stats.HealBoost, 1.5, 0.9)
		stats.CritDamageBoost = scalePercent(stats.CritDamageBoost, 1.5, 0.9)
		stats.CritDamageReduce = scalePercent(stats.CritDamageReduce, 1.5, 0.9)
		stats.FinalDamageBoost = scalePercent(stats.FinalDamageBoost, 1.5, 0.9)
		stats.FinalDamageReduce = scalePercent(stats.FinalDamageReduce, 1.5, 0.9)
		stats.CombatBoost = scalePercent(stats.CombatBoost, 1.5, 0.9)
		stats.ResistanceBoost = scalePercent(stats.ResistanceBoost, 1.5, 0.9)
	}

	normalNames := []string{"野狼", "山猪", "毒蛇", "黑熊", "猛虎", "恶狼", "巨蟒", "狂狮"}
	eliteNames := []string{"赤焰虎", "玄冰蟒", "紫电豹", "金刚猿", "幽冥狼", "碧水蛟", "雷霆鹰", "烈风豹"}
	bossNames := []string{"九尾天狐", "万年龙蟒", "太古神虎", "玄天冰凤", "幽冥魔龙", "混沌巨兽", "远古天蟒", "不死火凤"}

	enemyName := normalNames[floor%len(normalNames)]
	if enemyType == "elite" {
		enemyName = eliteNames[(floor/5)%len(eliteNames)]
	} else if enemyType == "boss" {
		enemyName = bossNames[(floor/10)%len(bossNames)]
	}

	return &dungeonEntity{
		Name:          enemyName,
		Stats:         stats,
		CurrentHealth: stats.MaxHealth,
	}
}

func runDungeonBattle(player *dungeonEntity, enemy *dungeonEntity, rng *rand.Rand) (string, []string) {
	logs := make([]string, 0, 24)
	maxRounds := 10
	for round := 1; round <= maxRounds; round++ {
		playerSpeed := player.Stats.Speed * (1 + player.Stats.CombatBoost)
		enemySpeed := enemy.Stats.Speed * (1 + enemy.Stats.CombatBoost)

		first := player
		second := enemy
		playerFirst := true
		if playerSpeed < enemySpeed {
			first = enemy
			second = player
			playerFirst = false
		}

		firstAttack := dungeonCalculateDamage(first, second, rng)
		firstOutcome := dungeonTakeDamage(second, first, firstAttack, rng)
		firstLog := fmt.Sprintf("%s率先发起攻击", first.Name)
		if firstOutcome.Dodged {
			firstLog += "，被闪避了！"
		} else {
			firstLog += fmt.Sprintf("，造成%.1f点伤害", firstOutcome.Damage)
			if firstAttack.IsCrit {
				firstLog += "（暴击！）"
			}
			if firstAttack.IsCombo {
				firstLog += "（连击！）"
			}
			if firstAttack.IsVampire {
				healAmount := firstOutcome.Damage * 0.3
				dungeonHeal(first, healAmount)
				firstLog += fmt.Sprintf("（吸血恢复%.1f点生命值！）", healAmount)
			}
			if firstAttack.IsStun {
				firstLog += "（眩晕目标！）"
			}
		}
		logs = append(logs, firstLog)

		if firstOutcome.IsDead {
			if playerFirst {
				logs = append(logs, fmt.Sprintf("%s获得胜利！", player.Name))
				return dungeonBattleStateVictory, logs
			}
			logs = append(logs, fmt.Sprintf("%s获得胜利！", enemy.Name))
			return dungeonBattleStateDefeat, logs
		}

		if !firstAttack.IsStun {
			secondAttack := dungeonCalculateDamage(second, first, rng)
			secondOutcome := dungeonTakeDamage(first, second, secondAttack, rng)
			if firstOutcome.IsCounter {
				logs = append(logs, fmt.Sprintf("%s触发了反击效果！", second.Name))
			}

			secondLog := fmt.Sprintf("%s进行攻击", second.Name)
			if firstOutcome.IsCounter {
				secondLog = fmt.Sprintf("%s的反击", second.Name)
			}
			if secondOutcome.Dodged {
				secondLog += "，被闪避了！"
			} else {
				secondLog += fmt.Sprintf("，造成%.1f点伤害", secondOutcome.Damage)
				if secondAttack.IsCrit {
					secondLog += "（暴击！）"
				}
				if secondAttack.IsCombo {
					secondLog += "（连击！）"
				}
				if secondAttack.IsVampire {
					healAmount := secondOutcome.Damage * 0.3
					dungeonHeal(second, healAmount)
					secondLog += fmt.Sprintf("（吸血恢复%.1f点生命值！）", healAmount)
				}
				if secondAttack.IsStun {
					secondLog += "（眩晕目标！）"
				}
			}
			logs = append(logs, secondLog)

			if secondOutcome.IsDead {
				if playerFirst {
					logs = append(logs, fmt.Sprintf("%s获得胜利！", enemy.Name))
					return dungeonBattleStateDefeat, logs
				}
				logs = append(logs, fmt.Sprintf("%s获得胜利！", player.Name))
				return dungeonBattleStateVictory, logs
			}
		}
	}

	logs = append(logs, fmt.Sprintf("战斗超过%d回合，战斗失败！", maxRounds))
	return dungeonBattleStateDefeat, logs
}

func dungeonCalculateDamage(attacker *dungeonEntity, defender *dungeonEntity, rng *rand.Rand) dungeonAttackOutcome {
	damage := math.Abs(attacker.Stats.Damage * (1 + attacker.Stats.CombatBoost))
	outcome := dungeonAttackOutcome{Damage: damage}

	finalCritRate := math.Max(
		0,
		attacker.Stats.CritRate*(1+attacker.Stats.CombatBoost)-defender.Stats.CritResist*(1+defender.Stats.ResistanceBoost),
	)
	if rng.Float64() < finalCritRate {
		outcome.IsCrit = true
		outcome.Damage *= 1.5 + attacker.Stats.CritDamageBoost
	}

	finalComboRate := math.Max(0, attacker.Stats.ComboRate*(1+attacker.Stats.CombatBoost)-defender.Stats.ComboResist)
	if rng.Float64() < finalComboRate {
		outcome.IsCombo = true
		outcome.Damage *= 1.3
	}

	finalVampireRate := math.Max(0, attacker.Stats.VampireRate*(1+attacker.Stats.CombatBoost)-defender.Stats.VampireResist)
	if rng.Float64() < finalVampireRate {
		outcome.IsVampire = true
	}

	finalStunRate := math.Max(0, attacker.Stats.StunRate*(1+attacker.Stats.CombatBoost)-defender.Stats.StunResist)
	if rng.Float64() < finalStunRate {
		outcome.IsStun = true
	}

	outcome.Damage *= 1 + attacker.Stats.FinalDamageBoost
	return outcome
}

func dungeonTakeDamage(defender *dungeonEntity, attacker *dungeonEntity, attack dungeonAttackOutcome, rng *rand.Rand) dungeonDamageOutcome {
	actualDodgeRate := defender.Stats.DodgeRate - attacker.Stats.DodgeResist
	if actualDodgeRate < 0 {
		actualDodgeRate = 0
	}
	if actualDodgeRate > 1 {
		actualDodgeRate = 1
	}
	if rng.Float64() < actualDodgeRate {
		return dungeonDamageOutcome{
			Dodged: true,
		}
	}

	damage := math.Abs(attack.Damage)
	effectiveDefense := defender.Stats.Defense * (1 + defender.Stats.CombatBoost)
	damage *= 100 / (100 + effectiveDefense)
	if attack.IsCrit {
		damage *= 1 - defender.Stats.CritDamageReduce
	}
	damage *= 1 - defender.Stats.FinalDamageReduce
	if damage < 0 {
		damage = 0
	}

	defender.CurrentHealth -= damage
	if defender.CurrentHealth < 0 {
		defender.CurrentHealth = 0
	}

	finalCounterRate := math.Max(0, defender.Stats.CounterRate-attacker.Stats.CounterResist)
	isCounter := rng.Float64() < finalCounterRate

	return dungeonDamageOutcome{
		Dodged:        false,
		Damage:        damage,
		CurrentHealth: defender.CurrentHealth,
		IsDead:        defender.CurrentHealth <= 0,
		IsCounter:     isCounter,
	}
}

func dungeonHeal(entity *dungeonEntity, amount float64) {
	if amount <= 0 {
		return
	}
	entity.CurrentHealth += amount
	if entity.CurrentHealth > entity.Stats.MaxHealth {
		entity.CurrentHealth = entity.Stats.MaxHealth
	}
}

func dungeonDecodeFloatMap(raw []byte) map[string]float64 {
	result := map[string]float64{}
	if len(raw) == 0 {
		return result
	}
	temp := map[string]any{}
	if err := json.Unmarshal(raw, &temp); err != nil {
		return result
	}
	for key, value := range temp {
		switch v := value.(type) {
		case float64:
			result[key] = v
		case int:
			result[key] = float64(v)
		case int64:
			result[key] = float64(v)
		}
	}
	return result
}

func dungeonReadWithDefault(input map[string]float64, key string, defaultValue float64) float64 {
	if value, ok := input[key]; ok {
		return value
	}
	return defaultValue
}

func dungeonValidDifficulty(difficulty int) bool {
	switch difficulty {
	case 1, 2, 5, 10, 100:
		return true
	default:
		return false
	}
}

func dungeonHighestFloorByDifficulty(difficulty int, floor1 int, floor2 int, floor5 int, floor10 int, floor100 int) int {
	switch difficulty {
	case 1:
		return floor1
	case 2:
		return floor2
	case 5:
		return floor5
	case 10:
		return floor10
	case 100:
		return floor100
	default:
		return floor1
	}
}

type dungeonOptionDefinition struct {
	ID          string
	Name        string
	Description string
	Quality     string
	Apply       func(stats *dungeonCombatStats)
}

var dungeonOptionPools = map[string][]dungeonOptionDefinition{
	"common": {
		{ID: "heal", Name: "气血增加", Description: "增加10%血量", Quality: "common", Apply: func(stats *dungeonCombatStats) {
			stats.MaxHealth *= 1.1
			stats.Health = math.Min(stats.MaxHealth, stats.Health+stats.MaxHealth*0.1)
		}},
		{ID: "small_buff", Name: "小幅强化", Description: "增加10%伤害", Quality: "common", Apply: func(stats *dungeonCombatStats) {
			stats.FinalDamageBoost += 0.1
		}},
		{ID: "defense_boost", Name: "铁壁", Description: "提升20%防御力", Quality: "common", Apply: func(stats *dungeonCombatStats) {
			stats.Defense *= 1.2
		}},
		{ID: "speed_boost", Name: "疾风", Description: "提升15%速度", Quality: "common", Apply: func(stats *dungeonCombatStats) {
			stats.Speed *= 1.15
		}},
		{ID: "crit_boost", Name: "会心", Description: "提升15%暴击率", Quality: "common", Apply: func(stats *dungeonCombatStats) {
			stats.CritRate += 0.15
		}},
		{ID: "dodge_boost", Name: "轻身", Description: "提升15%闪避率", Quality: "common", Apply: func(stats *dungeonCombatStats) {
			stats.DodgeRate += 0.15
		}},
		{ID: "vampire_boost", Name: "吸血", Description: "提升10%吸血率", Quality: "common", Apply: func(stats *dungeonCombatStats) {
			stats.VampireRate += 0.1
		}},
		{ID: "combat_boost", Name: "战意", Description: "提升10%战斗属性", Quality: "common", Apply: func(stats *dungeonCombatStats) {
			stats.CombatBoost += 0.1
		}},
	},
	"rare": {
		{ID: "defense_master", Name: "防御大师", Description: "防御力提升10%", Quality: "rare", Apply: func(stats *dungeonCombatStats) {
			stats.Defense *= 1.1
		}},
		{ID: "crit_mastery", Name: "会心精通", Description: "暴击率提升10%，暴击伤害提升20%", Quality: "rare", Apply: func(stats *dungeonCombatStats) {
			stats.CritRate += 0.1
			stats.CritDamageBoost += 0.2
		}},
		{ID: "dodge_master", Name: "无影", Description: "闪避率提升10%", Quality: "rare", Apply: func(stats *dungeonCombatStats) {
			stats.DodgeRate += 0.1
		}},
		{ID: "combo_master", Name: "连击精通", Description: "连击率提升10%", Quality: "rare", Apply: func(stats *dungeonCombatStats) {
			stats.ComboRate += 0.1
		}},
		{ID: "vampire_master", Name: "血魔", Description: "吸血率提升5%", Quality: "rare", Apply: func(stats *dungeonCombatStats) {
			stats.VampireRate += 0.05
		}},
		{ID: "stun_master", Name: "震慑", Description: "眩晕率提升5%", Quality: "rare", Apply: func(stats *dungeonCombatStats) {
			stats.StunRate += 0.05
		}},
	},
	"epic": {
		{ID: "ultimate_power", Name: "极限突破", Description: "所有战斗属性提升50%", Quality: "epic", Apply: func(stats *dungeonCombatStats) {
			stats.CombatBoost += 0.5
			stats.FinalDamageBoost += 0.5
		}},
		{ID: "divine_protection", Name: "天道庇护", Description: "最终减伤提升30%", Quality: "epic", Apply: func(stats *dungeonCombatStats) {
			stats.FinalDamageReduce += 0.3
		}},
		{ID: "combat_master", Name: "战斗大师", Description: "所有战斗属性和抗性提升25%", Quality: "epic", Apply: func(stats *dungeonCombatStats) {
			stats.CombatBoost += 0.25
			stats.ResistanceBoost += 0.25
		}},
		{ID: "immortal_body", Name: "不朽之躯", Description: "生命上限提升100%，最终减伤提升20%", Quality: "epic", Apply: func(stats *dungeonCombatStats) {
			stats.MaxHealth *= 2
			stats.Health *= 2
			stats.FinalDamageReduce += 0.2
		}},
		{ID: "celestial_might", Name: "天人合一", Description: "所有战斗属性提升40%，生命值增加50%", Quality: "epic", Apply: func(stats *dungeonCombatStats) {
			stats.CombatBoost += 0.4
			stats.MaxHealth *= 1.5
			stats.Health = math.Min(stats.MaxHealth, stats.Health+stats.MaxHealth*0.5)
		}},
		{ID: "battle_sage_supreme", Name: "战圣至尊", Description: "暴击率提升40%，暴击伤害提升80%，最终伤害提升20%", Quality: "epic", Apply: func(stats *dungeonCombatStats) {
			stats.CritRate += 0.4
			stats.CritDamageBoost += 0.8
			stats.FinalDamageBoost += 0.2
		}},
	},
}

var dungeonOptionMap = buildDungeonOptionMap()

func buildDungeonOptionMap() map[string]dungeonOptionDefinition {
	output := make(map[string]dungeonOptionDefinition)
	for _, quality := range []string{"common", "rare", "epic"} {
		for _, option := range dungeonOptionPools[quality] {
			output[option.ID] = option
		}
	}
	return output
}

func dungeonNeedsOption(floor int) bool {
	return floor == 1 || floor%5 == 0
}

func dungeonGenerateOptions(floor int, rng *rand.Rand) []DungeonOption {
	rareChance := 0.25
	epicChance := 0.05
	if floor%10 == 0 {
		rareChance = 0.3
		epicChance = 0.2
	} else if floor%5 == 0 {
		rareChance = 0.35
		epicChance = 0.15
	}

	selected := make([]DungeonOption, 0, 3)
	used := make(map[string]struct{}, 3)
	for len(selected) < 3 {
		randomValue := rng.Float64()
		pool := "common"
		if randomValue < epicChance {
			pool = "epic"
		} else if randomValue < epicChance+rareChance {
			pool = "rare"
		}

		candidates := make([]dungeonOptionDefinition, 0)
		for _, option := range dungeonOptionPools[pool] {
			if _, exists := used[option.ID]; exists {
				continue
			}
			candidates = append(candidates, option)
		}
		if len(candidates) == 0 {
			for _, quality := range []string{"common", "rare", "epic"} {
				for _, option := range dungeonOptionPools[quality] {
					if _, exists := used[option.ID]; exists {
						continue
					}
					candidates = append(candidates, option)
				}
			}
		}
		if len(candidates) == 0 {
			break
		}

		chosen := candidates[rng.Intn(len(candidates))]
		selected = append(selected, DungeonOption{
			ID:          chosen.ID,
			Name:        chosen.Name,
			Description: chosen.Description,
			Type:        chosen.Quality,
		})
		used[chosen.ID] = struct{}{}
	}

	return selected
}

func dungeonOptionIDs(options []DungeonOption) []string {
	ids := make([]string, 0, len(options))
	for _, option := range options {
		if option.ID == "" {
			continue
		}
		ids = append(ids, option.ID)
	}
	return ids
}

func dungeonOptionsFromIDs(ids []string) []DungeonOption {
	output := make([]DungeonOption, 0, len(ids))
	for _, id := range ids {
		definition, ok := dungeonOptionDefinitionByID(id)
		if !ok {
			continue
		}
		output = append(output, DungeonOption{
			ID:          definition.ID,
			Name:        definition.Name,
			Description: definition.Description,
			Type:        definition.Quality,
		})
	}
	return output
}

func dungeonOptionDefinitionByID(id string) (dungeonOptionDefinition, bool) {
	definition, ok := dungeonOptionMap[id]
	return definition, ok
}

func dungeonOptionInList(id string, ids []string) bool {
	for _, current := range ids {
		if current == id {
			return true
		}
	}
	return false
}

func dungeonAppendUniqueID(ids []string, next string) []string {
	if next == "" {
		return ids
	}
	if dungeonOptionInList(next, ids) {
		return ids
	}
	output := make([]string, 0, len(ids)+1)
	output = append(output, ids...)
	output = append(output, next)
	return output
}

func dungeonDecodeStringArray(raw []byte) []string {
	output := make([]string, 0)
	if len(raw) == 0 {
		return output
	}
	if err := json.Unmarshal(raw, &output); err != nil {
		return []string{}
	}
	return output
}

func dungeonApplyBuffs(stats *dungeonCombatStats, buffIDs []string) {
	if stats == nil {
		return
	}
	for _, buffID := range buffIDs {
		definition, ok := dungeonOptionDefinitionByID(buffID)
		if !ok || definition.Apply == nil {
			continue
		}
		definition.Apply(stats)
	}
	if stats.Health > stats.MaxHealth {
		stats.Health = stats.MaxHealth
	}
}
