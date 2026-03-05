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

type AlchemyService struct {
	pool     *pgxpool.Pool
	userRepo *repository.UserRepository
}

func NewAlchemyService(pool *pgxpool.Pool, userRepo *repository.UserRepository) *AlchemyService {
	return &AlchemyService{pool: pool, userRepo: userRepo}
}

type AlchemyCraftResult struct {
	Success  bool                       `json:"success"`
	Message  string                     `json:"message"`
	RecipeID string                     `json:"recipeId"`
	Snapshot *repository.PlayerSnapshot `json:"snapshot"`
}

type RecipeNotFoundError struct {
	RecipeID string
}

func (e *RecipeNotFoundError) Error() string {
	return fmt.Sprintf("recipe not found: %s", e.RecipeID)
}

type RecipeLockedError struct {
	RecipeID string
}

func (e *RecipeLockedError) Error() string {
	return fmt.Sprintf("recipe locked: %s", e.RecipeID)
}

type MissingMaterial struct {
	Herb     string `json:"herb"`
	Required int64  `json:"required"`
	Current  int64  `json:"current"`
}

type InsufficientMaterialsError struct {
	Missing []MissingMaterial
}

func (e *InsufficientMaterialsError) Error() string {
	return "insufficient materials"
}

func (s *AlchemyService) Craft(ctx context.Context, userID uuid.UUID, recipeID string) (*AlchemyCraftResult, error) {
	recipe, ok := alchemyRecipeByID(recipeID)
	if !ok {
		return nil, &RecipeNotFoundError{RecipeID: recipeID}
	}

	gradeDef, ok := alchemyGradeDefinitions[recipe.Grade]
	if !ok {
		return nil, fmt.Errorf("unknown alchemy grade: %s", recipe.Grade)
	}
	typeDef, ok := alchemyTypeDefinitions[recipe.Type]
	if !ok {
		return nil, fmt.Errorf("unknown alchemy type: %s", recipe.Type)
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin alchemy transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if err := ensureAlchemyRows(ctx, tx, userID); err != nil {
		return nil, err
	}

	state, err := loadAlchemyState(ctx, tx, userID)
	if err != nil {
		return nil, err
	}

	if !containsString(state.PillRecipes, recipe.ID) {
		return nil, &RecipeLockedError{RecipeID: recipe.ID}
	}

	missing := findMissingMaterials(state.Herbs, recipe.Materials)
	if len(missing) > 0 {
		return nil, &InsufficientMaterialsError{Missing: missing}
	}

	if state.AlchemyRate <= 0 {
		state.AlchemyRate = 1
	}
	successRate := gradeDef.SuccessRate * state.Luck * state.AlchemyRate
	if successRate < 0 {
		successRate = 0
	}
	if successRate > 1 {
		successRate = 1
	}

	crafted := rand.Float64() <= successRate
	result := &AlchemyCraftResult{
		Success:  crafted,
		Message:  "炼制失败",
		RecipeID: recipe.ID,
	}

	if crafted {
		state.Herbs = consumeAlchemyMaterials(state.Herbs, recipe.Materials)
		state.Items = append(state.Items, buildCraftedPillItem(recipe, gradeDef, typeDef, state.Level))
		state.PillsCrafted++
		if isHighQualityAlchemyRecipe(recipe.Grade) {
			state.HighQualityPillsCrafted++
		}
		result.Message = "炼制成功"

		if err := persistAlchemyState(ctx, tx, userID, state); err != nil {
			return nil, err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit alchemy transaction: %w", err)
	}

	snapshot, err := s.userRepo.GetSnapshot(ctx, userID)
	if err != nil {
		return nil, err
	}
	result.Snapshot = snapshot

	return result, nil
}

type alchemyState struct {
	Level                   int
	Luck                    float64
	AlchemyRate             float64
	PillsCrafted            int64
	HighQualityPillsCrafted int64
	Herbs                   []herbItem
	PillRecipes             []string
	Items                   []map[string]any
}

func ensureAlchemyRows(ctx context.Context, tx pgx.Tx, userID uuid.UUID) error {
	const inventoryStateSQL = `
		INSERT INTO player_inventory_state (user_id, herbs, pill_fragments, pill_recipes, items, updated_at)
		VALUES ($1, '[]'::jsonb, '{}'::jsonb, '[]'::jsonb, '[]'::jsonb, now())
		ON CONFLICT (user_id) DO NOTHING
	`
	if _, err := tx.Exec(ctx, inventoryStateSQL, userID); err != nil {
		return fmt.Errorf("ensure alchemy inventory state row: %w", err)
	}

	const alchemyStatsSQL = `
		INSERT INTO player_alchemy_stats (user_id, pills_crafted, high_quality_pills_crafted, updated_at)
		VALUES ($1, 0, 0, now())
		ON CONFLICT (user_id) DO NOTHING
	`
	if _, err := tx.Exec(ctx, alchemyStatsSQL, userID); err != nil {
		return fmt.Errorf("ensure alchemy stats row: %w", err)
	}
	return nil
}

func loadAlchemyState(ctx context.Context, tx pgx.Tx, userID uuid.UUID) (*alchemyState, error) {
	const query = `
		SELECT
			pp.level,
			pr.luck,
			pr.alchemy_rate,
			pas.pills_crafted,
			pas.high_quality_pills_crafted,
			pis.herbs,
			pis.pill_recipes,
			COALESCE(pis.items, '[]'::jsonb)
		FROM player_profiles pp
		JOIN player_resources pr ON pr.user_id = pp.user_id
		JOIN player_alchemy_stats pas ON pas.user_id = pp.user_id
		JOIN player_inventory_state pis ON pis.user_id = pp.user_id
		WHERE pp.user_id = $1
		FOR UPDATE OF pp, pr, pas, pis
	`

	state := &alchemyState{}
	var herbsRaw []byte
	var pillRecipesRaw []byte
	var itemsRaw []byte
	if err := tx.QueryRow(ctx, query, userID).Scan(
		&state.Level,
		&state.Luck,
		&state.AlchemyRate,
		&state.PillsCrafted,
		&state.HighQualityPillsCrafted,
		&herbsRaw,
		&pillRecipesRaw,
		&itemsRaw,
	); err != nil {
		return nil, fmt.Errorf("load alchemy state: %w", err)
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

	return state, nil
}

func findMissingMaterials(herbs []herbItem, required []alchemyMaterial) []MissingMaterial {
	countMap := map[string]int64{}
	for _, herb := range herbs {
		countMap[herb.ID]++
	}

	missing := make([]MissingMaterial, 0)
	for _, material := range required {
		current := countMap[material.Herb]
		if current < material.Count {
			missing = append(missing, MissingMaterial{
				Herb:     material.Herb,
				Required: material.Count,
				Current:  current,
			})
		}
	}
	return missing
}

func consumeAlchemyMaterials(herbs []herbItem, required []alchemyMaterial) []herbItem {
	left := make(map[string]int64, len(required))
	for _, material := range required {
		left[material.Herb] = material.Count
	}

	nextHerbs := make([]herbItem, 0, len(herbs))
	for _, herb := range herbs {
		if remain, ok := left[herb.ID]; ok && remain > 0 {
			left[herb.ID] = remain - 1
			continue
		}
		nextHerbs = append(nextHerbs, herb)
	}
	return nextHerbs
}

func buildCraftedPillItem(recipe alchemyRecipeDefinition, grade alchemyGradeDefinition, kind alchemyTypeDefinition, level int) map[string]any {
	levelMultiplier := 1 + float64(maxInt(0, level-1))*0.1
	value := recipe.BaseEffect.Value * kind.EffectMultiplier * levelMultiplier
	value = math.Round(value*10000) / 10000

	return map[string]any{
		"id":          fmt.Sprintf("%s_%d", recipe.ID, time.Now().UnixMilli()),
		"name":        recipe.Name,
		"description": recipe.Description,
		"type":        "pill",
		"effect": map[string]any{
			"type":        recipe.BaseEffect.Type,
			"value":       value,
			"duration":    recipe.BaseEffect.Duration,
			"successRate": grade.SuccessRate,
		},
	}
}

func persistAlchemyState(ctx context.Context, tx pgx.Tx, userID uuid.UUID, state *alchemyState) error {
	herbsJSON, err := json.Marshal(state.Herbs)
	if err != nil {
		return fmt.Errorf("marshal herbs for alchemy: %w", err)
	}

	itemsJSON, err := json.Marshal(state.Items)
	if err != nil {
		return fmt.Errorf("marshal items for alchemy: %w", err)
	}

	const updateSQL = `
		UPDATE player_inventory_state
		SET herbs = $2::jsonb, items = $3::jsonb, updated_at = now()
		WHERE user_id = $1
	`
	if _, err := tx.Exec(ctx, updateSQL, userID, string(herbsJSON), string(itemsJSON)); err != nil {
		return fmt.Errorf("update alchemy inventory state: %w", err)
	}

	const updateStatsSQL = `
		UPDATE player_alchemy_stats
		SET pills_crafted = $2, high_quality_pills_crafted = $3, updated_at = now()
		WHERE user_id = $1
	`
	if _, err := tx.Exec(ctx, updateStatsSQL, userID, state.PillsCrafted, state.HighQualityPillsCrafted); err != nil {
		return fmt.Errorf("update alchemy stats: %w", err)
	}

	return nil
}

func isHighQualityAlchemyRecipe(grade string) bool {
	switch grade {
	case "grade8", "grade9":
		return true
	default:
		return false
	}
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
