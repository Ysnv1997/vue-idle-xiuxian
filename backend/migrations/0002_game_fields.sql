ALTER TABLE player_resources
  ADD COLUMN IF NOT EXISTS luck DOUBLE PRECISION NOT NULL DEFAULT 1,
  ADD COLUMN IF NOT EXISTS cultivation_rate DOUBLE PRECISION NOT NULL DEFAULT 1;

INSERT INTO player_cultivation_stats (user_id, total_cultivation_time, breakthrough_count, updated_at)
SELECT user_id, 0, 0, now()
FROM player_profiles
ON CONFLICT (user_id) DO NOTHING;

INSERT INTO player_exploration_stats (user_id, exploration_count, events_triggered, items_found, updated_at)
SELECT user_id, 0, 0, 0, now()
FROM player_profiles
ON CONFLICT (user_id) DO NOTHING;

INSERT INTO player_dungeon_progress (
  user_id, highest_floor, highest_floor_2x, highest_floor_5x, highest_floor_10x, highest_floor_100x,
  last_failed_floor, total_runs, boss_kills, elite_kills, total_kills, death_count, total_rewards, updated_at
)
SELECT user_id, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, now()
FROM player_profiles
ON CONFLICT (user_id) DO NOTHING;
