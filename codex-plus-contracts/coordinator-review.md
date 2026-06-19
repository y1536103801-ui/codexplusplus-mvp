# Codex++ Phase 0 Coordinator Review

Date: 2026-06-16

Status: historical review, re-opened by correction gate.

Current effect:

- This review is no longer sufficient to start `01-backend-config-center`.
- `00-contract` must pass the active A/B/C parallel review in `codex-plus-dev-plan/STAGE-GATE-LEDGER.md`.
- Decisions below remain candidate decisions unless contradicted by the re-opened contract review.

## Reviewed Artifacts

- `api/client-openapi.yaml`
- `config/plan-catalog.schema.json`
- `config/model-catalog.schema.json`
- `config/usage-policy.schema.json`
- `config/feature-flags.schema.json`
- `status-error/client-status-errors.md`
- `events/client-events.schema.json`
- `test-fixtures/client/*.json`
- `compatibility-matrix.md`
- `change-review-policy.md`
- `storage-decision.md`

## Approved Decisions

- Desktop auth handoff paths are frozen as `/api/v1/auth/desktop/start`, `/api/v1/auth/desktop/complete` and `/api/v1/auth/desktop/poll`.
- Client runtime API paths are frozen as `/api/v1/client/bootstrap`, `/api/v1/client/usage`, `/api/v1/client/devices` and `/api/v1/client/redeem`.
- The current Codex++ config document lives in `settings.key = "codexplus_config_v1"` for MVP.
- Entitlements reuse existing Sub2API subscription, group, balance, quota and rate-limit foundations.
- User-side gateway key material remains in `api_keys`.
- Codex++ provider ownership and idempotency use `codexplus_managed_provider_keys` with unique `(user_id, managed_provider_id)`.
- Device state uses `codexplus_devices` with unique `(user_id, device_id)`.
- Runtime/gateway events use `codexplus_events` unless a general-purpose append-only audit table is confirmed before migration work.
- Successful authenticated bootstrap may return the full user-side gateway key in `provider.api_key`; every log, event, admin view, fixture and test artifact must redact it.
- Browser handoff `poll_token` is desktop-private; it must not appear in browser URLs, UI runtime state, diagnostic logs, event payloads or support exports.

## Worker Rules

- Implementation workers may consume these artifacts only after the active `00-contract` correction gate is approved.
- Workers must not invent response fields, config fields, status codes, error codes or event fields.
- Any contract change must follow `change-review-policy.md` before implementation changes.
- Client and UI workers may use fixtures for parallel work, but fixture behavior cannot override OpenAPI/status/error definitions.

## Remaining External Readiness Items

- Establish a source-control baseline for `sub2api-main` and `CodexPlusPlus-main`.
- Install Go before backend verification.
- Install Rust/Cargo before desktop core verification.
- Use `corepack pnpm` or install pnpm before frontend verification.
