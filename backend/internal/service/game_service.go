package service

import (
	"context"
	"errors"
	"fmt"
	"math"
	"math/rand"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/kowming/vue-idle-xiuxian/backend/internal/repository"
)

type GameService struct {
	pool     *pgxpool.Pool
	userRepo *repository.UserRepository
}

func NewGameService(pool *pgxpool.Pool, userRepo *repository.UserRepository) *GameService {
	return &GameService{
		pool:     pool,
		userRepo: userRepo,
	}
}

type CultivationActionResult struct {
	SpiritCost      int64                      `json:"spiritCost"`
	CultivationGain int64                      `json:"cultivationGain"`
	DoubleGain      bool                       `json:"doubleGain"`
	DoubleGainTimes int                        `json:"doubleGainTimes"`
	Breakthrough    bool                       `json:"breakthrough"`
	Snapshot        *repository.PlayerSnapshot `json:"snapshot"`
}

type HuntingMap struct {
	ID                string   `json:"id"`
	Name              string   `json:"name"`
	Description       string   `json:"description"`
	MinLevel          int      `json:"minLevel"`
	RewardFactor      float64  `json:"rewardFactor"`
	RecommendedPower  int64    `json:"recommendedPower"`
	RecommendedHealth int64    `json:"recommendedHealth"`
	EstimatedCost     int64    `json:"estimatedCost"`
	EstimatedGain     int64    `json:"estimatedGain"`
	Monsters          []string `json:"monsters"`
	ProgressionNote   string   `json:"progressionNote"`
	EstimatedPerHour  int64    `json:"estimatedPerHour"`
}

type HuntingMapListResult struct {
	Maps []HuntingMap `json:"maps"`
}

type HuntingActionResult struct {
	MapID           string                     `json:"mapId"`
	MapName         string                     `json:"mapName"`
	MonsterName     string                     `json:"monsterName"`
	SpiritCost      int64                      `json:"spiritCost"`
	CultivationGain int64                      `json:"cultivationGain"`
	DoubleGain      bool                       `json:"doubleGain"`
	DoubleGainTimes int                        `json:"doubleGainTimes"`
	Breakthrough    bool                       `json:"breakthrough"`
	Snapshot        *repository.PlayerSnapshot `json:"snapshot"`
}

type InsufficientSpiritError struct {
	Required          float64
	Current           float64
	RegenRate         float64
	RetryAfterSeconds int64
}

func (e *InsufficientSpiritError) Error() string {
	return fmt.Sprintf("insufficient spirit: required %.2f, current %.2f", e.Required, e.Current)
}

type BreakthroughUnavailableError struct {
	RequiredCultivation int64
	CurrentCultivation  int64
}

func (e *BreakthroughUnavailableError) Error() string {
	return fmt.Sprintf("breakthrough unavailable: current %d, required %d", e.CurrentCultivation, e.RequiredCultivation)
}

type InvalidHuntingMapError struct {
	MapID string
}

func (e *InvalidHuntingMapError) Error() string {
	return fmt.Sprintf("invalid hunting map: %s", e.MapID)
}

type HuntingMapLockedError struct {
	MapID         string
	RequiredLevel int
	CurrentLevel  int
}

func (e *HuntingMapLockedError) Error() string {
	return fmt.Sprintf(
		"hunting map locked: map %s requires level %d current level %d",
		e.MapID,
		e.RequiredLevel,
		e.CurrentLevel,
	)
}

func (s *GameService) CultivateOnce(ctx context.Context, userID uuid.UUID) (*CultivationActionResult, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin cultivate once transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	state, err := loadCultivationState(ctx, tx, userID)
	if err != nil {
		return nil, err
	}

	spiritCost := currentCultivationCost(state.Level)
	if state.Spirit < float64(spiritCost) {
		return nil, &InsufficientSpiritError{
			Required:          float64(spiritCost),
			Current:           state.Spirit,
			RegenRate:         state.SpiritRate,
			RetryAfterSeconds: estimateSpiritRetryAfterSeconds(float64(spiritCost), state.Spirit, state.SpiritRate),
		}
	}

	gain := currentCultivationGain(state.Level)
	doubleGain := shouldDoubleGain(state.Luck)
	if doubleGain {
		gain *= 2
	}
	gain = applyCultivationRate(gain, state.CultivationRate)

	state.Spirit -= float64(spiritCost)
	state.Cultivation += gain

	breakthrough := false
	if state.Cultivation >= state.MaxCultivation {
		breakthrough = applyBreakthrough(state)
	}

	if err := persistCultivationState(ctx, tx, userID, state, 1, breakthrough); err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit cultivate once transaction: %w", err)
	}

	snapshot, err := s.userRepo.GetSnapshot(ctx, userID)
	if err != nil {
		return nil, err
	}

	return &CultivationActionResult{
		SpiritCost:      spiritCost,
		CultivationGain: gain,
		DoubleGain:      doubleGain,
		DoubleGainTimes: boolToInt(doubleGain),
		Breakthrough:    breakthrough,
		Snapshot:        snapshot,
	}, nil
}

func (s *GameService) CultivateUntilBreakthrough(ctx context.Context, userID uuid.UUID) (*CultivationActionResult, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin cultivate until breakthrough transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	state, err := loadCultivationState(ctx, tx, userID)
	if err != nil {
		return nil, err
	}

	baseGain := currentCultivationGain(state.Level)
	if baseGain <= 0 {
		baseGain = 1
	}

	remainingCultivation := maxInt64(0, state.MaxCultivation-state.Cultivation)
	times := int64(0)
	if remainingCultivation > 0 {
		times = int64(math.Ceil(float64(remainingCultivation) / float64(baseGain)))
	}

	spiritCostPerTime := currentCultivationCost(state.Level)
	totalSpiritCost := spiritCostPerTime * times
	if state.Spirit < float64(totalSpiritCost) {
		return nil, &InsufficientSpiritError{
			Required:          float64(totalSpiritCost),
			Current:           state.Spirit,
			RegenRate:         state.SpiritRate,
			RetryAfterSeconds: estimateSpiritRetryAfterSeconds(float64(totalSpiritCost), state.Spirit, state.SpiritRate),
		}
	}

	totalGain := int64(0)
	doubleGainTimes := 0
	for i := int64(0); i < times; i++ {
		gain := baseGain
		if shouldDoubleGain(state.Luck) {
			doubleGainTimes++
			gain *= 2
		}
		totalGain += applyCultivationRate(gain, state.CultivationRate)
	}

	state.Spirit -= float64(totalSpiritCost)
	state.Cultivation += totalGain

	breakthrough := false
	if state.Cultivation >= state.MaxCultivation {
		breakthrough = applyBreakthrough(state)
	}

	if err := persistCultivationState(ctx, tx, userID, state, times, breakthrough); err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit cultivate until breakthrough transaction: %w", err)
	}

	snapshot, err := s.userRepo.GetSnapshot(ctx, userID)
	if err != nil {
		return nil, err
	}

	return &CultivationActionResult{
		SpiritCost:      totalSpiritCost,
		CultivationGain: totalGain,
		DoubleGain:      doubleGainTimes > 0,
		DoubleGainTimes: doubleGainTimes,
		Breakthrough:    breakthrough,
		Snapshot:        snapshot,
	}, nil
}

func (s *GameService) Breakthrough(ctx context.Context, userID uuid.UUID) (*CultivationActionResult, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin breakthrough transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	state, err := loadCultivationState(ctx, tx, userID)
	if err != nil {
		return nil, err
	}

	if state.Cultivation < state.MaxCultivation {
		return nil, &BreakthroughUnavailableError{
			RequiredCultivation: state.MaxCultivation,
			CurrentCultivation:  state.Cultivation,
		}
	}

	breakthrough := applyBreakthrough(state)
	if !breakthrough {
		return nil, errors.New("already reached max realm")
	}

	if err := persistCultivationState(ctx, tx, userID, state, 0, breakthrough); err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit breakthrough transaction: %w", err)
	}

	snapshot, err := s.userRepo.GetSnapshot(ctx, userID)
	if err != nil {
		return nil, err
	}

	return &CultivationActionResult{
		SpiritCost:      0,
		CultivationGain: 0,
		DoubleGain:      false,
		DoubleGainTimes: 0,
		Breakthrough:    true,
		Snapshot:        snapshot,
	}, nil
}

func (s *GameService) ListHuntingMaps(ctx context.Context, userID uuid.UUID) (*HuntingMapListResult, error) {
	snapshot, err := s.userRepo.GetSnapshot(ctx, userID)
	if err != nil {
		return nil, err
	}
	if snapshot == nil {
		return nil, fmt.Errorf("player snapshot not found")
	}

	baseCost := currentCultivationCost(snapshot.Level)
	baseGain := currentCultivationGain(snapshot.Level)
	maps := make([]HuntingMap, 0, len(huntingMapCatalog))
	for _, cfg := range huntingMapCatalog {
		estimatedCost := applyHuntingFactor(baseCost, cfg.RewardFactor)
		estimatedGain := int64(math.Ceil(float64(alignCultivationGainByCost(baseCost, baseGain, estimatedCost)) * huntingWinGainMultiplier))
		if estimatedGain <= 0 {
			estimatedGain = 1
		}
		estimatedPerHour := estimateCultivationPerHour(estimatedCost, estimatedGain, snapshot.SpiritRate)
		recommendedPower, recommendedHealth := estimateHuntingMapEntryRequirements(snapshot.Level, cfg)
		maps = append(maps, HuntingMap{
			ID:                cfg.ID,
			Name:              cfg.Name,
			Description:       cfg.Description,
			MinLevel:          cfg.MinLevel,
			RewardFactor:      cfg.RewardFactor,
			RecommendedPower:  recommendedPower,
			RecommendedHealth: recommendedHealth,
			EstimatedCost:     estimatedCost,
			EstimatedGain:     estimatedGain,
			Monsters:          cloneStringSlice(cfg.Monsters),
			ProgressionNote:   "日常刷图升级效率约为打坐的2倍，建议按等级选择地图长期挂机。",
			EstimatedPerHour:  estimatedPerHour,
		})
	}

	return &HuntingMapListResult{Maps: maps}, nil
}

func (s *GameService) HuntOnce(ctx context.Context, userID uuid.UUID, mapID string) (*HuntingActionResult, error) {
	mapID = strings.TrimSpace(mapID)
	if mapID == "" {
		return nil, &InvalidHuntingMapError{MapID: mapID}
	}
	targetMap, ok := findHuntingMapByID(mapID)
	if !ok {
		return nil, &InvalidHuntingMapError{MapID: mapID}
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin hunting transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	state, err := loadCultivationState(ctx, tx, userID)
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

	baseCost := currentCultivationCost(state.Level)
	baseGain := currentCultivationGain(state.Level)

	spiritCost := applyHuntingFactor(baseCost, targetMap.RewardFactor)
	if state.Spirit < float64(spiritCost) {
		return nil, &InsufficientSpiritError{
			Required:          float64(spiritCost),
			Current:           state.Spirit,
			RegenRate:         state.SpiritRate,
			RetryAfterSeconds: estimateSpiritRetryAfterSeconds(float64(spiritCost), state.Spirit, state.SpiritRate),
		}
	}

	gain := int64(math.Ceil(float64(alignCultivationGainByCost(baseCost, baseGain, spiritCost)) * huntingWinGainMultiplier))
	if gain <= 0 {
		gain = 1
	}
	doubleGain := shouldDoubleGain(state.Luck)
	if doubleGain {
		gain *= 2
	}
	gain = applyCultivationRate(gain, state.CultivationRate)

	state.Spirit -= float64(spiritCost)
	state.Cultivation += gain

	breakthrough := false
	if state.Cultivation >= state.MaxCultivation {
		breakthrough = applyBreakthrough(state)
	}

	if err := persistCultivationState(ctx, tx, userID, state, 1, breakthrough); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit hunting transaction: %w", err)
	}

	snapshot, err := s.userRepo.GetSnapshot(ctx, userID)
	if err != nil {
		return nil, err
	}

	return &HuntingActionResult{
		MapID:           targetMap.ID,
		MapName:         targetMap.Name,
		MonsterName:     randomHuntingMonster(targetMap.Monsters),
		SpiritCost:      spiritCost,
		CultivationGain: gain,
		DoubleGain:      doubleGain,
		DoubleGainTimes: boolToInt(doubleGain),
		Breakthrough:    breakthrough,
		Snapshot:        snapshot,
	}, nil
}

type cultivationState struct {
	Level           int
	Realm           string
	Cultivation     int64
	MaxCultivation  int64
	Spirit          float64
	SpiritRate      float64
	Luck            float64
	CultivationRate float64
}

func loadCultivationState(ctx context.Context, tx pgx.Tx, userID uuid.UUID) (*cultivationState, error) {
	const query = `
		SELECT
			pp.level,
			pp.realm,
			pp.cultivation,
			pp.max_cultivation,
				pr.spirit + (LEAST(GREATEST(EXTRACT(EPOCH FROM now() - pr.updated_at), 0), 43200) * pr.spirit_rate),
			pr.spirit_rate,
			pr.luck,
			pr.cultivation_rate
		FROM player_profiles pp
		JOIN player_resources pr ON pr.user_id = pp.user_id
		WHERE pp.user_id = $1
		FOR UPDATE OF pp, pr
	`

	state := &cultivationState{}
	if err := tx.QueryRow(ctx, query, userID).Scan(
		&state.Level,
		&state.Realm,
		&state.Cultivation,
		&state.MaxCultivation,
		&state.Spirit,
		&state.SpiritRate,
		&state.Luck,
		&state.CultivationRate,
	); err != nil {
		return nil, fmt.Errorf("load cultivation state: %w", err)
	}
	return state, nil
}

func estimateHuntingMapEntryRequirements(playerLevel int, cfg huntingMapConfig) (int64, int64) {
	referenceLevel := maxInt(playerLevel, cfg.MinLevel)
	effectiveKillCount := huntingDifficultyKillCount(cfg, 0)
	progressScale := 1 + math.Min(80, float64(effectiveKillCount))*0.015
	levelScale := 1 + float64(maxInt(0, referenceLevel-1))*0.06
	mapScale := 0.9 + cfg.RewardFactor*0.25
	scale := progressScale * levelScale * mapScale

	// 敌方强度按初始遭遇概率做加权：普通 88%，精英 10%，首领 2%。
	const (
		avgHealthMult  = 1.069
		avgDamageMult  = 1.051
		avgDefenseMult = 1.032
		avgSpeedMult   = 1.015
	)

	enemyHealth := (40 + float64(referenceLevel)*15) * scale * avgHealthMult
	enemyDamage := (5 + float64(referenceLevel)*1.4) * scale * avgDamageMult
	enemyDefense := (2 + float64(referenceLevel)*0.8) * scale * avgDefenseMult
	enemySpeed := (6 + float64(referenceLevel)*0.9) * scale * avgSpeedMult

	enemyPower := enemyDamage*2 + enemyDefense*1.5 + enemyHealth*0.2 + enemySpeed + float64(referenceLevel)*10
	recommendedPower := int64(math.Ceil(enemyPower * 1.15))
	if recommendedPower < 60 {
		recommendedPower = 60
	}

	recommendedHealth := int64(math.Ceil(enemyDamage * 11.5))
	minHealthByEnemyHP := int64(math.Ceil(enemyHealth * 0.25))
	if recommendedHealth < minHealthByEnemyHP {
		recommendedHealth = minHealthByEnemyHP
	}
	if recommendedHealth < 100 {
		recommendedHealth = 100
	}
	return recommendedPower, recommendedHealth
}

func persistCultivationState(ctx context.Context, tx pgx.Tx, userID uuid.UUID, state *cultivationState, cultivationTimes int64, breakthrough bool) error {
	const updateProfileSQL = `
		UPDATE player_profiles
		SET level = $2, realm = $3, cultivation = $4, max_cultivation = $5, updated_at = now()
		WHERE user_id = $1
	`
	if _, err := tx.Exec(ctx, updateProfileSQL, userID, state.Level, state.Realm, state.Cultivation, state.MaxCultivation); err != nil {
		return fmt.Errorf("update player profile: %w", err)
	}

	const updateResourceSQL = `
		UPDATE player_resources
		SET spirit = $2, spirit_rate = $3, updated_at = now()
		WHERE user_id = $1
	`
	if _, err := tx.Exec(ctx, updateResourceSQL, userID, state.Spirit, state.SpiritRate); err != nil {
		return fmt.Errorf("update player resources: %w", err)
	}

	breakthroughIncrement := 0
	if breakthrough {
		breakthroughIncrement = 1
	}

	const updateStatsSQL = `
		INSERT INTO player_cultivation_stats (user_id, total_cultivation_time, breakthrough_count, updated_at)
		VALUES ($1, $2, $3, now())
		ON CONFLICT (user_id)
		DO UPDATE SET
			total_cultivation_time = player_cultivation_stats.total_cultivation_time + EXCLUDED.total_cultivation_time,
			breakthrough_count = player_cultivation_stats.breakthrough_count + EXCLUDED.breakthrough_count,
			updated_at = now()
	`
	if _, err := tx.Exec(ctx, updateStatsSQL, userID, cultivationTimes, breakthroughIncrement); err != nil {
		return fmt.Errorf("update cultivation stats: %w", err)
	}

	return nil
}

func currentCultivationCost(level int) int64 {
	return int64(math.Floor(10 * math.Pow(1.5, float64(level-1))))
}

func currentCultivationGain(level int) int64 {
	gain := int64(math.Floor(1 * math.Pow(1.2, float64(level-1))))
	if gain <= 0 {
		return 1
	}
	return gain
}

func applyCultivationRate(baseGain int64, cultivationRate float64) int64 {
	if cultivationRate <= 0 {
		cultivationRate = 1
	}
	gain := int64(math.Floor(float64(baseGain) * cultivationRate))
	if gain <= 0 {
		return 1
	}
	return gain
}

func shouldDoubleGain(luck float64) bool {
	chance := 0.3 * luck
	if chance < 0 {
		chance = 0
	}
	if chance > 1 {
		chance = 1
	}
	return rand.Float64() < chance
}

func applyBreakthrough(state *cultivationState) bool {
	if state.Level >= realmCount() {
		return false
	}

	state.Level++
	realm := realmByLevel(state.Level)
	state.Realm = realm.Name
	state.MaxCultivation = realm.MaxCultivation
	state.Cultivation = 0
	state.Spirit += float64(100 * state.Level)
	state.SpiritRate *= 1.2
	return true
}

func maxInt64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}

func boolToInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

func applyHuntingFactor(base int64, factor float64) int64 {
	if factor <= 0 {
		factor = 1
	}
	value := int64(math.Floor(float64(base) * factor))
	if value <= 0 {
		return 1
	}
	return value
}

func alignCultivationGainByCost(baseCost int64, baseGain int64, targetCost int64) int64 {
	if baseCost <= 0 || baseGain <= 0 || targetCost <= 0 {
		return 1
	}
	gain := int64(math.Round(float64(targetCost) * (float64(baseGain) / float64(baseCost))))
	if gain <= 0 {
		return 1
	}
	return gain
}

func randomHuntingMonster(monsters []string) string {
	if len(monsters) == 0 {
		return "妖兽"
	}
	return monsters[rand.Intn(len(monsters))]
}

func estimateCultivationPerHour(cost int64, gain int64, spiritRate float64) int64 {
	if cost <= 0 || gain <= 0 {
		return 0
	}
	if spiritRate <= 0 {
		return 0
	}
	actionsPerSecond := spiritRate / float64(cost)
	if actionsPerSecond > 1 {
		actionsPerSecond = 1
	}
	if actionsPerSecond < 0 {
		actionsPerSecond = 0
	}
	perHour := int64(math.Floor(actionsPerSecond * float64(gain) * 3600))
	if perHour < 0 {
		return 0
	}
	return perHour
}

func cloneStringSlice(values []string) []string {
	if len(values) == 0 {
		return []string{}
	}
	out := make([]string, len(values))
	copy(out, values)
	return out
}

func estimateSpiritRetryAfterSeconds(required float64, current float64, spiritRate float64) int64 {
	if required <= current {
		return 1
	}
	if spiritRate <= 0 {
		return 3
	}
	missing := required - current
	seconds := int64(math.Ceil(missing / spiritRate))
	if seconds < 1 {
		return 1
	}
	if seconds > 24*60*60 {
		return 24 * 60 * 60
	}
	return seconds
}
