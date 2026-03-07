package service

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"strconv"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/kowming/vue-idle-xiuxian/backend/internal/repository"
)

type EquipmentService struct {
	pool           *pgxpool.Pool
	userRepo       *repository.UserRepository
	realtimeBroker *GameRealtimeBroker
}

func NewEquipmentService(pool *pgxpool.Pool, userRepo *repository.UserRepository, realtimeBroker *GameRealtimeBroker) *EquipmentService {
	return &EquipmentService{pool: pool, userRepo: userRepo, realtimeBroker: realtimeBroker}
}

type EquipmentActionResult struct {
	Success  bool                       `json:"success"`
	Message  string                     `json:"message"`
	Cost     int64                      `json:"cost,omitempty"`
	NewLevel int                        `json:"newLevel,omitempty"`
	OldStats map[string]any             `json:"oldStats,omitempty"`
	NewStats map[string]any             `json:"newStats,omitempty"`
	Snapshot *repository.PlayerSnapshot `json:"snapshot"`
}

type EquipmentItemNotFoundError struct {
	ItemID string
}

func (e *EquipmentItemNotFoundError) Error() string {
	return fmt.Sprintf("equipment item not found: %s", e.ItemID)
}

type EquipmentSlotInvalidError struct {
	Slot string
}

func (e *EquipmentSlotInvalidError) Error() string {
	return fmt.Sprintf("invalid equipment slot: %s", e.Slot)
}

type EquipmentSlotEmptyError struct {
	Slot string
}

func (e *EquipmentSlotEmptyError) Error() string {
	return fmt.Sprintf("equipment slot empty: %s", e.Slot)
}

type EquipmentRequirementError struct {
	Required int
	Current  int
}

func (e *EquipmentRequirementError) Error() string {
	return fmt.Sprintf("equipment requirement not met: required %d current %d", e.Required, e.Current)
}

type ReinforceStonesInsufficientError struct {
	Required int64
	Current  int64
}

func (e *ReinforceStonesInsufficientError) Error() string {
	return fmt.Sprintf("insufficient reinforce stones: required %d current %d", e.Required, e.Current)
}

type RefinementStonesInsufficientError struct {
	Required int64
	Current  int64
}

func (e *RefinementStonesInsufficientError) Error() string {
	return fmt.Sprintf("insufficient refinement stones: required %d current %d", e.Required, e.Current)
}

type EquipmentEnhanceMaxLevelError struct {
	CurrentLevel int
}

func (e *EquipmentEnhanceMaxLevelError) Error() string {
	return fmt.Sprintf("equipment already at max enhance level: %d", e.CurrentLevel)
}

type EquipmentInvalidTypeError struct {
	ItemID string
	Type   string
}

func (e *EquipmentInvalidTypeError) Error() string {
	return fmt.Sprintf("invalid equipment type: item %s type %s", e.ItemID, e.Type)
}

type equipmentState struct {
	Level            int
	ReinforceStones  int64
	RefinementStones int64

	Items    []map[string]any
	Equipped map[string]any

	BaseAttributes    map[string]float64
	CombatAttributes  map[string]float64
	CombatResistance  map[string]float64
	SpecialAttributes map[string]float64
}

func (s *EquipmentService) Equip(ctx context.Context, userID uuid.UUID, itemID string) (*EquipmentActionResult, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin equipment equip transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	state, err := loadEquipmentState(ctx, tx, userID)
	if err != nil {
		return nil, err
	}

	index := equipmentFindItemIndex(state.Items, itemID)
	if index < 0 {
		return nil, &EquipmentItemNotFoundError{ItemID: itemID}
	}

	item := state.Items[index]
	itemType := equipmentReadString(item["type"])
	if !inventoryIsEquipmentType(itemType) {
		return nil, &EquipmentInvalidTypeError{ItemID: itemID, Type: itemType}
	}

	requiredRealm := equipmentReadInt(item["requiredRealm"], 1)
	if state.Level < requiredRealm {
		return nil, &EquipmentRequirementError{Required: requiredRealm, Current: state.Level}
	}

	slot := equipmentReadString(item["slot"])
	if slot == "" {
		slot = itemType
	}
	if !equipmentIsValidSlot(slot) {
		return nil, &EquipmentSlotInvalidError{Slot: slot}
	}

	if existing, ok := equipmentReadMap(state.Equipped[slot]); ok {
		applyEquipmentStatDelta(state, existing, -1)
		state.Items = append(state.Items, existing)
	}

	state.Items = append(state.Items[:index], state.Items[index+1:]...)
	state.Equipped[slot] = item
	applyEquipmentStatDelta(state, item, 1)

	if err := persistEquipmentState(ctx, tx, userID, state); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit equipment equip transaction: %w", err)
	}

	snapshot, err := s.userRepo.GetSnapshot(ctx, userID)
	if err != nil {
		return nil, err
	}
	return &EquipmentActionResult{
		Success:  true,
		Message:  "装备成功",
		Snapshot: snapshot,
	}, nil
}

func (s *EquipmentService) Unequip(ctx context.Context, userID uuid.UUID, slot string) (*EquipmentActionResult, error) {
	if !equipmentIsValidSlot(slot) {
		return nil, &EquipmentSlotInvalidError{Slot: slot}
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin equipment unequip transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	state, err := loadEquipmentState(ctx, tx, userID)
	if err != nil {
		return nil, err
	}

	equipped, ok := equipmentReadMap(state.Equipped[slot])
	if !ok {
		return nil, &EquipmentSlotEmptyError{Slot: slot}
	}

	applyEquipmentStatDelta(state, equipped, -1)
	state.Items = append(state.Items, equipped)
	state.Equipped[slot] = nil

	if err := persistEquipmentState(ctx, tx, userID, state); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit equipment unequip transaction: %w", err)
	}

	snapshot, err := s.userRepo.GetSnapshot(ctx, userID)
	if err != nil {
		return nil, err
	}
	return &EquipmentActionResult{
		Success:  true,
		Message:  "当前装备已卸下",
		Snapshot: snapshot,
	}, nil
}

func (s *EquipmentService) Enhance(ctx context.Context, userID uuid.UUID, itemID string) (*EquipmentActionResult, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin equipment enhance transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	state, err := loadEquipmentState(ctx, tx, userID)
	if err != nil {
		return nil, err
	}

	location, item, err := equipmentFindItem(state, itemID)
	if err != nil {
		return nil, err
	}

	currentLevel := equipmentReadInt(item["enhanceLevel"], 0)
	if currentLevel >= 100 {
		return nil, &EquipmentEnhanceMaxLevelError{CurrentLevel: currentLevel}
	}

	cost := int64((currentLevel + 1) * 10)
	if state.ReinforceStones < cost {
		return nil, &ReinforceStonesInsufficientError{Required: cost, Current: state.ReinforceStones}
	}

	successRate := 1 - float64(currentLevel)*0.05
	if successRate < 0 {
		successRate = 0
	}

	oldStats := equipmentReadNumericMap(item["stats"])
	if oldStats == nil {
		oldStats = map[string]float64{}
	}

	if rand.Float64() >= successRate {
		if err := tx.Rollback(ctx); err != nil {
			return nil, fmt.Errorf("rollback equipment enhance fail tx: %w", err)
		}
		snapshot, snapErr := s.userRepo.GetSnapshot(ctx, userID)
		if snapErr != nil {
			return nil, snapErr
		}
		return &EquipmentActionResult{
			Success:  false,
			Message:  "强化失败",
			Cost:     cost,
			OldStats: equipmentToAnyMap(oldStats),
			NewStats: equipmentToAnyMap(oldStats),
			Snapshot: snapshot,
		}, nil
	}

	newStats := make(map[string]float64, len(oldStats))
	for stat, value := range oldStats {
		nextValue := value * 1.1
		if equipmentIsPercentStat(stat) {
			newStats[stat] = math.Round(nextValue*100) / 100
		} else {
			newStats[stat] = math.Round(nextValue)
		}
	}
	item["stats"] = equipmentToAnyMap(newStats)
	item["enhanceLevel"] = currentLevel + 1

	if location.IsEquipped {
		applyEquipmentStatDiff(state, oldStats, newStats)
		state.Equipped[location.Slot] = item
	} else {
		state.Items[location.InventoryIndex] = item
	}

	state.ReinforceStones -= cost

	if err := persistEquipmentState(ctx, tx, userID, state); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit equipment enhance transaction: %w", err)
	}

	snapshot, err := s.userRepo.GetSnapshot(ctx, userID)
	if err != nil {
		return nil, err
	}
	if snapshot != nil && s.realtimeBroker != nil && currentLevel+1 >= 10 {
		publishWorldAnnouncement(ctx, s.realtimeBroker, buildEnhanceAnnouncement(snapshot.Name, equipmentReadString(item["name"]), currentLevel+1))
	}
	return &EquipmentActionResult{
		Success:  true,
		Message:  "强化成功",
		Cost:     cost,
		NewLevel: currentLevel + 1,
		OldStats: equipmentToAnyMap(oldStats),
		NewStats: equipmentToAnyMap(newStats),
		Snapshot: snapshot,
	}, nil
}

func (s *EquipmentService) Reforge(ctx context.Context, userID uuid.UUID, itemID string) (*EquipmentActionResult, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin equipment reforge transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	state, err := loadEquipmentState(ctx, tx, userID)
	if err != nil {
		return nil, err
	}

	location, item, err := equipmentFindItem(state, itemID)
	if err != nil {
		return nil, err
	}

	const reforgeCost int64 = 10
	if state.RefinementStones < reforgeCost {
		return nil, &RefinementStonesInsufficientError{Required: reforgeCost, Current: state.RefinementStones}
	}

	itemType := equipmentReadString(item["type"])
	availableStats, ok := equipmentReforgeableStats[itemType]
	if !ok {
		return nil, &EquipmentInvalidTypeError{ItemID: itemID, Type: itemType}
	}

	oldStats := equipmentReadNumericMap(item["stats"])
	if oldStats == nil || len(oldStats) == 0 {
		oldStats = map[string]float64{}
	}

	originStats := make([]string, 0, len(oldStats))
	for stat := range oldStats {
		originStats = append(originStats, stat)
	}

	newStats := equipmentReforgeStats(oldStats, originStats, availableStats)
	if len(newStats) != len(originStats) {
		if err := tx.Rollback(ctx); err != nil {
			return nil, fmt.Errorf("rollback equipment reforge invalid result tx: %w", err)
		}
		snapshot, snapErr := s.userRepo.GetSnapshot(ctx, userID)
		if snapErr != nil {
			return nil, snapErr
		}
		return &EquipmentActionResult{
			Success:  false,
			Message:  "洗练过程出现异常",
			Cost:     0,
			OldStats: equipmentToAnyMap(oldStats),
			NewStats: equipmentToAnyMap(oldStats),
			Snapshot: snapshot,
		}, nil
	}

	item["stats"] = equipmentToAnyMap(newStats)

	if location.IsEquipped {
		applyEquipmentStatDiff(state, oldStats, newStats)
		state.Equipped[location.Slot] = item
	} else {
		state.Items[location.InventoryIndex] = item
	}

	state.RefinementStones -= reforgeCost

	if err := persistEquipmentState(ctx, tx, userID, state); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit equipment reforge transaction: %w", err)
	}

	snapshot, err := s.userRepo.GetSnapshot(ctx, userID)
	if err != nil {
		return nil, err
	}
	return &EquipmentActionResult{
		Success:  true,
		Message:  "洗练成功",
		Cost:     reforgeCost,
		OldStats: equipmentToAnyMap(oldStats),
		NewStats: equipmentToAnyMap(newStats),
		Snapshot: snapshot,
	}, nil
}

type equipmentItemLocation struct {
	IsEquipped     bool
	Slot           string
	InventoryIndex int
}

func equipmentFindItem(state *equipmentState, itemID string) (*equipmentItemLocation, map[string]any, error) {
	idx := equipmentFindItemIndex(state.Items, itemID)
	if idx >= 0 {
		item := state.Items[idx]
		itemType := equipmentReadString(item["type"])
		if !inventoryIsEquipmentType(itemType) {
			return nil, nil, &EquipmentInvalidTypeError{ItemID: itemID, Type: itemType}
		}
		return &equipmentItemLocation{IsEquipped: false, InventoryIndex: idx}, item, nil
	}

	for slot, raw := range state.Equipped {
		item, ok := equipmentReadMap(raw)
		if !ok {
			continue
		}
		if equipmentReadString(item["id"]) == itemID {
			itemType := equipmentReadString(item["type"])
			if !inventoryIsEquipmentType(itemType) {
				return nil, nil, &EquipmentInvalidTypeError{ItemID: itemID, Type: itemType}
			}
			return &equipmentItemLocation{IsEquipped: true, Slot: slot}, item, nil
		}
	}

	return nil, nil, &EquipmentItemNotFoundError{ItemID: itemID}
}

func loadEquipmentState(ctx context.Context, tx pgx.Tx, userID uuid.UUID) (*equipmentState, error) {
	const query = `
		SELECT
			pp.level,
			pr.reinforce_stones,
			pr.refinement_stones,
			pa.base_attributes,
			pa.combat_attributes,
			pa.combat_resistance,
			pa.special_attributes,
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
		JOIN player_attributes pa ON pa.user_id = pp.user_id
		JOIN player_inventory_state pis ON pis.user_id = pp.user_id
		WHERE pp.user_id = $1
		FOR UPDATE OF pp, pr, pa, pis
	`

	state := &equipmentState{}
	var baseRaw []byte
	var combatRaw []byte
	var resistRaw []byte
	var specialRaw []byte
	var itemsRaw []byte
	var equippedRaw []byte

	if err := tx.QueryRow(ctx, query, userID).Scan(
		&state.Level,
		&state.ReinforceStones,
		&state.RefinementStones,
		&baseRaw,
		&combatRaw,
		&resistRaw,
		&specialRaw,
		&itemsRaw,
		&equippedRaw,
	); err != nil {
		return nil, fmt.Errorf("load equipment state: %w", err)
	}

	state.BaseAttributes = equipmentDecodeFloatMap(baseRaw)
	state.CombatAttributes = equipmentDecodeFloatMap(combatRaw)
	state.CombatResistance = equipmentDecodeFloatMap(resistRaw)
	state.SpecialAttributes = equipmentDecodeFloatMap(specialRaw)

	if err := json.Unmarshal(itemsRaw, &state.Items); err != nil || state.Items == nil {
		state.Items = []map[string]any{}
	}

	if err := json.Unmarshal(equippedRaw, &state.Equipped); err != nil || state.Equipped == nil {
		state.Equipped = map[string]any{}
	}
	equipmentEnsureSlots(state.Equipped)

	return state, nil
}

func persistEquipmentState(ctx context.Context, tx pgx.Tx, userID uuid.UUID, state *equipmentState) error {
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
		return fmt.Errorf("marshal equipment items: %w", err)
	}
	equippedJSON, err := json.Marshal(state.Equipped)
	if err != nil {
		return fmt.Errorf("marshal equipped artifacts: %w", err)
	}

	const updateResourcesSQL = `
		UPDATE player_resources
		SET reinforce_stones = $2, refinement_stones = $3, updated_at = now()
		WHERE user_id = $1
	`
	if _, err := tx.Exec(ctx, updateResourcesSQL, userID, state.ReinforceStones, state.RefinementStones); err != nil {
		return fmt.Errorf("update reinforce/refinement stones: %w", err)
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
		return fmt.Errorf("update player attributes: %w", err)
	}

	const updateInventorySQL = `
		UPDATE player_inventory_state
		SET
			items = $2::jsonb,
			equipped_artifacts = $3::jsonb,
			updated_at = now()
		WHERE user_id = $1
	`
	if _, err := tx.Exec(ctx, updateInventorySQL, userID, string(itemsJSON), string(equippedJSON)); err != nil {
		return fmt.Errorf("update equipment inventory state: %w", err)
	}

	return nil
}

func equipmentFindItemIndex(items []map[string]any, itemID string) int {
	for i, item := range items {
		if equipmentReadString(item["id"]) == itemID {
			return i
		}
	}
	return -1
}

func equipmentReadString(v any) string {
	switch value := v.(type) {
	case string:
		return value
	case float64:
		return strconv.FormatFloat(value, 'f', -1, 64)
	case int:
		return strconv.Itoa(value)
	case int64:
		return strconv.FormatInt(value, 10)
	default:
		if v == nil {
			return ""
		}
		return fmt.Sprintf("%v", v)
	}
}

func equipmentReadInt(v any, defaultValue int) int {
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

func equipmentReadFloat(v any, defaultValue float64) float64 {
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

func equipmentReadMap(v any) (map[string]any, bool) {
	if v == nil {
		return nil, false
	}
	mapped, ok := v.(map[string]any)
	if !ok {
		return nil, false
	}
	return mapped, true
}

func equipmentDecodeFloatMap(raw []byte) map[string]float64 {
	decoded := map[string]float64{}
	if len(raw) == 0 {
		return decoded
	}
	temp := map[string]any{}
	if err := json.Unmarshal(raw, &temp); err != nil {
		return decoded
	}
	for k, v := range temp {
		decoded[k] = equipmentReadFloat(v, 0)
	}
	return decoded
}

func equipmentReadNumericMap(v any) map[string]float64 {
	stats, ok := equipmentReadMap(v)
	if !ok {
		return nil
	}
	result := make(map[string]float64, len(stats))
	for k, value := range stats {
		result[k] = equipmentReadFloat(value, 0)
	}
	return result
}

func equipmentToAnyMap(input map[string]float64) map[string]any {
	output := make(map[string]any, len(input))
	for k, v := range input {
		output[k] = v
	}
	return output
}

func equipmentEnsureSlots(equipped map[string]any) {
	for _, slot := range equipmentValidSlots() {
		if _, ok := equipped[slot]; !ok {
			equipped[slot] = nil
		}
	}
}

func equipmentValidSlots() []string {
	return []string{
		"weapon",
		"head",
		"body",
		"legs",
		"feet",
		"shoulder",
		"hands",
		"wrist",
		"necklace",
		"ring1",
		"ring2",
		"belt",
		"artifact",
	}
}

func equipmentIsValidSlot(slot string) bool {
	switch slot {
	case "weapon", "head", "body", "legs", "feet", "shoulder", "hands", "wrist", "necklace", "ring1", "ring2", "belt", "artifact":
		return true
	default:
		return false
	}
}

func equipmentApplyStat(state *equipmentState, stat string, delta float64) {
	if _, ok := state.BaseAttributes[stat]; ok {
		state.BaseAttributes[stat] += delta
		return
	}
	if current, ok := state.CombatAttributes[stat]; ok {
		next := current + delta
		if next < 0 {
			next = 0
		}
		if next > 1 {
			next = 1
		}
		state.CombatAttributes[stat] = next
		return
	}
	if current, ok := state.CombatResistance[stat]; ok {
		next := current + delta
		if next < 0 {
			next = 0
		}
		if next > 1 {
			next = 1
		}
		state.CombatResistance[stat] = next
		return
	}
	if _, ok := state.SpecialAttributes[stat]; ok {
		state.SpecialAttributes[stat] += delta
	}
}

func applyEquipmentStatDelta(state *equipmentState, item map[string]any, direction float64) {
	stats := equipmentReadNumericMap(item["stats"])
	if stats == nil {
		return
	}
	for stat, value := range stats {
		equipmentApplyStat(state, stat, direction*value)
	}
}

func applyEquipmentStatDiff(state *equipmentState, oldStats map[string]float64, newStats map[string]float64) {
	keys := map[string]struct{}{}
	for key := range oldStats {
		keys[key] = struct{}{}
	}
	for key := range newStats {
		keys[key] = struct{}{}
	}
	for key := range keys {
		delta := newStats[key] - oldStats[key]
		equipmentApplyStat(state, key, delta)
	}
}

func equipmentIsPercentStat(stat string) bool {
	switch stat {
	case "critRate", "critDamageBoost", "dodgeRate", "vampireRate", "finalDamageBoost", "finalDamageReduce":
		return true
	default:
		return false
	}
}

var equipmentReforgeableStats = map[string][]string{
	"weapon":   {"attack", "critRate", "critDamageBoost"},
	"head":     {"defense", "health", "stunResist"},
	"body":     {"defense", "health", "finalDamageReduce"},
	"legs":     {"defense", "speed", "dodgeRate"},
	"feet":     {"defense", "speed", "dodgeRate"},
	"shoulder": {"defense", "health", "counterRate"},
	"hands":    {"attack", "critRate", "comboRate"},
	"wrist":    {"defense", "counterRate", "vampireRate"},
	"necklace": {"health", "healBoost", "spiritRate"},
	"ring1":    {"attack", "critDamageBoost", "finalDamageBoost"},
	"ring2":    {"defense", "critDamageReduce", "resistanceBoost"},
	"belt":     {"health", "defense", "combatBoost"},
	"artifact": {"attack", "critRate", "comboRate"},
}

func equipmentReforgeStats(oldStats map[string]float64, originStats []string, availableStats []string) map[string]float64 {
	nextStats := make(map[string]float64, len(oldStats))
	for key, value := range oldStats {
		nextStats[key] = value
	}
	if len(originStats) == 0 {
		return nextStats
	}

	modifyCount := rand.Intn(3) + 1
	indexSet := map[int]struct{}{}
	for i := 0; i < modifyCount; i++ {
		indexSet[rand.Intn(len(originStats))] = struct{}{}
	}

	for index := range indexSet {
		originStat := originStats[index]
		currentStat := originStat
		baseValue, ok := nextStats[originStat]
		if !ok {
			continue
		}

		if rand.Float64() < 0.3 {
			availableNew := make([]string, 0, len(availableStats))
			for _, stat := range availableStats {
				if stat == originStat || equipmentContainsStat(originStats, stat) {
					continue
				}
				availableNew = append(availableNew, stat)
			}
			if len(availableNew) > 0 {
				newStat := availableNew[rand.Intn(len(availableNew))]
				delete(nextStats, originStat)
				currentStat = newStat
			}
		}

		delta := rand.Float64()*0.6 - 0.3
		rawValue := baseValue * (1 + delta)
		if equipmentIsPercentStat(currentStat) {
			value := math.Round(rawValue*100) / 100
			minValue := baseValue * 0.7
			maxValue := baseValue * 1.3
			value = math.Min(math.Max(value, minValue), maxValue)
			nextStats[currentStat] = value
			continue
		}

		value := math.Round(rawValue)
		minValue := math.Round(baseValue * 0.7)
		maxValue := math.Round(baseValue * 1.3)
		value = math.Min(math.Max(value, minValue), maxValue)
		nextStats[currentStat] = value
	}

	return nextStats
}

func equipmentContainsStat(stats []string, target string) bool {
	for _, stat := range stats {
		if stat == target {
			return true
		}
	}
	return false
}
