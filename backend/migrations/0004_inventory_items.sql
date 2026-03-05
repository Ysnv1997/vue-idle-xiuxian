ALTER TABLE player_inventory_state
  ADD COLUMN IF NOT EXISTS items JSONB NOT NULL DEFAULT '[]'::jsonb;

UPDATE player_inventory_state
SET items = '[]'::jsonb
WHERE items IS NULL;
