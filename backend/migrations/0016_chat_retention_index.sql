CREATE INDEX IF NOT EXISTS idx_chat_messages_created_id ON chat_messages (created_at DESC, id DESC);
