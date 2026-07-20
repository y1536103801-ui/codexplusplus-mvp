CREATE TABLE IF NOT EXISTS gateway_session_routes (
  user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  session_key TEXT NOT NULL,
  upstream_account_id TEXT NOT NULL REFERENCES upstream_accounts(id) ON DELETE CASCADE,
  expires_at TIMESTAMPTZ NOT NULL,
  updated_at TIMESTAMPTZ NOT NULL,
  PRIMARY KEY (user_id, session_key)
);

CREATE INDEX IF NOT EXISTS idx_gateway_session_routes_expiry
  ON gateway_session_routes(expires_at);

CREATE INDEX IF NOT EXISTS idx_gateway_session_routes_upstream
  ON gateway_session_routes(upstream_account_id, updated_at DESC);
