ALTER TABLE player_hunting_runs
  ADD COLUMN IF NOT EXISTS last_processed_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  ADD COLUMN IF NOT EXISTS failure_count INTEGER NOT NULL DEFAULT 0,
  ADD COLUMN IF NOT EXISTS last_error TEXT NOT NULL DEFAULT '';

ALTER TABLE player_meditation_runs
  ADD COLUMN IF NOT EXISTS last_processed_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  ADD COLUMN IF NOT EXISTS failure_count INTEGER NOT NULL DEFAULT 0,
  ADD COLUMN IF NOT EXISTS last_error TEXT NOT NULL DEFAULT '';

ALTER TABLE player_exploration_runs
  ADD COLUMN IF NOT EXISTS last_processed_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  ADD COLUMN IF NOT EXISTS failure_count INTEGER NOT NULL DEFAULT 0,
  ADD COLUMN IF NOT EXISTS last_error TEXT NOT NULL DEFAULT '';

UPDATE player_hunting_runs
SET last_processed_at = COALESCE(updated_at, now())
WHERE last_processed_at IS NULL;

UPDATE player_meditation_runs
SET last_processed_at = COALESCE(updated_at, now())
WHERE last_processed_at IS NULL;

UPDATE player_exploration_runs
SET last_processed_at = COALESCE(updated_at, now())
WHERE last_processed_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_player_hunting_runs_active_processed
  ON player_hunting_runs (is_active, last_processed_at ASC, updated_at ASC);

CREATE INDEX IF NOT EXISTS idx_player_meditation_runs_active_processed
  ON player_meditation_runs (last_processed_at ASC, updated_at ASC)
  WHERE is_active = TRUE;

CREATE INDEX IF NOT EXISTS idx_player_exploration_runs_active_processed
  ON player_exploration_runs (last_processed_at ASC, updated_at ASC)
  WHERE is_active = TRUE;
