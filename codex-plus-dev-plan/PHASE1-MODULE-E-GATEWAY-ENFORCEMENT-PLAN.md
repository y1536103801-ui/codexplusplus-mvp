# Codex++ Phase 1 Module E Gateway Enforcement Plan

本文档把 Module E 的网关强制执行落到可实现的 service、middleware/hook、事件、错误映射和测试边界。它解决的核心问题是：Codex++ 客户端 UI 只负责展示，真正的套餐、模型、额度、设备和限流必须由 Sub2API 网关在服务端强制执行。

## Status

- State: ready after Module C foundations and Module D device/key context are available
- Owner: Module E / gateway enforcement worker
- Date: 2026-06-16
- Source evidence:
  - `sub2api-main/backend/internal/server/routes/gateway.go`
  - `sub2api-main/backend/internal/server/middleware/api_key_auth.go`
  - `sub2api-main/backend/internal/server/middleware/api_key_auth_google.go`
  - `sub2api-main/backend/internal/server/middleware/middleware.go`
  - `sub2api-main/backend/internal/handler/gateway_handler.go`
  - `sub2api-main/backend/internal/handler/openai_gateway_handler.go`
  - `sub2api-main/backend/internal/service/billing_cache_service.go`
  - `sub2api-main/backend/internal/service/gateway_service.go`
  - `codex-plus-contracts/status-error/client-status-errors.md`
  - `codex-plus-contracts/events/client-events.schema.json`
  - `codex-plus-contracts/config/*.schema.json`

## Implementation Goal

Add Codex++ server-side policy enforcement for requests using the managed `Codex++ Cloud` user-side key:

- managed key ownership and status
- user entitlement and balance/subscription status
- model permission from backend config
- device revoke/block status when device context is present
- configured rate/usage policy
- structured rejection events

Existing generic Sub2API API key, subscription, balance, quota, RPM and usage billing behavior must remain intact. Module E adds Codex++ policy on top of existing enforcement, not a replacement for it.

## Non-Goals

- Do not implement `/api/v1/client/*` handlers.
- Do not create device/key/config tables.
- Do not alter payment fulfillment.
- Do not modify desktop provider-writing logic.
- Do not move existing billing code into a Codex++ fork.
- Do not make client-provided plan, price, model multiplier, quota or renewal text trusted.

## Enforcement Placement

Existing flow:

```text
gateway route
  -> API key auth middleware
  -> group requirement middleware
  -> gateway/openai handler parses requested model
  -> billing eligibility check
  -> account selection / forwarding
  -> usage recording / billing
```

Codex++ enforcement should run after API key auth has loaded `service.APIKey` and before account selection / forwarding.

Recommended placement:

- Add a pure service: `CodexPlusGatewayPolicyService`.
- Add small handler helper calls in `gateway_handler.go` and `openai_gateway_handler.go` after requested model is parsed and before `SelectAccount...`.
- Do not put requested-model enforcement only in auth middleware, because the auth middleware does not know the request model.

Optional helper files:

- `sub2api-main/backend/internal/service/codexplus_gateway_policy.go`
- `sub2api-main/backend/internal/handler/codexplus_gateway_policy_helper.go`
- tests beside changed service/handler files

## Policy Inputs

From request context:

- authenticated `APIKey` from `middleware.GetAPIKeyFromContext(c)`
- subscription from `middleware.GetSubscriptionFromContext(c)` if present
- requested model parsed by the handler
- inbound endpoint path
- request ID/client request ID
- optional Codex++ device ID from header

Suggested device header:

- `X-CodexPlus-Device-Id`

Rules:

- Device header is optional for generic Sub2API usage.
- For managed Codex++ Cloud keys, device header should be required once device enforcement is enabled in `FeatureFlags` / `UsagePolicy`.
- Missing device context under strict enforcement should map to a typed policy rejection, not silently bypass device checks.

From backend services:

- `codexplus_config_v1`
- `codexplus_managed_provider_keys`
- `codexplus_devices`
- `codexplus_events`
- existing user/subscription/API key/group data already loaded by auth middleware

## Policy Decision Output

Service shape:

```go
type CodexPlusGatewayPolicyDecision struct {
    Allowed       bool
    HTTPStatus    int
    ErrorCode     string
    ServiceStatus string
    Reason        string
    Retryable     bool
    EventType     string
}
```

Suggested method:

```go
func (s *CodexPlusGatewayPolicyService) Evaluate(ctx context.Context, input CodexPlusGatewayPolicyInput) (*CodexPlusGatewayPolicyDecision, error)
```

Important distinction:

- `Allowed=true` means the request can continue to existing billing/account selection.
- `Allowed=false` means handler must return protocol-appropriate error and append `gateway_policy_rejected`.
- `error != nil` means policy infrastructure failed; behavior depends on config. MVP should fail closed for entitlement/model/device checks.

## Managed Key Detection

Codex++ enforcement must only apply to API keys owned by `codexplus_managed_provider_keys` with `managed_provider_id = "codex-plus-cloud"` unless a future contract expands scope.

Rules:

- If no managed-provider mapping exists for the API key, treat it as a normal Sub2API key and skip Codex++ policy.
- If mapping exists but status is not `active`, reject.
- If mapping user ID does not match API key user ID, reject and emit an audit event.
- If mapping points to deleted/inactive/expired/quota-exhausted key, rely on existing API key auth for generic errors and emit Codex++ event if context is available.

## Checks

### Entitlement

Use existing subscription/balance data loaded by the auth and billing layers, but emit Codex++-specific event/status when the managed key fails:

- no eligible plan -> `not_purchased`
- expired/suspended subscription -> `expired`
- balance or quota exhausted -> `low_balance`

Do not duplicate billing math. Use existing `BillingCacheService.CheckBillingEligibility` as the monetary eligibility gate.

### Model Permission

Use `ModelCatalog` and plan/model policy from `codexplus_config_v1`.

Reject when:

- requested model is disabled
- requested model is not in the user's plan/model group
- default model referenced by bootstrap is no longer available and no fallback is configured

Map to:

- service status: `model_unavailable`
- error code: `CLIENT_MODEL_UNAVAILABLE` or gateway-specific policy code from `client-status-errors.md`
- event: `gateway_policy_rejected`

### Device

If device enforcement is enabled:

- Read `X-CodexPlus-Device-Id`.
- Load `codexplus_devices` by `user_id + device_id`.
- Reject `revoked` or `blocked`.
- If missing device and strict mode enabled, reject with a device-related typed error.

Map revoked/blocked to:

- service status: `device_revoked`
- event: `gateway_policy_rejected`

### Rate and Usage Policy

Do not bypass existing Sub2API API key rate limits, user RPM, group RPM, subscription usage windows, API key quota and user×platform quota.

Module E may add Codex++-specific policy gates only when sourced from `UsagePolicy`, for example:

- desktop-specific request frequency caps
- model-specific allow/deny gates
- feature-flagged strict device enforcement

Any new rate field requires config schema update before implementation.

## Error Response Mapping

Gateway routes speak different protocols. Do not return the `/api/v1/client` envelope from model gateway endpoints.

Required behavior:

- Anthropic/OpenAI-compatible routes return the existing gateway protocol error shape.
- Gemini routes return Google-style errors via the existing pattern used in `api_key_auth_google.go`.
- HTTP status should preserve retry semantics:
  - entitlement/model/device permission -> 403
  - usage/rate temporarily exhausted -> 429
  - policy infrastructure unavailable in fail-closed mode -> 503

Handler helper should centralize this so individual gateway methods do not hand-roll inconsistent responses.

## Event Emission

Emit `gateway_policy_rejected` to `codexplus_events`.

Required event fields:

- `event_type`
- `user_id`
- `device_id` if present
- `api_key_id`
- `request_id`
- `config_version`
- `status`
- `error_code`
- metadata:
  - requested model
  - endpoint
  - managed provider ID
  - redacted key summary if available

Forbidden event fields:

- full API key
- Authorization header
- JWT
- upstream provider credential
- raw prompt
- raw response body

Event write should be best-effort for observability, but policy rejection itself must not be silently allowed because event storage failed.

## Suggested Files

Module E may edit:

- `sub2api-main/backend/internal/service/codexplus_gateway_policy.go`
- `sub2api-main/backend/internal/service/codexplus_gateway_policy_test.go`
- `sub2api-main/backend/internal/handler/codexplus_gateway_policy_helper.go`
- minimal call sites in:
  - `sub2api-main/backend/internal/handler/gateway_handler.go`
  - `sub2api-main/backend/internal/handler/openai_gateway_handler.go`
  - Gemini/OpenAI alias handlers only where requested model and API key are available
- `sub2api-main/backend/internal/service/wire.go` only if the new policy service is injected
- handler/service tests

Module E must not edit:

- `migrations/*.sql`
- `ent/schema/**`
- `/api/v1/client/*` route/handler files owned by Module D
- admin UI
- desktop files
- payment provider code

## Integration With Existing Gateway Hot Paths

Anthropic-style handler checkpoints:

- after body/model parse
- after API key is read from context
- before `billingCacheService.CheckBillingEligibility`
- before account selection

OpenAI Responses / Chat checkpoints:

- after `reqModel` is parsed
- before content moderation/account selection/forwarding where possible
- ensure WebSocket initial message path is also covered

Gemini-native checkpoints:

- after requested model extraction
- before selecting/forwarding to Gemini accounts
- return Google-style error shape

Do not enforce Codex++ model policy after forwarding; rejection must happen before upstream spend.

## Tests Required

Service tests:

- unmanaged API key skips Codex++ policy
- active managed key with allowed model passes
- managed key with disabled model rejects
- managed key outside plan model set rejects
- revoked device rejects
- blocked device rejects
- missing device rejects only when strict enforcement is enabled
- event metadata is redacted
- config read failure fails closed for model/device policy

Handler tests:

- Anthropic route rejection returns protocol-compatible error
- OpenAI route rejection returns protocol-compatible error
- Gemini route rejection returns Google-style error
- rejected request does not call account selection / upstream forward mock
- valid managed request still reaches existing billing eligibility

Suggested commands once Go/toolchain is available:

```powershell
cd C:\Users\23293\Desktop\codex+++\sub2api-main\backend
go test ./internal/service ./internal/handler ./internal/server/middleware
```

Do not claim these commands passed until Go is installed and the commands actually run.

## Parallel Boundaries

Module E can start service-level policy tests once Module C publishes stable interfaces.

Module E should wait for Module D only for:

- exact device header adoption
- bootstrap/device context details
- status/error naming changes

Module E must not add fields to contracts. If it needs a new config/status/event field, stop and request a contract patch.

## Exit Gate

Module E is complete only when:

- Managed Codex++ API keys are identified through `codexplus_managed_provider_keys`.
- Unmanaged Sub2API API keys keep existing behavior.
- Model, entitlement, balance/quota and device policy are enforced server-side before upstream spend.
- Rejection events are emitted and redacted.
- Error responses match the gateway protocol shape.
- Tests prove major rejection paths and happy path.
- Module I can execute E2E cases for unauthorized model, expired/low balance and revoked device.
