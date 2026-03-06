ALTER TABLE player_hunting_runs
  ADD COLUMN IF NOT EXISTS revive_until TIMESTAMPTZ;

ALTER TABLE player_hunting_runs
  ADD COLUMN IF NOT EXISTS last_log_seq BIGINT NOT NULL DEFAULT 0;

ALTER TABLE player_hunting_runs
  ADD COLUMN IF NOT EXISTS last_log_message TEXT NOT NULL DEFAULT '';
