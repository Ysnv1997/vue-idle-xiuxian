package service

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type ActivityConflictError struct {
	Conflict string
}

func (e *ActivityConflictError) Error() string {
	return fmt.Sprintf("activity conflict: %s", e.Conflict)
}

func ensureNoActiveDungeonRunTx(ctx context.Context, tx pgx.Tx, userID uuid.UUID) error {
	if err := ensureDungeonRows(ctx, tx, userID); err != nil {
		return err
	}

	active, err := loadDungeonRunActiveForUpdate(ctx, tx, userID)
	if err != nil {
		return err
	}
	if active {
		return &ActivityConflictError{Conflict: "dungeon"}
	}
	return nil
}

func lockDungeonRunTx(ctx context.Context, tx pgx.Tx, userID uuid.UUID) error {
	if err := ensureDungeonRows(ctx, tx, userID); err != nil {
		return err
	}
	if _, err := loadDungeonRunActiveForUpdate(ctx, tx, userID); err != nil {
		return err
	}
	return nil
}

func loadDungeonRunActiveForUpdate(ctx context.Context, tx pgx.Tx, userID uuid.UUID) (bool, error) {
	const query = `
		SELECT is_active
		FROM player_dungeon_runs
		WHERE user_id = $1
		FOR UPDATE
	`

	var active bool
	if err := tx.QueryRow(ctx, query, userID).Scan(&active); err != nil {
		return false, fmt.Errorf("load dungeon active state: %w", err)
	}
	return active, nil
}

func stopHuntingForConflictTx(ctx context.Context, tx pgx.Tx, userID uuid.UUID, message string) error {
	const query = `
		UPDATE player_hunting_runs
		SET
			is_active = FALSE,
			last_state = $2,
			revive_until = NULL,
			last_log_seq = COALESCE(last_log_seq, 0) + 1,
			last_log_message = $3,
			ended_at = now(),
			updated_at = now(),
			last_processed_at = now(),
			failure_count = 0,
			last_error = ''
		WHERE user_id = $1
		  AND is_active = TRUE
	`
	if _, err := tx.Exec(ctx, query, userID, huntingRunStateStopped, message); err != nil {
		return fmt.Errorf("stop hunting for conflict: %w", err)
	}
	return nil
}

func stopExplorationForConflictTx(ctx context.Context, tx pgx.Tx, userID uuid.UUID, message string) error {
	const query = `
		UPDATE player_exploration_runs
		SET
			is_active = FALSE,
			last_state = $2,
			last_log_seq = COALESCE(last_log_seq, 0) + 1,
			last_log_message = $3,
			ended_at = now(),
			updated_at = now(),
			last_processed_at = now(),
			failure_count = 0,
			last_error = ''
		WHERE user_id = $1
		  AND is_active = TRUE
	`
	if _, err := tx.Exec(ctx, query, userID, explorationRunStateStopped, message); err != nil {
		return fmt.Errorf("stop exploration for conflict: %w", err)
	}
	return nil
}
