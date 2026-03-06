CREATE TABLE IF NOT EXISTS game_runtime_config_audit_logs (
    id BIGSERIAL PRIMARY KEY,
    key TEXT NOT NULL,
    action TEXT NOT NULL,
    old_value TEXT,
    old_value_type TEXT,
    old_category TEXT,
    old_description TEXT,
    new_value TEXT NOT NULL,
    new_value_type TEXT NOT NULL,
    new_category TEXT NOT NULL,
    new_description TEXT NOT NULL DEFAULT '',
    operator_user_id UUID REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_game_runtime_config_audit_logs_created_at
    ON game_runtime_config_audit_logs (created_at DESC);

CREATE INDEX IF NOT EXISTS idx_game_runtime_config_audit_logs_key_created_at
    ON game_runtime_config_audit_logs (key, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_game_runtime_config_audit_logs_operator_created_at
    ON game_runtime_config_audit_logs (operator_user_id, created_at DESC);
