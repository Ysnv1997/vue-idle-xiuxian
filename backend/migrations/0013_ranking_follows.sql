CREATE TABLE IF NOT EXISTS player_follows (
  follower_user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  followee_user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (follower_user_id, followee_user_id),
  CHECK (follower_user_id <> followee_user_id)
);

CREATE INDEX IF NOT EXISTS idx_player_follows_followee ON player_follows (followee_user_id);
CREATE INDEX IF NOT EXISTS idx_player_follows_follower_created_at ON player_follows (follower_user_id, created_at DESC);
