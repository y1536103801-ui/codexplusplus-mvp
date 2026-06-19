# Codex++ Implementation Status

本文档记录从计划进入实现后的当前并行状态。它用于避免多会话互相覆盖，不替代各模块 final report。

## 2026-06-19 Windows-Only MVP Completion Status

- Final release run stamp: `20260619-1940`.
- Final recommendation: `go with accepted risks` for the owner-approved Windows-only local MVP gate.
- Final handoff package: `codex-plus-dev-plan/test-runs/20260619-1940-release`.
- Evidence lanes: E2E `20260619-1940-e2e`, package `20260619-1940-package`, compatibility `20260619-1940-compatibility`, docs `20260619-1940-docs`, business readiness `20260619-1940-business`.
- Command results: static gate passed, stage gate passed, E2E evidence passed, Windows-only package evidence passed, compatibility evidence passed, docs/product-copy evidence passed, business readiness passed, aggregate release evidence passed, release coverage complete, release readiness go-candidate generated with owner authorization, Module J report verifier passed, and release handoff verifier passed.
- Scope boundary: this is not a production launch approval. Production release, public paid traffic, real user profile mutation, and macOS package evidence remain outside this Windows-only local MVP gate.
- Safety boundary: final evidence secret scan reported no secret-pattern hits, and the real user profile scan found no `Codex++ Cloud` managed provider markers after isolated-profile Desktop Manager/Codex testing.

## 2026-06-18 Local Runtime And Evidence Tooling Update

- Local current-source Sub2API compose stack is running at `http://127.0.0.1:8081`; `/` and `/health` return HTTP 200, and the app/postgres/redis containers are healthy.
- Historical status from 2026-06-18: release readiness was still not `go`; real E2E, package install, compatibility migration, business readiness and Module J handoff evidence were still required at that time.
- Added the 07 admin audit runner path to close the `09-usage-events-audit.md` gap with read-only, opt-in admin event correlation. It requires `-AllowAdminAuditReads`, numeric test user IDs, gateway-matched `request_id`, `config_version`, `gateway_policy_rejected`, `usage_recorded`, `GATEWAY_POLICY_*` and `redaction_applied` signals.
- Hardened the gateway policy evidence path so fixture/real rows must preserve safe `request_id`, `GATEWAY_POLICY_*` and service-status fields instead of treating any non-2xx as sufficient rejection evidence.
- Bound `report-07-release-gaps.ps1` into 07 static/stage checks and evidence tooling self-test as a read-only helper for open release evidence lanes.

## 2026-06-18 Module J Final Status

- Final release run stamp: `20260618-2124-release`.
- Final recommendation: no-go.
- 07 status: blocked by missing release evidence and approvals.
- Release package: `codex-plus-dev-plan/test-runs/20260618-2124-release`.
- Evidence used: E2E `20260618-2103-e2e`, package `20260618-2103-package`, compatibility `20260618-2108-compatibility`, docs `20260618-1403-docs`, business `20260618-2110-business`.
- Command results: aggregate release evidence exit 1; coverage summary exit 1 with incomplete coverage; readiness summary exit 1 with no-go posture; Module J report verifier exit 0; release handoff verifier exit 1.
- Docs product-copy evidence passes, but it does not replace real E2E, platform package/install, compatibility runtime, or business owner approval evidence.
- Minimum remaining MVP path: pass real E2E Level 3, produce Windows/macOS package artifacts and install checks, provide real provider compatibility snapshots/runtime proof, obtain owner/legal/payment/security/support/cost approvals, then rerun Module J on a same-stamp final evidence set.

## Current Correction Round

- Date: 2026-06-17
- User feedback: 前一轮没有真正体现多会话并行，也没有严格按计划文档逐步推进。
- Correction: 当前执行已完成 `00-contract`、`01-backend-config-center`、`02-backend-client-api`、`03-client-cloud-core`、`04-client-user-experience`、`05-admin-operations` 和 `06-commerce-and-enforcement` 阶段门禁，并打开最终 `07-integration-release`。
- Gate ledger: [STAGE-GATE-LEDGER.md](STAGE-GATE-LEDGER.md)
- Passed stages: `00-contract`, `01-backend-config-center`, `02-backend-client-api`, `03-client-cloud-core`, `04-client-user-experience`, `05-admin-operations`, `06-commerce-and-enforcement`
- Active stage: `07-integration-release`
- Blocked stages: none
- Final 2026-06-19 status: `07-integration-release` is passed for the owner-approved Windows-only local MVP gate; production release remains separately gated.

| Session | Scope | Mode | Status |
| --- | --- | --- | --- |
| Main coordinator | 阶段门禁、执行台账、合并复核 | write | active |
| Harvey | `task-client-api-contract.md` / client OpenAPI / client fixtures / worker A final report | write, bounded | completed, gate passed |
| Pauli | `task-admin-config-contract.md` / config schemas / worker B final report | write, bounded | completed, gate passed |
| Bernoulli | `task-status-and-error-model.md` / status-error / events schema / worker C final report | write, bounded | completed, gate passed |
| Singer | `05-admin-operations` plan management panel | write, bounded | completed, gate passed |
| Hilbert | `05-admin-operations` model management panel | write, bounded | completed, gate passed |
| Maxwell | `05-admin-operations` usage policy and feature flags panels | write, bounded | completed, gate passed |
| Bernoulli | `05-admin-operations` user entitlement support view | write, bounded | completed, gate passed |
| Zeno | `06-commerce-and-enforcement` payment entitlement flow | write, bounded | completed, gate passed |
| Hubble | `06-commerce-and-enforcement` gateway policy enforcement | write, bounded | shutdown after landing code; coordinator accepted via tests |
| Gauss | `06-commerce-and-enforcement` device management | write, bounded | completed, gate passed |
| Dewey | `06-commerce-and-enforcement` audit and risk control | write, bounded | completed, gate passed |

`00-contract` 已通过。旧 A/B/C worker 因额度限制未返回 final report；当前 A/B/C 已按 [00-contract/PARALLEL-RESTART-PACK.md](00-contract/PARALLEL-RESTART-PACK.md) 重新启动并完成。coordinator 已运行 stage gate，结果通过；下一步是按阶段启动 `02-backend-client-api` 的同级并行任务。

Worker B coordinator check:

- Pauli final report exists at `00-contract/reports/worker-b-admin-config-final.md`.
- Four config schemas parse with `ConvertFrom-Json`.
- B1-B4 are all answered as `fixed`.
- Full gate was blocked at that point until A/C final reports returned and coordinator running-status residue was cleared.

Worker C coordinator check:

- Bernoulli final report exists at `00-contract/reports/worker-c-status-error-event-final.md`.
- Event schema parses with `ConvertFrom-Json`.
- C1-C4 are all answered as `fixed`.
- Full gate was blocked at that point until Worker A final report returned and the remaining coordinator running-status residue was cleared.

Worker A coordinator check:

- Harvey final report exists at `00-contract/reports/worker-a-client-api-final.md`.
- Client fixtures parse with `ConvertFrom-Json`, including new `usage.available.json`, `devices.registered.json` and `redeem.applied.json`.
- A1-A5 are all answered as `fixed`.
- Coordinator running-status residue has been cleared; the next step is a full stage gate run.

01-backend-config-center restart:

- Added shared additive package skeleton: `sub2api-main/backend/internal/codexplus/configregistry/common.go`.
- Added 01 report folder: `codex-plus-dev-plan/01-backend-config-center/reports/README.md`.
- Spawned four parallel 01 workers with disjoint write scopes:
  - Boyle / P: Plan Catalog.
  - Ohm / M: Model Catalog.
  - Kierkegaard / U: Usage Policy.
  - Rawls / F: Feature Flags.
- Coordinator will not edit worker-owned `plan_catalog*`, `model_catalog*`, `usage_policy*` or `feature_flags*` files while those workers are running.

01 worker U update:

- Kierkegaard returned a final report at `01-backend-config-center/reports/worker-usage-policy-final.md`.
- `usage_policy.go` and `usage_policy_test.go` exist under `configregistry`.
- Static scan found usage policy validation coverage for quotas, limits, overage/expired behavior, device policy and copy keys.
- Go/gofmt are unavailable locally, so syntax formatting and tests remain unproven.

01 worker M update:

- Ohm returned a final report at `01-backend-config-center/reports/worker-model-catalog-final.md`.
- `model_catalog.go` and `model_catalog_test.go` exist under `configregistry`.
- Static scan found model catalog validation coverage for defaults, groups, rollout, quality tier, fallback, deprecation and disabled replacement.
- Go/gofmt are unavailable locally, so syntax formatting and tests remain unproven.

01 worker P update:

- Boyle returned a final report at `01-backend-config-center/reports/worker-plan-catalog-final.md`.
- `plan_catalog.go` and `plan_catalog_test.go` exist under `configregistry`.
- Static scan found plan catalog validation coverage for billing periods, display price, entitlement sources, usage policy link, commerce URLs/copy keys, listed status and plan status.
- Go/gofmt are unavailable locally, so syntax formatting and tests remain unproven.

01 worker F update:

- Rawls returned a final report at `01-backend-config-center/reports/worker-feature-flags-final.md`.
- `feature_flags.go` and `feature_flags_test.go` exist under `configregistry`.
- Static scan found feature flag validation coverage for exposure boundaries, copy keys, diagnostic redaction readiness and strict device server-only semantics.
- Go/gofmt are unavailable locally, so syntax formatting and tests remain unproven.

01 coordinator integration update:

- Closed all four 01 worker sessions after final reports were captured.
- Integrated the additive `configregistry` package into `sub2api-main/backend/internal/service/codexplus_config_service.go`.
- Preserved the existing `service.CodexPlusConfig` boundary used by client, gateway and admin code, while adding fields for backend-controlled price, usage policy linkage, copy keys, feature flag exposure and model governance metadata.
- `DefaultCodexPlusConfig` now composes defaults from `DefaultPlanCatalog`, `DefaultModelCatalog`, `DefaultUsagePolicyCatalog` and `DefaultFeatureFlags`.
- `ValidateCodexPlusConfig` now runs the existing compatibility checks plus the four registry validators.
- Added a coordinator default-reference alignment step so independently valid worker defaults produce a coherent combined config snapshot.
- Added targeted Go tests in `codexplus_config_service_test.go` for registry-backed defaults, cross-catalog default references, Plan Catalog registry validation and Feature Flag exposure validation.
- At that point, upgraded `tools/validate-stage-gate.ps1` to support `current`, `00-contract` and `01-backend-config-center`; later updates expanded it through the active `07-integration-release` gate.
- Added `tools/verify-01-static.ps1` for offline checks covering registry default composition, validator handoff, `display_price` validation, draft status lifecycle and downstream risk registration.
- Added `tools/verify-01-go.ps1` for the required Go/gofmt compile gate in CI or a prepared local environment.
- Added `01-backend-config-center/reports/coordinator-integration-static-gate.md`.
- `powershell -ExecutionPolicy Bypass -File codex-plus-dev-plan\tools\validate-stage-gate.ps1 -Stage 01-backend-config-center` passed.
- `powershell -ExecutionPolicy Bypass -File codex-plus-dev-plan\tools\validate-stage-gate.ps1` passed and resolved the current active gate to `01-backend-config-center`; the gate now also requires the targeted service integration tests.
- `powershell -ExecutionPolicy Bypass -File codex-plus-dev-plan\tools\verify-01-static.ps1` passed.
- `powershell -ExecutionPolicy Bypass -File codex-plus-dev-plan\tools\verify-01-go.ps1` passed after local Go was upgraded to 1.26.4.
- Go gate evidence: `gofmt -l` clean and `go test ./internal/codexplus/configregistry ./internal/service -run CodexPlus` passed.
- `01-backend-config-center` is now passed; `02-backend-client-api` has also passed its client API exit gate; `03-client-cloud-core` is active; `04-07` remain blocked.

02 coordinator integration update:

- Client bootstrap, usage, device and redeem APIs are wired through authenticated `/api/v1/client/*` routes.
- Client success responses now expose the contract-compatible envelope fields `code`, `status`, `message`, `reason`, `error_code` and `data`.
- DTOs include the backend-driven contract fields needed by desktop core: `message_key`, `commerce_action`, `action_copy_key`, `balance_summary`, `period_usage`, feature flags, announcements and version policy.
- Client entitlement now resolves the plan from `PlanCatalog.entitlement_sources`, the usage policy from `usage_policy_id`, and visible models from the selected plan model groups; the old `firstPlan` / `firstUsagePolicy` positional fallback was removed.
- Structured client events now include request context and preserve config version when a config snapshot is available.
- Added `CodexPlusClientRedeemer` as a narrow seam so client redeem status mapping can be tested without starting transaction infrastructure.
- Added targeted service tests for bootstrap contract fields and event context, usage contract shape, plan/policy/model selection, device upsert events and redeem status mapping.
- Added `tools/verify-02-static.ps1`, `tools/verify-02-go.ps1` and `02-backend-client-api/reports/coordinator-integration-static-gate.md`.
- `powershell -ExecutionPolicy Bypass -File codex-plus-dev-plan\tools\verify-02-static.ps1` passed.
- `powershell -ExecutionPolicy Bypass -File codex-plus-dev-plan\tools\verify-02-go.ps1` passed.
- `powershell -ExecutionPolicy Bypass -File codex-plus-dev-plan\tools\validate-stage-gate.ps1 -Stage 02-backend-client-api` passed.
- `02-backend-client-api` is now passed; `03-client-cloud-core` is active; `04-07` remain blocked.

03-client-cloud-core coordinator update:

- Desktop Rust bootstrap API types now accept the 02 client API fields `message_key`, `commerce_action`, `action_type`, `action_copy_key`, `announcements`, `force_update_prompt` and `strict_device_enforcement`.
- Usage projection now accepts the 02 usage snapshot keys `balance_display`, `usage_display` and structured `renew_action`, while keeping older mock `display` fallback fields.
- Local runtime state projects backend-driven action fields into entitlement/usage state without adding local price, quota, multiplier or entitlement rules.
- Manager cloud adapter/types now preserve backend-driven action metadata and feature flags when converting core runtime state into the UI bootstrap shape.
- Added Rust-side tests in source for action-field deserialization and local state redaction/projection.
- Fixed local runtime provider key projection so normalized `Codex++ Cloud` profiles expose only `has_api_key=true` without serializing the key text.
- Added `tools/verify-03-static.ps1`, `tools/verify-03-node.ps1`, `tools/verify-03-rust.ps1` and `03-client-cloud-core/reports/coordinator-static-verification.md`.
- `powershell -ExecutionPolicy Bypass -File codex-plus-dev-plan\tools\verify-03-static.ps1` passed.
- `npm ci` completed for `CodexPlusPlus-main/apps/codex-plus-manager`.
- `powershell -ExecutionPolicy Bypass -File codex-plus-dev-plan\tools\verify-03-node.ps1` passed via `npm run check`.
- A local workspace Rust/MinGW toolchain was prepared under `work/` for verification without changing project source or system PATH.
- `powershell -ExecutionPolicy Bypass -File codex-plus-dev-plan\tools\verify-03-rust.ps1` passed: Rust format, `codexplus_cloud`, `relay_config`, and `protocol_proxy` coverage all passed.
- `03-client-cloud-core` is now passed; `04-client-user-experience` is active; `05-07` remain blocked.

04-client-user-experience coordinator update:

- Four same-stage UI workers completed and were integrated: login binding, Cloud home/status/usage, install assistant and new user tutorial.
- Cloud Manager now exposes user-facing login, plan/expiry/balance/usage/default model, launch, diagnostics, local Codex install status, tutorial templates and remote tutorial/announcement copy without hardcoding commercial policy.
- Browser handoff is the primary login route; password login remains folded as compatibility; pending 2FA stays unauthenticated and accepts only six numeric digits.
- Browser-preview fallback now uses fixture state instead of showing Tauri IPC errors, and automatic refreshes no longer display initial blocking toasts.
- Narrow viewport CSS was hardened for the topbar, sidebar, Cloud cards and action buttons; Edge headless `1440x900` and `390x844` screenshots passed visual review.
- `powershell -ExecutionPolicy Bypass -File codex-plus-dev-plan\tools\verify-04-static.ps1` passed.
- `powershell -ExecutionPolicy Bypass -File codex-plus-dev-plan\tools\verify-04-node.ps1` passed via `npm run check`.
- `npm run vite:build` passed for `CodexPlusPlus-main/apps/codex-plus-manager`.
- `04-client-user-experience` is now passed; `05-admin-operations` is active; `06-07` remain blocked.

05-admin-operations coordinator update:

- Four same-stage admin workers completed and were integrated: plan management, model management, usage policy/feature flags and user entitlement support.
- Shared admin API types now expose backend-controlled commercial and enforcement fields instead of relying on narrow UI-only shapes.
- Plan management covers price/currency/period, purchase/renew URLs, model groups, entitlement source mappings, usage policy links and copy keys.
- Model management covers route model, model groups, default guards, lifecycle status, rollout/quality tier, fallback and disabled replacement metadata.
- Usage policy management covers quota, concurrency, RPM/TPM, grace/expired behavior, device policy and backend copy keys; feature flags keep `strict_device_enforcement` server-only.
- User entitlement support view covers user plan/expiry/balance, subscription/API key groups, devices, managed key summary, usage aggregates, recent events and integration status.
- `tools/verify-05-static.ps1` passed.
- `tools/verify-05-node.ps1` passed via `npm run typecheck` and `npm run build`.
- `tools/verify-05-go.ps1` passed after formatting: targeted admin Go tests passed.
- `tools/validate-stage-gate.ps1 -Stage 05-admin-operations` passed.
- `05-admin-operations` is now passed; `06-commerce-and-enforcement` is active; `07-integration-release` remains blocked.

06-commerce-and-enforcement coordinator update:

- Four same-stage commerce/enforcement lanes were completed or accepted by coordinator evidence: payment entitlement, gateway enforcement, device management and audit/risk control.
- Payment fulfillment now resolves Codex++ entitlement from backend Plan Catalog and subscription order state, records idempotent grants, and refreshes expired-grace access.
- Gateway enforcement now evaluates model, plan, usage policy and device decisions in `CodexPlusGatewayPolicyService`, records rejection events and normalizes risk payloads.
- Device management now lets admins list, revoke and restore user devices without allowing client heartbeats to revive revoked or blocked devices.
- Audit/risk helpers now record redacted user/device/request/config-context events and expose query projections for support and gateway rejection analysis.
- `tools/verify-06-static.ps1` passed.
- `tools/verify-06-go.ps1` passed after `gofmt -l` and targeted Go tests.
- `tools/validate-stage-gate.ps1 -Stage 06-commerce-and-enforcement` passed.
- `06-commerce-and-enforcement` is now passed; `07-integration-release` is active.

07-integration-release coordinator update:

- Four same-stage release workers produced final reports: E2E scaffold, compatibility migration scaffold, package install checklist and docs/product copy sync.
- E2E lane produced manual buy/login/bootstrap/provider-write/Codex-launch/gateway-request checklist and evidence folder template; status remains `E2E evidence pending`.
- Compatibility lane produced migration checklist, provider settings evidence template and rollback notes; status remains `compatibility evidence pending`.
- Package lane produced package install checklist, platform evidence template, local command evidence and pre-release blockers; status remains `package evidence pending`.
- Docs lane updated product plan/manual and created user/admin/release-note docs; coordinator then synced `codex-plus-product-spec.html` to remove fixed model/quota/API-key examples and align the architecture block with Control Plane/Data Plane/Client Runtime/Platform Ops wording. The docs/product-copy evidence lane is now backed by `tools/new-07-docs-product-copy-evidence.ps1` and `tools/verify-07-docs-product-copy-evidence.ps1`.
- Read-only HTML drift audit found the demo quota bar still had a fixed `65%` width and mixed display text with CSS width; coordinator fixed it by separating `quotaProgress` from quota display text.
- Local Chromium HTML visual evidence passed for desktop `1440x900` and mobile `390x844` after coordinator fixed mobile hero/lead text clipping. Later in-app Browser verification also passed through a local HTTP preview; direct `file://` navigation remains blocked by URL policy.
- Added `07-integration-release/release-local-verification.md` as coordinator local release verification evidence while keeping the stage recommendation `no-go`.
- Backend full local verification passed: `GOTOOLCHAIN=local go test ./...` from `sub2api-main/backend`.
- Sub2API frontend verification passed: `npm run typecheck` and `npm run build` from `sub2api-main/frontend`.
- Desktop manager local frontend build passed: `npm run vite:build` from `CodexPlusPlus-main/apps/codex-plus-manager`.
- Coordinator reused the workspace-local Rust toolchain and passed targeted desktop checks: `cargo fmt --check -p codex-plus-core`, `cargo test -p codex-plus-core codexplus_cloud`, `relay_config`, and `protocol_proxy`.
- Broader `cargo test --workspace` was attempted but is not passed: default MSVC target lacked `link.exe`; GNU with local `w64devkit` advanced to linking and then hit local disk exhaustion. Generated `target` build artifacts were cleaned afterward.
- `tools/verify-07-static.ps1` passed after checking all four worker final reports, generated 07 artifacts and `DocsEvidenceDir` wiring.
- `tools/validate-stage-gate.ps1 -Stage 07-integration-release` passed with the 07 `DocsEvidenceDir` checks included.
- Current `tools/validate-stage-gate.ps1` resolves to `07-integration-release` and passed.
- 2026-06-18 continuation reran `tools/verify-07-static.ps1` and `tools/validate-stage-gate.ps1 -Stage 07-integration-release`; both passed after package evidence wording was synchronized with the coordinator Rust follow-up.
- 2026-06-18 continuation added and strengthened `tools/verify-07-evidence.ps1`, which validates the final Module I evidence folder shape, key `Result: pass` / `Result: fail` markers, critical E2E scenario coverage, release report shape and obvious text secret leakage without printing matched values. The scaffold template fails as expected; temporary fully redacted 13-file fixtures passed.
- 2026-06-18 continuation added `tools/new-07-evidence-run.ps1`, which creates the timestamped 13-file evidence scaffold. Generated scaffolds intentionally fail verification until TODO placeholders and `Result: pending` markers are replaced with sanitized execution evidence.
- 2026-06-18 continuation added `tools/new-07-package-evidence.ps1` and `tools/verify-07-package-evidence.ps1`, which create and validate timestamped package evidence for Windows, macOS x64, macOS arm64, artifact inspection, and package gate reporting. Generated package scaffolds fail until TODO/pending placeholders are replaced with sanitized platform evidence.
- 2026-06-18 continuation added `tools/inspect-07-package-artifacts.ps1`, which inspects already generated Windows setup and macOS x64/arm64 DMG artifacts for names, SHA256 hashes, expected coverage, high-confidence secret/policy signatures and installer-script credential-write risks without printing matched values. Fixture artifacts pass; missing artifacts fail as expected. This is not a platform install substitute.
- 2026-06-18 continuation hardened `tools/verify-07-package-evidence.ps1` around artifact-inspection output: artifact metadata must be `Result: pass` with Windows/macOS x64/macOS arm64 expected coverage, and artifact inspection must be `Result: pass`, record the `inspect-07-package-artifacts.ps1` command, have clear scanner findings, no shared key, no user credentials, no fixed commercial policy and installer-script credential scan pass. `tools/test-07-evidence-tooling.ps1` now covers `package-metadata-result-fail-fails`, `package-artifact-coverage-missing-fails`, API-key-shaped token artifacts, and fixed commercial policy artifacts.
- 2026-06-18 continuation added `tools/new-07-compatibility-evidence.ps1` and `tools/verify-07-compatibility-evidence.ps1`, which create and validate timestamped compatibility evidence for legacy provider snapshots, managed cloud upgrade, logout boundaries, manual provider switching, provider sync, rollback rehearsal, and compatibility gate reporting. Generated compatibility scaffolds fail until TODO/pending placeholders are replaced with sanitized runtime evidence.
- 2026-06-18 continuation added `tools/inspect-07-compatibility-snapshots.ps1`, which compares pre-upgrade, post-upgrade, logout and rollback provider snapshots for manual provider preservation, managed `Codex++ Cloud` presence, logout token-field absence, rollback preservation and local commercial-policy absence without printing token values. Fixture snapshots pass and missing snapshots fail as expected. This is not desktop runtime compatibility evidence by itself.
- 2026-06-18 continuation hardened `tools/verify-07-compatibility-evidence.ps1` around snapshot-inspection output: snapshot context must be `Result: pass`, all four snapshots must be parsed, missing inputs and parse failures must be none, pre-upgrade evidence must include a manual provider, post-upgrade evidence must preserve manual providers, include managed `Codex++ Cloud` and avoid local commercial-policy writes, logout evidence must keep manual providers and clear token-field scans, rollback evidence must keep manual providers, and the gate report must record `inspect-07-compatibility-snapshots.ps1`. `inspect-07-compatibility-snapshots.ps1` now scans every supplied snapshot for token fields and commercial policy fields. `tools/test-07-evidence-tooling.ps1` now covers `compatibility-context-result-fail-fails`, `compatibility-missing-manual-provider-fails`, token-field snapshots, and commercial-policy snapshots.
- 2026-06-18 continuation added `tools/new-07-business-readiness-evidence.ps1` and `tools/verify-07-business-readiness.ps1`, which create and validate Phase 9 business readiness evidence for production values, business config decisions, server sizing, deployment automation, security, compliance/privacy/legal, observability, cost/abuse emergency stop, paid-user support and human decision ownership. Generated scaffolds fail until owner-approved decisions replace placeholders. The verifier now also scans required source docs (`PRODUCTION-ENVIRONMENT-MATRIX.md` and `BUSINESS-CONFIG-DECISION-TABLE.md`) for unresolved required launch markers, with `tools/test-07-evidence-tooling.ps1` covering both clean source-doc pass fixtures and unresolved source-doc failures.
- 2026-06-18 continuation added `tools/verify-07-release-evidence.ps1`, which runs the E2E, package, compatibility and docs/product-copy `DocsEvidenceDir` evidence verifiers together for Module J release-evidence hygiene. A missing package evidence folder fails as expected; temporary fully redacted E2E/package/compatibility/docs fixtures passed together and were deleted.
- 2026-06-18 continuation added `07-integration-release/reports/module-j-final-report-template.md` and `tools/verify-07-module-j-report.ps1`, which validate the final Module J report metadata, required sections, Module A-I report inputs, merge order, verification command/skipped/unavailable disposition fields, release evidence hygiene fields, conflict file/module/rule/result fields, contract drift/change-review fields, recommendation value, aggregate evidence boundary, coverage summary consistency, named go-policy evidence signals, rollback/risk fields, accepted-risk impact and redaction hygiene. The template fails as expected; a temporary fully redacted final-report fixture passed and was deleted.
- 2026-06-18 continuation added `tools/new-07-release-evidence-set.ps1`, which creates a matched timestamped release evidence workspace containing E2E, package, compatibility, docs/product-copy `DocsEvidenceDir`, business readiness, coverage/readiness summaries and Module J report-draft scaffolds. The generated scaffolds were verified to fail aggregate evidence, business readiness and Module J report verification until real sanitized evidence replaces placeholders.
- 2026-06-18 continuation added `tools/summarize-07-release-coverage.ps1`, which generates a coverage matrix for the required 07 release scenarios across E2E, package, compatibility and docs/product-copy `DocsEvidenceDir` evidence. Generated scaffolds remain incomplete, and an internally consistent sanitized handoff candidate produces complete coverage.
- 2026-06-18 continuation added and strengthened `tools/summarize-07-release-readiness.ps1`, which generates a conservative Module J readiness summary from aggregate evidence verification, coverage summary verification, business readiness verification and nonrelease markers. Generated scaffolds fail with no-go, sanitized fixtures also remain no-go because coverage/nonrelease markers are detected, coverage summaries whose E2E/package/compatibility/`DocsEvidenceDir` inputs do not match the readiness inputs are rejected, and the readiness summary's generated status, `Allow go candidate`, and `Nonrelease markers` fields must agree before any go-candidate posture is considered.
- 2026-06-18 continuation strengthened `tools/verify-07-module-j-report.ps1` with `-CoverageSummaryFile` and `-ReadinessSummaryFile` consistency checks, release evidence hygiene field checks, report-level coverage/business readiness pass checks, readiness coverage verification checks, readiness coverage summary path checks, explicit readiness `Allow go candidate` checks, generated readiness/no-readiness-marker checks, `DocsEvidenceDir` checks, summary path and evidence input consistency checks, Module A-I input and merge-order checks, contract drift checks, named go-policy signal checks, E2E Level 3 machine-checked go-policy input checks, skipped/unavailable disposition checks, conflict resolution field checks and accepted-risk impact checks. A Module J go/go-with-accepted-risks report cannot pass without a complete coverage summary, when coverage is incomplete or has missing/nonrelease markers, without a readiness summary, when readiness summary coverage verification is missing/failed, when readiness did not explicitly allow a go candidate, when the report's recorded summary/evidence paths do not match the generated summaries, when the paired summary is no-go, when generated readiness is missing, when readiness markers remain, when the summary omits passed business readiness, when the report omits business evidence hygiene fields or does not record business readiness verification as passed, when Module A-I inputs or merge order are incomplete, when contract drift is unapproved/pending/unreviewed, when named go-policy signals are missing, when E2E Level 3 pass is absent as machine-checked evidence, when skipped/unavailable disposition fields are missing, when conflict rule/result fields are missing, or when accepted-risk impact is missing.
- 2026-06-18 continuation added and strengthened `tools/verify-07-release-handoff.ps1`, which verifies a timestamped final release handoff workspace by deriving matching E2E/package/compatibility/docs (`DocsEvidenceDir`) and business folders, rerunning aggregate evidence and business readiness verification, regenerating coverage/readiness summaries for consistency, comparing stored and regenerated coverage/readiness input paths and counts, binding the handoff index run stamp to the `*-release` directory name, validating evidence paths field by field, comparing stored/generated readiness `Generated`, `Allow go candidate`, and `Nonrelease markers` values, requiring final verification result fields in the handoff index, comparing the index recommendation with the Module J report, and invoking the Module J final report verifier. Generated handoff scaffolds fail; a final-looking handoff index without verification results fails; stored coverage/readiness summary input mismatches fail; readiness consistency mismatches fail; an internally consistent marker-free synthetic candidate fixture passes for verifier coverage only.
- 2026-06-18 continuation retried in-app Browser HTML visual evidence. Browser runtime connection succeeded, direct navigation to the local `file://` HTML target remained blocked by URL policy, and local HTTP preview at `http://127.0.0.1:8099/codex-plus-product-spec.html` passed in the in-app Browser with desktop/mobile screenshots recorded under `07-integration-release/docs/html-visual-evidence/`.
- 2026-06-18 continuation added and passed `tools/test-07-evidence-tooling.ps1`, which makes the 07 evidence-tooling negative/positive checks, including docs/product-copy `DocsEvidenceDir` coverage, rerunnable instead of only documented from temporary ad hoc fixtures. It now also covers package/compatibility/docs coverage-input mismatches in readiness summaries, package/compatibility/docs/business evidence-input mismatches in Module J reports, and docs/business/final-recommendation mismatches in release handoff indexes. The script now accepts `-SkipHandoff` for faster non-handoff local reruns while preserving the full default handoff coverage path.
- 2026-06-18 continuation added and ran `tools/verify-07-rust-preflight.ps1`; current host is not ready for broader desktop Rust workspace tests because Rust toolchain/linker commands are absent, prior workspace-local toolchains are absent, and free disk space is 9.56GB, below the 20GB threshold. This is an environment blocker, not a Rust test failure.
- 2026-06-18 continuation added `tools/verify-07-e2e-readiness.ps1`, which checks `CODEXPLUS_07_E2E_*` backend/admin URLs, Manager build path, test-account tokens, test device ID and allowed/denied model names before E2E execution without printing token values. Current host fails until real test env is supplied; a temporary sanitized fixture passed and is covered by `tools/test-07-evidence-tooling.ps1`.
- 2026-06-18 continuation added `tools/new-07-e2e-env-template.ps1`, which generates `test-runs/<stamp>-e2e-env/e2e-env.template.ps1` and `e2e-env-checklist.md` for manually filling real E2E URLs, test-account tokens, model names and device ID. Generated templates are execution prep only, not release evidence, and the path is covered by `tools/test-07-evidence-tooling.ps1`.
- 2026-06-18 continuation strengthened `tools/verify-07-e2e-readiness.ps1` with `-EnvFile` and `-EndpointPreflight`. The previous upstream `8080` Docker service passed base backend/admin/gateway health probes, but `/api/v1/client/bootstrap`, `/api/v1/auth/desktop/poll` and `/api/v1/admin/codex-plus/config` returned HTTP 404, while `/v1/responses` returned HTTP 401. This proved the upstream image was reachable locally but was not a complete 07 Codex++ backend; the current workspace source route diagnostic is tracked separately on `127.0.0.1:8081`.
- 2026-06-18 continuation further strengthened `tools/verify-07-e2e-readiness.ps1` with explicit endpoint status allowlists, `-EndpointPreflightOnly` and `-OutputPath`. `test-runs/20260618-1425-e2e-env/8081-local-preflight.md` now records a passing local `8081` route diagnostic, while `full-readiness-placeholder-failure.md` records the expected full-readiness failure until real token/model/persona inputs replace generated placeholders.
- 2026-06-18 continuation fixed the local Docker image build path by allowing frontend-imported legal markdown files into the Docker build context and copying them into the frontend builder stage. `docker build -t sub2api-codexplus-local:20260618 -f sub2api-main/Dockerfile sub2api-main` now passes.
- 2026-06-18 continuation started the local source-built image as `sub2api-codexplus-local` on `http://127.0.0.1:8081` without stopping the existing upstream `8080` container. Health and root HTML return 200; 07 client/admin/desktop/gateway preflight returns expected auth/validation statuses rather than 404. Admin login succeeds, while Codex++ admin config remains behind the administrator compliance acknowledgement flow and was not auto-acknowledged.
- 2026-06-18 continuation added a repeatable isolated local-source runtime path: `sub2api-main/deploy/docker-compose.dev.yml`, `.env.codexplus-local.example`, git-ignored `.env.codexplus-local` / `.codexplus-local/`, and `sub2api-main/tools/e2e/codexplus/start-local-source-service.ps1` for source-build route preflight. This makes the local current-source service reproducible on `127.0.0.1:8081` without replacing the upstream `8080` container.
- 2026-06-18 continuation hardened the local-source runtime path: `.env.codexplus-local.example` now uses valid local-only 64-hex `JWT_SECRET` and `TOTP_ENCRYPTION_KEY` shapes, deploy/E2E READMEs document port conflict and lifecycle commands, and `start-local-source-service.ps1` now confirms the expected probe container and rejects 404, 5xx or connection failures with explicit route status allowlists.
- 2026-06-18 continuation added `sub2api-main/tools/e2e/codexplus/start-local-dev-compose.ps1` as the preferred one-command isolated dev compose entry. It can initialize the ignored local env file, runs `docker-compose.dev.yml` with the fixed `sub2api-codexplus-local` project name, rejects conflicting non-compose containers unless explicitly replaced, verifies compose ownership labels, and delegates 07 route preflight to the existing probe helper. The 07 aggregate evidence templates were also updated to include the now-required `-DocsEvidenceDir` input.
- 2026-06-18 continuation generated and filled `codex-plus-dev-plan/test-runs/20260618-1403-docs`; `verify-07-docs-product-copy-evidence.ps1` passes against that docs/product-copy evidence folder, including final guide/admin/release-notes/HTML copy decisions, PNG dimensions and approved local HTTP browser preview evidence.
- 2026-06-18 continuation added `sub2api-main/tools/e2e/codexplus/run-client-api-checks.ps1` and `run-local-e2e.ps1`; fixture runs passed and are now covered by `tools/test-07-evidence-tooling.ps1`. These runners generate/fill the client API subset evidence only and still require real browser handoff, desktop launch, gateway, package and compatibility execution before release.
- 2026-06-18 continuation added `sub2api-main/tools/e2e/codexplus/run-browser-handoff-checks.ps1`; fixture runs passed and are now covered by `tools/test-07-evidence-tooling.ps1`. Real desktop pending-session creation is gated by `-AllowSessionStart`, browser approval is gated by `-AllowBrowserComplete` plus `CODEXPLUS_07_E2E_BROWSER_AUTH_TOKEN`, and the runner records only redacted status/token-presence evidence so it cannot stand in for real Turnstile/Web login or Manager UI evidence.
- 2026-06-18 continuation added `sub2api-main/tools/e2e/codexplus/run-gateway-policy-checks.ps1`; fixture runs passed and default execution without `-AllowGatewayRequests` fails as expected. Real gateway policy evidence still requires approved low-cost test gateway keys and runtime execution.
- 2026-06-18 continuation strengthened the E2E, package, compatibility and business readiness lane verifiers so structurally complete evidence with final failed results is rejected before aggregate/readiness handoff. `tools/test-07-evidence-tooling.ps1` now covers failed-result negative cases for all four lanes.
- 2026-06-18 continuation corrected package artifact inspection boundary wording so the runner still states artifact inspection is package-hygiene-only without triggering readiness `runtime-evidence-required` markers for an otherwise marker-free synthetic handoff. The marker-free handoff self-test passes again.
- Release recommendation is currently no-go until production-equivalent E2E, platform package, compatibility runtime and business readiness evidence are complete.
- Local visibility status: `http://localhost:8080` currently exposes the base upstream Sub2API service and health endpoint, while the current workspace source is visible at `http://127.0.0.1:8081` via `sub2api-codexplus-local`. Complete release readiness still requires filling non-placeholder E2E environment values and proving browser handoff, client bootstrap, desktop launch, one gateway request, package install, compatibility runtime and business readiness evidence.

Coordinator preaudit:

- Added [00-contract/COORDINATOR-PREAUDIT.md](00-contract/COORDINATOR-PREAUDIT.md).
- The preaudit identifies concrete A/B/C gaps around envelope compatibility, desktop handoff user shape, feature flag alignment, device registration fields, config governance metadata, device policy, operator-controlled copy, event metadata allowlisting, handoff states and client action mapping.
- This preaudit is not an approval; it is required input for the next A/B/C parallel restart.

Gate automation:

- Added [tools/validate-stage-gate.ps1](tools/validate-stage-gate.ps1).
- Added [00-contract/reports/README.md](00-contract/reports/README.md) with fixed final-report file names.
- Added report templates:
  - `00-contract/reports/worker-a-client-api-final.template.md`
  - `00-contract/reports/worker-b-admin-config-final.template.md`
  - `00-contract/reports/worker-c-status-error-event-final.template.md`
- Strengthened `validate-stage-gate.ps1` so final reports must include `Report status: final`, matching `Worker lane`, required sections, `Forbidden edits: none`, no unfinished-marker wording and one explicit `fixed/deferred/rejected` decision per preaudit item.
- Latest pre-approval run: `powershell -ExecutionPolicy Bypass -File codex-plus-dev-plan\tools\validate-stage-gate.ps1`.
- Result: passed.
- All required files, OpenAPI paths, JSON parsing checks, blocked-stage checks, approval-residue checks and A/B/C final report checks passed in that run.

## Historical Prior Round

- Date: 2026-06-16
- Objective: 按 `PARALLEL-DISPATCH-PLAN.md` 并行实现 Phase 1 MVP。
- Rule: 主会话只做协调、Module C 当前基础层和最终集成检查；D/E/F/G/H 由并行 worker 分别实现。
- Status: D/E/F 已返回 final report；G/H 已落地文件但未返回完整 final report。主会话已完成第一轮集成补线，并启动独立并行只读检查会话 A/B 复核后端与桌面端计划符合度。A/B 报告已归档，主会话已修复报告中指出的两个后端硬伤、桌面端设备 header 网关闭环缺口、托管 profile 重启后 key 读取缺口，以及网关 entitlement 默认套餐兜底问题。本轮追加 C/D 只读复核用于检查桌面 auth/2FA 链路和计划文档一致性；C/D 报告已归档。随后追加 E/F 只读复核用于检查生产 Turnstile/browser handoff 缺口；E/F 报告已归档，主会话已落地 browser handoff 初始实现。整体仍按“待 CI/E2E 验证”处理。

Note: 本节是纠偏前的历史记录，不是当前允许继续派发的阶段。当前有效状态以上方 `Current Correction Round` 和 `STAGE-GATE-LEDGER.md` 为准。

## Historical Multi-Session Execution Trace

本轮执行不再只停留在文档分派，已按 `PARALLEL-DISPATCH-PLAN.md` 把剩余验收拆成并行会话：

| Session | Scope | Mode | Status |
| --- | --- | --- | --- |
| Main coordinator | 集成补线、状态文档、最终验收缺口收束 | write | active |
| Parallel A / Bohr | `sub2api-main` 后端 client API、gateway enforcement、admin aggregation 计划符合度 | read-only explorer | completed |
| Parallel B / Kuhn | `CodexPlusPlus-main` 桌面 runtime、provider writer、设备 header 闭环可行性 | read-only explorer | completed |
| Parallel C / Godel | `CodexPlusPlus-main` 桌面 auth/2FA runtime、Tauri、React UI 链路复核 | read-only explorer | completed |
| Parallel D / Locke | `codex-plus-dev-plan` 多会话计划、执行记录、2FA 文档一致性复核 | read-only explorer | completed |
| Parallel E / Volta | `sub2api-main` pending auth / desktop handoff 生产安全复核 | read-only explorer | completed |
| Parallel F / Hooke | `CodexPlusPlus-main` desktop handoff runtime/Tauri/UI 落点评估 | read-only explorer | completed |

本轮主会话只修改状态/集成文档和必要的小型集成补线；并行检查会话不写文件，避免与主会话产生冲突。

## Active Module Ownership

| Module | Owner | Write scope |
| --- | --- | --- |
| C Backend Foundation | main coordinator session | `sub2api-main/backend/ent/schema/codexplus_*.go`, `migrations/151_codexplus_foundation.sql`, `internal/service/codexplus_*`, `internal/repository/codexplus_*`, provider-set registration |
| D Client API | parallel worker Erdos | `/api/v1/client/*` handler/service/DTO/routes only |
| E Gateway Enforcement | parallel worker Meitner | additive `codexplus_gateway_policy_service*` only |
| F Desktop Runtime | parallel worker Hilbert | `CodexPlusPlus-main/crates/codex-plus-core/src/codexplus_cloud/**`, Tauri cloud commands |
| G Desktop UX | parallel worker Noether | `CodexPlusPlus-main/apps/codex-plus-manager/src/cloud/**`, minimal route wiring |
| H Admin Operations | parallel worker Sagan | Codex++ admin facade/page only |

## Main Session Module C Changes

Implemented in the current workspace:

- Added Ent schema declarations:
  - `sub2api-main/backend/ent/schema/codexplus_device.go`
  - `sub2api-main/backend/ent/schema/codexplus_managed_provider_key.go`
  - `sub2api-main/backend/ent/schema/codexplus_event.go`
- Added additive migration:
  - `sub2api-main/backend/migrations/151_codexplus_foundation.sql`
- Added config service:
  - `sub2api-main/backend/internal/service/codexplus_config_service.go`
  - `sub2api-main/backend/internal/service/codexplus_config_service_test.go`
- Added foundation interfaces and SQL repositories:
  - `sub2api-main/backend/internal/service/codexplus_foundation.go`
  - `sub2api-main/backend/internal/repository/codexplus_foundation_repo.go`
- Integrated worker interface seams:
  - Added `CodexPlusManagedProviderKeyRepository.GetByAPIKeyID` for Module E gateway policy lookup.
  - Added `CodexPlusDeviceRepository.ListByUser` and `CodexPlusEventRepository.ListByUser` for admin entitlement drill-down.
  - Updated Module D client service to reuse Module C `CodexPlusDevice` and `CodexPlusManagedProviderKey` foundation types instead of duplicate client-only records.
  - Updated desktop runtime envelope parsing to tolerate the existing backend `{code,message,data}` envelope while the final client contract is still being frozen.
  - Updated desktop runtime state to expose backend-driven feature flags/models/version summary from bootstrap snapshots without exposing raw provider keys.
  - Updated desktop UX command adapter to unwrap Tauri `CommandResult` and pass camelCase command arguments.
  - Wired `/api/v1/client/*` into the central backend handler/router graph through `handler.Handlers.Client`.
  - Wired Codex++ client service dependencies through `service.ProvideCodexPlusClientService`, including device store, managed provider key store, and event sink.
- Wired Codex++ admin service to real device/event repositories instead of empty stub lists.
- Wired Codex++ gateway policy service into the main production gateway paths after model parsing and before concurrency/upstream routing.
- Fixed Codex++ foundation repository defects found by parallel verification:
  - `codexplus_events` list queries now match the append-only schema and no longer reference a missing `deleted_at` column.
  - device upsert now preserves server-side `revoked` / `blocked` state when the desktop sends an `active` heartbeat.
- Added repository regression tests for the event schema query and revoked-device upsert behavior.
- Tightened backend-driven entitlement and device enforcement:
  - `CodexPlusPlan` now includes `entitlement_sources` for subscription group, API key group and group-name mapping.
  - Gateway policy no longer grants the first enabled plan as an implicit default; unmanaged or unmapped Codex++ managed keys fail closed as not purchased.
  - `FeatureFlags.strict_device_enforcement` can require device context at the gateway without a client release.
  - Added service and handler-helper tests for API key group mapping, subscription group mapping, unmapped entitlement rejection, and config-driven strict device enforcement.
  - Corrected the subscription group-object fallback so it reads `subscription_group_ids`, not `api_key_group_ids`.
  - Updated config schema contracts and migration seed JSON so `entitlement_sources` and `strict_device_enforcement` are represented consistently across code, default data and docs.
  - Added `strict_device_enforcement` to Codex++ admin options and updated admin task docs for entitlement source mapping and strict device management.
- Registered providers:
  - `sub2api-main/backend/internal/service/wire.go`
  - `sub2api-main/backend/internal/repository/wire.go`
  - `sub2api-main/backend/internal/handler/wire.go`
  - `sub2api-main/backend/cmd/server/wire_gen.go` was manually synchronized because the local Go/Wire toolchain is unavailable.

## Parallel Worker Results Captured

### Module D Client API - Erdos

- Added local `/api/v1/client/*` handler/DTO/routes/service skeleton for bootstrap, usage, devices, redeem.
- Handler reads user identity from JWT auth subject; it does not accept request `user_id`.
- Service only returns the full managed gateway key for allowed service states.
- Main session has now registered the client handler in `handler.Handlers`, `handler.ProviderSet`, `cmd/server/wire_gen.go`, and `server/router.go`.

### Module E Gateway Enforcement - Meitner

- Added `CodexPlusGatewayPolicyService` with managed key recognition, model entitlement checks, device checks, billing error mapping, and redacted rejection event payloads.
- Main session integrated the missing `GetByAPIKeyID` repository seam.
- Main session wired gateway hot-path invocation into Claude/OpenAI/Gemini compatible handlers through a shared handler helper.
- The integration now derives plan/model entitlement from backend-configured entitlement sources instead of a default plan fallback. Device revocation is enforced when a device header is present, and `FeatureFlags.strict_device_enforcement` can make device context mandatory.

### Module F Desktop Runtime - Hilbert

- Added `codexplus_cloud` Rust runtime, local session/device/snapshot storage, mock fixture mode, managed provider writer, redaction utilities, and Tauri cloud commands.
- Main session added compatibility for the existing backend response envelope and exposed backend feature flags/models/version summaries to the UI adapter.
- Main session changed `Codex++ Cloud` provider writing so Codex reads a local helper `base_url`, while the helper forwards to the backend gateway and injects `X-CodexPlus-Device-Id` from local device state.
- Added desktop regression tests for local-helper provider routing, Responses URL normalization, and helper startup for the `Codex++ Cloud` provider.
- Added `relay_config` protection for `Codex++ Cloud` so storage normalization keeps the real backend gateway URL in `base_url` / `upstream_base_url` while the generated Codex `config.toml` continues to point at the local helper.
- Added proxy auth recovery so the local helper uses `auth_contents.OPENAI_API_KEY` when the serialized profile omits the plain `api_key` field, covering app restart behavior.
- Desktop login now calls the real Sub2API `/api/v1/auth/login` route instead of a non-existent `/api/v1/client/login`, parses the nested `user` object from `AuthResponse`, stores a pending 2FA session when `requires_2fa=true`, and completes the same login flow through `/api/v1/auth/login/2fa`.
- The 2FA runtime/UI change is recorded as a coordinator integration patch across Module F and G, because it closes an auth contract mismatch found during integration rather than introducing a new independently owned feature area.

### Module G Desktop UX - Noether

- Files observed in `CodexPlusPlus-main/apps/codex-plus-manager/src/cloud/**` and `App.tsx`.
- Adds Cloud home screen, login/binding panel, usage/model panel, install assistant, tutorial, diagnostics, and route entry.
- Final worker report has not returned yet.
- Main session corrected the command adapter so it unwraps Tauri `CommandResult` and does not assume raw `CloudRuntimeState`.
- Main session added the 2FA completion control to the login/binding panel. The UI only reflects `connection.pendingTwoFactor` from runtime state and does not hardcode entitlement, pricing, model, or quota behavior.
- Godel review confirmed the desktop `/auth/login` -> pending 2FA -> `/auth/login/2fa` chain is connected and pending sessions are not treated as authenticated. The review also identified the production Turnstile blocker in the password-login path; the follow-up Volta/Hooke handoff reviews led to the browser handoff initial implementation below.
- The 2FA input now accepts only six numeric digits before calling the backend.
- Browser handoff has been added as the production-oriented default path:
  - backend exposes `/api/v1/auth/desktop/start`, `/api/v1/auth/desktop/poll`, and authenticated `/api/v1/auth/desktop/complete`;
  - web frontend adds `/auth/desktop/authorize` so the logged-in browser session performs Turnstile/Web-login-backed approval;
  - desktop runtime stores a pending handoff session with a private `poll_token`, exposes only the authorize URL and verification code to UI, and polls until the backend returns a completed token pair;
  - Manager UI now makes browser login the primary action and folds password login/service address into a compatibility section.
- Contract catch-up for browser handoff has been completed:
  - `codex-plus-contracts/api/client-openapi.yaml` now freezes `/api/v1/auth/desktop/start`, `/complete`, and `/poll`;
  - `codex-plus-contracts/test-fixtures/client/desktop-handoff.*.json` covers start, complete, pending poll, and completed poll;
  - `codex-plus-contracts/status-error/client-status-errors.md` and `events/client-events.schema.json` include desktop handoff errors/events;
  - `INTEGRATION-VERIFICATION-CHECKLIST.md` now gates Turnstile-safe browser handoff and `poll_token` non-disclosure.
- Backend handoff events now reuse `codexplus_events` instead of creating a parallel audit path:
  - `desktop_login_started` records pending handoff creation without `session_token` or `poll_token`;
  - `desktop_login_completed` records browser approval with user/device/request context and no desktop token;
  - `desktop_login_polled` records terminal desktop redemption with token issuance indicated as a boolean only.
- Desktop redaction now treats JSON `poll_token`/`session_token` and URL `poll_token` as sensitive. The runtime UI still receives the browser authorize URL needed to open the Web flow, while `poll_token` remains local-only.

### Module H Admin Operations - Sagan

- Files observed in `sub2api-main/backend/internal/service/codexplus_admin_service.go`, `handler/admin/codexplus_handler.go`, and `server/routes/codexplus_admin.go`.
- Adds admin config get/validate/publish/version/rollback/options and user entitlement facade.
- Final worker report has not returned yet.
- Main session has replaced the empty device/event stub behavior with repository-backed reads and integrated those repositories into admin service construction.

## Validation So Far

- `migrations.go` embeds `*.sql`; new migration will be included by the existing embed rule.
- Seeded `codexplus_config_v1` JSON was extracted from the migration and parsed successfully (`plans=1`, `models=1`, `policies=1`).
- Braces/parentheses/square-bracket balance checks passed for representative newly added Go/Rust/TS files.
- Brace balance checks passed for the current integration edits:
  - `sub2api-main/backend/cmd/server/wire_gen.go`
  - `sub2api-main/backend/internal/handler/handler.go`
  - `sub2api-main/backend/internal/handler/wire.go`
  - `sub2api-main/backend/internal/server/router.go`
  - `sub2api-main/backend/internal/service/wire.go`
  - `sub2api-main/backend/internal/service/codexplus_admin_service.go`
  - `sub2api-main/backend/internal/service/codexplus_foundation.go`
  - `sub2api-main/backend/internal/repository/codexplus_foundation_repo.go`
- Targeted scans over the current Codex++ backend integration files found no unfinished-marker wording or unsafe abort markers.
- Added handler-level unit test coverage for the shared gateway policy helper:
  - unmanaged API Key skips Codex++ policy.
  - managed Codex++ key rejects unauthorized model.
  - `X-CodexPlus-Device-Id` is passed through and revoked devices reject.
- Route/wire searches confirm:
  - `RegisterClientRoutes` is called from the central router.
  - `clienthandler.NewClientHandler` is constructed in both handler provider set and generated server wire.
  - `CodexPlusAdminService` is constructed with device/event repositories.
  - Repository provider set includes Codex++ device, managed provider key, and event repositories.
  - `CodexPlusGatewayPolicyService` is created in service provider wiring and injected into both Claude-compatible and OpenAI-compatible gateway handlers.
  - Gateway policy calls are present in messages, count_tokens, responses, chat completions, embeddings, images, Gemini native, and OpenAI Responses WebSocket initial request paths.
- Sensitive sample scan over the current Codex++ new-file scope has no long `sk-*`, `Authorization: Bearer`, or `api_key.*sk-` matches.
- `npm.cmd run check` was attempted for `CodexPlusPlus-main/apps/codex-plus-manager`, but local `node_modules/typescript/bin/tsc` is missing.
- Browser handoff contract validation:
  - all `codex-plus-contracts/**/*.json` schemas and fixtures parse with PowerShell `ConvertFrom-Json`;
  - targeted scan found no legacy `DESKTOP_LOGIN_*` custom error codes in the new handoff handler or contract files;
  - targeted scan found no `approved_user_email` or `browser_user` remnants in the desktop handoff handler;
  - constructor scan confirms `AuthHandler` receives the shared `CodexPlusEventRepository` in `wire_gen.go` and direct tests use the updated signature;
  - Node delimiter scan passed for the handoff handler, AuthHandler, generated wire file, Rust cloud files, and updated contract files.
- `npm.cmd run check` was retried after the auth endpoint change and still fails because `tsc` is not installed/resolvable in the local environment.
- Static scans now confirm the desktop auth flow references `/api/v1/auth/login` and `/api/v1/auth/login/2fa`; no `/api/v1/client/login` call path remains in the implementation.
- Parallel C/D/E/F read-only reviews have completed. C found no obvious command/type disconnect by static reading, but flagged the Turnstile production-login blocker. D found documentation drift around current C/D status, web login wording, and sequential gates. E/F specified the browser handoff shape. The documents and implementation were updated accordingly.

## Current Tooling Blockers

- Go 1.26.4 is now installed locally; the 01 Go compile gate has passed.
- Rust/Cargo were prepared in the workspace for 03 verification; `verify-03-rust.ps1` has passed.
- `npm ci` has been run for `CodexPlusPlus-main/apps/codex-plus-manager`; `verify-03-node.ps1` has passed via `npm run check`.
- Broader frontend/backend/release checks still belong to later stage gates and CI.

Because of this, full-product compile/test status is still not the same as 03 completion. Later release gates must still run broader Go/Wire/Rust/Node checks in a prepared environment or CI before marking the whole implementation complete.

## Current Integration Gaps

- Backend response envelope still differs from the document contract; desktop runtime currently supports both as a temporary bridge.
- Desktop password login still uses the existing `/api/v1/auth/login` contract and completes TOTP-style 2FA through `/api/v1/auth/login/2fa`, but it is now a compatibility path. Browser handoff has been implemented for Turnstile-enabled production login and still needs real E2E verification before release.
- CI must regenerate Wire and run full Go/Rust/Node checks in a prepared environment before this implementation can be marked production-ready.
- Gateway policy, payment entitlement, device management and audit/risk now have targeted Go test evidence; broader protocol-level and release checks still belong to 07.
- Desktop runtime sends `X-CodexPlus-Device-Id` to client API calls and now routes `Codex++ Cloud` Codex main gateway traffic through the local helper so the helper can inject the same device header. The provider storage path preserves the real backend gateway URL separately from the local helper URL, and proxy auth can recover from `auth_contents`. Browser handoff now avoids relying on Turnstile-disabled desktop password login. These still need real browser login, Codex launch and gateway-log verification before strict device enforcement is enabled by default.
- 2026-06-18 07 tooling update: compatibility snapshot inspection now parses legacy `settings.relayProfiles` / root relay settings, compares manual provider URL/API-key fingerprints without printing values, scans snake_case and camelCase token fields, and writes only a snapshot subset pass. `verify-07-compatibility-evidence.ps1` now rejects snapshot-only evidence until runtime login/logout, manual provider request, provider sync log review, rollback rehearsal, and `Runtime compatibility result: pass` evidence are present. `report-07-release-gaps.ps1` reports missing sibling evidence directories and key files for a run stamp.
