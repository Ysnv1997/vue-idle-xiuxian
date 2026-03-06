CREATE TABLE IF NOT EXISTS player_hunting_events (
  id BIGSERIAL PRIMARY KEY,
  user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  map_id TEXT NOT NULL DEFAULT '',
  map_name TEXT NOT NULL DEFAULT '',
  monster_name TEXT NOT NULL DEFAULT '',
  enemy_tier TEXT NOT NULL DEFAULT '',
  state TEXT NOT NULL DEFAULT 'running',
  spirit_cost BIGINT NOT NULL DEFAULT 0,
  cultivation_gain BIGINT NOT NULL DEFAULT 0,
  double_gain_times INTEGER NOT NULL DEFAULT 0,
  breakthrough BOOLEAN NOT NULL DEFAULT FALSE,
  dropped_equipments JSONB NOT NULL DEFAULT '[]'::jsonb,
  logs JSONB NOT NULL DEFAULT '[]'::jsonb,
  current_hp DOUBLE PRECISION NOT NULL DEFAULT 0,
  max_hp DOUBLE PRECISION NOT NULL DEFAULT 0,
  kill_count BIGINT NOT NULL DEFAULT 0,
  total_spirit_cost BIGINT NOT NULL DEFAULT 0,
  total_cultivation_gain BIGINT NOT NULL DEFAULT 0,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_player_hunting_events_user_id_id
  ON player_hunting_events (user_id, id DESC);
