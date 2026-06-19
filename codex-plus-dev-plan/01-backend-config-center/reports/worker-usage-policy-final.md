# Worker U Usage Policy Final Report

Report status: final

Worker lane: U

Forbidden edits: none

## Changed files

- `sub2api-main/backend/internal/codexplus/configregistry/usage_policy.go`
- `sub2api-main/backend/internal/codexplus/configregistry/usage_policy_test.go`
- `codex-plus-dev-plan/01-backend-config-center/task-usage-policy.md`
- `codex-plus-dev-plan/01-backend-config-center/reports/worker-usage-policy-final.md`

## Contract inputs

- `codex-plus-contracts/config/usage-policy.schema.json`
- `sub2api-main/backend/internal/codexplus/configregistry/common.go`
- `codex-plus-dev-plan/01-backend-config-center/task-usage-policy.md`
- `codex-plus-dev-plan/01-backend-config-center/README.md`
- `codex-plus-dev-plan/STAGE-GATE-LEDGER.md`

## Implementation summary

- Added additive `UsagePolicyCatalog`, `UsagePolicyRule`, applies-to, copy key, device policy, and device message key types in `configregistry`.
- Added `DefaultUsagePolicyCatalog`, `SampleUsagePolicyCatalog`, default rule, copy key, device policy, revoke taxonomy, and device message key constructors.
- Added `ValidateUsagePolicyCatalog` with schema-aligned validation for governance metadata, duplicate `policy_id`, applies-to IDs, `low_balance_threshold`, daily/monthly quota, concurrency, RPM, TPM, burst/window limits, expired behavior, overage behavior, copy keys, device policy, revoke taxonomy, and device message keys.
- Kept implementation isolated from `codexplus_config_service.go`; coordinator can integrate the registry output later.

## Verification

- Static file existence and `rg` self-check completed for Usage Policy fields and tests.
- `gofmt` could not run because `gofmt` is not available in PATH.
- `go test` could not run because `go` is not available in PATH.

## Downstream assumptions

- Client API/bootstrap work must consume a backend-computed snapshot only.
- Clients must not calculate low balance, decide expired behavior, enforce model/request limits, or replace copy/message keys with hardcoded localized text.
- Gateway enforcement must later consume the same Usage Policy decision and enforce quota, concurrency, RPM, TPM, burst/window, expired/overage, and device revocation behavior.
- The existing `common.go` draft status helper does not yet match the frozen config schemas; this worker validates Usage Policy draft statuses locally without modifying shared common code.

## Remaining risks

- Go syntax and tests need a real Go toolchain pass in a coordinator or CI environment.
- The additive registry is not wired into the existing shared config service yet by design.
- Cross-document checks, such as `PlanCatalog.usage_policy_id` referencing an existing Usage Policy, remain for coordinator or later integration tests.
