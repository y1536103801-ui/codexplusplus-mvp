# Worker Feature Flags Final Report

Report status: final
Worker lane: F
Forbidden edits: none

## Changed files

- `sub2api-main/backend/internal/codexplus/configregistry/feature_flags.go`
- `sub2api-main/backend/internal/codexplus/configregistry/feature_flags_test.go`
- `codex-plus-dev-plan/01-backend-config-center/task-feature-flags.md`
- `codex-plus-dev-plan/01-backend-config-center/reports/worker-feature-flags-final.md`

## Contract inputs

- `codex-plus-contracts/config/feature-flags.schema.json`
- `sub2api-main/backend/internal/codexplus/configregistry/common.go`
- `codex-plus-dev-plan/STAGE-GATE-LEDGER.md`
- `codex-plus-dev-plan/01-backend-config-center/README.md`
- `codex-plus-dev-plan/01-backend-config-center/task-feature-flags.md`

## Implementation summary

- Added additive FeatureFlags registry types matching the frozen schema fields: metadata, `flags`, `exposure`, and `copy_keys`.
- Added default and sample constructors for all frozen flags: `advanced_provider_config`, `install_assistant`, `new_user_tutorial`, `model_selector`, `diagnostic_export`, `announcements`, `force_update_prompt`, and `strict_device_enforcement`.
- Added `ValidateFeatureFlags` with checks for metadata, copy key presence/patterns, valid/complete exposure partitioning, redaction-ready diagnostics semantics, and strict-device server-only rollout behavior.
- Added JSON unmarshal protection so the required boolean flag fields cannot silently disappear into Go zero values.

## Verification

- `go version`: blocked; Go toolchain is not installed in this environment (`go` command not found).
- `go test ./internal/codexplus/configregistry`: blocked for the same reason (`go` command not found).
- `gofmt -w ...`: blocked; Go formatter is not installed in this environment (`gofmt` command not found).
- Static file existence check: `rg --files sub2api-main/backend/internal/codexplus/configregistry` confirms `feature_flags.go` and `feature_flags_test.go` exist.
- Static self-check: `rg` confirmed FeatureFlags symbols, tests, `copy_keys`, `exposure`, `diagnostic_export`, and `strict_device_enforcement` coverage in the owned files.

## Downstream assumptions

- Main/coordinator integration will wire this registry into the shared config service; this worker intentionally did not modify `sub2api-main/backend/internal/service/codexplus_config_service.go`.
- `diagnostic_export=true` requires a caller to set `FeatureFlags.Semantics.DiagnosticExportRedactionReady=true` only after the diagnostic export path is proven redaction-ready.
- `strict_device_enforcement` remains a server/gateway rollout switch. Gateway enforcement must still read UsagePolicy device rules for limits, revocation behavior, replacement cooldowns, and message keys.
- If 02-backend-client-api mirrors `strict_device_enforcement` in bootstrap, it should expose it as read-only status, not as a client-side bypass or local permission decision.

## Remaining risks

- Targeted Go tests could not be executed because the Go toolchain is unavailable in this workspace.
- Manual formatting was applied, but `gofmt` could not be run.
- Cross-document integration with PlanCatalog, ModelCatalog, UsagePolicy, and the existing CodexPlus config service remains for the coordinator/main session.
