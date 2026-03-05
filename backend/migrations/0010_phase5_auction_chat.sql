CREATE TABLE IF NOT EXISTS auction_orders (
  id BIGSERIAL PRIMARY KEY,
  seller_user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  buyer_user_id UUID REFERENCES users(id) ON DELETE SET NULL,
  item_id TEXT NOT NULL,
  item_payload JSONB NOT NULL,
  price BIGINT NOT NULL CHECK (price > 0),
  fee_rate DOUBLE PRECISION NOT NULL DEFAULT 0.05,
  fee_amount BIGINT NOT NULL DEFAULT 0,
  seller_income BIGINT NOT NULL DEFAULT 0,
  status TEXT NOT NULL DEFAULT 'open',
  expires_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  closed_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_auction_orders_status_created ON auction_orders (status, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_auction_orders_seller_created ON auction_orders (seller_user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_auction_orders_buyer_created ON auction_orders (buyer_user_id, created_at DESC);

CREATE TABLE IF NOT EXISTS chat_messages (
  id BIGSERIAL PRIMARY KEY,
  channel TEXT NOT NULL,
  sender_user_id UUID REFERENCES users(id) ON DELETE SET NULL,
  sender_name TEXT NOT NULL,
  content TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_chat_messages_channel_time ON chat_messages (channel, created_at DESC);

CREATE TABLE IF NOT EXISTS chat_mutes (
  id BIGSERIAL PRIMARY KEY,
  user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  reason TEXT,
  muted_until TIMESTAMPTZ NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_chat_mutes_user_until ON chat_mutes (user_id, muted_until DESC);
