CREATE TABLE IF NOT EXISTS player_meditation_runs (
  user_id UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
  is_active BOOLEAN NOT NULL DEFAULT FALSE,
  total_spirit_gain DOUBLE PRECISION NOT NULL DEFAULT 0,
  last_state TEXT NOT NULL DEFAULT 'stopped',
  last_log_seq BIGINT NOT NULL DEFAULT 0,
  last_log_message TEXT NOT NULL DEFAULT '',
  started_at TIMESTAMPTZ,
  ended_at TIMESTAMPTZ,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_player_meditation_runs_active
  ON player_meditation_runs (updated_at)
  WHERE is_active = TRUE;

INSERT INTO player_meditation_runs (user_id)
SELECT id
FROM users
ON CONFLICT (user_id) DO NOTHING;

UPDATE player_resources AS pr
SET spirit_rate = GREATEST(
  pr.spirit_rate / POWER(1.2, GREATEST(pp.level - 1, 0)),
  1
)
FROM player_profiles AS pp
WHERE pp.user_id = pr.user_id
  AND pr.spirit_rate > 0;
