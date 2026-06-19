# 02 Backend Client API Coordinator Gate

Report status: final
Worker lane: Coordinator
Forbidden edits: none

## Changed files

- `sub2api-main/backend/internal/service/codexplus_client.go`
- `sub2api-main/backend/internal/service/codexplus_client_test.go`
- `sub2api-main/backend/internal/service/wire.go`
- `sub2api-main/backend/internal/handler/client/client_handler.go`
- `sub2api-main/backend/internal/handler/dto/codexplus_client.go`
- `codex-plus-dev-plan/tools/verify-02-static.ps1`
- `codex-plus-dev-plan/tools/verify-02-go.ps1`
- `codex-plus-dev-plan/tools/validate-stage-gate.ps1`
- `codex-plus-dev-plan/02-backend-client-api/reports/coordinator-integration-static-gate.md`

## Integration summary

- `/api/v1/client/bootstrap`, `/usage`, `/devices`, and `/redeem` are implemented behind authenticated client routes.
- Client success responses now use a contract-compatible envelope with `code`, `status`, `message`, `reason`, `error_code`, and `data`.
- Bootstrap and usage DTOs include client contract fields such as `message_key`, `commerce_action`, `balance_summary`, `period_usage`, `action_copy_key`, `announcements`, `force_update_prompt`, and `strict_device_enforcement`.
- Client events now carry request context and preserve config version when the endpoint has a config snapshot.
- `firstPlan` and `firstUsagePolicy` fallback behavior was removed. Client entitlement now resolves a plan through backend-configured `entitlement_sources`, resolves the matching usage policy through `usage_policy_id`, and filters visible models by the selected plan model groups.
- Redeem integration now depends on the narrow `CodexPlusClientRedeemer` interface so client API status mapping can be verified independently while production still injects the real `RedeemService`.

## Verification

- `powershell -ExecutionPolicy Bypass -File .\tools\verify-02-static.ps1` passed.
- `powershell -ExecutionPolicy Bypass -File .\tools\verify-02-go.ps1` passed.
- Direct targeted check passed: `go test ./internal/service ./internal/handler/client ./internal/handler/dto ./internal/server/routes -run "CodexPlus|Client"`.
- Direct service check passed: `go test ./internal/service -run CodexPlusClient`.

## Gate decisions

- Bootstrap API: fixed. Managed provider key creation/reuse, service status, contract fields, config/snapshot version, feature flags, event context, and plan/model entitlement selection are covered by targeted tests.
- Usage API: fixed. Client usage now returns structured balance, period usage, rate-limit state, renewal action, request event, and config-version context.
- Device API: fixed. Device upsert is idempotent at the client service boundary and emits structured `device_registered` events with user, device, request, and client metadata.
- Redeem API: fixed. Client redeem maps applied, invalid, already-used, and expired statuses without exposing redeem internals, and emits `redeem_attempted` events.
- Backend-config policy source: fixed. Usage status and visible models now share the same backend-configured plan and usage policy source used by gateway enforcement.

## Downstream handoff

- `03-client-cloud-core` may consume the client bootstrap/usage/device/redeem API and contract fields after this gate.
- `04-client-user-experience` should treat `message_key`, `action_copy_key`, and `feature_flags` as backend-driven display controls.
- `05-admin-operations` and `06-commerce-and-enforcement` still own deeper admin, payment, and enforcement workflows; this stage only consumes their existing service seams.

## Remaining risks

- This gate uses targeted backend tests, not full repository CI.
- Rust/Cargo and frontend TypeScript checks remain outside `02-backend-client-api`.
- Real JWT HTTP calls and live gateway logs still belong to later integration release verification.
