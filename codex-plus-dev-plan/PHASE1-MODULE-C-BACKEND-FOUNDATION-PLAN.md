# Codex++ Phase 1 Module C Backend Foundation Plan

本文档把 Phase 1 Module C 从“可派工”进一步落到可实现文件、schema、migration、service/repository 边界。它只规划后端基础层，不实现 `/api/v1/client/*` handler，不改桌面端，不改 admin UI。

## Status

- State: ready for implementation
- Owner: Module C / backend foundation worker
- Date: 2026-06-16
- Source evidence:
  - `sub2api-main/backend/ent/schema/api_key.go`
  - `sub2api-main/backend/ent/schema/setting.go`
  - `sub2api-main/backend/ent/schema/user_subscription.go`
  - `sub2api-main/backend/internal/repository/setting_repo.go`
  - `sub2api-main/backend/internal/repository/api_key_repo.go`
  - `sub2api-main/backend/migrations/README.md`
  - `codex-plus-contracts/storage-decision.md`

## Implementation Goal

Module C must provide the backend foundations consumed later by Module D client API, Module E gateway enforcement, Module H admin operations and Module I E2E:

- Versioned Codex++ config registry stored under `settings.key = "codexplus_config_v1"`.
- Typed device state through `codexplus_devices`.
- Typed managed-provider key mapping through `codexplus_managed_provider_keys`.
- Append-only Codex++ event stream through `codexplus_events`.
- Repository/service interfaces that are idempotent, testable and redaction-safe.

## Non-Goals

- Do not register `/api/v1/client/*` routes.
- Do not write gateway enforcement hooks.
- Do not change payment provider code.
- Do not change desktop provider-writing code.
- Do not generate admin UI.
- Do not add a second billing or entitlement system.

## Source Pattern Notes

- Migrations are embedded by `sub2api-main/backend/migrations/migrations.go` and run through checksum validation. Existing migration files must not be edited.
- Next Codex++ migration should be a new forward-only file. At the current snapshot, the next available number appears to be `151`.
- Regular migrations run in a transaction. Use `*_notx.sql` only for concurrent index migrations.
- `settings` is a unique key/value table and works for the MVP config JSON, but it has no built-in history.
- `api_keys.name` is not unique and there is no metadata column, so Codex++ managed key ownership must not rely on API key display name alone.
- Existing repository constructors are registered in `internal/repository/wire.go`; services are registered in `internal/service/wire.go`.

## Proposed File Changes

### Ent Schema

Additive schema files:

- `sub2api-main/backend/ent/schema/codexplus_device.go`
- `sub2api-main/backend/ent/schema/codexplus_managed_provider_key.go`
- `sub2api-main/backend/ent/schema/codexplus_event.go`

Do not edit unrelated Ent schemas except adding required edges if the project convention requires them. Prefer explicit foreign key fields over broad bidirectional edge rewrites.

### Migration

Add:

- `sub2api-main/backend/migrations/151_codexplus_foundation.sql`

The migration must be idempotent and forward-only:

- `CREATE TABLE IF NOT EXISTS codexplus_devices`
- `CREATE TABLE IF NOT EXISTS codexplus_managed_provider_keys`
- `CREATE TABLE IF NOT EXISTS codexplus_events`
- `CREATE UNIQUE INDEX IF NOT EXISTS ...` for idempotency constraints
- `CREATE INDEX IF NOT EXISTS ...` for query paths
- optional seed/upsert of `settings.key = 'codexplus_config_v1'` with a minimal valid JSON document

### Repository

Additive repository files:

- `sub2api-main/backend/internal/repository/codexplus_device_repo.go`
- `sub2api-main/backend/internal/repository/codexplus_managed_provider_key_repo.go`
- `sub2api-main/backend/internal/repository/codexplus_event_repo.go`
- optional `sub2api-main/backend/internal/repository/codexplus_config_repo.go` if `SettingRepository` is too generic for tests

Register constructors in:

- `sub2api-main/backend/internal/repository/wire.go`

### Service

Additive service files:

- `sub2api-main/backend/internal/service/codexplus_config.go`
- `sub2api-main/backend/internal/service/codexplus_device.go`
- `sub2api-main/backend/internal/service/codexplus_managed_provider_key.go`
- `sub2api-main/backend/internal/service/codexplus_event.go`

Register constructors in:

- `sub2api-main/backend/internal/service/wire.go`

Only add the minimum service surface required by Module D/E/H. Handler-facing orchestration can be left to Module D.

## Schema Details

### `codexplus_devices`

Required columns:

- `id BIGSERIAL PRIMARY KEY`
- `user_id BIGINT NOT NULL`
- `device_id VARCHAR(128) NOT NULL`
- `platform VARCHAR(64) NOT NULL DEFAULT ''`
- `app_version VARCHAR(64) NOT NULL DEFAULT ''`
- `codex_version VARCHAR(64) NOT NULL DEFAULT ''`
- `status VARCHAR(24) NOT NULL DEFAULT 'active'`
- `first_seen_at TIMESTAMPTZ NOT NULL DEFAULT NOW()`
- `last_seen_at TIMESTAMPTZ NOT NULL DEFAULT NOW()`
- `revoked_at TIMESTAMPTZ NULL`
- `revoked_by BIGINT NULL`
- `revocation_reason TEXT NULL`
- `created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()`
- `updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()`

Indexes:

- unique `(user_id, device_id)`
- `(user_id, status)`
- `(last_seen_at)`

Status enum in service layer:

- `active`
- `revoked`
- `blocked`

Upsert behavior:

- Same `user_id + device_id` updates `last_seen_at`, `platform`, `app_version`, `codex_version`.
- Existing `revoked` or `blocked` status must not be overwritten by a normal refresh.
- A different user cannot read or mutate another user's device.

### `codexplus_managed_provider_keys`

Required columns:

- `id BIGSERIAL PRIMARY KEY`
- `user_id BIGINT NOT NULL`
- `managed_provider_id VARCHAR(64) NOT NULL`
- `api_key_id BIGINT NOT NULL`
- `status VARCHAR(24) NOT NULL DEFAULT 'active'`
- `created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()`
- `updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()`
- `rotated_at TIMESTAMPTZ NULL`
- `revoked_at TIMESTAMPTZ NULL`
- `revoked_by BIGINT NULL`
- `revocation_reason TEXT NULL`

Indexes:

- unique `(user_id, managed_provider_id)`
- `(api_key_id)`
- `(status)`

Behavior:

- MVP `managed_provider_id` is `codex-plus-cloud`.
- The actual secret remains in `api_keys.key`.
- API key display name should be `Codex++ Cloud`, but display name is not a source of truth.
- The ensure/reuse service must be idempotent. Repeated calls return the same active mapping unless explicit rotation/revocation occurred.
- If the mapped `api_key_id` is deleted/inactive/expired/quota-exhausted, return a typed repair-needed status or perform an approved repair flow. Do not create duplicate unmanaged keys.

### `codexplus_events`

Required columns:

- `id BIGSERIAL PRIMARY KEY`
- `event_type VARCHAR(64) NOT NULL`
- `user_id BIGINT NULL`
- `device_id VARCHAR(128) NULL`
- `api_key_id BIGINT NULL`
- `request_id VARCHAR(128) NULL`
- `config_version VARCHAR(64) NULL`
- `status VARCHAR(64) NULL`
- `error_code VARCHAR(96) NULL`
- `metadata JSONB NOT NULL DEFAULT '{}'::jsonb`
- `created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()`

Indexes:

- `(event_type, created_at)`
- `(user_id, created_at)`
- `(request_id)`
- `(api_key_id, created_at)`

Rules:

- Append-only for MVP.
- No full API Key, JWT, Authorization header, upstream credential, raw prompt or raw response body.
- `metadata` may store model ID, plan ID, policy reason and redacted key summary.

## Config Registry Contract

MVP setting key:

- `codexplus_config_v1`

The value is one JSON document containing:

- `PlanCatalog`
- `ModelCatalog`
- `UsagePolicy`
- `FeatureFlags`
- metadata: `config_version`, `publish_scope`, `updated_by`, `updated_at`, `change_reason`, `rollback_from`

Service methods should be small and explicit:

- `GetCodexPlusConfig(ctx) (*CodexPlusConfig, error)`
- `ValidateCodexPlusConfig(ctx, cfg) error`
- `SaveCodexPlusConfig(ctx, cfg, actorID, changeReason) error`
- `GetCodexPlusConfigVersion(ctx) (string, error)`

Validation requirements:

- duplicate plan IDs rejected
- duplicate model IDs rejected
- negative prices rejected
- negative or nonsensical limits rejected
- default model must exist and be enabled
- disabled model cannot be the default model
- public client snapshot must exclude admin-only/internal cost fields

Versioning behavior:

- Every successful save increments `config_version`.
- Rollback saves a previous validated payload as a new version and fills `rollback_from`.
- Invalid JSON in settings must fail closed for policy and fail visibly for admin/config tests.

## Repository and Service Interfaces

### Device Repository

Minimum methods:

- `UpsertSeen(ctx, device) (*CodexPlusDevice, error)`
- `GetByUserAndDeviceID(ctx, userID, deviceID) (*CodexPlusDevice, error)`
- `ListByUserID(ctx, userID) ([]CodexPlusDevice, error)`
- `UpdateStatus(ctx, userID, deviceID, status, actorID, reason) error`

### Managed Provider Key Repository

Minimum methods:

- `GetByUserAndProvider(ctx, userID, managedProviderID) (*CodexPlusManagedProviderKey, error)`
- `CreateMapping(ctx, mapping) (*CodexPlusManagedProviderKey, error)`
- `UpdateStatus(ctx, id, status, actorID, reason) error`

The service layer, not the repository layer, should orchestrate API key creation through existing `APIKeyService`.

### Event Repository

Minimum methods:

- `Append(ctx, event) error`
- `ListByUserID(ctx, userID, limit) ([]CodexPlusEvent, error)`
- `ListByRequestID(ctx, requestID) ([]CodexPlusEvent, error)`

## Integration With Existing Services

Module C should expose foundations, not full bootstrap orchestration:

- It may define `CodexPlusManagedProviderKeyService.EnsureForUser(ctx, userID, groupID)` if it only wraps existing `APIKeyService.Create` and mapping persistence.
- It must not implement `/api/v1/client/bootstrap`; Module D owns handler aggregation.
- It must not implement gateway policy decisions; Module E owns request-path enforcement.
- It may define pure policy read models consumed later by Module E if they are covered by tests.

## Tests Required

Backend unit/integration tests should cover:

- `codexplus_config_v1` default load when setting is missing.
- config validation rejects duplicate IDs, invalid default model and invalid limits.
- config save increments version and records metadata.
- device upsert creates first record.
- repeated device upsert updates `last_seen_at` without duplicating.
- revoked/blocked device is not reactivated by refresh.
- managed provider key mapping is idempotent for `user_id + codex-plus-cloud`.
- mapped key summary redacts full key.
- event append rejects or sanitizes secret-looking metadata.

Suggested commands once Go/toolchain is available:

```powershell
cd C:\Users\23293\Desktop\codex+++\sub2api-main\backend
go test ./ent/schema ./internal/repository ./internal/service
```

If Ent code generation is required:

```powershell
cd C:\Users\23293\Desktop\codex+++\sub2api-main\backend
go generate ./ent
```

Do not claim tests passed until Go is installed and the commands have actually run.

## Parallel Boundaries

Allowed in parallel:

- Module D may create handler skeletons against interfaces after Module C publishes interface names.
- Module F/G may use fixtures without backend implementation.
- Module H may draft admin UI from schemas without writing backend migrations.

Not allowed in parallel:

- Two modules editing `migrations/*.sql`.
- Two modules editing `ent/schema/*codexplus*`.
- Module D creating its own device/key storage.
- Module E reading raw settings JSON directly instead of using the Module C config interface.

## Exit Gate

Module C is complete only when:

- Migration and Ent schema names match this plan.
- Repository/service interfaces are registered in Wire or documented if Wire generation is deferred.
- Tests prove config validation, device upsert, managed key idempotency and event redaction.
- Module D can consume stable interfaces without inventing storage names.
- No frontend, desktop or route registration changes are included unless explicitly assigned by the coordinator.
