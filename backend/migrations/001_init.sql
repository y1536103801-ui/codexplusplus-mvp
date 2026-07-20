CREATE TABLE IF NOT EXISTS admins (
  id TEXT PRIMARY KEY,
  account TEXT NOT NULL UNIQUE,
  password_salt TEXT NOT NULL,
  password_hash TEXT NOT NULL,
  must_change_password BOOLEAN NOT NULL DEFAULT TRUE,
  created_at TIMESTAMPTZ NOT NULL,
  updated_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE IF NOT EXISTS users (
  id TEXT PRIMARY KEY,
  account TEXT NOT NULL UNIQUE,
  password_salt TEXT NOT NULL,
  password_hash TEXT NOT NULL,
  status TEXT NOT NULL CHECK (status IN ('active', 'disabled')),
  token_balance BIGINT NOT NULL DEFAULT 0 CHECK (token_balance >= 0),
  last_login_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL,
  updated_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE IF NOT EXISTS devices (
  id TEXT PRIMARY KEY,
  user_id TEXT NOT NULL REFERENCES users(id),
  name TEXT NOT NULL,
  fingerprint TEXT NOT NULL,
  status TEXT NOT NULL CHECK (status IN ('active', 'disabled')),
  last_seen_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL,
  updated_at TIMESTAMPTZ NOT NULL,
  UNIQUE (user_id, fingerprint)
);

CREATE TABLE IF NOT EXISTS token_topups (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  price_cents BIGINT NOT NULL CHECK (price_cents >= 0),
  tokens BIGINT NOT NULL CHECK (tokens >= 0),
  enabled BOOLEAN NOT NULL DEFAULT TRUE,
  sort_order INTEGER NOT NULL DEFAULT 0,
  description TEXT NOT NULL DEFAULT '',
  created_at TIMESTAMPTZ NOT NULL,
  updated_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_token_topups_enabled_sort
  ON token_topups(enabled, sort_order, created_at);

CREATE TABLE IF NOT EXISTS account_orders (
  id TEXT PRIMARY KEY,
  buyer_cipher TEXT NOT NULL,
  topup_id TEXT NOT NULL,
  topup_name TEXT NOT NULL,
  price_cents BIGINT NOT NULL CHECK (price_cents > 0),
  tokens BIGINT NOT NULL CHECK (tokens > 0),
  status TEXT NOT NULL CHECK (status IN ('pending', 'contacted', 'fulfilled', 'rejected')),
  user_id TEXT NOT NULL DEFAULT '',
  admin_remark TEXT NOT NULL DEFAULT '',
  created_at TIMESTAMPTZ NOT NULL,
  updated_at TIMESTAMPTZ NOT NULL,
  fulfilled_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_account_orders_status_created
  ON account_orders(status, created_at DESC);

CREATE TABLE IF NOT EXISTS recharge_requests (
  id TEXT PRIMARY KEY,
  user_id TEXT NOT NULL REFERENCES users(id),
  topup_id TEXT NOT NULL REFERENCES token_topups(id),
  topup_name TEXT NOT NULL,
  price_cents BIGINT NOT NULL CHECK (price_cents >= 0),
  tokens BIGINT NOT NULL CHECK (tokens >= 0),
  status TEXT NOT NULL CHECK (status IN ('pending', 'approved', 'rejected', 'cancelled')),
  status_transitions JSONB NOT NULL DEFAULT '[]'::jsonb,
  submitted_at TIMESTAMPTZ NOT NULL,
  confirmed_at TIMESTAMPTZ,
  updated_at TIMESTAMPTZ NOT NULL
);

ALTER TABLE recharge_requests
  ADD COLUMN IF NOT EXISTS status_transitions JSONB NOT NULL DEFAULT '[]'::jsonb;

CREATE INDEX IF NOT EXISTS idx_recharge_requests_status_submitted
  ON recharge_requests(status, submitted_at DESC);

CREATE INDEX IF NOT EXISTS idx_recharge_requests_user_submitted
  ON recharge_requests(user_id, submitted_at DESC);

CREATE TABLE IF NOT EXISTS token_ledgers (
  id TEXT PRIMARY KEY,
  user_id TEXT NOT NULL REFERENCES users(id),
  type TEXT NOT NULL CHECK (type IN ('recharge', 'adjustment', 'debit')),
  delta_tokens BIGINT NOT NULL,
  balance_after BIGINT NOT NULL CHECK (balance_after >= 0),
  source TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL
);

ALTER TABLE token_ledgers
  DROP CONSTRAINT IF EXISTS token_ledgers_type_check;

ALTER TABLE token_ledgers
  ADD CONSTRAINT token_ledgers_type_check
  CHECK (type IN ('recharge', 'adjustment', 'debit'));

CREATE INDEX IF NOT EXISTS idx_token_ledgers_user_created
  ON token_ledgers(user_id, created_at DESC);

CREATE TABLE IF NOT EXISTS upstream_accounts (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  account_group TEXT NOT NULL DEFAULT '',
  remark TEXT NOT NULL DEFAULT '',
  credential_type TEXT NOT NULL,
  source_type TEXT NOT NULL DEFAULT '',
  authorization_status TEXT NOT NULL DEFAULT 'authorized',
  access_token_cipher TEXT NOT NULL DEFAULT '',
  refresh_token_cipher TEXT NOT NULL DEFAULT '',
  auth_json_cipher TEXT NOT NULL DEFAULT '',
  password_cipher TEXT NOT NULL DEFAULT '',
  last_authorization_error TEXT NOT NULL DEFAULT '',
  token_type TEXT NOT NULL DEFAULT '',
  chatgpt_account_id TEXT NOT NULL DEFAULT '',
  expires_at TIMESTAMPTZ,
  email TEXT NOT NULL DEFAULT '',
  subscription_tier TEXT NOT NULL DEFAULT '',
  entitlement_status TEXT NOT NULL DEFAULT '',
  status TEXT NOT NULL CHECK (status IN ('active', 'disabled')),
  balance_status TEXT NOT NULL CHECK (balance_status IN ('available', 'unavailable')),
  risk_status TEXT NOT NULL CHECK (risk_status IN ('available', 'unavailable')),
  usage_tokens BIGINT NOT NULL DEFAULT 0,
  rate_limit_used_percent DOUBLE PRECISION,
  rate_limit_resets_at TIMESTAMPTZ,
  credit_balance DOUBLE PRECISION,
  credit_balance_label TEXT NOT NULL DEFAULT '',
  last_checked_at TIMESTAMPTZ,
  credential_fingerprint TEXT NOT NULL DEFAULT '',
  created_at TIMESTAMPTZ NOT NULL,
  updated_at TIMESTAMPTZ NOT NULL
);

ALTER TABLE upstream_accounts
  ADD COLUMN IF NOT EXISTS remark TEXT NOT NULL DEFAULT '';
ALTER TABLE upstream_accounts
  ADD COLUMN IF NOT EXISTS usage_tokens BIGINT NOT NULL DEFAULT 0;
ALTER TABLE upstream_accounts
  ADD COLUMN IF NOT EXISTS rate_limit_used_percent DOUBLE PRECISION;
ALTER TABLE upstream_accounts
  ADD COLUMN IF NOT EXISTS rate_limit_resets_at TIMESTAMPTZ;
ALTER TABLE upstream_accounts
  ADD COLUMN IF NOT EXISTS credit_balance DOUBLE PRECISION;
ALTER TABLE upstream_accounts
  ADD COLUMN IF NOT EXISTS credit_balance_label TEXT NOT NULL DEFAULT '';
ALTER TABLE upstream_accounts
  ADD COLUMN IF NOT EXISTS chatgpt_account_id TEXT NOT NULL DEFAULT '';
ALTER TABLE upstream_accounts
  ADD COLUMN IF NOT EXISTS auth_json_cipher TEXT NOT NULL DEFAULT '';
ALTER TABLE upstream_accounts
  ADD COLUMN IF NOT EXISTS source_type TEXT NOT NULL DEFAULT '';
ALTER TABLE upstream_accounts
  ADD COLUMN IF NOT EXISTS authorization_status TEXT NOT NULL DEFAULT 'authorized';
ALTER TABLE upstream_accounts
  ADD COLUMN IF NOT EXISTS password_cipher TEXT NOT NULL DEFAULT '';
ALTER TABLE upstream_accounts
  ADD COLUMN IF NOT EXISTS last_authorization_error TEXT NOT NULL DEFAULT '';

ALTER TABLE upstream_accounts
  DROP COLUMN IF EXISTS run_url_cipher;

CREATE INDEX IF NOT EXISTS idx_upstream_accounts_available
  ON upstream_accounts(status, authorization_status, balance_status, risk_status, account_group);

CREATE TABLE IF NOT EXISTS api_keys (
  id TEXT PRIMARY KEY,
  key_cipher TEXT NOT NULL DEFAULT '',
  key_hash TEXT NOT NULL UNIQUE,
  public_prefix TEXT NOT NULL,
  upstream_account_id TEXT NOT NULL REFERENCES upstream_accounts(id),
  user_id TEXT NOT NULL DEFAULT '',
  status TEXT NOT NULL CHECK (status IN ('active', 'disabled')),
  last_used_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL,
  updated_at TIMESTAMPTZ NOT NULL
);

ALTER TABLE api_keys
  ADD COLUMN IF NOT EXISTS key_cipher TEXT NOT NULL DEFAULT '';
ALTER TABLE api_keys
  ADD COLUMN IF NOT EXISTS user_id TEXT NOT NULL DEFAULT '';
ALTER TABLE api_keys
  ADD COLUMN IF NOT EXISTS last_used_at TIMESTAMPTZ;

CREATE INDEX IF NOT EXISTS idx_api_keys_upstream_status
  ON api_keys(upstream_account_id, status);

CREATE TABLE IF NOT EXISTS client_access_keys (
  id TEXT PRIMARY KEY,
  key_cipher TEXT NOT NULL DEFAULT '',
  key_hash TEXT NOT NULL UNIQUE,
  public_prefix TEXT NOT NULL,
  user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  status TEXT NOT NULL CHECK (status IN ('active', 'disabled')),
  last_used_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL,
  updated_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_client_access_keys_user_status
  ON client_access_keys(user_id, status);

CREATE TABLE IF NOT EXISTS usage_records (
  id TEXT PRIMARY KEY,
  user_id TEXT NOT NULL REFERENCES users(id),
  upstream_account_id TEXT NOT NULL DEFAULT '',
  api_key_id TEXT NOT NULL DEFAULT '',
  client_access_key_id TEXT NOT NULL DEFAULT '',
  session_id TEXT NOT NULL DEFAULT '',
  model TEXT NOT NULL,
  input_tokens BIGINT NOT NULL CHECK (input_tokens >= 0),
  cached_input_tokens BIGINT NOT NULL CHECK (cached_input_tokens >= 0),
  output_tokens BIGINT NOT NULL CHECK (output_tokens >= 0),
  total_tokens BIGINT NOT NULL CHECK (total_tokens >= 0),
  created_at TIMESTAMPTZ NOT NULL
);

ALTER TABLE usage_records
  ADD COLUMN IF NOT EXISTS upstream_account_id TEXT NOT NULL DEFAULT '';
ALTER TABLE usage_records
  ADD COLUMN IF NOT EXISTS api_key_id TEXT NOT NULL DEFAULT '';
ALTER TABLE usage_records
  ADD COLUMN IF NOT EXISTS client_access_key_id TEXT NOT NULL DEFAULT '';
ALTER TABLE usage_records
  ADD COLUMN IF NOT EXISTS session_id TEXT NOT NULL DEFAULT '';

CREATE INDEX IF NOT EXISTS idx_usage_records_user_created
  ON usage_records(user_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_usage_records_created
  ON usage_records(created_at DESC);

CREATE INDEX IF NOT EXISTS idx_usage_records_upstream_created
  ON usage_records(upstream_account_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_usage_records_session_created
  ON usage_records(session_id, created_at DESC);

CREATE TABLE IF NOT EXISTS audit_logs (
  id TEXT PRIMARY KEY,
  actor_id TEXT NOT NULL,
  actor_role TEXT NOT NULL CHECK (actor_role IN ('admin', 'client', 'system')),
  action TEXT NOT NULL,
  target_id TEXT NOT NULL DEFAULT '',
  detail TEXT NOT NULL DEFAULT '',
  created_at TIMESTAMPTZ NOT NULL
);

ALTER TABLE audit_logs
  DROP CONSTRAINT IF EXISTS audit_logs_actor_role_check;

ALTER TABLE audit_logs
  ADD CONSTRAINT audit_logs_actor_role_check
  CHECK (actor_role IN ('admin', 'client', 'system'));

CREATE INDEX IF NOT EXISTS idx_audit_logs_created
  ON audit_logs(created_at DESC);

CREATE TABLE IF NOT EXISTS sessions (
  token TEXT PRIMARY KEY,
  role TEXT NOT NULL CHECK (role IN ('admin', 'client')),
  subject_id TEXT NOT NULL,
  device_id TEXT NOT NULL DEFAULT '',
  expires_at TIMESTAMPTZ NOT NULL,
  created_at TIMESTAMPTZ NOT NULL
);

DELETE FROM sessions
  WHERE role = 'codex';

ALTER TABLE sessions
  DROP CONSTRAINT IF EXISTS sessions_role_check;

ALTER TABLE sessions
  ADD CONSTRAINT sessions_role_check
  CHECK (role IN ('admin', 'client'));

CREATE INDEX IF NOT EXISTS idx_sessions_subject
  ON sessions(role, subject_id, expires_at DESC);

DO $$
BEGIN
  IF EXISTS (
    SELECT 1 FROM information_schema.tables
    WHERE table_schema = 'public' AND table_name = 'idempotency_records'
  ) AND NOT EXISTS (
    SELECT 1 FROM information_schema.columns
    WHERE table_schema = 'public' AND table_name = 'idempotency_records' AND column_name = 'request_id'
  ) THEN
    DROP TABLE idempotency_records;
  END IF;
END $$;

CREATE TABLE IF NOT EXISTS idempotency_records (
  id TEXT PRIMARY KEY,
  user_id TEXT NOT NULL REFERENCES users(id),
  request_id TEXT NOT NULL,
  status TEXT NOT NULL CHECK (status IN ('reserved', 'completed', 'failed')),
  reserved_tokens BIGINT NOT NULL CHECK (reserved_tokens >= 0),
  charged_tokens BIGINT NOT NULL CHECK (charged_tokens >= 0),
  usage_record_id TEXT NOT NULL DEFAULT '',
  upstream_status INTEGER NOT NULL DEFAULT 0,
  error TEXT NOT NULL DEFAULT '',
  result_text TEXT NOT NULL DEFAULT '',
  result_body TEXT NOT NULL DEFAULT '',
  result_type TEXT NOT NULL DEFAULT '',
  result_headers TEXT NOT NULL DEFAULT '',
  created_at TIMESTAMPTZ NOT NULL,
  updated_at TIMESTAMPTZ NOT NULL,
  UNIQUE(user_id, request_id)
);

ALTER TABLE idempotency_records
  ADD COLUMN IF NOT EXISTS result_text TEXT NOT NULL DEFAULT '';
ALTER TABLE idempotency_records
  ADD COLUMN IF NOT EXISTS result_body TEXT NOT NULL DEFAULT '';
ALTER TABLE idempotency_records
  ADD COLUMN IF NOT EXISTS result_type TEXT NOT NULL DEFAULT '';
ALTER TABLE idempotency_records
  ADD COLUMN IF NOT EXISTS result_headers TEXT NOT NULL DEFAULT '';

CREATE INDEX IF NOT EXISTS idx_idempotency_records_user_status
  ON idempotency_records(user_id, status, created_at DESC);

CREATE TABLE IF NOT EXISTS app_meta (
  key TEXT PRIMARY KEY,
  value TEXT NOT NULL
);
