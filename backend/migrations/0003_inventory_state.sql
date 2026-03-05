CREATE TABLE IF NOT EXISTS player_inventory_state (
  user_id UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
  herbs JSONB NOT NULL DEFAULT '[]'::jsonb,
  pill_fragments JSONB NOT NULL DEFAULT '{}'::jsonb,
  pill_recipes JSONB NOT NULL DEFAULT '[]'::jsonb,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

INSERT INTO player_inventory_state (user_id, herbs, pill_fragments, pill_recipes, updated_at)
SELECT user_id, '[]'::jsonb, '{}'::jsonb, '[]'::jsonb, now()
FROM player_profiles
ON CONFLICT (user_id) DO NOTHING;
