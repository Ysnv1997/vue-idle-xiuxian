package service

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"math/rand"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/kowming/vue-idle-xiuxian/backend/internal/repository"
)

type ExplorationService struct {
	pool     *pgxpool.Pool
	userRepo *repository.UserRepository
}

func NewExplorationService(pool *pgxpool.Pool, userRepo *repository.UserRepository) *ExplorationService {
	return &ExplorationService{pool: pool, userRepo: userRepo}
}

type ExplorationActionResult struct {
	LocationID       string                     `json:"locationId"`
	LocationName     string                     `json:"locationName"`
	SpiritCost       int64                      `json:"spiritCost"`
	RewardType       string                     `json:"rewardType,omitempty"`
	RewardAmount     int64                      `json:"rewardAmount,omitempty"`
	RewardMultiplier float64                    `json:"rewardMultiplier"`
	EventTriggered   bool                       `json:"eventTriggered"`
	EventName        string                     `json:"eventName,omitempty"`
	Messages         []string                   `json:"messages"`
	Snapshot         *repository.PlayerSnapshot `json:"snapshot"`
}

type InvalidLocationError struct {
	LocationID string
}

func (e *InvalidLocationError) Error() string {
	return fmt.Sprintf("invalid location: %s", e.LocationID)
}

type LocationLockedError struct {
	RequiredLevel int
	CurrentLevel  int
}

func (e *LocationLockedError) Error() string {
	return fmt.Sprintf("location locked: current level %d required level %d", e.CurrentLevel, e.RequiredLevel)
}

func (s *ExplorationService) Start(ctx context.Context, userID uuid.UUID, locationID string) (*ExplorationActionResult, error) {
	location, ok := explorationLocationByID(locationID)
	if !ok {
		return nil, &InvalidLocationError{LocationID: locationID}
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin exploration transaction: %w", err)
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

	state, err := loadExplorationState(ctx, tx, userID)
	if err != nil {
		return nil, err
	}

	if state.Level < location.MinLevel {
		return nil, &LocationLockedError{RequiredLevel: location.MinLevel, CurrentLevel: state.Level}
	}

	if state.Spirit < float64(location.SpiritCost) {
		return nil, &InsufficientSpiritError{Required: float64(location.SpiritCost), Current: state.Spirit}
	}

	state.Spirit -= float64(location.SpiritCost)
	state.ExplorationCount++

	result := &ExplorationActionResult{
		LocationID:       location.ID,
		LocationName:     location.Name,
		SpiritCost:       location.SpiritCost,
		RewardMultiplier: 1,
		Messages:         make([]string, 0, 6),
	}

	if shouldTriggerExplorationEvent(state.Luck) {
		event := rollExplorationEvent()
		if event != nil {
			result.EventTriggered = true
			result.EventName = event.Name
			state.EventTriggered++
			result.Messages = append(result.Messages, fmt.Sprintf("[%s]%s", event.Name, event.Description))
			applyExplorationEventEffect(state, event, result)
		}
	} else {
		rewardMultiplier := calculateExplorationRewardMultiplier(state.Luck)
		result.RewardMultiplier = rewardMultiplier
		rewardRule := rollLocationReward(location.Rewards)
		if rewardRule != nil {
			amount := randomAmount(rewardRule.MinAmount, rewardRule.MaxAmount)
			if rewardMultiplier > 1 {
				amount = int64(math.Floor(float64(amount) * rewardMultiplier))
				result.Messages = append(result.Messages, "福缘加持，获得了更多奖励！")
			}
			applyExplorationReward(state, rewardRule.RewardType, amount, result)
		}
	}

	if err := persistExplorationState(ctx, tx, userID, state); err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit exploration transaction: %w", err)
	}

	snapshot, err := s.userRepo.GetSnapshot(ctx, userID)
	if err != nil {
		return nil, err
	}
	result.Snapshot = snapshot
	return result, nil
}

type explorationState struct {
	Level           int
	Realm           string
	Cultivation     int64
	MaxCultivation  int64
	Spirit          float64
	SpiritRate      float64
	HerbRate        float64
	Luck            float64
	CultivationRate float64
	SpiritStones    int64

	ExplorationCount int64
	EventTriggered   int64
	ItemsFound       int64

	Herbs         []herbItem
	PillFragments map[string]int64
	PillRecipes   []string
}

type herbItem struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	Description string  `json:"description"`
	BaseValue   int64   `json:"baseValue"`
	Category    string  `json:"category"`
	Chance      float64 `json:"chance"`
	Quality     string  `json:"quality"`
	Value       int64   `json:"value"`
}

func ensureExplorationRows(ctx context.Context, tx pgx.Tx, userID uuid.UUID) error {
	const explorationStatsSQL = `
		INSERT INTO player_exploration_stats (user_id, exploration_count, events_triggered, items_found, updated_at)
		VALUES ($1, 0, 0, 0, now())
		ON CONFLICT (user_id) DO NOTHING
	`
	if _, err := tx.Exec(ctx, explorationStatsSQL, userID); err != nil {
		return fmt.Errorf("ensure exploration stats row: %w", err)
	}

	const inventoryStateSQL = `
		INSERT INTO player_inventory_state (user_id, herbs, pill_fragments, pill_recipes, updated_at)
		VALUES ($1, '[]'::jsonb, '{}'::jsonb, '[]'::jsonb, now())
		ON CONFLICT (user_id) DO NOTHING
	`
	if _, err := tx.Exec(ctx, inventoryStateSQL, userID); err != nil {
		return fmt.Errorf("ensure inventory state row: %w", err)
	}
	return nil
}

func loadExplorationState(ctx context.Context, tx pgx.Tx, userID uuid.UUID) (*explorationState, error) {
	const query = `
		SELECT
			pp.level,
			pp.realm,
			pp.cultivation,
			pp.max_cultivation,
			pr.spirit,
			pr.spirit_rate,
			pr.herb_rate,
			pr.luck,
			pr.cultivation_rate,
			pr.spirit_stones,
			pes.exploration_count,
			pes.events_triggered,
			pes.items_found,
			pis.herbs,
			pis.pill_fragments,
			pis.pill_recipes
		FROM player_profiles pp
		JOIN player_resources pr ON pr.user_id = pp.user_id
		JOIN player_exploration_stats pes ON pes.user_id = pp.user_id
		JOIN player_inventory_state pis ON pis.user_id = pp.user_id
		WHERE pp.user_id = $1
		FOR UPDATE OF pp, pr, pes, pis
	`

	state := &explorationState{}
	var herbsRaw []byte
	var pillFragmentsRaw []byte
	var pillRecipesRaw []byte

	if err := tx.QueryRow(ctx, query, userID).Scan(
		&state.Level,
		&state.Realm,
		&state.Cultivation,
		&state.MaxCultivation,
		&state.Spirit,
		&state.SpiritRate,
		&state.HerbRate,
		&state.Luck,
		&state.CultivationRate,
		&state.SpiritStones,
		&state.ExplorationCount,
		&state.EventTriggered,
		&state.ItemsFound,
		&herbsRaw,
		&pillFragmentsRaw,
		&pillRecipesRaw,
	); err != nil {
		return nil, fmt.Errorf("load exploration state: %w", err)
	}

	if err := json.Unmarshal(herbsRaw, &state.Herbs); err != nil {
		state.Herbs = []herbItem{}
	}
	if state.Herbs == nil {
		state.Herbs = []herbItem{}
	}

	if err := json.Unmarshal(pillFragmentsRaw, &state.PillFragments); err != nil {
		state.PillFragments = map[string]int64{}
	}
	if state.PillFragments == nil {
		state.PillFragments = map[string]int64{}
	}

	if err := json.Unmarshal(pillRecipesRaw, &state.PillRecipes); err != nil {
		state.PillRecipes = []string{}
	}
	if state.PillRecipes == nil {
		state.PillRecipes = []string{}
	}

	return state, nil
}

func persistExplorationState(ctx context.Context, tx pgx.Tx, userID uuid.UUID, state *explorationState) error {
	const updateProfileSQL = `
		UPDATE player_profiles
		SET level = $2, realm = $3, cultivation = $4, max_cultivation = $5, updated_at = now()
		WHERE user_id = $1
	`
	if _, err := tx.Exec(ctx, updateProfileSQL, userID, state.Level, state.Realm, state.Cultivation, state.MaxCultivation); err != nil {
		return fmt.Errorf("update exploration profile state: %w", err)
	}

	const updateResourcesSQL = `
		UPDATE player_resources
		SET spirit = $2, spirit_rate = $3, spirit_stones = $4, updated_at = now()
		WHERE user_id = $1
	`
	if _, err := tx.Exec(ctx, updateResourcesSQL, userID, state.Spirit, state.SpiritRate, state.SpiritStones); err != nil {
		return fmt.Errorf("update exploration resources state: %w", err)
	}

	herbsJSON, err := json.Marshal(state.Herbs)
	if err != nil {
		return fmt.Errorf("marshal herbs: %w", err)
	}
	pillFragmentsJSON, err := json.Marshal(state.PillFragments)
	if err != nil {
		return fmt.Errorf("marshal pill fragments: %w", err)
	}
	pillRecipesJSON, err := json.Marshal(state.PillRecipes)
	if err != nil {
		return fmt.Errorf("marshal pill recipes: %w", err)
	}

	const updateInventorySQL = `
		UPDATE player_inventory_state
		SET herbs = $2::jsonb, pill_fragments = $3::jsonb, pill_recipes = $4::jsonb, updated_at = now()
		WHERE user_id = $1
	`
	if _, err := tx.Exec(ctx, updateInventorySQL, userID, string(herbsJSON), string(pillFragmentsJSON), string(pillRecipesJSON)); err != nil {
		return fmt.Errorf("update exploration inventory state: %w", err)
	}

	const updateExplorationStatsSQL = `
		UPDATE player_exploration_stats
		SET exploration_count = $2, events_triggered = $3, items_found = $4, updated_at = now()
		WHERE user_id = $1
	`
	if _, err := tx.Exec(ctx, updateExplorationStatsSQL, userID, state.ExplorationCount, state.EventTriggered, state.ItemsFound); err != nil {
		return fmt.Errorf("update exploration stats state: %w", err)
	}

	return nil
}

func shouldTriggerExplorationEvent(luck float64) bool {
	return rand.Float64() < (0.3 * luck)
}

func calculateExplorationRewardMultiplier(luck float64) float64 {
	if rand.Float64() < (0.5 * luck) {
		return 1.5
	}
	return 1
}

func rollExplorationEvent() *explorationEvent {
	for _, event := range explorationEvents {
		if rand.Float64() <= event.Chance {
			picked := event
			return &picked
		}
	}
	return nil
}

func rollLocationReward(rewards []locationRewardRule) *locationRewardRule {
	randNum := rand.Float64()
	cumulative := 0.0
	for _, reward := range rewards {
		cumulative += reward.Chance
		if randNum <= cumulative {
			picked := reward
			return &picked
		}
	}
	return nil
}

func randomAmount(minAmount, maxAmount int64) int64 {
	if maxAmount <= minAmount {
		return minAmount
	}
	return minAmount + rand.Int63n(maxAmount-minAmount+1)
}

func applyExplorationEventEffect(state *explorationState, event *explorationEvent, result *ExplorationActionResult) {
	switch event.ID {
	case "ancient_tablet":
		bonus := int64(math.Floor(30 * (float64(state.Level)/5 + 1)))
		state.Cultivation += bonus
		result.Messages = append(result.Messages, fmt.Sprintf("[古老石碑]领悟石碑上的功法，获得%d点修为", bonus))
	case "spirit_spring":
		bonus := int64(math.Floor(60 * (float64(state.Level)/3 + 1)))
		state.Spirit += float64(bonus)
		result.Messages = append(result.Messages, fmt.Sprintf("[灵泉]饮用灵泉，灵力增加%d点", bonus))
	case "ancient_master":
		cultivationBonus := int64(math.Floor(120 * (float64(state.Level)/2 + 1)))
		spiritBonus := int64(math.Floor(180 * (float64(state.Level)/2 + 1)))
		state.Cultivation += cultivationBonus
		state.Spirit += float64(spiritBonus)
		result.Messages = append(result.Messages, fmt.Sprintf("[古修遗府]获得上古大能传承，修为增加%d点，灵力增加%d点", cultivationBonus, spiritBonus))
	case "monster_attack":
		damage := int64(math.Floor(80 * (float64(state.Level)/4 + 1)))
		state.Spirit = maxFloat64(0, state.Spirit-float64(damage))
		result.Messages = append(result.Messages, fmt.Sprintf("[妖兽袭击]与妖兽激战，损失%d点灵力", damage))
	case "cultivation_deviation":
		damage := int64(math.Floor(50 * (float64(state.Level)/3 + 1)))
		state.Cultivation = maxInt64(0, state.Cultivation-damage)
		result.Messages = append(result.Messages, fmt.Sprintf("[走火入魔]走火入魔，损失%d点修为", damage))
	case "treasure_trove":
		stoneBonus := int64(math.Floor(30 * (float64(state.Level)/2 + 1)))
		state.SpiritStones += stoneBonus
		result.Messages = append(result.Messages, fmt.Sprintf("[秘境宝藏]发现宝藏，获得%d颗灵石", stoneBonus))
	case "enlightenment":
		bonus := int64(math.Floor(50 * (float64(state.Level)/4 + 1)))
		state.Cultivation += bonus
		state.SpiritRate *= 1.05
		result.Messages = append(result.Messages, fmt.Sprintf("[顿悟]突然顿悟，获得%d点修为，打坐恢复效率提升5%%", bonus))
	case "qi_deviation":
		damage := int64(math.Floor(60 * (float64(state.Level)/3 + 1)))
		state.Spirit = maxFloat64(0, state.Spirit-float64(damage))
		state.Cultivation = maxInt64(0, state.Cultivation-damage)
		result.Messages = append(result.Messages, fmt.Sprintf("[心魔侵扰]遭受心魔侵扰，损失%d点灵力和修为", damage))
	}
}

func applyExplorationReward(state *explorationState, rewardType string, amount int64, result *ExplorationActionResult) {
	result.RewardType = rewardType
	result.RewardAmount = amount

	switch rewardType {
	case "spirit_stone":
		state.SpiritStones += amount
		result.Messages = append(result.Messages, fmt.Sprintf("[灵石获取]获得%d颗灵石", amount))
	case "herb":
		herbAmount := amount
		if state.HerbRate <= 0 {
			state.HerbRate = 1
		}
		if state.HerbRate > 1 {
			herbAmount = int64(math.Floor(float64(amount) * state.HerbRate))
			if herbAmount < 1 {
				herbAmount = 1
			}
		}
		result.RewardAmount = herbAmount
		for i := int64(0); i < herbAmount; i++ {
			herb := randomHerbItem()
			if herb.ID == "" {
				continue
			}
			state.Herbs = append(state.Herbs, herb)
			state.ItemsFound++
			result.Messages = append(result.Messages, fmt.Sprintf("[灵草获取]获得%s品质的%s", herbQualityDisplayName(herb.Quality), herb.Name))
		}
	case "cultivation":
		state.Cultivation += amount
		result.Messages = append(result.Messages, fmt.Sprintf("[修为获取]获得%d点修为", amount))
		if state.Cultivation >= state.MaxCultivation {
			if applyBreakthroughForExploration(state) {
				result.Messages = append(result.Messages, fmt.Sprintf("[突破]突破成功！当前境界：%s", state.Realm))
			}
		}
	case "pill_fragment":
		for i := int64(0); i < amount; i++ {
			recipe := randomPillRecipe()
			if recipe.ID == "" {
				continue
			}
			gainPillFragment(state, recipe)
			state.ItemsFound++
			result.Messages = append(result.Messages, fmt.Sprintf("[丹方获取]获得%s的丹方残页", recipe.Name))
		}
	}
}

func applyBreakthroughForExploration(state *explorationState) bool {
	if state.Level >= realmCount() {
		return false
	}

	state.Level++
	realm := realmByLevel(state.Level)
	state.Realm = realm.Name
	state.MaxCultivation = realm.MaxCultivation
	state.Cultivation = 0
	state.Spirit += float64(100 * state.Level)
	return true
}

func randomHerbItem() herbItem {
	randNum := rand.Float64()
	cumulative := 0.0
	for _, definition := range herbDefinitions {
		cumulative += definition.Chance
		if randNum <= cumulative {
			quality, multiplier := randomHerbQuality()
			return herbItem{
				ID:          definition.ID,
				Name:        definition.Name,
				Description: definition.Description,
				BaseValue:   definition.BaseValue,
				Category:    definition.Category,
				Chance:      definition.Chance,
				Quality:     quality,
				Value:       int64(math.Floor(float64(definition.BaseValue) * multiplier)),
			}
		}
	}
	return herbItem{}
}

func randomHerbQuality() (string, float64) {
	randNum := rand.Float64()
	switch {
	case randNum < 0.5:
		return "common", 1
	case randNum < 0.8:
		return "uncommon", 1.5
	case randNum < 0.95:
		return "rare", 2
	case randNum < 0.99:
		return "epic", 3
	default:
		return "legendary", 5
	}
}

func herbQualityDisplayName(quality string) string {
	switch quality {
	case "common":
		return "普通"
	case "uncommon":
		return "优质"
	case "rare":
		return "稀有"
	case "epic":
		return "极品"
	case "legendary":
		return "仙品"
	default:
		return "未知"
	}
}

func randomPillRecipe() pillRecipeDefinition {
	if len(pillRecipeDefinitions) == 0 {
		return pillRecipeDefinition{}
	}
	idx := rand.Intn(len(pillRecipeDefinitions))
	return pillRecipeDefinitions[idx]
}

func gainPillFragment(state *explorationState, recipe pillRecipeDefinition) {
	if state.PillFragments == nil {
		state.PillFragments = map[string]int64{}
	}
	state.PillFragments[recipe.ID]++

	if state.PillFragments[recipe.ID] >= recipe.FragmentsNeeded {
		state.PillFragments[recipe.ID] -= recipe.FragmentsNeeded
		if !containsString(state.PillRecipes, recipe.ID) {
			state.PillRecipes = append(state.PillRecipes, recipe.ID)
		}
	}
}

func containsString(items []string, target string) bool {
	for _, item := range items {
		if item == target {
			return true
		}
	}
	return false
}

func maxFloat64(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}
