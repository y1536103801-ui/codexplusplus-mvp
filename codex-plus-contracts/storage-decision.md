# Codex++ MVP Storage and Migration Decision

This document records the MVP storage decision required by `CONTRACT-GATE.md`.
It is based on the Phase 1 startup audit of `sub2api-main` and
`CodexPlusPlus-main`.

## Decision Summary

Use a hybrid storage approach:

- Reuse existing Sub2API subscription, group, API key, redeem, payment and usage
  foundations where they already model the required concept.
- Reuse `api_keys` for the actual user-side gateway key, but add a typed
  Codex++ managed-provider mapping table so `user_id + managed_provider_id`
  is idempotent and cannot collide with a manually named user key.
- Add a small typed Codex++ device table for device upsert/revoke/block
  behavior.
- Store Codex++ catalog/config metadata in the existing `settings` key/value
  table for MVP under a single versioned JSON key, but include
  `config_version` and change metadata from day one.
- Prefer structured audit/event storage if Sub2API already has a general audit
  table; otherwise add `codexplus_events` as a minimal append-only event table.

## Source Evidence

- `sub2api-main/backend/ent/schema/api_key.go` already stores user-side gateway
  keys, group binding, status, quota, expiry and rate-limit windows, but has no
  metadata field and does not make `name` unique per user.
- `sub2api-main/backend/internal/service/api_key_service.go` already validates
  group binding against active subscriptions and generates user-side keys with
  the configured `sk-` prefix.
- `sub2api-main/backend/ent/schema/setting.go` is a simple unique key/value
  table, which is suitable for a single MVP config JSON document but not enough
  for fine-grained history by itself.
- `sub2api-main/backend/ent/schema/user_subscription.go` and
  `subscription_plan.go` already cover user entitlement periods, group mapping,
  pricing and sale visibility.
- `sub2api-main/backend/ent/schema/payment_audit_log.go` is payment-specific,
  so Codex++ runtime/gateway events should not be forced into it.

## Domain Mapping

| Domain | MVP storage decision | Reason |
| --- | --- | --- |
| Plan entitlement | Reuse `subscription_plans`, `user_subscriptions`, `groups` | Existing Sub2API plan/group model already maps plan, group and expiry windows. |
| User-side gateway key | Reuse `api_keys` for the secret and gateway auth; add `codexplus_managed_provider_keys` mapping | `api_keys.name` is not unique and has no metadata field, so the Codex++ identity must not rely on display name alone. |
| Redeem | Reuse existing redeem code service | Keeps activation code logic centralized. |
| Payment | Reuse existing payment orders/providers later | MVP may start with manual entitlement/test orders. |
| Model catalog | Map from existing group/model/channel config plus Codex++ metadata | Avoids client-side model truth. |
| Usage summary | Reuse `usage_logs`, API key usage windows and subscription usage fields | Existing usage surfaces are enough for MVP summary. |
| Device state | Add `codexplus_devices` typed Ent schema/table | Device revoke/block needs user isolation, explicit status and a unique `(user_id, device_id)` constraint. |
| Config version | Store current config in `settings.key = "codexplus_config_v1"` | Fits the existing settings table and keeps MVP additive. |
| Events | Add `codexplus_events` unless a general-purpose audit table is confirmed during implementation | Payment audit logs are payment-specific; runtime/gateway events need their own append-only stream. |

## Required Managed Provider Key Mapping

Add a typed table or repository equivalent named `codexplus_managed_provider_keys`.

Minimum fields:

- `id`
- `user_id`
- `managed_provider_id`: MVP value `codex-plus-cloud`
- `api_key_id`
- `status`: `active`, `revoked`
- `created_at`
- `updated_at`
- `rotated_at`
- `revoked_at`
- `revoked_by`
- `revocation_reason`

Unique constraint:

- `(user_id, managed_provider_id)` for non-deleted active mappings.

Behavior:

- The actual secret remains in `api_keys.key`.
- The API key display name should be `Codex++ Cloud`, but display name is not a
  source of truth.
- Bootstrap must ensure or reuse the mapping before returning provider data.
- Repeated bootstrap must return the same mapped key unless the key was revoked
  or rotated by an explicit admin/user operation.
- If the mapped API key is missing, inactive, expired or quota-exhausted,
  bootstrap must either repair it through the approved key-ensure path or return
  a typed error; it must not create duplicate unmanaged keys.

## Provider Key Reveal Decision

MVP decision: `GET /api/v1/client/bootstrap` returns the full user-side gateway
key in `provider.api_key` for authenticated users whose service status is
`available` or otherwise allowed to self-heal local provider configuration.

Rules:

- The returned key is only the Sub2API user-side gateway key.
- The returned key is never an upstream provider credential.
- Logs, events, fixtures, screenshots and admin views must redact the key.
- `provider.key_summary` must always be populated so UI and admin views can show
  identity without exposing the secret.
- A future one-time reveal mode may be added as an additive contract change, but
  it is not required for MVP.

## Required Device Fields

Minimum typed fields:

- `id`
- `user_id`
- `device_id`
- `platform`
- `app_version`
- `codex_version`
- `status`: `active`, `revoked`, `blocked`
- `first_seen_at`
- `last_seen_at`
- `revoked_at`
- `revoked_by`
- `revocation_reason`

Unique constraint:

- `(user_id, device_id)`

Idempotency behavior:

- same user + same `device_id` updates `last_seen_at`
- different user cannot read or mutate another user's device
- revoked or blocked devices keep their status during refresh

## Required Config Version Fields

Minimum metadata:

- `config_version`
- `publish_scope`
- `updated_by`
- `updated_at`
- `change_reason`
- `rollback_from`

MVP can implement these in settings JSON or a typed metadata table. The selected
implementation must expose a read interface for:

- client bootstrap aggregation
- gateway policy resolver
- admin UI
- E2E verification

MVP key:

- `settings.key = "codexplus_config_v1"`

MVP value shape:

- one JSON document containing `PlanCatalog`, `ModelCatalog`, `UsagePolicy`,
  `FeatureFlags` and the metadata fields above.

Write behavior:

- Admin writes must validate the full document against
  `codex-plus-contracts/config/*.schema.json`.
- Every successful write increments `config_version`.
- Rollback writes a previous validated document as a new version and records
  `rollback_from`.
- Backend readers must fail closed for invalid model/entitlement policy and fail
  open only for non-critical display metadata.

## Idempotency Keys

| Operation | Required key |
| --- | --- |
| Codex++ provider key create/reuse | `user_id + managed_provider_id` in `codexplus_managed_provider_keys` |
| Device upsert | `user_id + device_id` |
| Redeem | existing redeem idempotency plus `user_id + code` |
| Future payment fulfillment | payment provider event ID + order ID |
| Usage pre-charge | request ID + user key ID |
| Usage refund/reconcile | usage event ID |

## Migration Strategy

MVP path:

1. Prefer additive tables/fields.
2. Do not alter unrelated payment, group or API key semantics.
3. If settings JSON is used for config, add backend validation and defaults.
4. If a device table is added, include reversible migration notes before code work.
5. If event storage is added, make it append-only and low-risk.

Rollback path:

- config rollback: select previous `config_version`
- device rollback: preserve table, disable device enforcement in policy decision if needed
- event rollback: stop writes, keep existing rows for audit
- key behavior rollback: keep existing user-side keys, disable Codex++ key auto-create

## Coordinator Review Result

Status: historical storage decision, pending active `00-contract` re-approval.

Date: 2026-06-16

Current effect:

- Storage decisions below remain the preferred MVP candidate.
- They do not by themselves approve `01-backend-config-center`.
- The active correction gate in `codex-plus-dev-plan/STAGE-GATE-LEDGER.md` must pass before downstream implementation resumes.

Resolved decisions:

- Codex++ config uses `settings.key = "codexplus_config_v1"` for the current MVP
  JSON document.
- User-side key material is stored in existing `api_keys`, but Codex++ ownership
  and idempotency use `codexplus_managed_provider_keys`.
- `provider.api_key` is returned on successful bootstrap as a full user-side
  gateway key, with mandatory redaction everywhere outside the authenticated
  response body.
- Device state uses a typed Ent schema/table named `codexplus_devices`.
- Codex++ events use `codexplus_events` unless implementation confirms a
  general-purpose append-only audit table before migrations are written.
