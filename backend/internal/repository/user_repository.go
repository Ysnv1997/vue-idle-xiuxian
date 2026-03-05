package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type UserRepository struct {
	pool *pgxpool.Pool
}

func NewUserRepository(pool *pgxpool.Pool) *UserRepository {
	return &UserRepository{pool: pool}
}

func (r *UserRepository) FindByLinuxDoUserID(ctx context.Context, linuxDoUserID string) (*User, error) {
	const query = `
		SELECT id, linux_do_user_id, linux_do_username, COALESCE(linux_do_avatar, ''), COALESCE(last_login_at, now())
		FROM users
		WHERE linux_do_user_id = $1
	`

	var user User
	err := r.pool.QueryRow(ctx, query, linuxDoUserID).Scan(
		&user.ID,
		&user.LinuxDoUserID,
		&user.LinuxDoUsername,
		&user.LinuxDoAvatar,
		&user.LastLoginAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("find user by linux do user id: %w", err)
	}
	return &user, nil
}

func (r *UserRepository) FindByID(ctx context.Context, userID uuid.UUID) (*User, error) {
	const query = `
		SELECT id, linux_do_user_id, linux_do_username, COALESCE(linux_do_avatar, ''), COALESCE(last_login_at, now())
		FROM users
		WHERE id = $1
	`

	var user User
	err := r.pool.QueryRow(ctx, query, userID).Scan(
		&user.ID,
		&user.LinuxDoUserID,
		&user.LinuxDoUsername,
		&user.LinuxDoAvatar,
		&user.LastLoginAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("find user by id: %w", err)
	}
	return &user, nil
}

func (r *UserRepository) UpsertLinuxDoUser(ctx context.Context, linuxDoUserID, username, avatar string) (*User, error) {
	const query = `
		INSERT INTO users (linux_do_user_id, linux_do_username, linux_do_avatar, last_login_at)
		VALUES ($1, $2, $3, now())
		ON CONFLICT (linux_do_user_id)
		DO UPDATE SET
			linux_do_username = EXCLUDED.linux_do_username,
			linux_do_avatar = EXCLUDED.linux_do_avatar,
			last_login_at = now(),
			updated_at = now()
		RETURNING id, linux_do_user_id, linux_do_username, COALESCE(linux_do_avatar, ''), last_login_at
	`

	var user User
	err := r.pool.QueryRow(ctx, query, linuxDoUserID, username, avatar).Scan(
		&user.ID,
		&user.LinuxDoUserID,
		&user.LinuxDoUsername,
		&user.LinuxDoAvatar,
		&user.LastLoginAt,
	)
	if err != nil {
		return nil, fmt.Errorf("upsert linux do user: %w", err)
	}

	if err := r.ensureDefaultPlayerData(ctx, user.ID, username); err != nil {
		return nil, err
	}

	return &user, nil
}

func (r *UserRepository) ensureDefaultPlayerData(ctx context.Context, userID uuid.UUID, username string) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin ensure default player data transaction: %w", err)
	}
	defer func() {
		if tx != nil {
			_ = tx.Rollback(ctx)
		}
	}()

	const profileSQL = `
		INSERT INTO player_profiles (user_id, player_name, level, realm, cultivation, max_cultivation)
		VALUES ($1, $2, 1, '练气期一层', 0, 100)
		ON CONFLICT (user_id) DO NOTHING
	`
	if _, err := tx.Exec(ctx, profileSQL, userID, defaultPlayerName(username)); err != nil {
		return fmt.Errorf("ensure player profile: %w", err)
	}

	const resourcesSQL = `
		INSERT INTO player_resources (user_id, spirit, spirit_rate, luck, cultivation_rate, spirit_stones, reinforce_stones, refinement_stones, pet_essence)
		VALUES ($1, 0, 1, 1, 1, 0, 0, 0, 0)
		ON CONFLICT (user_id) DO NOTHING
	`
	if _, err := tx.Exec(ctx, resourcesSQL, userID); err != nil {
		return fmt.Errorf("ensure player resources: %w", err)
	}

	baseAttributes, combatAttributes, combatResistance, specialAttributes := defaultAttributesJSON()
	const attributesSQL = `
		INSERT INTO player_attributes (
			user_id,
			base_attributes,
			combat_attributes,
			combat_resistance,
			special_attributes,
			version
		)
		VALUES ($1, $2::jsonb, $3::jsonb, $4::jsonb, $5::jsonb, 1)
		ON CONFLICT (user_id) DO NOTHING
	`
	if _, err := tx.Exec(ctx, attributesSQL, userID, baseAttributes, combatAttributes, combatResistance, specialAttributes); err != nil {
		return fmt.Errorf("ensure player attributes: %w", err)
	}

	const inventoryStateSQL = `
		INSERT INTO player_inventory_state (user_id, herbs, pill_fragments, pill_recipes, items, active_pet_id, active_effects, equipped_artifacts)
		VALUES (
			$1,
			'[]'::jsonb,
			'{}'::jsonb,
			'[]'::jsonb,
			'[]'::jsonb,
			NULL,
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
			}'::jsonb
		)
		ON CONFLICT (user_id) DO NOTHING
	`
	if _, err := tx.Exec(ctx, inventoryStateSQL, userID); err != nil {
		return fmt.Errorf("ensure player inventory state: %w", err)
	}

	const cultivationStatsSQL = `
		INSERT INTO player_cultivation_stats (user_id, total_cultivation_time, breakthrough_count)
		VALUES ($1, 0, 0)
		ON CONFLICT (user_id) DO NOTHING
	`
	if _, err := tx.Exec(ctx, cultivationStatsSQL, userID); err != nil {
		return fmt.Errorf("ensure player cultivation stats: %w", err)
	}

	const explorationStatsSQL = `
		INSERT INTO player_exploration_stats (user_id, exploration_count, events_triggered, items_found)
		VALUES ($1, 0, 0, 0)
		ON CONFLICT (user_id) DO NOTHING
	`
	if _, err := tx.Exec(ctx, explorationStatsSQL, userID); err != nil {
		return fmt.Errorf("ensure player exploration stats: %w", err)
	}

	const dungeonProgressSQL = `
		INSERT INTO player_dungeon_progress (
			user_id, highest_floor, highest_floor_2x, highest_floor_5x, highest_floor_10x, highest_floor_100x,
			last_failed_floor, total_runs, boss_kills, elite_kills, total_kills, death_count, total_rewards
		)
		VALUES ($1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0)
		ON CONFLICT (user_id) DO NOTHING
	`
	if _, err := tx.Exec(ctx, dungeonProgressSQL, userID); err != nil {
		return fmt.Errorf("ensure player dungeon progress: %w", err)
	}

	const alchemyStatsSQL = `
		INSERT INTO player_alchemy_stats (user_id, pills_crafted, high_quality_pills_crafted)
		VALUES ($1, 0, 0)
		ON CONFLICT (user_id) DO NOTHING
	`
	if _, err := tx.Exec(ctx, alchemyStatsSQL, userID); err != nil {
		return fmt.Errorf("ensure player alchemy stats: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit ensure default player data transaction: %w", err)
	}
	tx = nil
	return nil
}

func (r *UserRepository) GetSnapshot(ctx context.Context, userID uuid.UUID) (*PlayerSnapshot, error) {
	const query = `
		SELECT
			pp.user_id,
			pp.player_name,
			pp.level,
			pp.realm,
			pp.cultivation,
			pp.max_cultivation,
			pr.spirit + (GREATEST(EXTRACT(EPOCH FROM now() - pr.updated_at), 0) * pr.spirit_rate),
			pr.spirit_rate,
			pr.luck,
			pr.cultivation_rate,
			pr.spirit_stones,
			pr.reinforce_stones,
			pr.refinement_stones,
			COALESCE(pr.pet_essence, 0),
			COALESCE(pes.exploration_count, 0),
			COALESCE(pes.events_triggered, 0),
			COALESCE(pdp.highest_floor, 0),
			COALESCE(pdp.highest_floor_2x, 0),
			COALESCE(pdp.highest_floor_5x, 0),
			COALESCE(pdp.highest_floor_10x, 0),
			COALESCE(pdp.highest_floor_100x, 0),
			COALESCE(pdp.last_failed_floor, 0),
			COALESCE(pdp.total_runs, 0),
			COALESCE(pdp.boss_kills, 0),
			COALESCE(pdp.elite_kills, 0),
			COALESCE(pdp.total_kills, 0),
			COALESCE(pdp.death_count, 0),
			COALESCE(pdp.total_rewards, 0),
			pa.base_attributes,
			pa.combat_attributes,
			pa.combat_resistance,
			pa.special_attributes,
			pis.herbs,
			pis.pill_fragments,
			pis.pill_recipes,
			COALESCE(pis.items, '[]'::jsonb),
			COALESCE(pis.active_pet_id, ''),
			COALESCE(pis.active_effects, '[]'::jsonb),
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
		LEFT JOIN player_exploration_stats pes ON pes.user_id = pp.user_id
		LEFT JOIN player_dungeon_progress pdp ON pdp.user_id = pp.user_id
		LEFT JOIN player_inventory_state pis ON pis.user_id = pp.user_id
		WHERE pp.user_id = $1
	`

	var snapshot PlayerSnapshot
	if err := r.pool.QueryRow(ctx, query, userID).Scan(
		&snapshot.UserID,
		&snapshot.Name,
		&snapshot.Level,
		&snapshot.Realm,
		&snapshot.Cultivation,
		&snapshot.MaxCultivation,
		&snapshot.Spirit,
		&snapshot.SpiritRate,
		&snapshot.Luck,
		&snapshot.CultivationRate,
		&snapshot.SpiritStones,
		&snapshot.ReinforceStones,
		&snapshot.RefinementStones,
		&snapshot.PetEssence,
		&snapshot.ExplorationCount,
		&snapshot.EventTriggered,
		&snapshot.DungeonHighestFloor,
		&snapshot.DungeonHighestFloor2,
		&snapshot.DungeonHighestFloor5,
		&snapshot.DungeonHighestFloor10,
		&snapshot.DungeonHighestFloor100,
		&snapshot.DungeonLastFailedFloor,
		&snapshot.DungeonTotalRuns,
		&snapshot.DungeonBossKills,
		&snapshot.DungeonEliteKills,
		&snapshot.DungeonTotalKills,
		&snapshot.DungeonDeathCount,
		&snapshot.DungeonTotalRewards,
		&snapshot.BaseAttributes,
		&snapshot.CombatAttributes,
		&snapshot.CombatResistance,
		&snapshot.SpecialAttributes,
		&snapshot.Herbs,
		&snapshot.PillFragments,
		&snapshot.PillRecipes,
		&snapshot.Items,
		&snapshot.ActivePetID,
		&snapshot.ActiveEffects,
		&snapshot.EquippedArtifacts,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("get player snapshot: %w", err)
	}

	return &snapshot, nil
}

func defaultPlayerName(username string) string {
	if username == "" {
		return "无名修士"
	}
	return username
}

func defaultAttributesJSON() (string, string, string, string) {
	baseAttributes := mustMarshalJSON(map[string]float64{
		"attack":  10,
		"health":  100,
		"defense": 5,
		"speed":   10,
	})
	combatAttributes := mustMarshalJSON(map[string]float64{
		"critRate":    0,
		"comboRate":   0,
		"counterRate": 0,
		"stunRate":    0,
		"dodgeRate":   0,
		"vampireRate": 0,
	})
	combatResistance := mustMarshalJSON(map[string]float64{
		"critResist":    0,
		"comboResist":   0,
		"counterResist": 0,
		"stunResist":    0,
		"dodgeResist":   0,
		"vampireResist": 0,
	})
	specialAttributes := mustMarshalJSON(map[string]float64{
		"healBoost":         0,
		"critDamageBoost":   0,
		"critDamageReduce":  0,
		"finalDamageBoost":  0,
		"finalDamageReduce": 0,
		"combatBoost":       0,
		"resistanceBoost":   0,
	})
	return baseAttributes, combatAttributes, combatResistance, specialAttributes
}

func mustMarshalJSON(payload map[string]float64) string {
	raw, err := json.Marshal(payload)
	if err != nil {
		panic(fmt.Errorf("marshal default attributes: %w", err))
	}
	return string(raw)
}

func (r *UserRepository) InsertEconomyLog(ctx context.Context, userID uuid.UUID, changeType string, amount int64, balanceAfter int64, detail string) error {
	const query = `
		INSERT INTO economy_logs (user_id, currency, change_type, amount, balance_after, detail, occurred_at)
		VALUES ($1, 'spirit_stones', $2, $3, $4, $5, $6)
	`
	_, err := r.pool.Exec(ctx, query, userID, changeType, amount, balanceAfter, detail, time.Now().UTC())
	if err != nil {
		return fmt.Errorf("insert economy log: %w", err)
	}
	return nil
}
