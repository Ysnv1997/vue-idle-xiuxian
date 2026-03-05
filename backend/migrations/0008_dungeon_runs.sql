CREATE TABLE IF NOT EXISTS player_dungeon_runs (
  user_id UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
  is_active BOOLEAN NOT NULL DEFAULT FALSE,
  difficulty INT NOT NULL DEFAULT 1,
  current_floor INT NOT NULL DEFAULT 0,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

INSERT INTO player_dungeon_runs (user_id, is_active, difficulty, current_floor, updated_at)
SELECT user_id, FALSE, 1, 0, now()
FROM player_profiles
ON CONFLICT (user_id) DO NOTHING;
