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

type GachaService struct {
	pool     *pgxpool.Pool
	userRepo *repository.UserRepository
}

func NewGachaService(pool *pgxpool.Pool, userRepo *repository.UserRepository) *GachaService {
	return &GachaService{pool: pool, userRepo: userRepo}
}

type GachaDrawInput struct {
	GachaType                string
	Times                    int
	WishlistEnabled          bool
	SelectedWishEquipQuality string
	SelectedWishPetRarity    string
	AutoSellQualities        []string
	AutoReleaseRarities      []string
}

type GachaDrawResult struct {
	GachaType         string                     `json:"gachaType"`
	Times             int                        `json:"times"`
	CostPerDraw       int64                      `json:"costPerDraw"`
	TotalCost         int64                      `json:"totalCost"`
	Results           []map[string]any           `json:"results"`
	AutoSoldCount     int                        `json:"autoSoldCount"`
	AutoSoldIncome    int64                      `json:"autoSoldIncome"`
	AutoReleasedCount int                        `json:"autoReleasedCount"`
	Snapshot          *repository.PlayerSnapshot `json:"snapshot"`
}

type InvalidGachaTypeError struct {
	GachaType string
}

func (e *InvalidGachaTypeError) Error() string {
	return fmt.Sprintf("invalid gacha type: %s", e.GachaType)
}

type InvalidGachaTimesError struct {
	Times int
}

func (e *InvalidGachaTimesError) Error() string {
	return fmt.Sprintf("invalid gacha times: %d", e.Times)
}

type InsufficientSpiritStonesError struct {
	Required int64
	Current  int64
}

func (e *InsufficientSpiritStonesError) Error() string {
	return fmt.Sprintf("insufficient spirit stones: required %d, current %d", e.Required, e.Current)
}

type PetInventoryFullError struct {
	Limit   int
	Current int
}

func (e *PetInventoryFullError) Error() string {
	return fmt.Sprintf("pet inventory full: %d/%d", e.Current, e.Limit)
}

type gachaState struct {
	Level           int
	SpiritStones    int64
	ReinforceStones int64
	PetEssence      int64
	Items           []map[string]any
}

func (s *GachaService) Draw(ctx context.Context, userID uuid.UUID, input GachaDrawInput) (*GachaDrawResult, error) {
	if input.Times <= 0 || input.Times > 100 {
		return nil, &InvalidGachaTimesError{Times: input.Times}
	}
	if input.GachaType != "all" && input.GachaType != "equipment" && input.GachaType != "pet" {
		return nil, &InvalidGachaTypeError{GachaType: input.GachaType}
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin gacha transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if err := ensureGachaRows(ctx, tx, userID); err != nil {
		return nil, err
	}

	state, err := loadGachaState(ctx, tx, userID)
	if err != nil {
		return nil, err
	}

	if input.GachaType != "equipment" {
		petCount := gachaCountPets(state.Items)
		if petCount >= 100 {
			return nil, &PetInventoryFullError{Limit: 100, Current: petCount}
		}
	}

	costPerDraw := int64(100)
	if input.WishlistEnabled {
		costPerDraw = 200
	}
	totalCost := int64(input.Times) * costPerDraw
	if state.SpiritStones < totalCost {
		return nil, &InsufficientSpiritStonesError{Required: totalCost, Current: state.SpiritStones}
	}

	state.SpiritStones -= totalCost

	result := &GachaDrawResult{
		GachaType:   input.GachaType,
		Times:       input.Times,
		CostPerDraw: costPerDraw,
		TotalCost:   totalCost,
		Results:     make([]map[string]any, 0, input.Times),
	}

	for i := 0; i < input.Times; i++ {
		drawItem := gachaDrawSingle(input, state.Level)
		if drawItem == nil {
			continue
		}

		result.Results = append(result.Results, drawItem)

		itemType := gachaReadString(drawItem["type"])
		if itemType == "pet" {
			rarity := gachaReadString(drawItem["rarity"])
			if rarityConfig, ok := gachaPetRarities[rarity]; ok {
				state.PetEssence += rarityConfig.EssenceBonus
			}

			if gachaShouldAutoRelease(input.AutoReleaseRarities, rarity) {
				result.AutoReleasedCount++
				continue
			}
		} else {
			quality := gachaReadString(drawItem["quality"])
			if gachaShouldAutoSell(input.AutoSellQualities, quality) {
				price := gachaEquipmentPriceByQuality[quality]
				if price <= 0 {
					price = 1
				}
				state.ReinforceStones += price
				result.AutoSoldCount++
				result.AutoSoldIncome += price
				continue
			}
		}

		state.Items = append(state.Items, drawItem)
	}

	if err := persistGachaState(ctx, tx, userID, state); err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit gacha transaction: %w", err)
	}

	snapshot, err := s.userRepo.GetSnapshot(ctx, userID)
	if err != nil {
		return nil, err
	}
	result.Snapshot = snapshot
	return result, nil
}

func ensureGachaRows(ctx context.Context, tx pgx.Tx, userID uuid.UUID) error {
	const inventoryStateSQL = `
		INSERT INTO player_inventory_state (user_id, herbs, pill_fragments, pill_recipes, items, updated_at)
		VALUES ($1, '[]'::jsonb, '{}'::jsonb, '[]'::jsonb, '[]'::jsonb, now())
		ON CONFLICT (user_id) DO NOTHING
	`
	if _, err := tx.Exec(ctx, inventoryStateSQL, userID); err != nil {
		return fmt.Errorf("ensure gacha inventory state row: %w", err)
	}
	return nil
}

func loadGachaState(ctx context.Context, tx pgx.Tx, userID uuid.UUID) (*gachaState, error) {
	const query = `
		SELECT
			pp.level,
			pr.spirit_stones,
			pr.reinforce_stones,
			COALESCE(pr.pet_essence, 0),
			COALESCE(pis.items, '[]'::jsonb)
		FROM player_profiles pp
		JOIN player_resources pr ON pr.user_id = pp.user_id
		JOIN player_inventory_state pis ON pis.user_id = pp.user_id
		WHERE pp.user_id = $1
		FOR UPDATE OF pp, pr, pis
	`

	state := &gachaState{}
	var itemsRaw []byte
	if err := tx.QueryRow(ctx, query, userID).Scan(
		&state.Level,
		&state.SpiritStones,
		&state.ReinforceStones,
		&state.PetEssence,
		&itemsRaw,
	); err != nil {
		return nil, fmt.Errorf("load gacha state: %w", err)
	}

	if err := json.Unmarshal(itemsRaw, &state.Items); err != nil {
		state.Items = []map[string]any{}
	}
	if state.Items == nil {
		state.Items = []map[string]any{}
	}

	return state, nil
}

func persistGachaState(ctx context.Context, tx pgx.Tx, userID uuid.UUID, state *gachaState) error {
	const updateResourcesSQL = `
		UPDATE player_resources
		SET spirit_stones = $2, reinforce_stones = $3, pet_essence = $4, updated_at = now()
		WHERE user_id = $1
	`
	if _, err := tx.Exec(ctx, updateResourcesSQL, userID, state.SpiritStones, state.ReinforceStones, state.PetEssence); err != nil {
		return fmt.Errorf("update gacha resources state: %w", err)
	}

	itemsJSON, err := json.Marshal(state.Items)
	if err != nil {
		return fmt.Errorf("marshal gacha items: %w", err)
	}

	const updateInventorySQL = `
		UPDATE player_inventory_state
		SET items = $2::jsonb, updated_at = now()
		WHERE user_id = $1
	`
	if _, err := tx.Exec(ctx, updateInventorySQL, userID, string(itemsJSON)); err != nil {
		return fmt.Errorf("update gacha inventory state: %w", err)
	}

	return nil
}

func gachaDrawSingle(input GachaDrawInput, level int) map[string]any {
	switch input.GachaType {
	case "equipment":
		return gachaDrawSingleEquipment(level, input)
	case "pet":
		return gachaDrawSinglePet(input)
	default:
		if rand.Float64() < 0.5 {
			return gachaDrawSingleEquipment(level, GachaDrawInput{})
		}
		return gachaDrawSinglePet(GachaDrawInput{})
	}
}

func gachaDrawSingleEquipment(level int, input GachaDrawInput) map[string]any {
	probabilities := gachaAdjustedEquipmentProbabilities(input.WishlistEnabled, input.SelectedWishEquipQuality)
	r := rand.Float64()
	accumulated := 0.0
	quality := "common"
	for _, q := range gachaEquipmentQualityOrder {
		accumulated += probabilities[q]
		if r <= accumulated {
			quality = q
			break
		}
	}
	return gachaGenerateEquipment(level, quality)
}

func gachaDrawSinglePet(input GachaDrawInput) map[string]any {
	probabilities := gachaAdjustedPetProbabilities(input.WishlistEnabled, input.SelectedWishPetRarity)
	r := rand.Float64()
	accumulated := 0.0
	rarity := "mortal"
	for _, key := range gachaPetRarityOrder {
		accumulated += probabilities[key]
		if r <= accumulated {
			rarity = key
			break
		}
	}
	return gachaGeneratePet(rarity)
}

func gachaAdjustedEquipmentProbabilities(wishlistEnabled bool, selectedQuality string) map[string]float64 {
	base := make(map[string]float64, len(gachaEquipmentQualities))
	for _, key := range gachaEquipmentQualityOrder {
		base[key] = gachaEquipmentQualities[key].Probability
	}

	if !wishlistEnabled {
		return base
	}
	if _, ok := base[selectedQuality]; !ok || selectedQuality == "" {
		return base
	}

	bonus := math.Min(1.0, 0.2/base[selectedQuality])
	base[selectedQuality] = base[selectedQuality] * (1 + bonus)

	totalOther := 0.0
	for _, key := range gachaEquipmentQualityOrder {
		if key == selectedQuality {
			continue
		}
		totalOther += base[key]
	}
	if totalOther <= 0 {
		return base
	}

	reductionFactor := (1 - base[selectedQuality]) / totalOther
	for _, key := range gachaEquipmentQualityOrder {
		if key == selectedQuality {
			continue
		}
		base[key] = base[key] * reductionFactor
	}
	return base
}

func gachaAdjustedPetProbabilities(wishlistEnabled bool, selectedRarity string) map[string]float64 {
	base := make(map[string]float64, len(gachaPetRarities))
	for _, key := range gachaPetRarityOrder {
		base[key] = gachaPetRarities[key].Probability
	}

	if !wishlistEnabled {
		return base
	}
	if _, ok := base[selectedRarity]; !ok || selectedRarity == "" {
		return base
	}

	bonus := math.Min(1.0, 0.2/base[selectedRarity])
	base[selectedRarity] = base[selectedRarity] * (1 + bonus)

	totalOther := 0.0
	for _, key := range gachaPetRarityOrder {
		if key == selectedRarity {
			continue
		}
		totalOther += base[key]
	}
	if totalOther <= 0 {
		return base
	}

	reductionFactor := (1 - base[selectedRarity]) / totalOther
	for _, key := range gachaPetRarityOrder {
		if key == selectedRarity {
			continue
		}
		base[key] = base[key] * reductionFactor
	}
	return base
}

func gachaGenerateEquipment(level int, quality string) map[string]any {
	if level <= 0 {
		level = 1
	}

	equipType := gachaEquipmentTypes[rand.Intn(len(gachaEquipmentTypes))]
	qualityInfo, ok := gachaEquipmentQualities[quality]
	if !ok {
		qualityInfo = gachaEquipmentQualities["common"]
		quality = "common"
	}

	randomLevel := rand.Intn(level) + 1
	levelMod := 1 + float64(randomLevel)*0.1
	baseStats := map[string]any{}

	statDefs := gachaEquipmentBaseStats[equipType.ID]
	for stat, cfg := range statDefs {
		base := cfg.Min + rand.Float64()*(cfg.Max-cfg.Min)
		value := base * qualityInfo.StatMod * levelMod

		if gachaIsPercentStat(stat) {
			// Keep the legacy scaling behavior from the current frontend.
			baseStats[stat] = math.Round(value*1) / 100
		} else {
			baseStats[stat] = math.Round(value)
		}
	}

	return map[string]any{
		"id":            fmt.Sprintf("eq_%d_%d", time.Now().UnixMilli(), rand.Int63n(1000000)),
		"name":          gachaGenerateEquipmentName(equipType, quality),
		"type":          equipType.ID,
		"slot":          equipType.Slot,
		"quality":       quality,
		"level":         randomLevel,
		"requiredRealm": randomLevel,
		"stats":         baseStats,
		"equipType":     equipType.ID,
		"qualityInfo": map[string]any{
			"name":  qualityInfo.Name,
			"color": qualityInfo.Color,
		},
	}
}

func gachaGenerateEquipmentName(equipType gachaEquipmentType, quality string) string {
	prefix := equipType.Prefixes[rand.Intn(len(equipType.Prefixes))]
	suffix := ""
	switch quality {
	case "mythic":
		suffix = "·神"
	case "legendary":
		suffix = "·圣"
	case "epic":
		suffix = "·仙"
	case "rare":
		suffix = "·天"
	case "uncommon":
		suffix = "·道"
	default:
		suffix = ""
	}
	return prefix + equipType.Name + suffix
}

func gachaGeneratePet(rarity string) map[string]any {
	rarityCfg, ok := gachaPetRarities[rarity]
	if !ok {
		rarity = "mortal"
		rarityCfg = gachaPetRarities[rarity]
	}

	pool := gachaPetPool[rarity]
	if len(pool) == 0 {
		pool = gachaPetPool["mortal"]
	}
	template := pool[rand.Intn(len(pool))]

	return map[string]any{
		"id":          fmt.Sprintf("pet_%d_%d", time.Now().UnixMilli(), rand.Int63n(1000000)),
		"name":        template.Name,
		"description": template.Description,
		"rarity":      rarity,
		"type":        "pet",
		"quality": map[string]any{
			"strength":     rand.Intn(10) + 1,
			"agility":      rand.Intn(10) + 1,
			"intelligence": rand.Intn(10) + 1,
			"constitution": rand.Intn(10) + 1,
		},
		"power":            0,
		"experience":       0,
		"maxExperience":    100,
		"level":            1,
		"star":             0,
		"upgradeItems":     gachaUpgradeItemsByRarity(rarity),
		"combatAttributes": gachaGeneratePetCombatAttributes(rarity),
		"rarityInfo": map[string]any{
			"name":  rarityCfg.Name,
			"color": rarityCfg.Color,
		},
	}
}

func gachaUpgradeItemsByRarity(rarity string) int {
	switch rarity {
	case "divine":
		return 5
	case "celestial":
		return 4
	case "mystic":
		return 3
	case "spiritual":
		return 2
	default:
		return 1
	}
}

func gachaGeneratePetCombatAttributes(rarity string) map[string]any {
	baseMultiplier, percentMultiplier := gachaRarityMultipliers(rarity)
	genBase := func(min, max float64, multiplier float64) float64 {
		val := min + rand.Float64()*(max-min)
		return math.Round(val * multiplier)
	}
	genPercent := func(min, max float64) float64 {
		val := min + rand.Float64()*(max-min)
		return math.Min(1, math.Round(val*percentMultiplier*100)/100)
	}

	return map[string]any{
		"attack":            genBase(10, 15, baseMultiplier),
		"health":            genBase(100, 120, baseMultiplier),
		"defense":           genBase(5, 8, baseMultiplier),
		"speed":             genBase(10, 15, baseMultiplier*0.6),
		"critRate":          genPercent(0.05, 0.1),
		"comboRate":         genPercent(0.05, 0.1),
		"counterRate":       genPercent(0.05, 0.1),
		"stunRate":          genPercent(0.05, 0.1),
		"dodgeRate":         genPercent(0.05, 0.1),
		"vampireRate":       genPercent(0.05, 0.1),
		"critResist":        genPercent(0.05, 0.1),
		"comboResist":       genPercent(0.05, 0.1),
		"counterResist":     genPercent(0.05, 0.1),
		"stunResist":        genPercent(0.05, 0.1),
		"dodgeResist":       genPercent(0.05, 0.1),
		"vampireResist":     genPercent(0.05, 0.1),
		"healBoost":         genPercent(0.05, 0.1),
		"critDamageBoost":   genPercent(0.05, 0.1),
		"critDamageReduce":  genPercent(0.05, 0.1),
		"finalDamageBoost":  genPercent(0.05, 0.1),
		"finalDamageReduce": genPercent(0.05, 0.1),
		"combatBoost":       genPercent(0.05, 0.1),
		"resistanceBoost":   genPercent(0.05, 0.1),
	}
}

func gachaRarityMultipliers(rarity string) (float64, float64) {
	switch rarity {
	case "divine":
		return 5, 2
	case "celestial":
		return 4, 1.8
	case "mystic":
		return 3, 1.6
	case "spiritual":
		return 2, 1.4
	default:
		return 1, 1
	}
}

func gachaIsPercentStat(stat string) bool {
	switch stat {
	case "critRate", "critDamageBoost", "dodgeRate", "vampireRate", "finalDamageBoost", "finalDamageReduce":
		return true
	default:
		return false
	}
}

func gachaShouldAutoRelease(settings []string, rarity string) bool {
	if len(settings) == 0 {
		return false
	}
	for _, setting := range settings {
		if setting == "all" || setting == rarity {
			return true
		}
	}
	return false
}

func gachaShouldAutoSell(settings []string, quality string) bool {
	if len(settings) == 0 {
		return false
	}
	for _, setting := range settings {
		if setting == "all" || setting == quality {
			return true
		}
	}
	return false
}

func gachaCountPets(items []map[string]any) int {
	count := 0
	for _, item := range items {
		if gachaReadString(item["type"]) == "pet" {
			count++
		}
	}
	return count
}

func gachaReadString(value any) string {
	if value == nil {
		return ""
	}
	if str, ok := value.(string); ok {
		return str
	}
	return fmt.Sprintf("%v", value)
}
