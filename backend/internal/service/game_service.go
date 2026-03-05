package service

import (
	"context"
	"errors"
	"fmt"
	"math"
	"math/rand"

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
			pr.spirit + (GREATEST(EXTRACT(EPOCH FROM now() - pr.updated_at), 0) * pr.spirit_rate),
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
