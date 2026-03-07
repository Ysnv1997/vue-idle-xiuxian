CREATE TABLE IF NOT EXISTS player_exploration_runs (
  user_id UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
  is_active BOOLEAN NOT NULL DEFAULT FALSE,
  location_id TEXT NOT NULL DEFAULT '',
  location_name TEXT NOT NULL DEFAULT '',
  total_runs BIGINT NOT NULL DEFAULT 0,
  total_spirit_cost BIGINT NOT NULL DEFAULT 0,
  last_state TEXT NOT NULL DEFAULT 'stopped',
  last_log_seq BIGINT NOT NULL DEFAULT 0,
  last_log_message TEXT NOT NULL DEFAULT '',
  started_at TIMESTAMPTZ,
  ended_at TIMESTAMPTZ,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_player_exploration_runs_active
  ON player_exploration_runs (updated_at)
  WHERE is_active = TRUE;

INSERT INTO player_exploration_runs (user_id)
SELECT id
FROM users
ON CONFLICT (user_id) DO NOTHING;
