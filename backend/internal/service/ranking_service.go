package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	rankingScopeGlobal  = "global"
	rankingScopeFriends = "friends"
)

type RankingService struct {
	pool *pgxpool.Pool
}

func NewRankingService(pool *pgxpool.Pool) *RankingService {
	return &RankingService{pool: pool}
}

type RankingEntry struct {
	Rank   int64     `json:"rank"`
	UserID uuid.UUID `json:"userId"`
	Name   string    `json:"name"`
	Level  int       `json:"level"`
	Realm  string    `json:"realm"`
	Value  int64     `json:"value"`
}

type RankingsResult struct {
	Type    string         `json:"type"`
	Scope   string         `json:"scope"`
	Entries []RankingEntry `json:"entries"`
	Self    *RankingEntry  `json:"self,omitempty"`
}

type RankingFollowEntry struct {
	UserID     uuid.UUID `json:"userId"`
	Name       string    `json:"name"`
	Level      int       `json:"level"`
	Realm      string    `json:"realm"`
	FollowedAt time.Time `json:"followedAt"`
}

type RankingFollowListResult struct {
	Follows []RankingFollowEntry `json:"follows"`
}

type InvalidRankingTypeError struct {
	Type string
}

func (e *InvalidRankingTypeError) Error() string {
	return fmt.Sprintf("invalid ranking type: %s", e.Type)
}

type InvalidRankingScopeError struct {
	Scope string
}

func (e *InvalidRankingScopeError) Error() string {
	return fmt.Sprintf("invalid ranking scope: %s", e.Scope)
}

type RankingFollowSelfNotAllowedError struct {
	TargetUserID uuid.UUID
}

func (e *RankingFollowSelfNotAllowedError) Error() string {
	return fmt.Sprintf("ranking follow self not allowed: %s", e.TargetUserID.String())
}

type RankingFollowTargetNotFoundError struct {
	TargetUserID uuid.UUID
}

func (e *RankingFollowTargetNotFoundError) Error() string {
	return fmt.Sprintf("ranking follow target not found: %s", e.TargetUserID.String())
}

type rankingQueryConfig struct {
	Type          string
	Joins         string
	ScoreExpr     string
	SecondaryExpr string
}

func (s *RankingService) List(ctx context.Context, userID uuid.UUID, rankingType string, scope string, limit int) (*RankingsResult, error) {
	config, err := rankingConfigForType(rankingType)
	if err != nil {
		return nil, err
	}

	normalizedScope, err := normalizeRankingScope(scope)
	if err != nil {
		return nil, err
	}

	if limit <= 0 {
		limit = 50
	}
	if limit > 100 {
		limit = 100
	}

	entries, err := s.listWithConfigAndScope(ctx, userID, config, normalizedScope, limit)
	if err != nil {
		return nil, err
	}

	self, err := s.selfWithConfigAndScope(ctx, userID, config, normalizedScope)
	if err != nil {
		return nil, err
	}

	return &RankingsResult{
		Type:    config.Type,
		Scope:   normalizedScope,
		Entries: entries,
		Self:    self,
	}, nil
}

func (s *RankingService) Self(ctx context.Context, userID uuid.UUID, rankingType string, scope string) (*RankingEntry, error) {
	config, err := rankingConfigForType(rankingType)
	if err != nil {
		return nil, err
	}

	normalizedScope, err := normalizeRankingScope(scope)
	if err != nil {
		return nil, err
	}

	return s.selfWithConfigAndScope(ctx, userID, config, normalizedScope)
}

func (s *RankingService) Follow(ctx context.Context, followerUserID uuid.UUID, targetUserID uuid.UUID) (bool, error) {
	if targetUserID == uuid.Nil {
		return false, &RankingFollowTargetNotFoundError{TargetUserID: targetUserID}
	}
	if followerUserID == targetUserID {
		return false, &RankingFollowSelfNotAllowedError{TargetUserID: targetUserID}
	}

	const targetExistsQuery = `SELECT EXISTS(SELECT 1 FROM users WHERE id = $1)`
	var exists bool
	if err := s.pool.QueryRow(ctx, targetExistsQuery, targetUserID).Scan(&exists); err != nil {
		return false, fmt.Errorf("query follow target exists: %w", err)
	}
	if !exists {
		return false, &RankingFollowTargetNotFoundError{TargetUserID: targetUserID}
	}

	const query = `
		INSERT INTO player_follows (follower_user_id, followee_user_id, created_at)
		VALUES ($1, $2, now())
		ON CONFLICT (follower_user_id, followee_user_id) DO NOTHING
	`
	tag, err := s.pool.Exec(ctx, query, followerUserID, targetUserID)
	if err != nil {
		return false, fmt.Errorf("insert player follow relation: %w", err)
	}
	return tag.RowsAffected() > 0, nil
}

func (s *RankingService) Unfollow(ctx context.Context, followerUserID uuid.UUID, targetUserID uuid.UUID) (bool, error) {
	if targetUserID == uuid.Nil {
		return false, &RankingFollowTargetNotFoundError{TargetUserID: targetUserID}
	}

	const query = `
		DELETE FROM player_follows
		WHERE follower_user_id = $1
		  AND followee_user_id = $2
	`
	tag, err := s.pool.Exec(ctx, query, followerUserID, targetUserID)
	if err != nil {
		return false, fmt.Errorf("delete player follow relation: %w", err)
	}
	return tag.RowsAffected() > 0, nil
}

func (s *RankingService) ListFollows(ctx context.Context, followerUserID uuid.UUID, limit int) (*RankingFollowListResult, error) {
	if limit <= 0 {
		limit = 100
	}
	if limit > 200 {
		limit = 200
	}

	const query = `
		SELECT
			pf.followee_user_id,
			COALESCE(pp.player_name, '未知修士'),
			COALESCE(pp.level, 1),
			COALESCE(pp.realm, '练气期一层'),
			pf.created_at
		FROM player_follows pf
		LEFT JOIN player_profiles pp ON pp.user_id = pf.followee_user_id
		WHERE pf.follower_user_id = $1
		ORDER BY pf.created_at DESC
		LIMIT $2
	`

	rows, err := s.pool.Query(ctx, query, followerUserID, limit)
	if err != nil {
		return nil, fmt.Errorf("query ranking follows: %w", err)
	}
	defer rows.Close()

	follows := make([]RankingFollowEntry, 0, limit)
	for rows.Next() {
		entry := RankingFollowEntry{}
		if scanErr := rows.Scan(
			&entry.UserID,
			&entry.Name,
			&entry.Level,
			&entry.Realm,
			&entry.FollowedAt,
		); scanErr != nil {
			return nil, fmt.Errorf("scan ranking follows row: %w", scanErr)
		}
		follows = append(follows, entry)
	}
	if rows.Err() != nil {
		return nil, fmt.Errorf("iterate ranking follows rows: %w", rows.Err())
	}

	return &RankingFollowListResult{Follows: follows}, nil
}

func (s *RankingService) listWithConfigAndScope(ctx context.Context, userID uuid.UUID, config rankingQueryConfig, scope string, limit int) ([]RankingEntry, error) {
	var (
		rows pgx.Rows
		err  error
	)

	switch scope {
	case rankingScopeGlobal:
		rows, err = s.pool.Query(ctx, rankingListQueryGlobal(config), limit)
	case rankingScopeFriends:
		rows, err = s.pool.Query(ctx, rankingListQueryFriends(config), userID, limit)
	default:
		return nil, &InvalidRankingScopeError{Scope: scope}
	}
	if err != nil {
		return nil, fmt.Errorf("query rankings list: %w", err)
	}
	defer rows.Close()

	entries := make([]RankingEntry, 0, limit)
	for rows.Next() {
		var entry RankingEntry
		if scanErr := rows.Scan(
			&entry.UserID,
			&entry.Name,
			&entry.Level,
			&entry.Realm,
			&entry.Value,
			&entry.Rank,
		); scanErr != nil {
			return nil, fmt.Errorf("scan rankings list row: %w", scanErr)
		}
		entries = append(entries, entry)
	}
	if rows.Err() != nil {
		return nil, fmt.Errorf("iterate rankings list rows: %w", rows.Err())
	}

	return entries, nil
}

func (s *RankingService) selfWithConfigAndScope(ctx context.Context, userID uuid.UUID, config rankingQueryConfig, scope string) (*RankingEntry, error) {
	var (
		entry RankingEntry
		err   error
	)

	switch scope {
	case rankingScopeGlobal:
		err = s.pool.QueryRow(ctx, rankingSelfQueryGlobal(config), userID).Scan(
			&entry.UserID,
			&entry.Name,
			&entry.Level,
			&entry.Realm,
			&entry.Value,
			&entry.Rank,
		)
	case rankingScopeFriends:
		err = s.pool.QueryRow(ctx, rankingSelfQueryFriends(config), userID).Scan(
			&entry.UserID,
			&entry.Name,
			&entry.Level,
			&entry.Realm,
			&entry.Value,
			&entry.Rank,
		)
	default:
		return nil, &InvalidRankingScopeError{Scope: scope}
	}
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("query self ranking: %w", err)
	}
	return &entry, nil
}

func rankingConfigForType(input string) (rankingQueryConfig, error) {
	normalized := strings.ToLower(strings.TrimSpace(input))
	switch normalized {
	case "", "realm", "level":
		return rankingQueryConfig{
			Type:          "realm",
			ScoreExpr:     "pp.level::BIGINT",
			SecondaryExpr: "pp.cultivation",
		}, nil
	case "cultivation":
		return rankingQueryConfig{
			Type:          "cultivation",
			ScoreExpr:     "pp.cultivation",
			SecondaryExpr: "pp.level::BIGINT",
		}, nil
	case "dungeon":
		return rankingQueryConfig{
			Type:          "dungeon",
			Joins:         "LEFT JOIN player_dungeon_progress pdp ON pdp.user_id = pp.user_id",
			ScoreExpr:     "COALESCE(pdp.highest_floor, 0)::BIGINT",
			SecondaryExpr: "pp.level::BIGINT",
		}, nil
	case "wealth":
		return rankingQueryConfig{
			Type:          "wealth",
			Joins:         "LEFT JOIN player_resources pr ON pr.user_id = pp.user_id",
			ScoreExpr:     "COALESCE(pr.spirit_stones, 0)",
			SecondaryExpr: "pp.level::BIGINT",
		}, nil
	case "power":
		return rankingQueryConfig{
			Type:  "power",
			Joins: "LEFT JOIN player_attributes pa ON pa.user_id = pp.user_id",
			ScoreExpr: `(
				COALESCE((pa.base_attributes ->> 'attack')::DOUBLE PRECISION, 0) * 2 +
				COALESCE((pa.base_attributes ->> 'defense')::DOUBLE PRECISION, 0) * 1.5 +
				COALESCE((pa.base_attributes ->> 'health')::DOUBLE PRECISION, 0) * 0.2 +
				COALESCE((pa.base_attributes ->> 'speed')::DOUBLE PRECISION, 0) +
				pp.level * 10
			)::BIGINT`,
			SecondaryExpr: "pp.cultivation",
		}, nil
	default:
		return rankingQueryConfig{}, &InvalidRankingTypeError{Type: input}
	}
}

func normalizeRankingScope(input string) (string, error) {
	normalized := strings.ToLower(strings.TrimSpace(input))
	switch normalized {
	case "", rankingScopeGlobal:
		return rankingScopeGlobal, nil
	case rankingScopeFriends, "friend":
		return rankingScopeFriends, nil
	default:
		return "", &InvalidRankingScopeError{Scope: input}
	}
}

func rankingCTE(config rankingQueryConfig, filterClause string) string {
	return fmt.Sprintf(`
		WITH scored AS (
			SELECT
				pp.user_id,
				pp.player_name,
				pp.level,
				pp.realm,
				%s AS score,
				%s AS secondary_score
			FROM player_profiles pp
			%s
			%s
		),
		ranked AS (
			SELECT
				user_id,
				player_name,
				level,
				realm,
				score,
				ROW_NUMBER() OVER (ORDER BY score DESC, secondary_score DESC, user_id) AS rank
			FROM scored
		)
	`, config.ScoreExpr, config.SecondaryExpr, config.Joins, filterClause)
}

func rankingFriendsFilter(userPlaceholder string) string {
	return fmt.Sprintf(`
			WHERE pp.user_id = %[1]s
			   OR EXISTS (
				SELECT 1
				FROM player_follows pf
				WHERE pf.follower_user_id = %[1]s
				  AND pf.followee_user_id = pp.user_id
			)
	`, userPlaceholder)
}

func rankingListQueryGlobal(config rankingQueryConfig) string {
	return rankingCTE(config, "") + `
		SELECT user_id, player_name, level, realm, score, rank
		FROM ranked
		ORDER BY rank
		LIMIT $1
	`
}

func rankingListQueryFriends(config rankingQueryConfig) string {
	return rankingCTE(config, rankingFriendsFilter("$1")) + `
		SELECT user_id, player_name, level, realm, score, rank
		FROM ranked
		ORDER BY rank
		LIMIT $2
	`
}

func rankingSelfQueryGlobal(config rankingQueryConfig) string {
	return rankingCTE(config, "") + `
		SELECT user_id, player_name, level, realm, score, rank
		FROM ranked
		WHERE user_id = $1
	`
}

func rankingSelfQueryFriends(config rankingQueryConfig) string {
	return rankingCTE(config, rankingFriendsFilter("$1")) + `
		SELECT user_id, player_name, level, realm, score, rank
		FROM ranked
		WHERE user_id = $1
	`
}
