CREATE TABLE IF NOT EXISTS player_hunting_runs (
  user_id UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
  is_active BOOLEAN NOT NULL DEFAULT FALSE,
  map_id TEXT NOT NULL DEFAULT '',
  map_name TEXT NOT NULL DEFAULT '',
  current_hp DOUBLE PRECISION NOT NULL DEFAULT 0,
  max_hp DOUBLE PRECISION NOT NULL DEFAULT 0,
  kill_count BIGINT NOT NULL DEFAULT 0,
  total_spirit_cost BIGINT NOT NULL DEFAULT 0,
  total_cultivation_gain BIGINT NOT NULL DEFAULT 0,
  last_state TEXT NOT NULL DEFAULT '',
  started_at TIMESTAMPTZ,
  ended_at TIMESTAMPTZ,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_player_hunting_runs_active ON player_hunting_runs (is_active, updated_at DESC);
