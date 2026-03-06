ALTER TABLE game_admin_users
    ADD COLUMN IF NOT EXISTS role TEXT NOT NULL DEFAULT 'super_admin';

UPDATE game_admin_users
SET role = 'super_admin'
WHERE COALESCE(TRIM(role), '') = '';

ALTER TABLE game_admin_users
    DROP CONSTRAINT IF EXISTS chk_game_admin_users_role;

ALTER TABLE game_admin_users
    ADD CONSTRAINT chk_game_admin_users_role
    CHECK (role IN ('super_admin', 'ops_admin', 'chat_admin'));

CREATE INDEX IF NOT EXISTS idx_game_admin_users_role_created_at
    ON game_admin_users (role, created_at DESC);
