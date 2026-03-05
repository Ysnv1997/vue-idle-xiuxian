CREATE TABLE IF NOT EXISTS auction_bids (
  id BIGSERIAL PRIMARY KEY,
  order_id BIGINT NOT NULL REFERENCES auction_orders(id) ON DELETE CASCADE,
  bidder_user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  amount BIGINT NOT NULL CHECK (amount > 0),
  status TEXT NOT NULL DEFAULT 'active',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_auction_bids_order_status_amount ON auction_bids (order_id, status, amount DESC, id DESC);
CREATE INDEX IF NOT EXISTS idx_auction_bids_bidder_time ON auction_bids (bidder_user_id, created_at DESC);
