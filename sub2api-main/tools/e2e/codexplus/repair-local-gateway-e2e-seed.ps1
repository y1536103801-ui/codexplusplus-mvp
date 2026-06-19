param(
    [string]$PostgresContainer = "sub2api-codexplus-postgres",
    [string]$PostgresUser = "sub2api",
    [string]$Database = "sub2api",
    [string]$RedisContainer = "sub2api-codexplus-redis",
    [string]$GatewayBaseUrl = "http://127.0.0.1:8081",
    [string]$MockBaseUrl = "http://host.docker.internal:18081",
    [string]$OutputEnvFile,
    [switch]$FlushRedis
)

$ErrorActionPreference = "Stop"

if ([string]::IsNullOrWhiteSpace($OutputEnvFile)) {
    $OutputEnvFile = Join-Path $PSScriptRoot "e2e-env.local.generated.ps1"
}

function Escape-SqlLiteral {
    param([string]$Value)
    return ($Value -replace "'", "''")
}

function Escape-PSLiteral {
    param([string]$Value)
    return ($Value -replace "'", "''")
}

$mockBaseSql = Escape-SqlLiteral $MockBaseUrl

$sql = @"
BEGIN;

INSERT INTO users (
    id, email, password_hash, role, balance, concurrency, status,
    username, notes, created_at, updated_at
)
VALUES
    (8,  'codexplus-e2e-active@local.test',          'local-e2e-password-hash', 'user', 200, 5, 'active', 'codexplus-e2e-active',          '07 gateway e2e active persona', NOW(), NOW()),
    (9,  'codexplus-e2e-not-purchased@local.test',   'local-e2e-password-hash', 'user', 200, 5, 'active', 'codexplus-e2e-not-purchased',   '07 gateway e2e not purchased persona', NOW(), NOW()),
    (10, 'codexplus-e2e-expired@local.test',         'local-e2e-password-hash', 'user', 200, 5, 'active', 'codexplus-e2e-expired',         '07 gateway e2e expired persona', NOW(), NOW()),
    (11, 'codexplus-e2e-low-balance@local.test',     'local-e2e-password-hash', 'user', 20,  5, 'active', 'codexplus-e2e-low-balance',     '07 gateway e2e low balance and quota exhausted persona', NOW(), NOW()),
    (12, 'codexplus-e2e-device-revoked@local.test',  'local-e2e-password-hash', 'user', 200, 5, 'active', 'codexplus-e2e-device-revoked',  '07 gateway e2e revoked device persona', NOW(), NOW()),
    (13, 'codexplus-e2e-model-denied@local.test',    'local-e2e-password-hash', 'user', 200, 5, 'active', 'codexplus-e2e-model-denied',    '07 gateway e2e model denied persona', NOW(), NOW())
ON CONFLICT (id)
DO UPDATE SET
    email = EXCLUDED.email,
    password_hash = EXCLUDED.password_hash,
    role = EXCLUDED.role,
    balance = EXCLUDED.balance,
    concurrency = EXCLUDED.concurrency,
    status = EXCLUDED.status,
    username = EXCLUDED.username,
    notes = EXCLUDED.notes,
    deleted_at = NULL,
    updated_at = NOW();

SELECT setval('users_id_seq', GREATEST((SELECT COALESCE(MAX(id), 1) FROM users), 13), true);

INSERT INTO groups (
    id, name, description, platform, status, subscription_type,
    daily_limit_usd, weekly_limit_usd, monthly_limit_usd, rpm_limit,
    supported_model_scopes, models_list_config, created_at, updated_at
)
VALUES (
    1, 'default', 'Codex++ local release E2E group', 'openai', 'active', 'subscription',
    1, NULL, NULL, 0,
    '["openai"]'::jsonb, '{}'::jsonb, NOW(), NOW()
)
ON CONFLICT (id)
DO UPDATE SET
    name = EXCLUDED.name,
    description = EXCLUDED.description,
    platform = EXCLUDED.platform,
    status = EXCLUDED.status,
    subscription_type = EXCLUDED.subscription_type,
    daily_limit_usd = EXCLUDED.daily_limit_usd,
    weekly_limit_usd = EXCLUDED.weekly_limit_usd,
    monthly_limit_usd = EXCLUDED.monthly_limit_usd,
    rpm_limit = EXCLUDED.rpm_limit,
    supported_model_scopes = EXCLUDED.supported_model_scopes,
    models_list_config = EXCLUDED.models_list_config,
    deleted_at = NULL,
    updated_at = NOW();

WITH seeded_keys(user_id, key, name) AS (
    VALUES
        (8,  'cp07-active-local-key',         '07 active local gateway key'),
        (9,  'cp07-not-purchased-local-key',  '07 not purchased local gateway key'),
        (10, 'cp07-expired-local-key',        '07 expired local gateway key'),
        (11, 'cp07-low-balance-local-key',    '07 low balance local gateway key'),
        (12, 'cp07-device-revoked-local-key', '07 device revoked local gateway key'),
        (13, 'cp07-model-denied-local-key',   '07 model denied local gateway key')
)
INSERT INTO api_keys (
    user_id, key, name, group_id, status, quota, quota_used,
    rate_limit_5h, rate_limit_1d, rate_limit_7d,
    usage_5h, usage_1d, usage_7d,
    created_at, updated_at
)
SELECT
    user_id, key, name, 1, 'active', 0, 0,
    0, 0, 0,
    0, 0, 0,
    NOW(), NOW()
FROM seeded_keys
ON CONFLICT (key)
DO UPDATE SET
    user_id = EXCLUDED.user_id,
    name = EXCLUDED.name,
    group_id = EXCLUDED.group_id,
    status = EXCLUDED.status,
    quota = EXCLUDED.quota,
    quota_used = EXCLUDED.quota_used,
    rate_limit_5h = EXCLUDED.rate_limit_5h,
    rate_limit_1d = EXCLUDED.rate_limit_1d,
    rate_limit_7d = EXCLUDED.rate_limit_7d,
    usage_5h = EXCLUDED.usage_5h,
    usage_1d = EXCLUDED.usage_1d,
    usage_7d = EXCLUDED.usage_7d,
    expires_at = NULL,
    deleted_at = NULL,
    updated_at = NOW();

UPDATE codexplus_managed_provider_keys
SET deleted_at = NOW(), updated_at = NOW()
WHERE user_id BETWEEN 8 AND 13
  AND deleted_at IS NULL;

INSERT INTO codexplus_managed_provider_keys (
    user_id, api_key_id, managed_provider_id, display_name, key_prefix, status,
    created_at, updated_at
)
SELECT
    ak.user_id,
    ak.id,
    'codex-plus-cloud',
    'Codex++ Local E2E',
    LEFT(ak.key, 16),
    'active',
    NOW(),
    NOW()
FROM api_keys ak
WHERE ak.user_id BETWEEN 8 AND 13
  AND ak.deleted_at IS NULL;

INSERT INTO codexplus_devices (
    user_id, device_id, device_name, platform, app_version, fingerprint_hash,
    status, last_seen_at, revoked_at, metadata, created_at, updated_at
)
SELECT
    u.id,
    'codexplus-07-e2e-device',
    'Codex++ 07 local E2E device',
    'windows',
    'local-e2e',
    'local-e2e-fingerprint',
    CASE WHEN u.id = 12 THEN 'revoked' ELSE 'active' END,
    NOW(),
    CASE WHEN u.id = 12 THEN NOW() ELSE NULL END,
    '{"source":"07-local-e2e-seed"}'::jsonb,
    NOW(),
    NOW()
FROM users u
WHERE u.id BETWEEN 8 AND 13
ON CONFLICT (user_id, device_id) WHERE deleted_at IS NULL
DO UPDATE SET
    device_name = EXCLUDED.device_name,
    platform = EXCLUDED.platform,
    app_version = EXCLUDED.app_version,
    fingerprint_hash = EXCLUDED.fingerprint_hash,
    status = EXCLUDED.status,
    last_seen_at = EXCLUDED.last_seen_at,
    revoked_at = EXCLUDED.revoked_at,
    metadata = EXCLUDED.metadata,
    updated_at = NOW();

UPDATE user_subscriptions
SET deleted_at = NOW(), updated_at = NOW()
WHERE user_id = 9
  AND group_id = 1
  AND deleted_at IS NULL;

INSERT INTO user_subscriptions (
    user_id, group_id, starts_at, expires_at, status,
    daily_window_start, weekly_window_start, monthly_window_start,
    daily_usage_usd, weekly_usage_usd, monthly_usage_usd,
    notes, created_at, updated_at
)
VALUES
    (8,  1, NOW() - INTERVAL '1 hour',  NOW() + INTERVAL '30 days', 'active',  NOW(), NOW(), NOW(), 0, 0, 0, '07 gateway e2e active persona', NOW(), NOW()),
    (10, 1, NOW() - INTERVAL '60 days', NOW() - INTERVAL '1 day',   'expired', NOW() - INTERVAL '60 days', NOW() - INTERVAL '60 days', NOW() - INTERVAL '60 days', 0, 0, 0, '07 gateway e2e expired persona', NOW(), NOW()),
    (11, 1, NOW() - INTERVAL '1 hour',  NOW() + INTERVAL '30 days', 'active',  NOW(), NOW(), NOW(), 1, 0, 0, '07 gateway e2e quota exhausted persona', NOW(), NOW()),
    (12, 1, NOW() - INTERVAL '1 hour',  NOW() + INTERVAL '30 days', 'active',  NOW(), NOW(), NOW(), 0, 0, 0, '07 gateway e2e revoked device persona', NOW(), NOW()),
    (13, 1, NOW() - INTERVAL '1 hour',  NOW() + INTERVAL '30 days', 'active',  NOW(), NOW(), NOW(), 0, 0, 0, '07 gateway e2e model denied persona', NOW(), NOW())
ON CONFLICT (user_id, group_id) WHERE deleted_at IS NULL
DO UPDATE SET
    starts_at = EXCLUDED.starts_at,
    expires_at = EXCLUDED.expires_at,
    status = EXCLUDED.status,
    daily_window_start = EXCLUDED.daily_window_start,
    weekly_window_start = EXCLUDED.weekly_window_start,
    monthly_window_start = EXCLUDED.monthly_window_start,
    daily_usage_usd = EXCLUDED.daily_usage_usd,
    weekly_usage_usd = EXCLUDED.weekly_usage_usd,
    monthly_usage_usd = EXCLUDED.monthly_usage_usd,
    notes = EXCLUDED.notes,
    updated_at = NOW();

INSERT INTO accounts (
    name, platform, type, credentials, extra, concurrency, priority, status,
    schedulable, notes, created_at, updated_at
)
SELECT
    'codexplus-local-openai-mock',
    'openai',
    'apikey',
    jsonb_build_object('api_key', 'sk-local-codexplus-mock', 'base_url', '$mockBaseSql'),
    '{"openai_passthrough":true,"openai_responses_mode":"force_responses","openai_responses_supported":true}'::jsonb,
    5,
    1,
    'active',
    TRUE,
    '07 local E2E mock upstream account',
    NOW(),
    NOW()
WHERE NOT EXISTS (
    SELECT 1 FROM accounts
    WHERE name = 'codexplus-local-openai-mock'
      AND deleted_at IS NULL
);

UPDATE accounts
SET
    platform = 'openai',
    type = 'apikey',
    credentials = jsonb_build_object('api_key', 'sk-local-codexplus-mock', 'base_url', '$mockBaseSql'),
    extra = '{"openai_passthrough":true,"openai_responses_mode":"force_responses","openai_responses_supported":true}'::jsonb,
    concurrency = 5,
    priority = 1,
    status = 'active',
    schedulable = TRUE,
    error_message = NULL,
    deleted_at = NULL,
    notes = '07 local E2E mock upstream account',
    updated_at = NOW()
WHERE name = 'codexplus-local-openai-mock'
  AND deleted_at IS NULL;

DELETE FROM account_groups
WHERE group_id = 1
  AND account_id IN (
      SELECT id FROM accounts
      WHERE name = 'codexplus-local-openai-mock'
  );

INSERT INTO account_groups (account_id, group_id, priority, created_at)
SELECT id, 1, 1, NOW()
FROM accounts
WHERE name = 'codexplus-local-openai-mock'
  AND deleted_at IS NULL;

UPDATE settings
SET
    value = jsonb_set(
        jsonb_set(
            jsonb_set(
                jsonb_set(
                    jsonb_set(
                        value::jsonb,
                        '{plan_catalog,plans,0,entitlement_sources}',
                        '{"subscription_group_ids":[1],"api_key_group_ids":[],"group_names":[]}'::jsonb,
                        true
                    ),
                    '{plan_catalog,plans,0,model_groups}',
                    '["codex"]'::jsonb,
                    true
                ),
                '{model_catalog,models,0,model_group}',
                '"codex"'::jsonb,
                true
            ),
            '{usage_policy,policies,0,applies_to,model_groups}',
            '["codex","legacy"]'::jsonb,
            true
        ),
        '{usage_policy,policies,0,low_balance_threshold}',
        '100'::jsonb,
        true
    )::text,
    updated_at = NOW()
WHERE key = 'codexplus_config_v1';

COMMIT;

SELECT
    u.id AS user_id,
    u.email,
    u.balance,
    ak.name AS gateway_key_name,
    COALESCE(us.status, 'none') AS subscription_status,
    us.expires_at,
    us.daily_usage_usd,
    d.status AS device_status,
    mkp.status AS managed_provider_status
FROM users u
LEFT JOIN api_keys ak
    ON ak.user_id = u.id AND ak.deleted_at IS NULL
LEFT JOIN codexplus_managed_provider_keys mkp
    ON mkp.api_key_id = ak.id AND mkp.deleted_at IS NULL
LEFT JOIN user_subscriptions us
    ON us.user_id = u.id AND us.group_id = 1 AND us.deleted_at IS NULL
LEFT JOIN codexplus_devices d
    ON d.user_id = u.id AND d.device_id = 'codexplus-07-e2e-device'
WHERE u.id BETWEEN 8 AND 13
ORDER BY u.id;

SELECT
    g.id AS group_id,
    g.platform,
    g.subscription_type,
    a.name AS account_name,
    a.platform AS account_platform,
    a.status AS account_status,
    a.schedulable AS account_schedulable
FROM groups g
LEFT JOIN account_groups ag ON ag.group_id = g.id
LEFT JOIN accounts a ON a.id = ag.account_id AND a.deleted_at IS NULL
WHERE g.id = 1;
"@

$sql | docker exec -i $PostgresContainer psql -U $PostgresUser -d $Database -v ON_ERROR_STOP=1
if ($LASTEXITCODE -ne 0) {
    throw "Local Codex++ gateway E2E seed failed. See psql output above."
}

if ($FlushRedis) {
    docker exec $RedisContainer sh -lc 'env -u REDISCLI_AUTH redis-cli FLUSHDB >/dev/null 2>&1 || redis-cli FLUSHDB >/dev/null 2>&1'
    if ($LASTEXITCODE -ne 0) {
        throw "Redis cache flush failed for local E2E seed refresh."
    }
    Write-Host "Redis cache flushed for local E2E seed refresh."
}

$envLines = @(
    "# Generated by repair-local-gateway-e2e-seed.ps1 for isolated local release E2E.",
    "# Contains only local mock credentials; do not use against production.",
    "`$env:CODEXPLUS_07_E2E_GATEWAY_BASE_URL = '$(Escape-PSLiteral $GatewayBaseUrl)'",
    "`$env:CODEXPLUS_07_E2E_ADMIN_BASE_URL = '$(Escape-PSLiteral $GatewayBaseUrl)'",
    "`$env:CODEXPLUS_07_E2E_ALLOWED_TEST_MODEL = 'codex-standard'",
    "`$env:CODEXPLUS_07_E2E_DENIED_TEST_MODEL = 'codex-denied-local'",
    "`$env:CODEXPLUS_07_E2E_TEST_DEVICE_ID = 'codexplus-07-e2e-device'",
    "`$env:CODEXPLUS_07_E2E_USER_ACTIVE_ID = '8'",
    "`$env:CODEXPLUS_07_E2E_USER_NOT_PURCHASED_ID = '9'",
    "`$env:CODEXPLUS_07_E2E_USER_EXPIRED_ID = '10'",
    "`$env:CODEXPLUS_07_E2E_USER_LOW_BALANCE_ID = '11'",
    "`$env:CODEXPLUS_07_E2E_USER_DEVICE_REVOKED_ID = '12'",
    "`$env:CODEXPLUS_07_E2E_USER_MODEL_DENIED_ID = '13'",
    "`$env:CODEXPLUS_07_E2E_USER_ACTIVE_GATEWAY_KEY = 'cp07-active-local-key'",
    "`$env:CODEXPLUS_07_E2E_USER_NOT_PURCHASED_GATEWAY_KEY = 'cp07-not-purchased-local-key'",
    "`$env:CODEXPLUS_07_E2E_USER_EXPIRED_GATEWAY_KEY = 'cp07-expired-local-key'",
    "`$env:CODEXPLUS_07_E2E_USER_LOW_BALANCE_GATEWAY_KEY = 'cp07-low-balance-local-key'",
    "`$env:CODEXPLUS_07_E2E_USER_DEVICE_REVOKED_GATEWAY_KEY = 'cp07-device-revoked-local-key'",
    "`$env:CODEXPLUS_07_E2E_USER_MODEL_DENIED_GATEWAY_KEY = 'cp07-model-denied-local-key'"
)
Set-Content -LiteralPath $OutputEnvFile -Encoding UTF8 -Value ($envLines -join [Environment]::NewLine)

Write-Host "Local Codex++ gateway E2E seed repaired."
Write-Host "Generated local E2E env file: $OutputEnvFile"
