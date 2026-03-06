CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE IF NOT EXISTS users (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  linux_do_user_id TEXT NOT NULL UNIQUE,
  linux_do_username TEXT NOT NULL,
  linux_do_avatar TEXT,
  last_login_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS oauth_accounts (
  id BIGSERIAL PRIMARY KEY,
  user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  provider TEXT NOT NULL,
  subject TEXT NOT NULL,
  access_token TEXT,
  refresh_token TEXT,
  expires_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (provider, subject),
  UNIQUE (user_id, provider)
);

CREATE TABLE IF NOT EXISTS player_profiles (
  user_id UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
  player_name TEXT NOT NULL,
  level INT NOT NULL DEFAULT 1,
  realm TEXT NOT NULL DEFAULT '练气期一层',
  cultivation BIGINT NOT NULL DEFAULT 0,
  max_cultivation BIGINT NOT NULL DEFAULT 100,
  avatar_url TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS player_resources (
  user_id UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
  spirit DOUBLE PRECISION NOT NULL DEFAULT 0,
  spirit_rate DOUBLE PRECISION NOT NULL DEFAULT 1,
  spirit_stones BIGINT NOT NULL DEFAULT 0,
  reinforce_stones BIGINT NOT NULL DEFAULT 0,
  refinement_stones BIGINT NOT NULL DEFAULT 0,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS player_attributes (
  user_id UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
  base_attributes JSONB NOT NULL,
  combat_attributes JSONB NOT NULL,
  combat_resistance JSONB NOT NULL,
  special_attributes JSONB NOT NULL,
  version INT NOT NULL DEFAULT 1,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS player_achievements (
  user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  achievement_id TEXT NOT NULL,
  completed_at TIMESTAMPTZ,
  claimed_at TIMESTAMPTZ,
  PRIMARY KEY (user_id, achievement_id)
);

CREATE TABLE IF NOT EXISTS player_dungeon_progress (
  user_id UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
  highest_floor INT NOT NULL DEFAULT 0,
  highest_floor_2x INT NOT NULL DEFAULT 0,
  highest_floor_5x INT NOT NULL DEFAULT 0,
  highest_floor_10x INT NOT NULL DEFAULT 0,
  highest_floor_100x INT NOT NULL DEFAULT 0,
  last_failed_floor INT NOT NULL DEFAULT 0,
  total_runs BIGINT NOT NULL DEFAULT 0,
  boss_kills BIGINT NOT NULL DEFAULT 0,
  elite_kills BIGINT NOT NULL DEFAULT 0,
  total_kills BIGINT NOT NULL DEFAULT 0,
  death_count BIGINT NOT NULL DEFAULT 0,
  total_rewards BIGINT NOT NULL DEFAULT 0,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS player_exploration_stats (
  user_id UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
  exploration_count BIGINT NOT NULL DEFAULT 0,
  events_triggered BIGINT NOT NULL DEFAULT 0,
  items_found BIGINT NOT NULL DEFAULT 0,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS player_cultivation_stats (
  user_id UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
  total_cultivation_time BIGINT NOT NULL DEFAULT 0,
  breakthrough_count BIGINT NOT NULL DEFAULT 0,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS economy_logs (
  id BIGSERIAL PRIMARY KEY,
  user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  currency TEXT NOT NULL,
  change_type TEXT NOT NULL,
  amount BIGINT NOT NULL,
  balance_after BIGINT NOT NULL,
  detail TEXT,
  occurred_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_economy_logs_user_time ON economy_logs (user_id, occurred_at DESC);

CREATE TABLE IF NOT EXISTS game_action_logs (
  id BIGSERIAL PRIMARY KEY,
  user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  action_type TEXT NOT NULL,
  payload JSONB,
  occurred_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_game_action_logs_user_time ON game_action_logs (user_id, occurred_at DESC);

CREATE TABLE IF NOT EXISTS risk_events (
  id BIGSERIAL PRIMARY KEY,
  user_id UUID REFERENCES users(id) ON DELETE SET NULL,
  event_type TEXT NOT NULL,
  severity TEXT NOT NULL DEFAULT 'medium',
  detail JSONB,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS recharge_products (
  id BIGSERIAL PRIMARY KEY,
  code TEXT NOT NULL UNIQUE,
  credit_amount INT NOT NULL,
  spirit_stones BIGINT NOT NULL,
  bonus_rate DOUBLE PRECISION NOT NULL DEFAULT 0,
  enabled BOOLEAN NOT NULL DEFAULT TRUE,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS recharge_orders (
  id BIGSERIAL PRIMARY KEY,
  user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  product_code TEXT NOT NULL,
  credit_amount INT NOT NULL,
  spirit_stones BIGINT NOT NULL,
  external_order_id TEXT,
  status TEXT NOT NULL,
  idempotency_key TEXT NOT NULL UNIQUE,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  paid_at TIMESTAMPTZ,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (external_order_id)
);

CREATE INDEX IF NOT EXISTS idx_recharge_orders_user_time ON recharge_orders (user_id, created_at DESC);

CREATE TABLE IF NOT EXISTS recharge_callbacks (
  id BIGSERIAL PRIMARY KEY,
  external_order_id TEXT,
  payload JSONB NOT NULL,
  signature_valid BOOLEAN NOT NULL DEFAULT FALSE,
  received_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

INSERT INTO recharge_products (code, credit_amount, spirit_stones, bonus_rate, enabled)
VALUES
  ('starter_6', 6, 600, 0, TRUE),
  ('growth_30', 30, 3300, 0.1, TRUE),
  ('ascend_68', 68, 8160, 0.2, TRUE),
  ('immortal_128', 128, 16640, 0.3, TRUE)
ON CONFLICT (code) DO NOTHING;
