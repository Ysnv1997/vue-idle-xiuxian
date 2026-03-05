package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"math"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/kowming/vue-idle-xiuxian/backend/internal/repository"
)

type AchievementService struct {
	pool     *pgxpool.Pool
	userRepo *repository.UserRepository
}

func NewAchievementService(pool *pgxpool.Pool, userRepo *repository.UserRepository) *AchievementService {
	return &AchievementService{pool: pool, userRepo: userRepo}
}

type achievementReward struct {
	Spirit      int64   `json:"spirit,omitempty"`
	SpiritRate  float64 `json:"spiritRate,omitempty"`
	HerbRate    float64 `json:"herbRate,omitempty"`
	AlchemyRate float64 `json:"alchemyRate,omitempty"`
	Luck        float64 `json:"luck,omitempty"`
}

type achievementDefinition struct {
	Category    string
	ID          string
	Name        string
	Description string
	Reward      achievementReward
}

type achievementStatus struct {
	CompletedAt *time.Time
	ClaimedAt   *time.Time
}

type achievementState struct {
	Level                int
	Spirit               float64
	SpiritRate           float64
	Luck                 float64
	HerbRate             float64
	AlchemyRate          float64
	SpiritStones         int64
	TotalCultivationTime int64
	BreakthroughCount    int64
	ExplorationCount     int64
	EventTriggered       int64
	ItemsFound           int64

	DungeonHighestFloor int64
	DungeonTotalRuns    int64
	DungeonBossKills    int64
	DungeonEliteKills   int64
	DungeonTotalKills   int64
	DungeonStreakKills  int64

	PillsCrafted            int64
	HighQualityPillsCrafted int64

	Herbs             []herbItem
	PillRecipes       []string
	Items             []map[string]any
	EquippedArtifacts map[string]any
}

type achievementDerivedStats struct {
	TotalEquipment       int64
	UniqueEquipmentTypes int64
	LegendaryEquipCount  int64
	MythicEquipCount     int64
	LegendaryTypeCount   int64
	MythicTypeCount      int64
	MaxEnhanceLevel      int64

	TotalHerbs      int64
	UniqueHerbTypes int64
	RareHerbCount   int64
	EpicHerbCount   int64
	MythicHerbCount int64
}

type AchievementView struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Reward      achievementReward `json:"reward"`
	Completed   bool              `json:"completed"`
	Claimed     bool              `json:"claimed"`
	Progress    float64           `json:"progress"`
	CompletedAt *time.Time        `json:"completedAt,omitempty"`
	ClaimedAt   *time.Time        `json:"claimedAt,omitempty"`
}

type AchievementCategoryView struct {
	Key          string            `json:"key"`
	Name         string            `json:"name"`
	Achievements []AchievementView `json:"achievements"`
}

type AchievementListResult struct {
	Categories     []AchievementCategoryView `json:"categories"`
	CompletedCount int                       `json:"completedCount"`
	TotalCount     int                       `json:"totalCount"`
}

type AchievementUnlockNotice struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Reward      achievementReward `json:"reward"`
}

type AchievementSyncResult struct {
	NewlyCompleted []AchievementUnlockNotice  `json:"newlyCompleted"`
	Achievements   *AchievementListResult     `json:"achievements"`
	Snapshot       *repository.PlayerSnapshot `json:"snapshot"`
}

func (s *AchievementService) List(ctx context.Context, userID uuid.UUID) (*AchievementListResult, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin achievements list transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if err := ensureAchievementRows(ctx, tx, userID); err != nil {
		return nil, err
	}

	state, err := loadAchievementState(ctx, tx, userID, false)
	if err != nil {
		return nil, err
	}

	statuses, err := loadAchievementStatuses(ctx, tx, userID, false)
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit achievements list transaction: %w", err)
	}

	return buildAchievementList(state, statuses), nil
}

func (s *AchievementService) Sync(ctx context.Context, userID uuid.UUID) (*AchievementSyncResult, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin achievements sync transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if err := ensureAchievementRows(ctx, tx, userID); err != nil {
		return nil, err
	}

	state, err := loadAchievementState(ctx, tx, userID, true)
	if err != nil {
		return nil, err
	}

	statuses, err := loadAchievementStatuses(ctx, tx, userID, true)
	if err != nil {
		return nil, err
	}

	derived := deriveAchievementStats(state)
	now := time.Now().UTC()
	resourceChanged := false
	newlyCompleted := make([]AchievementUnlockNotice, 0)

	for _, def := range achievementDefinitions {
		status := statuses[def.ID]
		if status.CompletedAt == nil {
			completed, _ := evaluateAchievement(def.ID, state, derived)
			if !completed {
				continue
			}
			if applyAchievementReward(state, def.Reward) {
				resourceChanged = true
			}

			completedAt := now
			claimedAt := now
			status.CompletedAt = &completedAt
			status.ClaimedAt = &claimedAt
			statuses[def.ID] = status

			if err := upsertAchievementStatus(ctx, tx, userID, def.ID, status.CompletedAt, status.ClaimedAt); err != nil {
				return nil, err
			}

			newlyCompleted = append(newlyCompleted, AchievementUnlockNotice{
				ID:          def.ID,
				Name:        def.Name,
				Description: def.Description,
				Reward:      def.Reward,
			})
			continue
		}

		if status.ClaimedAt == nil {
			if applyAchievementReward(state, def.Reward) {
				resourceChanged = true
			}
			claimedAt := now
			status.ClaimedAt = &claimedAt
			statuses[def.ID] = status
			if err := upsertAchievementStatus(ctx, tx, userID, def.ID, status.CompletedAt, status.ClaimedAt); err != nil {
				return nil, err
			}
		}
	}

	if resourceChanged {
		if err := persistAchievementResourceState(ctx, tx, userID, state); err != nil {
			return nil, err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit achievements sync transaction: %w", err)
	}

	snapshot, err := s.userRepo.GetSnapshot(ctx, userID)
	if err != nil {
		return nil, err
	}

	return &AchievementSyncResult{
		NewlyCompleted: newlyCompleted,
		Achievements:   buildAchievementList(state, statuses),
		Snapshot:       snapshot,
	}, nil
}

func ensureAchievementRows(ctx context.Context, tx pgx.Tx, userID uuid.UUID) error {
	const cultivationStatsSQL = `
		INSERT INTO player_cultivation_stats (user_id, total_cultivation_time, breakthrough_count, updated_at)
		VALUES ($1, 0, 0, now())
		ON CONFLICT (user_id) DO NOTHING
	`
	if _, err := tx.Exec(ctx, cultivationStatsSQL, userID); err != nil {
		return fmt.Errorf("ensure achievement cultivation stats row: %w", err)
	}

	const explorationStatsSQL = `
		INSERT INTO player_exploration_stats (user_id, exploration_count, events_triggered, items_found, updated_at)
		VALUES ($1, 0, 0, 0, now())
		ON CONFLICT (user_id) DO NOTHING
	`
	if _, err := tx.Exec(ctx, explorationStatsSQL, userID); err != nil {
		return fmt.Errorf("ensure achievement exploration stats row: %w", err)
	}

	const dungeonProgressSQL = `
		INSERT INTO player_dungeon_progress (
			user_id, highest_floor, highest_floor_2x, highest_floor_5x, highest_floor_10x, highest_floor_100x,
			last_failed_floor, total_runs, boss_kills, elite_kills, total_kills, death_count, total_rewards, streak_kills, updated_at
		)
		VALUES ($1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, now())
		ON CONFLICT (user_id) DO NOTHING
	`
	if _, err := tx.Exec(ctx, dungeonProgressSQL, userID); err != nil {
		return fmt.Errorf("ensure achievement dungeon progress row: %w", err)
	}

	const inventoryStateSQL = `
		INSERT INTO player_inventory_state (user_id, herbs, pill_fragments, pill_recipes, items, equipped_artifacts, updated_at)
		VALUES (
			$1,
			'[]'::jsonb,
			'{}'::jsonb,
			'[]'::jsonb,
			'[]'::jsonb,
			'{
				"weapon": null,
				"head": null,
				"body": null,
				"legs": null,
				"feet": null,
				"shoulder": null,
				"hands": null,
				"wrist": null,
				"necklace": null,
				"ring1": null,
				"ring2": null,
				"belt": null,
				"artifact": null
			}'::jsonb,
			now()
		)
		ON CONFLICT (user_id) DO NOTHING
	`
	if _, err := tx.Exec(ctx, inventoryStateSQL, userID); err != nil {
		return fmt.Errorf("ensure achievement inventory state row: %w", err)
	}

	const alchemyStatsSQL = `
		INSERT INTO player_alchemy_stats (user_id, pills_crafted, high_quality_pills_crafted, updated_at)
		VALUES ($1, 0, 0, now())
		ON CONFLICT (user_id) DO NOTHING
	`
	if _, err := tx.Exec(ctx, alchemyStatsSQL, userID); err != nil {
		return fmt.Errorf("ensure achievement alchemy stats row: %w", err)
	}

	return nil
}

func loadAchievementState(ctx context.Context, tx pgx.Tx, userID uuid.UUID, forUpdate bool) (*achievementState, error) {
	lockClause := ""
	if forUpdate {
		lockClause = " FOR UPDATE OF pp, pr, pcs, pes, pdp, pas, pis"
	}

	query := `
		SELECT
			pp.level,
			pr.spirit + (GREATEST(EXTRACT(EPOCH FROM now() - pr.updated_at), 0) * pr.spirit_rate),
			pr.spirit_rate,
			pr.luck,
			pr.herb_rate,
			pr.alchemy_rate,
			pr.spirit_stones,
			pcs.total_cultivation_time,
			pcs.breakthrough_count,
			pes.exploration_count,
			pes.events_triggered,
			pes.items_found,
			pdp.highest_floor,
			pdp.total_runs,
			pdp.boss_kills,
			pdp.elite_kills,
			pdp.total_kills,
			pdp.streak_kills,
			pas.pills_crafted,
			pas.high_quality_pills_crafted,
			COALESCE(pis.herbs, '[]'::jsonb),
			COALESCE(pis.pill_recipes, '[]'::jsonb),
			COALESCE(pis.items, '[]'::jsonb),
			COALESCE(
				pis.equipped_artifacts,
				'{
					"weapon": null,
					"head": null,
					"body": null,
					"legs": null,
					"feet": null,
					"shoulder": null,
					"hands": null,
					"wrist": null,
					"necklace": null,
					"ring1": null,
					"ring2": null,
					"belt": null,
					"artifact": null
				}'::jsonb
			)
		FROM player_profiles pp
		JOIN player_resources pr ON pr.user_id = pp.user_id
		JOIN player_cultivation_stats pcs ON pcs.user_id = pp.user_id
		JOIN player_exploration_stats pes ON pes.user_id = pp.user_id
		JOIN player_dungeon_progress pdp ON pdp.user_id = pp.user_id
		JOIN player_alchemy_stats pas ON pas.user_id = pp.user_id
		JOIN player_inventory_state pis ON pis.user_id = pp.user_id
		WHERE pp.user_id = $1` + lockClause

	state := &achievementState{}
	var herbsRaw []byte
	var pillRecipesRaw []byte
	var itemsRaw []byte
	var equippedRaw []byte
	if err := tx.QueryRow(ctx, query, userID).Scan(
		&state.Level,
		&state.Spirit,
		&state.SpiritRate,
		&state.Luck,
		&state.HerbRate,
		&state.AlchemyRate,
		&state.SpiritStones,
		&state.TotalCultivationTime,
		&state.BreakthroughCount,
		&state.ExplorationCount,
		&state.EventTriggered,
		&state.ItemsFound,
		&state.DungeonHighestFloor,
		&state.DungeonTotalRuns,
		&state.DungeonBossKills,
		&state.DungeonEliteKills,
		&state.DungeonTotalKills,
		&state.DungeonStreakKills,
		&state.PillsCrafted,
		&state.HighQualityPillsCrafted,
		&herbsRaw,
		&pillRecipesRaw,
		&itemsRaw,
		&equippedRaw,
	); err != nil {
		return nil, fmt.Errorf("load achievement state: %w", err)
	}

	if err := json.Unmarshal(herbsRaw, &state.Herbs); err != nil {
		state.Herbs = []herbItem{}
	}
	if state.Herbs == nil {
		state.Herbs = []herbItem{}
	}

	if err := json.Unmarshal(pillRecipesRaw, &state.PillRecipes); err != nil {
		state.PillRecipes = []string{}
	}
	if state.PillRecipes == nil {
		state.PillRecipes = []string{}
	}

	if err := json.Unmarshal(itemsRaw, &state.Items); err != nil {
		state.Items = []map[string]any{}
	}
	if state.Items == nil {
		state.Items = []map[string]any{}
	}

	if err := json.Unmarshal(equippedRaw, &state.EquippedArtifacts); err != nil {
		state.EquippedArtifacts = map[string]any{}
	}
	if state.EquippedArtifacts == nil {
		state.EquippedArtifacts = map[string]any{}
	}

	return state, nil
}

func loadAchievementStatuses(ctx context.Context, tx pgx.Tx, userID uuid.UUID, forUpdate bool) (map[string]achievementStatus, error) {
	query := `
		SELECT achievement_id, completed_at, claimed_at
		FROM player_achievements
		WHERE user_id = $1
	`
	if forUpdate {
		query += " FOR UPDATE"
	}

	rows, err := tx.Query(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("query achievement statuses: %w", err)
	}
	defer rows.Close()

	statusMap := make(map[string]achievementStatus)
	for rows.Next() {
		var achievementID string
		var completedAt sql.NullTime
		var claimedAt sql.NullTime
		if err := rows.Scan(&achievementID, &completedAt, &claimedAt); err != nil {
			return nil, fmt.Errorf("scan achievement status: %w", err)
		}

		status := achievementStatus{}
		if completedAt.Valid {
			t := completedAt.Time.UTC()
			status.CompletedAt = &t
		}
		if claimedAt.Valid {
			t := claimedAt.Time.UTC()
			status.ClaimedAt = &t
		}
		statusMap[achievementID] = status
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate achievement statuses: %w", err)
	}

	return statusMap, nil
}

func upsertAchievementStatus(
	ctx context.Context,
	tx pgx.Tx,
	userID uuid.UUID,
	achievementID string,
	completedAt *time.Time,
	claimedAt *time.Time,
) error {
	const query = `
		INSERT INTO player_achievements (user_id, achievement_id, completed_at, claimed_at)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (user_id, achievement_id)
		DO UPDATE SET
			completed_at = COALESCE(player_achievements.completed_at, EXCLUDED.completed_at),
			claimed_at = COALESCE(player_achievements.claimed_at, EXCLUDED.claimed_at)
	`
	if _, err := tx.Exec(ctx, query, userID, achievementID, completedAt, claimedAt); err != nil {
		return fmt.Errorf("upsert achievement status %s: %w", achievementID, err)
	}
	return nil
}

func persistAchievementResourceState(ctx context.Context, tx pgx.Tx, userID uuid.UUID, state *achievementState) error {
	if state.SpiritRate <= 0 {
		state.SpiritRate = 1
	}
	if state.Luck <= 0 {
		state.Luck = 1
	}
	if state.HerbRate <= 0 {
		state.HerbRate = 1
	}
	if state.AlchemyRate <= 0 {
		state.AlchemyRate = 1
	}

	const query = `
		UPDATE player_resources
		SET spirit = $2, spirit_rate = $3, luck = $4, herb_rate = $5, alchemy_rate = $6, updated_at = now()
		WHERE user_id = $1
	`
	if _, err := tx.Exec(ctx, query, userID, state.Spirit, state.SpiritRate, state.Luck, state.HerbRate, state.AlchemyRate); err != nil {
		return fmt.Errorf("update resources after achievement reward: %w", err)
	}
	return nil
}

func buildAchievementList(state *achievementState, statuses map[string]achievementStatus) *AchievementListResult {
	derived := deriveAchievementStats(state)
	completedCount := 0
	categoryBuckets := make(map[string][]AchievementView, len(achievementCategoryOrder))

	for _, def := range achievementDefinitions {
		status := statuses[def.ID]
		completed := status.CompletedAt != nil
		claimed := status.ClaimedAt != nil
		progress := 100.0
		if !completed {
			_, progress = evaluateAchievement(def.ID, state, derived)
		}

		if completed {
			completedCount++
		}

		categoryBuckets[def.Category] = append(categoryBuckets[def.Category], AchievementView{
			ID:          def.ID,
			Name:        def.Name,
			Description: def.Description,
			Reward:      def.Reward,
			Completed:   completed,
			Claimed:     claimed,
			Progress:    progress,
			CompletedAt: status.CompletedAt,
			ClaimedAt:   status.ClaimedAt,
		})
	}

	categories := make([]AchievementCategoryView, 0, len(achievementCategoryOrder))
	for _, categoryKey := range achievementCategoryOrder {
		categories = append(categories, AchievementCategoryView{
			Key:          categoryKey,
			Name:         achievementCategoryNames[categoryKey],
			Achievements: categoryBuckets[categoryKey],
		})
	}

	return &AchievementListResult{
		Categories:     categories,
		CompletedCount: completedCount,
		TotalCount:     len(achievementDefinitions),
	}
}

func deriveAchievementStats(state *achievementState) *achievementDerivedStats {
	stats := &achievementDerivedStats{}
	legendaryTypes := map[string]struct{}{}
	mythicTypes := map[string]struct{}{}
	allTypes := map[string]struct{}{}

	collectEquipment := func(item map[string]any) {
		if item == nil {
			return
		}
		itemType := inventoryReadString(item["type"])
		if !inventoryIsEquipmentType(itemType) {
			return
		}
		stats.TotalEquipment++
		allTypes[itemType] = struct{}{}

		quality := inventoryReadString(item["quality"])
		enhanceLevel := int64(inventoryReadInt(item["enhanceLevel"], 0))
		if enhanceLevel > stats.MaxEnhanceLevel {
			stats.MaxEnhanceLevel = enhanceLevel
		}

		switch quality {
		case "legendary":
			stats.LegendaryEquipCount++
			legendaryTypes[itemType] = struct{}{}
		case "mythic":
			stats.MythicEquipCount++
			mythicTypes[itemType] = struct{}{}
		}
	}

	for _, item := range state.Items {
		collectEquipment(item)
	}
	for _, raw := range state.EquippedArtifacts {
		collectEquipment(inventoryReadMap(raw))
	}

	stats.UniqueEquipmentTypes = int64(len(allTypes))
	stats.LegendaryTypeCount = int64(len(legendaryTypes))
	stats.MythicTypeCount = int64(len(mythicTypes))

	herbTypes := map[string]struct{}{}
	for _, herb := range state.Herbs {
		stats.TotalHerbs++
		herbTypes[herb.ID] = struct{}{}
		switch herb.Quality {
		case "rare":
			stats.RareHerbCount++
		case "epic":
			stats.EpicHerbCount++
		case "mythic", "legendary":
			// Frontend local achievement used "mythic" by mistake; treat legendary as same tier here.
			stats.MythicHerbCount++
		}
	}
	stats.UniqueHerbTypes = int64(len(herbTypes))

	return stats
}

func evaluateAchievement(id string, state *achievementState, derived *achievementDerivedStats) (bool, float64) {
	switch id {
	case "equipment_1":
		return metricProgress(derived.TotalEquipment, 1)
	case "equipment_2":
		return metricProgress(derived.TotalEquipment, 10)
	case "equipment_3":
		return metricProgress(derived.LegendaryEquipCount, 1)
	case "equipment_4":
		return metricProgress(derived.MaxEnhanceLevel, 10)
	case "equipment_5":
		return metricProgress(derived.LegendaryTypeCount, 4)
	case "equipment_6":
		return metricProgress(derived.MaxEnhanceLevel, 1)
	case "equipment_7":
		return metricProgress(derived.UniqueEquipmentTypes, 10)
	case "equipment_8":
		return metricProgress(derived.MythicEquipCount, 5)
	case "equipment_9":
		return metricProgress(derived.MaxEnhanceLevel, 15)
	case "equipment_10":
		return metricProgress(derived.MythicTypeCount, 13)

	case "dungeon_1":
		return metricProgress(state.DungeonTotalRuns, 1)
	case "dungeon_2":
		return metricProgress(state.DungeonHighestFloor, 5)
	case "dungeon_3":
		return metricProgress(state.DungeonHighestFloor, 10)
	case "dungeon_4":
		return metricProgress(state.DungeonHighestFloor, 20)
	case "dungeon_5":
		return metricProgress(state.DungeonHighestFloor, 30)
	case "dungeon_6":
		return metricProgress(state.DungeonHighestFloor, 50)
	case "dungeon_7":
		return metricProgress(state.DungeonHighestFloor, 75)
	case "dungeon_8":
		return metricProgress(state.DungeonHighestFloor, 100)
	case "dungeon_9":
		return metricProgress(state.DungeonHighestFloor, 150)
	case "dungeon_10":
		return metricProgress(state.DungeonHighestFloor, 200)

	case "dungeon_combat_1":
		return metricProgress(state.DungeonTotalKills, 10)
	case "dungeon_combat_2":
		return metricProgress(state.DungeonStreakKills, 50)
	case "dungeon_combat_3":
		return metricProgress(state.DungeonTotalKills, 100)
	case "dungeon_combat_4":
		return metricProgress(state.DungeonTotalKills, 500)
	case "dungeon_combat_5":
		return metricProgress(state.DungeonEliteKills, 50)
	case "dungeon_combat_6":
		return metricProgress(state.DungeonBossKills, 10)
	case "dungeon_combat_7":
		return metricProgress(state.DungeonBossKills, 50)
	case "dungeon_combat_8":
		return metricProgress(state.DungeonBossKills, 100)
	case "dungeon_combat_9":
		return metricProgress(state.DungeonTotalKills, 1000)
	case "dungeon_combat_10":
		return metricProgress(state.DungeonTotalKills, 10000)

	case "cultivation_1":
		return metricProgress(minInt64(state.TotalCultivationTime, 1), 1)
	case "cultivation_2":
		return metricProgress(state.TotalCultivationTime, 1800)
	case "cultivation_3":
		return metricProgress(state.TotalCultivationTime, 3600)
	case "cultivation_4":
		return metricProgress(state.TotalCultivationTime, 43200)
	case "cultivation_5":
		return metricProgress(state.TotalCultivationTime, 172800)
	case "cultivation_6":
		return metricProgress(state.TotalCultivationTime, 86400)
	case "cultivation_7":
		return metricProgress(state.TotalCultivationTime, 604800)
	case "cultivation_8":
		return metricProgress(state.TotalCultivationTime, 1296000)
	case "cultivation_9":
		return metricProgress(state.TotalCultivationTime, 2592000)
	case "cultivation_10":
		return metricProgress(state.TotalCultivationTime, 8640000)

	case "breakthrough_1":
		return metricProgress(state.BreakthroughCount, 1)
	case "breakthrough_2":
		return metricProgress(state.BreakthroughCount, 5)
	case "breakthrough_3":
		return metricProgress(state.BreakthroughCount, 10)
	case "breakthrough_4":
		return metricProgress(state.BreakthroughCount, 50)
	case "breakthrough_5":
		return metricProgress(state.BreakthroughCount, 100)
	case "breakthrough_6":
		return metricProgress(int64(state.Level), 37)
	case "breakthrough_7":
		return metricProgress(int64(state.Level), 46)
	case "breakthrough_8":
		return metricProgress(int64(state.Level), 64)
	case "breakthrough_9":
		return metricProgress(int64(state.Level), 82)
	case "breakthrough_10":
		return metricProgress(int64(state.Level), 126)

	case "exploration_1":
		return metricProgress(state.ExplorationCount, 1)
	case "exploration_2":
		return metricProgress(state.ExplorationCount, 10)
	case "exploration_3":
		return metricProgress(state.ExplorationCount, 50)
	case "exploration_4":
		return metricProgress(state.ExplorationCount, 100)
	case "exploration_5":
		return metricProgress(state.ExplorationCount, 200)
	case "exploration_6":
		return metricProgress(state.ExplorationCount, 500)
	case "exploration_7":
		return metricProgress(state.ExplorationCount, 1000)
	case "exploration_8":
		return metricProgress(state.ItemsFound, 100)
	case "exploration_9":
		return metricProgress(state.EventTriggered, 100)
	case "exploration_10":
		return metricProgress(state.EventTriggered, 500)

	case "collection_1":
		return metricProgress(derived.TotalHerbs, 1)
	case "collection_2":
		return metricProgress(derived.UniqueHerbTypes, 5)
	case "collection_3":
		return metricProgress(derived.UniqueHerbTypes, 10)
	case "collection_4":
		return metricProgress(derived.TotalHerbs, 50)
	case "collection_5":
		return metricProgress(derived.TotalHerbs, 100)
	case "collection_6":
		return metricProgress(derived.TotalHerbs, 200)
	case "collection_7":
		return metricProgress(derived.RareHerbCount, 100)
	case "collection_8":
		return metricProgress(derived.EpicHerbCount, 100)
	case "collection_9":
		return metricProgress(derived.MythicHerbCount, 100)
	case "collection_10":
		return metricProgress(derived.UniqueHerbTypes, 15)

	case "resources_1":
		return metricProgress(state.SpiritStones, 1)
	case "resources_2":
		return metricProgress(state.SpiritStones, 1000)
	case "resources_3":
		return metricProgress(state.SpiritStones, 5000)
	case "resources_4":
		return metricProgress(state.SpiritStones, 10000)
	case "resources_5":
		return metricProgress(state.SpiritStones, 50000)
	case "resources_6":
		return metricProgress(state.SpiritStones, 100000)
	case "resources_7":
		return metricProgress(state.SpiritStones, 500000)
	case "resources_8":
		return metricProgress(state.SpiritStones, 1000000)
	case "resources_9":
		return metricProgress(state.SpiritStones, 5000000)
	case "resources_10":
		return metricProgress(state.SpiritStones, 10000000)

	case "alchemy_1":
		return metricProgress(state.PillsCrafted, 1)
	case "alchemy_2":
		return metricProgress(state.PillsCrafted, 5)
	case "alchemy_3":
		return metricProgress(state.PillsCrafted, 10)
	case "alchemy_4":
		return metricProgress(state.PillsCrafted, 50)
	case "alchemy_5":
		return metricProgress(state.PillsCrafted, 100)
	case "alchemy_6":
		return metricProgress(state.PillsCrafted, 500)
	case "alchemy_7":
		return metricProgress(state.PillsCrafted, 1000)
	case "alchemy_8":
		return metricProgress(state.PillsCrafted, 10000)
	case "alchemy_9":
		return metricProgress(int64(len(state.PillRecipes)), 8)
	case "alchemy_10":
		return metricProgress(state.HighQualityPillsCrafted, 100)
	default:
		return false, 0
	}
}

func metricProgress(current int64, target int64) (bool, float64) {
	if target <= 0 {
		return false, 0
	}
	if current < 0 {
		current = 0
	}
	progress := (float64(current) / float64(target)) * 100
	if progress > 100 {
		progress = 100
	}
	progress = math.Round(progress*100) / 100
	return current >= target, progress
}

func applyAchievementReward(state *achievementState, reward achievementReward) bool {
	changed := false
	if reward.Spirit != 0 {
		state.Spirit += float64(reward.Spirit)
		changed = true
	}
	if reward.SpiritRate != 0 {
		state.SpiritRate *= reward.SpiritRate
		changed = true
	}
	if reward.HerbRate != 0 {
		state.HerbRate *= reward.HerbRate
		changed = true
	}
	if reward.AlchemyRate != 0 {
		state.AlchemyRate *= reward.AlchemyRate
		changed = true
	}
	if reward.Luck != 0 {
		state.Luck *= reward.Luck
		changed = true
	}
	return changed
}

func minInt64(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
}
