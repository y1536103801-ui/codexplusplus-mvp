# Codex++ Phase 1 Module D Client API Plan

本文档把 Phase 1 Module D 的 `/api/v1/client/*` 实现落到路由、handler、service、DTO、测试和并行边界。它消费 Phase 0 契约和 Module C 后端基础，不负责新增存储 schema，也不负责网关请求路径 enforcement。

## Status

- State: ready after Module C interface names are published
- Owner: Module D / backend client API worker
- Date: 2026-06-16
- Source evidence:
  - `sub2api-main/backend/internal/server/router.go`
  - `sub2api-main/backend/internal/server/routes/user.go`
  - `sub2api-main/backend/internal/handler/handler.go`
  - `sub2api-main/backend/internal/handler/wire.go`
  - `sub2api-main/backend/internal/handler/api_key_handler.go`
  - `sub2api-main/backend/internal/handler/redeem_handler.go`
  - `sub2api-main/backend/internal/pkg/response/response.go`
  - `sub2api-main/backend/internal/server/middleware/auth_subject.go`
  - `codex-plus-contracts/api/client-openapi.yaml`
  - `codex-plus-contracts/test-fixtures/client/*.json`
  - `codex-plus-dev-plan/PHASE1-MODULE-C-BACKEND-FOUNDATION-PLAN.md`

## Implementation Goal

Expose a stable desktop-client facade:

- `GET /api/v1/client/bootstrap`
- `GET /api/v1/client/usage`
- `POST /api/v1/client/devices`
- `POST /api/v1/client/redeem`

The facade aggregates existing Sub2API auth, subscription, balance, API key, config, device and redeem foundations into the contract shape required by Codex++ desktop.

## Non-Goals

- Do not add or modify Ent schemas.
- Do not add migrations.
- Do not implement gateway policy enforcement.
- Do not change payment callback behavior.
- Do not change existing `/api/v1/user/*`, `/api/v1/keys/*`, `/api/v1/redeem/*` behavior.
- Do not make the desktop client compute price, model multiplier, quota, package status or renewal copy.

## Routing Plan

Add a new route file:

- `sub2api-main/backend/internal/server/routes/client.go`

Register it from `registerRoutes` in:

- `sub2api-main/backend/internal/server/router.go`

Route group:

```go
authenticated := v1.Group("")
authenticated.Use(gin.HandlerFunc(jwtAuth))
authenticated.Use(middleware.BackendModeUserGuard(settingService))

client := authenticated.Group("/client")
client.GET("/bootstrap", h.Client.Bootstrap)
client.GET("/usage", h.Client.Usage)
client.POST("/devices", h.Client.UpsertDevice)
client.POST("/redeem", h.Client.Redeem)
```

Rationale:

- Existing user routes already use JWT and `BackendModeUserGuard`.
- `/client` stays separate from `/user` so desktop facade behavior does not leak into browser/admin user APIs.
- Gateway routes under `/v1`, `/responses`, `/backend-api/codex/*` remain owned by Module E.

## Handler Plan

Add:

- `sub2api-main/backend/internal/handler/client_handler.go`
- `sub2api-main/backend/internal/handler/dto/codexplus_client.go`
- `sub2api-main/backend/internal/handler/client_handler_test.go`

Update:

- `sub2api-main/backend/internal/handler/handler.go`
- `sub2api-main/backend/internal/handler/wire.go`

New handler field:

```go
type Handlers struct {
    ...
    Client *ClientHandler
}
```

Constructor:

```go
func NewClientHandler(clientService *service.CodexPlusClientService) *ClientHandler
```

Handler identity:

- Use `middleware.GetAuthSubjectFromContext(c)` to obtain `subject.UserID`.
- Return `response.Unauthorized(c, "User not authenticated")` if missing.
- Never accept `user_id` from request bodies or query parameters.

Response envelope:

- Use existing `response.Success(c, data)` and `response.ErrorFrom(c, err)` unless a contract patch explicitly changes the envelope.
- The data payload must match `codex-plus-contracts/api/client-openapi.yaml`.

## Service Orchestration Plan

Add a facade service:

- `sub2api-main/backend/internal/service/codexplus_client.go`

This service can depend on Module C foundations and existing Sub2API services:

- `CodexPlusConfigService`
- `CodexPlusDeviceService`
- `CodexPlusManagedProviderKeyService`
- `CodexPlusEventService`
- `APIKeyService`
- `RedeemService`
- `SubscriptionService`
- user/balance repository or existing user service where available
- usage/usage summary service or repository

Suggested methods:

- `Bootstrap(ctx, input CodexPlusBootstrapInput) (*CodexPlusBootstrapSnapshot, error)`
- `Usage(ctx, userID int64) (*CodexPlusUsageSnapshot, error)`
- `UpsertDevice(ctx, userID int64, input CodexPlusDeviceInput) (*CodexPlusDeviceSnapshot, error)`
- `Redeem(ctx, userID int64, code string, deviceID *string) (*CodexPlusRedeemResult, error)`

Module D owns aggregation and mapping to client-facing statuses. Module C owns persistence primitives.

## Endpoint Behavior

### `GET /api/v1/client/bootstrap`

Inputs:

- JWT user identity
- optional `device_id` query parameter
- optional client headers such as request ID, app version or platform if adopted by OpenAPI

Behavior:

- Load `codexplus_config_v1`.
- Load user entitlement from existing subscription/group/balance state.
- If `device_id` is present, load device state from `codexplus_devices`.
- If user can use the service, ensure/reuse `codexplus_managed_provider_keys` for `codex-plus-cloud`.
- Return `provider.api_key` only for authenticated responses where local provider repair is allowed.
- Never return upstream provider credentials.
- Emit `bootstrap_requested`.

Required statuses:

- `available`
- `not_purchased`
- `expired`
- `low_balance`
- `device_revoked`
- `model_unavailable`
- `gateway_unhealthy`

### `GET /api/v1/client/usage`

Behavior:

- Return balance/usage/rate-limit summary from existing usage logs, API key quota fields and subscription usage windows.
- Use server-calculated values only.
- Do not return hidden cost multipliers or upstream cost fields unless explicitly in the contract.

### `POST /api/v1/client/devices`

Request:

- `device_id`
- `platform`
- `app_version`
- `codex_version`

Behavior:

- Idempotently upsert `user_id + device_id`.
- Preserve `revoked` or `blocked` status during refresh.
- Return current device status and timestamps.
- Emit `device_registered`.

Idempotency:

- The database unique key `(user_id, device_id)` is the primary guard.
- If an `Idempotency-Key` header is present, the handler may also use `executeUserIdempotentJSON`.

### `POST /api/v1/client/redeem`

Request:

- `code`
- optional `device_id`

Behavior:

- Delegate redemption to existing redeem service.
- Refresh entitlement snapshot after success.
- Return contract-shaped result, not the old browser redeem DTO unless it already matches.
- Emit `redeem_attempted`.

Idempotency:

- Use existing redeem idempotency plus `user_id + code`.
- Use `executeUserIdempotentJSON` if request replay behavior is needed.

## DTO Rules

DTO names should map one-to-one with contract terms:

- `ClientBootstrapResponse`
- `ClientServiceStatus`
- `ManagedProvider`
- `ClientPlanSummary`
- `ClientModel`
- `ClientUsageSummary`
- `ClientDevice`
- `ClientFeatureFlags`
- `ClientRedeemResponse`

DTO redaction:

- `provider.api_key` may contain the full user-side gateway key only inside the authenticated bootstrap response body.
- `key_summary.masked_key` must be used for logs, admin summaries, fixtures and test reports.
- No DTO may include upstream account tokens, upstream API keys, Authorization headers, JWTs, raw prompt text or raw response bodies.

## Error and Status Mapping

Use `codex-plus-contracts/status-error/client-status-errors.md` as the only source of shared client-facing status/error names.

Mapping rules:

- Auth failures are HTTP 401 with existing response envelope.
- Business states such as `not_purchased`, `expired`, `low_balance` should normally be HTTP 200 bootstrap snapshots with `service.status`.
- Invalid request shape is HTTP 400.
- Cross-user device/key access must behave as not found or forbidden without leaking the target object.
- Backend dependency failure should map to `gateway_unhealthy` or a typed retryable error if it affects bootstrap usability.

## Tests Required

Add focused tests around handler/service behavior:

- unauthenticated request returns 401
- bootstrap available matches `bootstrap.available.json` shape
- not purchased user maps to `not_purchased`
- expired subscription maps to `expired`
- low balance maps to `low_balance`
- revoked device maps to `device_revoked`
- model removed/default disabled maps to `model_unavailable`
- repeated bootstrap reuses the same `codexplus_managed_provider_keys` mapping
- `provider.api_key` is present only where contract allows and never appears in logs
- device upsert is idempotent and does not reactivate revoked/blocked devices
- redeem returns refreshed entitlement/usage summary or a clear retry instruction

Suggested commands once Go/toolchain is available:

```powershell
cd C:\Users\23293\Desktop\codex+++\sub2api-main\backend
go test ./internal/handler ./internal/server ./internal/service
```

If route registration touches generated Wire output:

```powershell
cd C:\Users\23293\Desktop\codex+++\sub2api-main\backend
go generate ./cmd/server ./internal/handler ./internal/service ./internal/repository
```

Do not claim these commands passed unless Go and generation tools are installed and the commands actually run.

## Parallel Boundaries

Module D may edit:

- `internal/server/routes/client.go`
- minimal route registration in `internal/server/router.go`
- `internal/handler/client_handler.go`
- `internal/handler/dto/codexplus_client.go`
- `internal/service/codexplus_client.go`
- associated tests
- handler/service Wire registration if implementation requires it

Module D must not edit:

- `migrations/*.sql`
- `ent/schema/**`
- gateway request handlers under `/v1`, `/responses`, `/backend-api/codex/*`
- admin Vue pages
- desktop files
- payment provider callback code

If Module D needs a storage method not provided by Module C, it must request a Module C interface addition. It must not create a second storage path.

## Exit Gate

Module D is complete only when:

- All four `/api/v1/client/*` routes exist and require JWT.
- Handler responses match OpenAPI and fixtures.
- Bootstrap key creation/reuse is idempotent through `codexplus_managed_provider_keys`.
- Device status affects bootstrap.
- Logs and events redact secrets.
- Contract tests cover success and major failure states.
- Module E can consume stable status, device and policy context without scraping handler internals.
