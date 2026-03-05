ALTER TABLE player_resources
  ADD COLUMN IF NOT EXISTS herb_rate DOUBLE PRECISION NOT NULL DEFAULT 1,
  ADD COLUMN IF NOT EXISTS alchemy_rate DOUBLE PRECISION NOT NULL DEFAULT 1;

ALTER TABLE player_dungeon_progress
  ADD COLUMN IF NOT EXISTS streak_kills BIGINT NOT NULL DEFAULT 0;

CREATE TABLE IF NOT EXISTS player_alchemy_stats (
  user_id UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
  pills_crafted BIGINT NOT NULL DEFAULT 0,
  high_quality_pills_crafted BIGINT NOT NULL DEFAULT 0,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

INSERT INTO player_alchemy_stats (user_id, pills_crafted, high_quality_pills_crafted, updated_at)
SELECT user_id, 0, 0, now()
FROM player_profiles
ON CONFLICT (user_id) DO NOTHING;
