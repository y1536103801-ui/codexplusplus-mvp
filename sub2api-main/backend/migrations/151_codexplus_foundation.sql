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
      "draft_status": "published",
      "publish_scope": "production",
      "updated_by": "system",
      "updated_at": "2026-06-16T00:00:00Z",
      "change_reason": "initial hidden Codex++ MVP config",
      "rollback_from": null,
      "plan_catalog": {
        "config_version": "codexplus-mvp-1",
        "draft_status": "published",
        "publish_scope": "production",
        "updated_by": "system",
        "updated_at": "2026-06-16T00:00:00Z",
        "change_reason": "initial hidden Codex++ MVP plan catalog",
        "plans": [{
          "plan_id": "starter",
          "name": "Starter",
          "description": "Hidden bootstrap plan until admin publishes saleable plans.",
          "billing_period": "none",
          "currency": "USD",
          "price_amount_minor": 0,
          "display_price": "TBD",
          "entitlement_grant": {"balance_credit": 0, "duration_days": 0, "daily_quota": null, "period_quota": null},
          "entitlement_sources": {"subscription_group_ids": [], "api_key_group_ids": [], "group_names": []},
          "model_groups": ["default"],
          "usage_policy_id": "default",
          "purchase_url": null,
          "renew_url": null,
          "copy_keys": {
            "purchase_action": "billing.action.purchase",
            "renew_action": "billing.action.renew",
            "upgrade_action": "billing.action.upgrade",
            "not_purchased_message": "billing.message.not_purchased",
            "expired_message": "billing.message.expired",
            "low_balance_message": "billing.message.low_balance"
          },
          "is_listed": false,
          "status": "hidden",
          "sort_order": 10,
          "external_billing_refs": {"product_id": null, "sku_id": null}
        }]
      },
      "model_catalog": {
        "config_version": "codexplus-mvp-1",
        "draft_status": "published",
        "publish_scope": "production",
        "updated_by": "system",
        "updated_at": "2026-06-16T00:00:00Z",
        "change_reason": "initial Codex++ MVP model catalog",
        "models": [{
          "model_id": "codex-standard",
          "display_name": "Codex Standard",
          "route_model": "gpt-5-mini",
          "model_group": "default",
          "context_window": 128000,
          "billing_multiplier": 1,
          "is_default": true,
          "is_enabled": true,
          "is_hidden": false,
          "disabled_reason": null,
          "rollout_channel": "stable",
          "quality_tier": "standard",
          "fallback_model_id": null,
          "deprecation_at": null,
          "disabled_replacement_model_id": null,
          "disabled_message_key": null,
          "sort_order": 10,
          "operator_tags": ["default", "public"]
        }]
      },
      "usage_policy": {
        "config_version": "codexplus-mvp-1",
        "draft_status": "published",
        "publish_scope": "production",
        "updated_by": "system",
        "updated_at": "2026-06-16T00:00:00Z",
        "change_reason": "initial Codex++ MVP usage policy",
        "policies": [{
          "policy_id": "default",
          "applies_to": {"plan_ids": ["starter"], "model_groups": ["default"], "user_segments": ["all"]},
          "low_balance_threshold": 0,
          "daily_quota": 0,
          "monthly_quota": 3000000,
          "concurrency_limit": 1,
          "rpm_limit": 1,
          "tpm_limit": 1000,
          "burst_limit": 1,
          "rate_limit_window_seconds": 60,
          "expired_behavior": "block",
          "grace_period_hours": 0,
          "overage_behavior": "block",
          "copy_keys": {
            "low_balance_message": "usage.low_balance",
            "insufficient_balance_message": "usage.insufficient_balance",
            "rate_limited_message": "usage.rate_limited",
            "expired_message": "usage.expired",
            "renew_action": "usage.renew_action",
            "purchase_action": "usage.purchase_action",
            "device_revoked_message": "device.revoked"
          },
          "device_policy": {
            "registration_required": true,
            "max_devices_per_user": 1,
            "allow_self_service_replacement": false,
            "replacement_cooldown_hours": 0,
            "strict_enforcement_default": false,
            "revoke_reason_taxonomy": ["user_requested", "admin_revoked", "device_limit_exceeded", "risk_control", "inactive", "compromised", "support_unlock", "unknown"],
            "support_unlock_policy": "support_only",
            "revoked_behavior": "block_bootstrap",
            "message_keys": {
              "limit_reached": "device.limit_reached",
              "replacement_cooldown": "device.replacement_cooldown",
              "revoked": "device.revoked",
              "support_unlock_required": "device.support_unlock_required"
            }
          },
          "insufficient_balance_message": "usage.insufficient_balance",
          "rate_limited_message": "usage.rate_limited"
        }]
      },
      "feature_flags": {
        "config_version": "codexplus-mvp-1",
        "draft_status": "published",
        "publish_scope": "production",
        "updated_by": "system",
        "updated_at": "2026-06-16T00:00:00Z",
        "change_reason": "initial Codex++ MVP feature flags",
        "flags": {
          "advanced_provider_config": false,
          "install_assistant": true,
          "new_user_tutorial": true,
          "model_selector": true,
          "diagnostic_export": true,
          "announcements": true,
          "force_update_prompt": false,
          "strict_device_enforcement": false
        },
        "exposure": {
          "client_visible": ["advanced_provider_config", "install_assistant", "new_user_tutorial", "model_selector", "diagnostic_export", "announcements", "force_update_prompt"],
          "server_only": ["strict_device_enforcement"]
        },
        "copy_keys": {
          "force_update_prompt": "codexplus.update.force_prompt",
          "install_assistant_entry": "codexplus.install_assistant.entry",
          "new_user_tutorial_entry": "codexplus.tutorial.new_user_entry",
          "diagnostic_export_entry": "codexplus.diagnostics.redacted_export_entry",
          "announcement_entry": "codexplus.announcements.entry"
        }
      }
    }',
    NOW()
)
ON CONFLICT (key) DO NOTHING;
