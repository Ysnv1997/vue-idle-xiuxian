CREATE TABLE IF NOT EXISTS chat_block_words (
  word TEXT PRIMARY KEY,
  enabled BOOLEAN NOT NULL DEFAULT TRUE,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

INSERT INTO chat_block_words (word, enabled, created_at, updated_at)
VALUES
  ('傻逼', TRUE, now(), now()),
  ('fuck', TRUE, now(), now()),
  ('shit', TRUE, now(), now()),
  ('nmsl', TRUE, now(), now())
ON CONFLICT (word) DO NOTHING;
