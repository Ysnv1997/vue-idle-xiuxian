ALTER TABLE player_inventory_state
  ADD COLUMN IF NOT EXISTS active_pet_id TEXT,
  ADD COLUMN IF NOT EXISTS active_effects JSONB NOT NULL DEFAULT '[]'::jsonb;

UPDATE player_inventory_state
SET active_effects = '[]'::jsonb
WHERE active_effects IS NULL;
