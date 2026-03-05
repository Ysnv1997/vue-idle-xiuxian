ALTER TABLE chat_mutes
  ADD COLUMN IF NOT EXISTS created_by_user_id UUID REFERENCES users(id) ON DELETE SET NULL;

CREATE INDEX IF NOT EXISTS idx_chat_mutes_created_by ON chat_mutes (created_by_user_id, created_at DESC);
