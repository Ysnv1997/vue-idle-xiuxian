CREATE TABLE IF NOT EXISTS player_activity (
    user_id UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    last_seen_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_player_activity_last_seen_at
    ON player_activity (last_seen_at DESC);
