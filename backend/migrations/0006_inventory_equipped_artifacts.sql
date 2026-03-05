ALTER TABLE player_inventory_state
  ADD COLUMN IF NOT EXISTS equipped_artifacts JSONB NOT NULL DEFAULT '{
    "weapon": null,
    "head": null,
    "body": null,
    "legs": null,
    "feet": null,
    "shoulder": null,
    "hands": null,
    "wrist": null,
    "necklace": null,
    "ring1": null,
    "ring2": null,
    "belt": null,
    "artifact": null
  }'::jsonb;

UPDATE player_inventory_state
SET equipped_artifacts = '{
  "weapon": null,
  "head": null,
  "body": null,
  "legs": null,
  "feet": null,
  "shoulder": null,
  "hands": null,
  "wrist": null,
  "necklace": null,
  "ring1": null,
  "ring2": null,
  "belt": null,
  "artifact": null
}'::jsonb
WHERE equipped_artifacts IS NULL;
