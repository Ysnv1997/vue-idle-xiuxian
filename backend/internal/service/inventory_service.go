package service

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/kowming/vue-idle-xiuxian/backend/internal/repository"
)

type InventoryService struct {
	pool     *pgxpool.Pool
	userRepo *repository.UserRepository
}

func NewInventoryService(pool *pgxpool.Pool, userRepo *repository.UserRepository) *InventoryService {
	return &InventoryService{pool: pool, userRepo: userRepo}
}

type InventoryActionResult struct {
	Message       string                     `json:"message"`
	SoldCount     int                        `json:"soldCount,omitempty"`
	SoldIncome    int64                      `json:"soldIncome,omitempty"`
	ReleasedCount int                        `json:"releasedCount,omitempty"`
	Snapshot      *repository.PlayerSnapshot `json:"snapshot"`
}

type InventoryItemNotFoundError struct {
	ItemID string
}

func (e *InventoryItemNotFoundError) Error() string {
	return fmt.Sprintf("inventory item not found: %s", e.ItemID)
}

type InvalidInventoryItemTypeError struct {
	ItemID   string
	Expected string
	Actual   string
}

func (e *InvalidInventoryItemTypeError) Error() string {
	return fmt.Sprintf("invalid inventory item type: item %s expected %s actual %s", e.ItemID, e.Expected, e.Actual)
}

type InvalidRarityError struct {
	Rarity string
}

func (e *InvalidRarityError) Error() string {
	return fmt.Sprintf("invalid rarity: %s", e.Rarity)
}

type PetEssenceInsufficientError struct {
	Required int64
	Current  int64
}

func (e *PetEssenceInsufficientError) Error() string {
	return fmt.Sprintf("insufficient pet essence: required %d current %d", e.Required, e.Current)
}

type PetEvolveInvalidFoodError struct {
	Message string
}

func (e *PetEvolveInvalidFoodError) Error() string {
	return e.Message
}

func (s *InventoryService) UseItem(ctx context.Context, userID uuid.UUID, itemID string) (*InventoryActionResult, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin inventory use item transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	state, err := loadInventoryState(ctx, tx, userID)
	if err != nil {
		return nil, err
	}

	index := inventoryFindItemIndex(state.Items, itemID)
	if index < 0 {
		return nil, &InventoryItemNotFoundError{ItemID: itemID}
	}

	item := state.Items[index]
	itemType := inventoryReadString(item["type"])

	message := ""
	switch itemType {
	case "pill":
		now := time.Now().UnixMilli()
		state.ActiveEffects = inventoryFilterActiveEffects(state.ActiveEffects, now)
		effect := inventoryReadMap(item["effect"])
		effectType := inventoryReadString(effect["type"])
		if effectType == "spiritRecovery" {
			restoreAmount := calculateSpiritRecoveryAmount(state.Level, state.SpiritRate, state.ActiveEffects, effect, now)
			_, bonus := resolveMeditationEffectBonus(state.ActiveEffects, now)
			spiritCap := resolveMeditationSpiritCap(state.Level, bonus)
			targetCap := recoverableSpiritCap(math.Max(0, state.Spirit), spiritCap)
			beforeSpirit := math.Max(0, state.Spirit)
			state.Spirit = beforeSpirit + restoreAmount
			if state.Spirit > targetCap {
				state.Spirit = targetCap
			}
			message = fmt.Sprintf("服用回灵丹，恢复%.1f点灵力", math.Max(0, state.Spirit-beforeSpirit))
		} else {
			activeEffect := map[string]any{}
			for key, value := range effect {
				activeEffect[key] = value
			}
			duration := inventoryReadInt(activeEffect["duration"], 0)
			activeEffect["startTime"] = now
			activeEffect["endTime"] = now + int64(duration)*1000
			state.ActiveEffects = append(state.ActiveEffects, activeEffect)
			message = "使用丹药成功"
		}
		state.Items = append(state.Items[:index], state.Items[index+1:]...)
	case "pet":
		if state.ActivePetID == itemID {
			inventoryApplyPetBonus(state, item, -1)
			state.ActivePetID = ""
			message = "召回成功"
		} else {
			if state.ActivePetID != "" {
				if activeIndex := inventoryFindItemIndex(state.Items, state.ActivePetID); activeIndex >= 0 {
					inventoryApplyPetBonus(state, state.Items[activeIndex], -1)
				}
			}
			inventoryApplyPetBonus(state, item, 1)
			state.ActivePetID = itemID
			message = "出战成功"
		}
	default:
		return nil, &InvalidInventoryItemTypeError{ItemID: itemID, Expected: "pill or pet", Actual: itemType}
	}

	if err := persistInventoryState(ctx, tx, userID, state); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit inventory use item transaction: %w", err)
	}

	snapshot, err := s.userRepo.GetSnapshot(ctx, userID)
	if err != nil {
		return nil, err
	}

	return &InventoryActionResult{
		Message:  message,
		Snapshot: snapshot,
	}, nil
}

func (s *InventoryService) SellEquipment(ctx context.Context, userID uuid.UUID, itemID string) (*InventoryActionResult, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin inventory sell transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	state, err := loadInventoryState(ctx, tx, userID)
	if err != nil {
		return nil, err
	}

	index := inventoryFindItemIndex(state.Items, itemID)
	if index < 0 {
		return nil, &InventoryItemNotFoundError{ItemID: itemID}
	}

	item := state.Items[index]
	itemType := inventoryReadString(item["type"])
	if !inventoryIsEquipmentType(itemType) {
		return nil, &InvalidInventoryItemTypeError{ItemID: itemID, Expected: "equipment", Actual: itemType}
	}

	quality := inventoryReadString(item["quality"])
	price := gachaEquipmentPriceByQuality[quality]
	if price <= 0 {
		price = 1
	}

	state.Items = append(state.Items[:index], state.Items[index+1:]...)
	state.ReinforceStones += price

	if err := persistInventoryState(ctx, tx, userID, state); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit inventory sell transaction: %w", err)
	}

	snapshot, err := s.userRepo.GetSnapshot(ctx, userID)
	if err != nil {
		return nil, err
	}

	return &InventoryActionResult{
		Message:    fmt.Sprintf("成功卖出装备，获得%d个强化石", price),
		SoldCount:  1,
		SoldIncome: price,
		Snapshot:   snapshot,
	}, nil
}

func (s *InventoryService) BatchSellEquipment(ctx context.Context, userID uuid.UUID, quality string, equipmentType string) (*InventoryActionResult, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin inventory batch sell transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	state, err := loadInventoryState(ctx, tx, userID)
	if err != nil {
		return nil, err
	}

	nextItems := make([]map[string]any, 0, len(state.Items))
	soldCount := 0
	soldIncome := int64(0)
	for _, item := range state.Items {
		itemType := inventoryReadString(item["type"])
		if !inventoryIsEquipmentType(itemType) {
			nextItems = append(nextItems, item)
			continue
		}
		if equipmentType != "" && itemType != equipmentType {
			nextItems = append(nextItems, item)
			continue
		}
		itemQuality := inventoryReadString(item["quality"])
		if quality != "" && itemQuality != quality {
			nextItems = append(nextItems, item)
			continue
		}

		price := gachaEquipmentPriceByQuality[itemQuality]
		if price <= 0 {
			price = 1
		}
		soldCount++
		soldIncome += price
	}

	state.Items = nextItems
	state.ReinforceStones += soldIncome

	if err := persistInventoryState(ctx, tx, userID, state); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit inventory batch sell transaction: %w", err)
	}

	snapshot, err := s.userRepo.GetSnapshot(ctx, userID)
	if err != nil {
		return nil, err
	}

	return &InventoryActionResult{
		Message:    fmt.Sprintf("成功卖出%d件装备，获得%d个强化石", soldCount, soldIncome),
		SoldCount:  soldCount,
		SoldIncome: soldIncome,
		Snapshot:   snapshot,
	}, nil
}

func (s *InventoryService) ReleasePet(ctx context.Context, userID uuid.UUID, itemID string) (*InventoryActionResult, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin inventory release pet transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	state, err := loadInventoryState(ctx, tx, userID)
	if err != nil {
		return nil, err
	}

	index := inventoryFindItemIndex(state.Items, itemID)
	if index < 0 {
		return nil, &InventoryItemNotFoundError{ItemID: itemID}
	}
	itemType := inventoryReadString(state.Items[index]["type"])
	if itemType != "pet" {
		return nil, &InvalidInventoryItemTypeError{ItemID: itemID, Expected: "pet", Actual: itemType}
	}

	if state.ActivePetID == itemID {
		inventoryApplyPetBonus(state, state.Items[index], -1)
		state.ActivePetID = ""
	}
	state.Items = append(state.Items[:index], state.Items[index+1:]...)

	if err := persistInventoryState(ctx, tx, userID, state); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit inventory release pet transaction: %w", err)
	}

	snapshot, err := s.userRepo.GetSnapshot(ctx, userID)
	if err != nil {
		return nil, err
	}

	return &InventoryActionResult{
		Message:       "已放生灵宠",
		ReleasedCount: 1,
		Snapshot:      snapshot,
	}, nil
}

func (s *InventoryService) BatchReleasePets(ctx context.Context, userID uuid.UUID, rarity string) (*InventoryActionResult, error) {
	if rarity == "" {
		rarity = "all"
	}
	if rarity != "all" {
		if _, ok := gachaPetRarities[rarity]; !ok {
			return nil, &InvalidRarityError{Rarity: rarity}
		}
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin inventory batch release pets transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	state, err := loadInventoryState(ctx, tx, userID)
	if err != nil {
		return nil, err
	}

	nextItems := make([]map[string]any, 0, len(state.Items))
	releasedCount := 0
	for _, item := range state.Items {
		itemType := inventoryReadString(item["type"])
		if itemType != "pet" {
			nextItems = append(nextItems, item)
			continue
		}
		if inventoryReadString(item["id"]) == state.ActivePetID {
			nextItems = append(nextItems, item)
			continue
		}
		itemRarity := inventoryReadString(item["rarity"])
		if rarity != "all" && itemRarity != rarity {
			nextItems = append(nextItems, item)
			continue
		}
		releasedCount++
	}
	state.Items = nextItems

	if err := persistInventoryState(ctx, tx, userID, state); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit inventory batch release pets transaction: %w", err)
	}

	snapshot, err := s.userRepo.GetSnapshot(ctx, userID)
	if err != nil {
		return nil, err
	}

	return &InventoryActionResult{
		Message:       fmt.Sprintf("已放生%d只灵宠", releasedCount),
		ReleasedCount: releasedCount,
		Snapshot:      snapshot,
	}, nil
}

func (s *InventoryService) UpgradePet(ctx context.Context, userID uuid.UUID, itemID string) (*InventoryActionResult, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin inventory upgrade pet transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	state, err := loadInventoryState(ctx, tx, userID)
	if err != nil {
		return nil, err
	}

	index := inventoryFindItemIndex(state.Items, itemID)
	if index < 0 {
		return nil, &InventoryItemNotFoundError{ItemID: itemID}
	}
	pet := state.Items[index]
	itemType := inventoryReadString(pet["type"])
	if itemType != "pet" {
		return nil, &InvalidInventoryItemTypeError{ItemID: itemID, Expected: "pet", Actual: itemType}
	}

	level := inventoryReadInt(pet["level"], 1)
	cost := int64(level * 10)
	if state.PetEssence < cost {
		return nil, &PetEssenceInsufficientError{Required: cost, Current: state.PetEssence}
	}

	state.PetEssence -= cost
	newLevel := level + 1
	pet["level"] = newLevel

	rarity := inventoryReadString(pet["rarity"])
	qualityMultiplier := inventoryPetQualityMultiplier(rarity)
	combat := inventoryReadMap(pet["combatAttributes"])
	if combat == nil {
		combat = map[string]any{}
	}
	oldCombat := inventoryReadNumericMap(combat)

	baseKeys := []string{"attack", "health", "defense", "speed"}
	for _, key := range baseKeys {
		current := inventoryReadFloat(combat[key], 0)
		combat[key] = int64(current * (1 + 0.01*qualityMultiplier))
	}

	percentKeys := []string{
		"critRate", "comboRate", "counterRate", "stunRate", "dodgeRate", "vampireRate",
		"critResist", "comboResist", "counterResist", "stunResist", "dodgeResist", "vampireResist",
		"healBoost", "critDamageBoost", "critDamageReduce", "finalDamageBoost", "finalDamageReduce",
		"combatBoost", "resistanceBoost",
	}
	for _, key := range percentKeys {
		current := inventoryReadFloat(combat[key], 0)
		combat[key] = current + 0.01*qualityMultiplier
	}

	pet["combatAttributes"] = combat
	state.Items[index] = pet
	if state.ActivePetID == itemID {
		inventoryApplyPetBonusDiff(state, oldCombat, inventoryReadNumericMap(combat))
	}

	if err := persistInventoryState(ctx, tx, userID, state); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit inventory upgrade pet transaction: %w", err)
	}

	snapshot, err := s.userRepo.GetSnapshot(ctx, userID)
	if err != nil {
		return nil, err
	}

	return &InventoryActionResult{
		Message:  "升级成功",
		Snapshot: snapshot,
	}, nil
}

func (s *InventoryService) EvolvePet(ctx context.Context, userID uuid.UUID, itemID string, foodItemID string) (*InventoryActionResult, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin inventory evolve pet transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	state, err := loadInventoryState(ctx, tx, userID)
	if err != nil {
		return nil, err
	}

	petIndex := inventoryFindItemIndex(state.Items, itemID)
	if petIndex < 0 {
		return nil, &InventoryItemNotFoundError{ItemID: itemID}
	}
	foodIndex := inventoryFindItemIndex(state.Items, foodItemID)
	if foodIndex < 0 {
		return nil, &InventoryItemNotFoundError{ItemID: foodItemID}
	}
	if petIndex == foodIndex {
		return nil, &PetEvolveInvalidFoodError{Message: "不能选择自身作为升星材料"}
	}

	pet := state.Items[petIndex]
	food := state.Items[foodIndex]
	if inventoryReadString(pet["type"]) != "pet" {
		return nil, &InvalidInventoryItemTypeError{ItemID: itemID, Expected: "pet", Actual: inventoryReadString(pet["type"])}
	}
	if inventoryReadString(food["type"]) != "pet" {
		return nil, &InvalidInventoryItemTypeError{ItemID: foodItemID, Expected: "pet", Actual: inventoryReadString(food["type"])}
	}

	if inventoryReadString(pet["rarity"]) != inventoryReadString(food["rarity"]) ||
		inventoryReadString(pet["name"]) != inventoryReadString(food["name"]) {
		return nil, &PetEvolveInvalidFoodError{Message: "只能使用相同品质和名字的灵宠进行升星"}
	}

	returnEssence := int64(maxInt(0, inventoryReadInt(food["level"], 1)-1) * 10)
	state.PetEssence += returnEssence

	pet["star"] = inventoryReadInt(pet["star"], 0) + 1
	state.Items[petIndex] = pet
	if state.ActivePetID == foodItemID {
		inventoryApplyPetBonus(state, food, -1)
		state.ActivePetID = ""
	}

	if foodIndex > petIndex {
		state.Items = append(state.Items[:foodIndex], state.Items[foodIndex+1:]...)
	} else {
		state.Items = append(state.Items[:foodIndex], state.Items[foodIndex+1:]...)
		petIndex--
		state.Items[petIndex] = pet
	}

	if err := persistInventoryState(ctx, tx, userID, state); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit inventory evolve pet transaction: %w", err)
	}

	snapshot, err := s.userRepo.GetSnapshot(ctx, userID)
	if err != nil {
		return nil, err
	}

	return &InventoryActionResult{
		Message:  "升星成功",
		Snapshot: snapshot,
	}, nil
}

type inventoryState struct {
	Level             int
	Spirit            float64
	SpiritRate        float64
	ReinforceStones   int64
	PetEssence        int64
	ActivePetID       string
	ActiveEffects     []map[string]any
	Items             []map[string]any
	BaseAttributes    map[string]float64
	CombatAttributes  map[string]float64
	CombatResistance  map[string]float64
	SpecialAttributes map[string]float64
}

func loadInventoryState(ctx context.Context, tx pgx.Tx, userID uuid.UUID) (*inventoryState, error) {
	const query = `
		SELECT
			pp.level,
			pr.spirit,
			pr.spirit_rate,
			pr.reinforce_stones,
			COALESCE(pr.pet_essence, 0),
			pa.base_attributes,
			pa.combat_attributes,
			pa.combat_resistance,
			pa.special_attributes,
			COALESCE(pis.items, '[]'::jsonb),
			COALESCE(pis.active_pet_id, ''),
			COALESCE(pis.active_effects, '[]'::jsonb)
		FROM player_profiles pp
		JOIN player_resources pr ON pr.user_id = pp.user_id
		JOIN player_attributes pa ON pa.user_id = pp.user_id
		JOIN player_inventory_state pis ON pis.user_id = pp.user_id
		WHERE pp.user_id = $1
		FOR UPDATE OF pp, pr, pa, pis
	`

	state := &inventoryState{}
	var baseRaw []byte
	var combatRaw []byte
	var resistRaw []byte
	var specialRaw []byte
	var itemsRaw []byte
	var effectsRaw []byte
	if err := tx.QueryRow(ctx, query, userID).Scan(
		&state.Level,
		&state.Spirit,
		&state.SpiritRate,
		&state.ReinforceStones,
		&state.PetEssence,
		&baseRaw,
		&combatRaw,
		&resistRaw,
		&specialRaw,
		&itemsRaw,
		&state.ActivePetID,
		&effectsRaw,
	); err != nil {
		return nil, fmt.Errorf("load inventory state: %w", err)
	}

	state.BaseAttributes = inventoryDecodeFloatMap(baseRaw)
	state.CombatAttributes = inventoryDecodeFloatMap(combatRaw)
	state.CombatResistance = inventoryDecodeFloatMap(resistRaw)
	state.SpecialAttributes = inventoryDecodeFloatMap(specialRaw)

	if err := json.Unmarshal(itemsRaw, &state.Items); err != nil {
		state.Items = []map[string]any{}
	}
	if state.Items == nil {
		state.Items = []map[string]any{}
	}
	if err := json.Unmarshal(effectsRaw, &state.ActiveEffects); err != nil {
		state.ActiveEffects = []map[string]any{}
	}
	if state.ActiveEffects == nil {
		state.ActiveEffects = []map[string]any{}
	}

	return state, nil
}

func persistInventoryState(ctx context.Context, tx pgx.Tx, userID uuid.UUID, state *inventoryState) error {
	baseJSON, err := json.Marshal(state.BaseAttributes)
	if err != nil {
		return fmt.Errorf("marshal base attributes: %w", err)
	}
	combatJSON, err := json.Marshal(state.CombatAttributes)
	if err != nil {
		return fmt.Errorf("marshal combat attributes: %w", err)
	}
	resistJSON, err := json.Marshal(state.CombatResistance)
	if err != nil {
		return fmt.Errorf("marshal combat resistance: %w", err)
	}
	specialJSON, err := json.Marshal(state.SpecialAttributes)
	if err != nil {
		return fmt.Errorf("marshal special attributes: %w", err)
	}
	itemsJSON, err := json.Marshal(state.Items)
	if err != nil {
		return fmt.Errorf("marshal inventory items: %w", err)
	}
	activeEffectsJSON, err := json.Marshal(state.ActiveEffects)
	if err != nil {
		return fmt.Errorf("marshal inventory active effects: %w", err)
	}

	const updateResourcesSQL = `
		UPDATE player_resources
		SET spirit = $2, reinforce_stones = $3, pet_essence = $4, updated_at = now()
		WHERE user_id = $1
	`
	if _, err := tx.Exec(ctx, updateResourcesSQL, userID, state.Spirit, state.ReinforceStones, state.PetEssence); err != nil {
		return fmt.Errorf("update inventory resources: %w", err)
	}

	const updateAttributesSQL = `
		UPDATE player_attributes
		SET
			base_attributes = $2::jsonb,
			combat_attributes = $3::jsonb,
			combat_resistance = $4::jsonb,
			special_attributes = $5::jsonb,
			updated_at = now()
		WHERE user_id = $1
	`
	if _, err := tx.Exec(ctx, updateAttributesSQL, userID, string(baseJSON), string(combatJSON), string(resistJSON), string(specialJSON)); err != nil {
		return fmt.Errorf("update inventory attributes: %w", err)
	}

	const updateInventorySQL = `
		UPDATE player_inventory_state
		SET
			items = $2::jsonb,
			active_pet_id = NULLIF($3, ''),
			active_effects = $4::jsonb,
			updated_at = now()
		WHERE user_id = $1
	`
	if _, err := tx.Exec(ctx, updateInventorySQL, userID, string(itemsJSON), state.ActivePetID, string(activeEffectsJSON)); err != nil {
		return fmt.Errorf("update inventory items: %w", err)
	}
	return nil
}

func inventoryFindItemIndex(items []map[string]any, itemID string) int {
	for i, item := range items {
		if inventoryReadString(item["id"]) == itemID {
			return i
		}
	}
	return -1
}

func inventoryReadString(v any) string {
	switch value := v.(type) {
	case string:
		return value
	case float64:
		return strconv.FormatFloat(value, 'f', -1, 64)
	case int64:
		return strconv.FormatInt(value, 10)
	case int:
		return strconv.Itoa(value)
	default:
		if v == nil {
			return ""
		}
		return fmt.Sprintf("%v", v)
	}
}

func inventoryReadInt(v any, defaultValue int) int {
	switch value := v.(type) {
	case int:
		return value
	case int64:
		return int(value)
	case float64:
		return int(value)
	case string:
		if parsed, err := strconv.Atoi(value); err == nil {
			return parsed
		}
	}
	return defaultValue
}

func inventoryReadInt64(v any, defaultValue int64) int64 {
	switch value := v.(type) {
	case int:
		return int64(value)
	case int64:
		return value
	case float64:
		return int64(value)
	case string:
		if parsed, err := strconv.ParseInt(value, 10, 64); err == nil {
			return parsed
		}
	}
	return defaultValue
}

func inventoryReadFloat(v any, defaultValue float64) float64 {
	switch value := v.(type) {
	case float64:
		return value
	case int:
		return float64(value)
	case int64:
		return float64(value)
	case string:
		if parsed, err := strconv.ParseFloat(value, 64); err == nil {
			return parsed
		}
	}
	return defaultValue
}

func inventoryReadMap(v any) map[string]any {
	if v == nil {
		return nil
	}
	if mapped, ok := v.(map[string]any); ok {
		return mapped
	}
	return nil
}

func inventoryReadNumericMap(v any) map[string]float64 {
	input := inventoryReadMap(v)
	if input == nil {
		return map[string]float64{}
	}
	output := make(map[string]float64, len(input))
	for key, value := range input {
		output[key] = inventoryReadFloat(value, 0)
	}
	return output
}

func inventoryDecodeFloatMap(raw []byte) map[string]float64 {
	decoded := map[string]float64{}
	if len(raw) == 0 {
		return decoded
	}
	temp := map[string]any{}
	if err := json.Unmarshal(raw, &temp); err != nil {
		return decoded
	}
	for key, value := range temp {
		decoded[key] = inventoryReadFloat(value, 0)
	}
	return decoded
}

func inventoryApplyStatDelta(state *inventoryState, stat string, delta float64) {
	if _, ok := state.BaseAttributes[stat]; ok {
		state.BaseAttributes[stat] += delta
		return
	}
	if _, ok := state.CombatAttributes[stat]; ok {
		state.CombatAttributes[stat] += delta
		return
	}
	if _, ok := state.CombatResistance[stat]; ok {
		state.CombatResistance[stat] += delta
		return
	}
	if _, ok := state.SpecialAttributes[stat]; ok {
		state.SpecialAttributes[stat] += delta
	}
}

func inventoryApplyPetBonus(state *inventoryState, pet map[string]any, direction float64) {
	combat := inventoryReadMap(pet["combatAttributes"])
	for stat, raw := range combat {
		value := inventoryReadFloat(raw, 0)
		inventoryApplyStatDelta(state, stat, direction*value)
	}
}

func inventoryApplyPetBonusDiff(state *inventoryState, oldCombat map[string]float64, newCombat map[string]float64) {
	keys := map[string]struct{}{}
	for key := range oldCombat {
		keys[key] = struct{}{}
	}
	for key := range newCombat {
		keys[key] = struct{}{}
	}
	for key := range keys {
		inventoryApplyStatDelta(state, key, newCombat[key]-oldCombat[key])
	}
}

func inventoryFilterActiveEffects(activeEffects []map[string]any, nowMilli int64) []map[string]any {
	filtered := make([]map[string]any, 0, len(activeEffects))
	for _, effect := range activeEffects {
		if inventoryReadInt64(effect["endTime"], 0) > nowMilli {
			filtered = append(filtered, effect)
		}
	}
	return filtered
}

func inventoryIsEquipmentType(itemType string) bool {
	switch itemType {
	case "weapon", "head", "body", "legs", "feet", "shoulder", "hands", "wrist", "necklace", "ring1", "ring2", "belt", "artifact":
		return true
	default:
		return false
	}
}

func inventoryPetQualityMultiplier(rarity string) float64 {
	switch rarity {
	case "divine":
		return 2.0
	case "celestial":
		return 1.8
	case "mystic":
		return 1.6
	case "spiritual":
		return 1.4
	default:
		return 1.2
	}
}
