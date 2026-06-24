-- Codex++ Phase 1 backend foundation.
-- Adds device state, managed provider key mapping, append-only events and the
-- initial hidden config document. This migration is additive and forward-only.

SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '10min';

CREATE TABLE IF NOT EXISTS codexplus_devices (
    id               BIGSERIAL PRIMARY KEY,
    user_id          BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    device_id        VARCHAR(128) NOT NULL,
    device_name      VARCHAR(160),
    platform         VARCHAR(40),
    app_version      VARCHAR(64),
    fingerprint_hash VARCHAR(128),
    status           VARCHAR(32) NOT NULL DEFAULT 'active',
    last_seen_at     TIMESTAMPTZ,
    revoked_at       TIMESTAMPTZ,
    metadata         JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at       TIMESTAMPTZ
);

CREATE UNIQUE INDEX IF NOT EXISTS codexplusdevice_user_device_uq
    ON codexplus_devices (user_id, device_id)
    WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS codexplusdevice_user_id ON codexplus_devices (user_id);
CREATE INDEX IF NOT EXISTS codexplusdevice_status ON codexplus_devices (status);
CREATE INDEX IF NOT EXISTS codexplusdevice_last_seen_at ON codexplus_devices (last_seen_at);

CREATE TABLE IF NOT EXISTS codexplus_managed_provider_keys (
    id                  BIGSERIAL PRIMARY KEY,
    user_id             BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    api_key_id          BIGINT NOT NULL REFERENCES api_keys(id) ON DELETE CASCADE,
    managed_provider_id VARCHAR(80) NOT NULL DEFAULT 'codex-plus-cloud',
    display_name        VARCHAR(100) NOT NULL DEFAULT 'Codex++ Cloud',
    key_prefix          VARCHAR(32),
    status              VARCHAR(32) NOT NULL DEFAULT 'active',
    last_used_at        TIMESTAMPTZ,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at          TIMESTAMPTZ
);

CREATE UNIQUE INDEX IF NOT EXISTS codexplusmanagedkey_user_provider_uq
    ON codexplus_managed_provider_keys (user_id, managed_provider_id)
    WHERE deleted_at IS NULL;
CREATE UNIQUE INDEX IF NOT EXISTS codexplusmanagedkey_api_key_id_uq
    ON codexplus_managed_provider_keys (api_key_id)
    WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS codexplusmanagedkey_status
    ON codexplus_managed_provider_keys (status);

CREATE TABLE IF NOT EXISTS codexplus_events (
    id             BIGSERIAL PRIMARY KEY,
    user_id        BIGINT REFERENCES users(id) ON DELETE SET NULL,
    device_id      VARCHAR(128),
    event_type     VARCHAR(80) NOT NULL,
    severity       VARCHAR(24) NOT NULL DEFAULT 'info',
    request_id     VARCHAR(128),
    config_version VARCHAR(64),
    payload        JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS codexplusevent_user_id ON codexplus_events (user_id);
CREATE INDEX IF NOT EXISTS codexplusevent_device_id ON codexplus_events (device_id);
CREATE INDEX IF NOT EXISTS codexplusevent_event_type ON codexplus_events (event_type);
CREATE INDEX IF NOT EXISTS codexplusevent_severity ON codexplus_events (severity);
CREATE INDEX IF NOT EXISTS codexplusevent_config_version ON codexplus_events (config_version);
CREATE INDEX IF NOT EXISTS codexplusevent_created_at ON codexplus_events (created_at);

INSERT INTO settings (key, value, updated_at)
VALUES (
    'codexplus_config_v1',
    '{
      "config_version": "codexplus-mvp-1",
      "publish_scope": "draft",
      "updated_by": "system",
      "updated_at": "2026-06-16T00:00:00Z",
      "change_reason": "initial hidden Codex++ MVP config",
      "rollback_from": null,
      "plan_catalog": {
        "config_version": "codexplus-mvp-1",
        "plans": [{
          "plan_id": "starter",
          "name": "Starter",
          "description": "Hidden bootstrap plan until admin publishes saleable plans.",
          "billing_period": "none",
          "currency": "USD",
          "display_price": "TBD",
          "entitlement_grant": {"balance_credit": 0, "duration_days": 0, "daily_quota": null},
          "entitlement_sources": {"subscription_group_ids": [], "api_key_group_ids": [], "group_names": []},
          "model_groups": ["default"],
          "renew_url": null,
          "is_listed": false,
          "status": "hidden"
        }]
      },
      "model_catalog": {
        "config_version": "codexplus-mvp-1",
        "models": [{
          "model_id": "codex-default",
          "display_name": "Default",
          "route_model": "codex-default",
          "model_group": "default",
          "context_window": 8192,
          "billing_multiplier": 1,
          "is_default": true,
          "is_enabled": true,
          "is_hidden": true,
          "disabled_reason": null
        }]
      },
      "usage_policy": {
        "config_version": "codexplus-mvp-1",
        "policies": [{
          "policy_id": "default",
          "low_balance_threshold": 0,
          "daily_quota": 0,
          "concurrency_limit": 1,
          "rpm_limit": 1,
          "tpm_limit": 1000,
          "expired_behavior": "block",
          "grace_period_hours": 0,
          "insufficient_balance_message": "Codex++ entitlement is not active.",
          "rate_limited_message": "Codex++ usage is temporarily limited."
        }]
      },
      "feature_flags": {
        "config_version": "codexplus-mvp-1",
        "flags": {
          "advanced_provider_config": false,
          "install_assistant": true,
          "new_user_tutorial": true,
          "model_selector": true,
          "diagnostic_export": true,
          "announcements": true,
          "force_update_prompt": false,
          "strict_device_enforcement": false
        }
      }
    }',
    NOW()
)
ON CONFLICT (key) DO NOTHING;
