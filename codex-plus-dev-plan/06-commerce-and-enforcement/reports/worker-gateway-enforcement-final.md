Report status: final
Worker lane: Gateway
Forbidden edits: none

## Changed files

- `sub2api-main/backend/internal/service/codexplus_gateway_policy_service.go`
- `sub2api-main/backend/internal/service/codexplus_gateway_policy_service_test.go`
- `codex-plus-dev-plan/06-commerce-and-enforcement/reports/worker-gateway-enforcement-final.md`

## Implementation

- Completed Codex++ gateway policy resolution so managed gateway requests fail closed when no PlanCatalog entitlement source matches the API key group, subscription group, or group name.
- Added resolved plan and usage policy context for request-path enforcement.
- Added model authorization checks against backend-controlled plan model groups.
- Added usage-policy rejection paths for missing policy, exhausted daily quota, invalid concurrency/RPM/TPM settings, and estimated tokens exceeding TPM.
- Preserved strict device enforcement from both feature flags and usage policy device defaults.
- Added allowed-request usage event payloads so successful gateway decisions can be reconciled with request ID, model, route model, plan, usage policy, device, managed key, config version, and estimated tokens.
- Normalized rejection event payloads through the Codex++ audit/risk redaction helper so gateway policy failures are queryable without leaking secrets.

## Verification

- `gofmt` was run on gateway policy files.
- Passed targeted service tests:
  - unmanaged API key skips policy;
  - managed allowed model passes;
  - disabled/out-of-plan model rejects;
  - missing entitlement source fails closed;
  - API key group and subscription group mappings resolve backend plans;
  - revoked, blocked, unknown and strict-mode missing devices reject;
  - billing errors map to protocol status and retryability;
  - config failure fails closed.
- Coordinator reran:
  - `go test ./internal/service -run "CodexPlus(Device|Gateway|Audit|Risk|Commerce|Payment|Subscription)"`
  - `go test ./internal/service ./internal/handler ./internal/handler/admin ./internal/repository ./internal/server/routes -run "CodexPlus|Payment|Subscription|Gateway|Device|Audit|Risk"`
  - `tools/verify-06-go.ps1`

## Coordinator follow-up

- Request-path pre-deduct / actual-settle / rollback accounting should be exercised in `07-integration-release` with real gateway traffic and billing logs.
- If production needs live concurrency/RPM counters beyond existing billing services, add a dedicated counter backend in a later scoped change rather than putting mutable counters into static config.

## Remaining risks

- This lane enforces configured policy decisions and emits usage/rejection payloads, but real token-level settlement still needs release-gate E2E evidence.
- Successful usage events are append-only best effort; downstream reconciliation must tolerate event recorder errors.
