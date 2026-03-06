CREATE TABLE IF NOT EXISTS game_admin_users (
    id BIGSERIAL PRIMARY KEY,
    linux_do_user_id TEXT NOT NULL UNIQUE,
    note TEXT NOT NULL DEFAULT '',
    source TEXT NOT NULL DEFAULT 'manual',
    created_by_user_id UUID REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_game_admin_users_created_at
    ON game_admin_users (created_at DESC);
