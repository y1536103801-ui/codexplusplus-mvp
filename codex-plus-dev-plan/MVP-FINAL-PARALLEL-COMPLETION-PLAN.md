# Codex++ MVP Final Parallel Completion Plan

本文档用于从当前 `07-integration-release` 停机点继续推进到 MVP 完成。它不是新的产品范围文档，而是给后续多个 Codex 会话并行收口用的执行调度文档。

## Current Baseline

- Workspace: `F:\codex++\codex+++(2)\codex+++\`
- Current active stage: `07-integration-release`
- Passed stages recorded in `STAGE-GATE-LEDGER.md`: `00-contract` through `06-commerce-and-enforcement`
- Release posture: `no-go`
- Current completion judgment:
  - Core product implementation: mostly landed.
  - Final MVP release evidence: incomplete.
  - Current blockers are release validation, real environment evidence, packaging evidence, compatibility evidence, business readiness and Module J handoff.

Important current facts from local inspection:

- Root workspace is not a git repository; treat subfolders as a composite workspace.
- `report-07-release-gaps.ps1` runs and reports only `test-runs/20260618-1403-docs` exists for the current release evidence set.
- Missing same-stamp evidence lanes: `e2e`, `package`, `compatibility`, `business`, `release`.
- `validate-stage-gate.ps1 -Stage 07-integration-release` currently has a PowerShell parse failure.
- `verify-07-docs-product-copy-evidence.ps1` currently has a PowerShell parse failure near the evidence scan block.
- `verify-07-static.ps1` runs but currently fails on 07 task metadata sections and one README boundary phrase.
- Docker is not currently running on the inspected host; `8080` and `8081` are not currently reachable.
- `go`, `rustc`, `cargo` and global `pnpm` are not on PATH; `node`, `npm` and `corepack pnpm` are available.

## 2026-06-19 MVP Scope Update

Owner decision: MVP package scope is Windows x64 only. macOS x64 and macOS arm64 package artifacts, install evidence, Gatekeeper behavior, and notarization/unsigned acceptance decisions are deferred post-MVP.

This scope update only changes the package platform target for MVP. It does not waive Windows package evidence, E2E, compatibility, docs, business readiness, Module J, or release go/no-go gates. Full cross-platform release still requires macOS package evidence.

## MVP Completion Definition

MVP is complete only when all of these are true:

- `07` tooling and stage gates parse and run.
- Local or CI backend/frontend/desktop verification has a current pass/fail record.
- One timestamped release evidence set exists with sibling folders:
  - `YYYYMMDD-HHMM-e2e`
  - `YYYYMMDD-HHMM-package`
  - `YYYYMMDD-HHMM-compatibility`
  - `YYYYMMDD-HHMM-docs`
  - `YYYYMMDD-HHMM-business`
  - `YYYYMMDD-HHMM-release`
- Real E2E evidence proves purchase or entitlement, browser handoff login, bootstrap, managed provider write, Codex launch and one gateway request.
- Real negative-path evidence covers not purchased, expired, low balance, revoked device, denied model and gateway or service unhealthy behavior.
- Package evidence covers Windows x64 artifact metadata and Windows install evidence. macOS x64/arm64 package evidence is deferred post-MVP by the 2026-06-19 owner decision.
- Compatibility evidence proves old manual providers are preserved through Cloud upgrade, logout, provider sync, manual switching and rollback.
- Business readiness evidence is owner-approved for production env values, launch decisions, legal/privacy/payment terms, observability, cost/abuse controls and paid-user support.
- Module J final report passes its verifier and gives a clear `go`, `go with accepted risks` or `no-go` recommendation.

## Non Goals

- Do not add new product features while closing MVP.
- Do not weaken release gates to make the project appear complete.
- Do not replace real E2E, package or compatibility evidence with scaffold fixtures.
- Do not run production payment, customer-impacting gateway requests, credential mutation or public release actions without explicit user approval.
- Do not print secrets, tokens, API keys, JWTs or full provider URLs in reports.
- Do not use destructive git commands.

## Global Dispatch Rules

- Use one coordinator session plus bounded worker sessions.
- Workers may run in parallel only when their write scopes do not overlap.
- Coordinator owns `IMPLEMENTATION-STATUS.md`, `STAGE-GATE-LEDGER.md`, `MULTI-SESSION-EXECUTION-TRACE.md` and final release handoff files.
- Tooling workers own only their assigned scripts and local documentation references.
- Evidence workers own only their evidence lane folders and lane-specific helper scripts.
- If a worker needs to edit outside its scope, it must stop and report the required file path and reason.
- All generated evidence must be sanitized and must not include secrets.
- Every final report must distinguish:
  - executed evidence
  - scaffold or fixture evidence
  - unavailable environment checks
  - accepted risks

## Phase 0: Serial Unblock Before Broad Parallel Work

Run this first with a single owner. Do not dispatch final evidence workers until these scripts parse.

### Worker 0A: 07 Gate And Script Repair

Goal:

- Restore the 07 verification toolchain so future workers can trust the gates.

Write scope:

- `codex-plus-dev-plan/tools/validate-stage-gate.ps1`
- `codex-plus-dev-plan/tools/verify-07-docs-product-copy-evidence.ps1`
- `codex-plus-dev-plan/tools/verify-07-static.ps1` only if its checks are wrong or stale
- `codex-plus-dev-plan/07-integration-release/task-e2e-buy-login-launch.md`
- `codex-plus-dev-plan/07-integration-release/task-compatibility-and-migration.md`
- `codex-plus-dev-plan/07-integration-release/task-package-install-check.md`
- `codex-plus-dev-plan/07-integration-release/task-docs-and-product-copy.md`
- `codex-plus-dev-plan/07-integration-release/README.md`
- `codex-plus-dev-plan/07-integration-release/reports/coordinator-integration-release-verification.md`

Required fixes:

- Fix `validate-stage-gate.ps1` parse errors without deleting existing checks.
- Fix `verify-07-docs-product-copy-evidence.ps1` parse errors near the scan/leak-rule block.
- Make `verify-07-static.ps1` pass for legitimate current 07 state.
- Add required sections to each 07 task file:
  - `## 解耦要求`
  - `## 禁止改动范围`
  - `## 测试要求`
  - `## 交付物`
- Add the README boundary wording required by the verifier, including `business/legal approval`.

Verification:

```powershell
powershell -ExecutionPolicy Bypass -File codex-plus-dev-plan/tools/verify-07-static.ps1
powershell -ExecutionPolicy Bypass -File codex-plus-dev-plan/tools/validate-stage-gate.ps1 -Stage 07-integration-release
powershell -ExecutionPolicy Bypass -File codex-plus-dev-plan/tools/verify-07-docs-product-copy-evidence.ps1 -EvidenceDir codex-plus-dev-plan/test-runs/20260618-1403-docs
```

Exit criteria:

- All three commands above run without PowerShell parser errors.
- Any remaining failed checks are real evidence gaps, not broken scripts or stale task metadata.
- Coordinator report records the repair and keeps release recommendation `no-go`.

## Phase 1: Parallel Readiness Work

After Phase 0 passes, run these workers in parallel.

### Worker 1A: Local Source Runtime And Toolchain Readiness

Goal:

- Make the current workspace source bootable and verifiable on this host or document exact external CI/toolchain requirements.

Write scope:

- `sub2api-main/deploy/.env.codexplus-local.example`
- `sub2api-main/deploy/docker-compose.dev.yml`
- `sub2api-main/tools/e2e/codexplus/start-local-source-service.ps1`
- `sub2api-main/tools/e2e/codexplus/start-local-dev-compose.ps1`
- `sub2api-main/tools/e2e/codexplus/README.md`
- `codex-plus-dev-plan/test-runs/YYYYMMDD-HHMM-e2e-env/*`
- `codex-plus-dev-plan/07-integration-release/release-local-verification.md`

Do not edit:

- Backend business logic.
- Desktop runtime code.
- Admin UI code.
- Final release evidence folders except the env/readiness folder assigned to this worker.

Tasks:

- Start Docker Desktop or document that Docker is unavailable.
- Use `sub2api-main/tools/e2e/codexplus/start-local-source-service.ps1` or the documented compose command to boot local source on `127.0.0.1:8081`.
- Confirm `/` and `/health` respond.
- Confirm Codex++ 07 client/admin/desktop routes return expected auth or validation responses, not 404.
- Check current toolchain:
  - `go version`
  - `node --version`
  - `npm --version`
  - `corepack pnpm --version`
  - `rustc --version`
  - `cargo --version`
  - Docker status
- If Go/Rust are unavailable, document whether verification must run in CI or with a local bundled toolchain.

Verification:

```powershell
powershell -ExecutionPolicy Bypass -File codex-plus-dev-plan/tools/verify-07-e2e-readiness.ps1 -EndpointPreflightOnly -OutputPath codex-plus-dev-plan/test-runs/YYYYMMDD-HHMM-e2e-env/local-route-preflight.md
powershell -ExecutionPolicy Bypass -File codex-plus-dev-plan/tools/verify-07-rust-preflight.ps1
```

Exit criteria:

- A current route preflight record exists.
- Toolchain gaps are exact and actionable.
- No secret values are printed.

### Worker 1B: Docs Product Copy Evidence Repair

Goal:

- Revalidate the existing docs evidence folder after Phase 0 script repair and prepare it for final same-stamp release aggregation.

Write scope:

- `codex-plus-dev-plan/test-runs/YYYYMMDD-HHMM-docs/*`
- `codex-plus-dev-plan/07-integration-release/docs/*`
- `codex-plus-product-spec.html`
- `codex-plus-dev-plan/07-integration-release/reports/worker-docs-product-copy-final.md`

Do not edit:

- E2E/package/compatibility/business/release folders.
- Backend or desktop source code.

Tasks:

- Choose the final run stamp with coordinator.
- If reusing `20260618-1403-docs`, copy or regenerate equivalent docs evidence under the final run stamp.
- Ensure user guide, admin guide, release notes and HTML evidence contain no stale fixed model/quota/API key examples.
- Ensure visual evidence remains linked and documented.
- Keep direct `file://` in-app Browser policy limitation explicit.

Verification:

```powershell
powershell -ExecutionPolicy Bypass -File codex-plus-dev-plan/tools/verify-07-docs-product-copy-evidence.ps1 -EvidenceDir codex-plus-dev-plan/test-runs/YYYYMMDD-HHMM-docs
```

Exit criteria:

- Docs product copy evidence verifier passes on the final run stamp.
- The docs lane does not claim E2E/package/compatibility/business completion.

## Phase 2: Parallel Final Evidence Lanes

After Phase 0 passes, and after Worker 1A provides a usable environment or CI route, run these workers in parallel where resources allow.

### Worker 2A: Real E2E Evidence

Goal:

- Produce real Module I E2E evidence for the MVP happy path and required negative paths.

Write scope:

- `codex-plus-dev-plan/test-runs/YYYYMMDD-HHMM-e2e/*`
- `sub2api-main/tools/e2e/codexplus/run-client-api-checks.ps1`
- `sub2api-main/tools/e2e/codexplus/run-browser-handoff-checks.ps1`
- `sub2api-main/tools/e2e/codexplus/run-gateway-policy-checks.ps1`
- `sub2api-main/tools/e2e/codexplus/run-admin-audit-checks.ps1`
- `sub2api-main/tools/e2e/codexplus/run-local-e2e.ps1`
- `sub2api-main/tools/e2e/codexplus/README.md`

Do not edit:

- Core backend business logic unless a verified defect blocks the E2E lane and coordinator approves a narrow fix.
- Desktop UI/runtime code unless assigned by coordinator.
- Package/compatibility/business evidence folders.

Required inputs from user or environment:

- Backend/admin/gateway base URLs.
- Admin token or test admin credentials.
- Active paid/entitled user token.
- Not-purchased user token.
- Expired entitlement user token.
- Low-balance user token.
- Revoked-device user token.
- Model-denied user token.
- Test device id.
- Allowed model and denied model names.
- Permission to execute gateway test requests.
- Permission to start/complete browser handoff sessions.
- Permission to read admin audit rows for the test users.

Required evidence files:

- `00-environment.md`
- `01-test-accounts.md`
- `02-contract-checks.md`
- `03-admin-setup.md`
- `04-client-api-e2e.md`
- `05-gateway-policy-e2e.md`
- `06-desktop-manager-e2e.md`
- `07-package-install-check.md`
- `08-compatibility-migration.md`
- `09-usage-events-audit.md`
- `10-rollback-notes.md`
- `11-defects.md`
- `12-release-gate-report.md`

Verification:

```powershell
powershell -ExecutionPolicy Bypass -File codex-plus-dev-plan/tools/verify-07-e2e-readiness.ps1 -EnvFile codex-plus-dev-plan/test-runs/YYYYMMDD-HHMM-e2e-env/e2e-env.ps1 -ProbeHttp -EndpointPreflight
powershell -ExecutionPolicy Bypass -File sub2api-main/tools/e2e/codexplus/run-local-e2e.ps1 -EvidenceDir codex-plus-dev-plan/test-runs/YYYYMMDD-HHMM-e2e
powershell -ExecutionPolicy Bypass -File codex-plus-dev-plan/tools/verify-07-evidence.ps1 -EvidenceDir codex-plus-dev-plan/test-runs/YYYYMMDD-HHMM-e2e
```

Exit criteria:

- `verify-07-evidence.ps1` passes.
- Happy path reaches managed provider write, Codex launch and one gateway request.
- Required negative paths are either passed or recorded as explicit defects.

### Worker 2B: Package Artifact And Install Evidence

Goal:

- Produce package evidence for the Windows x64 MVP distribution target.

Write scope:

- `codex-plus-dev-plan/test-runs/YYYYMMDD-HHMM-package/*`
- `codex-plus-dev-plan/07-integration-release/package-install/*`
- `codex-plus-dev-plan/tools/inspect-07-package-artifacts.ps1`
- `codex-plus-dev-plan/tools/verify-07-package-evidence.ps1`
- Packaging docs only if required.

Do not edit:

- Desktop runtime source except narrow packaging metadata fixes approved by coordinator.
- Backend code.
- E2E/compatibility/business/release folders.

Required inputs:

- Windows package build host or CI run.
- Installer artifacts and hashes.
- macOS x64, macOS arm64, and unsigned/unnotarized macOS decisions are post-MVP inputs and must not block Windows-only MVP package evidence.

Tasks:

- Build or collect Windows installer artifact.
- Run artifact inspection and record SHA256.
- Execute install/open/overwrite/uninstall/reinstall checks where available.
- Record explicit Windows pass lines for `Manager login`, `Manager install assistant`, `Manager diagnostics`, `Manager advanced configuration`, and `Missing-Codex first-run` when those surfaces are actually proven.
- Record macOS as deferred post-MVP using `06-mvp-scope-decision.md`; do not mark default full cross-platform package verification as complete.

Verification:

```powershell
powershell -ExecutionPolicy Bypass -File codex-plus-dev-plan/tools/inspect-07-package-artifacts.ps1 -ArtifactDir <artifact-dir> -EvidenceDir codex-plus-dev-plan/test-runs/YYYYMMDD-HHMM-package
powershell -ExecutionPolicy Bypass -File codex-plus-dev-plan/tools/verify-07-package-evidence.ps1 -EvidenceDir codex-plus-dev-plan/test-runs/YYYYMMDD-HHMM-package -WindowsOnlyMvp
```

Exit criteria:

- Package evidence verifier passes, or Module J records explicit `no-go` / accepted-risk reason.
- No shared key, user credentials or fixed commercial policy is embedded in artifacts.

### Worker 2C: Compatibility Migration And Rollback Evidence

Goal:

- Prove old manual provider settings survive Cloud upgrade, logout, provider sync, manual switching and rollback.

Write scope:

- `codex-plus-dev-plan/test-runs/YYYYMMDD-HHMM-compatibility/*`
- `codex-plus-dev-plan/07-integration-release/compatibility/*`
- `codex-plus-dev-plan/tools/inspect-07-compatibility-snapshots.ps1`
- `codex-plus-dev-plan/tools/verify-07-compatibility-evidence.ps1`

Do not edit:

- Provider writer code unless a verified defect blocks compatibility and coordinator approves a narrow fix.
- Package/E2E/business/release folders.

Required inputs:

- Pre-upgrade settings snapshot with at least one manual provider.
- Post-upgrade settings snapshot.
- Logout snapshot.
- Rollback snapshot.
- Desktop runtime capable of executing provider sync and manual provider switching.

Tasks:

- Collect sanitized pre-upgrade, post-upgrade, logout and rollback snapshots.
- Run snapshot inspector.
- Prove manual provider count and fingerprints are preserved.
- Prove managed `Codex++ Cloud` provider exists after upgrade.
- Prove local commercial policy fields are not written into local provider settings.
- Prove logout clears token fields while preserving manual providers.
- Prove rollback does not delete manual providers.
- Record runtime manual-provider request and provider sync logs with secret scan.

Verification:

```powershell
powershell -ExecutionPolicy Bypass -File codex-plus-dev-plan/tools/inspect-07-compatibility-snapshots.ps1 -PreUpgradeSnapshot <path> -PostUpgradeSnapshot <path> -LogoutSnapshot <path> -RollbackSnapshot <path> -EvidenceDir codex-plus-dev-plan/test-runs/YYYYMMDD-HHMM-compatibility
powershell -ExecutionPolicy Bypass -File codex-plus-dev-plan/tools/verify-07-compatibility-evidence.ps1 -EvidenceDir codex-plus-dev-plan/test-runs/YYYYMMDD-HHMM-compatibility
```

Exit criteria:

- Compatibility evidence verifier passes.
- Manual providers are preserved through upgrade/logout/rollback.

### Worker 2D: Business Readiness Evidence

Goal:

- Produce owner-approved business readiness evidence for MVP launch.

Write scope:

- `codex-plus-dev-plan/test-runs/YYYYMMDD-HHMM-business/*`
- `codex-plus-dev-plan/BUSINESS-CONFIG-DECISION-TABLE.md`
- `codex-plus-dev-plan/PRODUCTION-ENVIRONMENT-MATRIX.md`
- `codex-plus-dev-plan/SECURITY-REVIEW-PLAN.md`
- `codex-plus-dev-plan/COMPLIANCE-PRIVACY-LEGAL-CHECKLIST.md`
- `codex-plus-dev-plan/OBSERVABILITY-SLO-ALERTING-PLAN.md`
- `codex-plus-dev-plan/COST-CONTROL-AND-ABUSE-RUNBOOK.md`
- `codex-plus-dev-plan/SUPPORT-OPERATIONS-RUNBOOK.md`

Do not edit:

- Runtime source code.
- E2E/package/compatibility/release evidence folders.

Required inputs from user or owners:

- MVP launch region and payment processor decision.
- Plan/pricing decision.
- Production domain and environment owner.
- Legal/privacy/payment terms approval status.
- Support owner and escalation channel.
- Cost/abuse response owner.
- Observability owner.

Tasks:

- Generate business evidence scaffold if needed.
- Fill `11-business-readiness.md` with owner-approved decisions.
- Do not mark legal/privacy/payment terms approved unless the user explicitly confirms.
- Record unresolved decisions as blockers, not as pass.

Verification:

```powershell
powershell -ExecutionPolicy Bypass -File codex-plus-dev-plan/tools/verify-07-business-readiness.ps1 -EvidenceDir codex-plus-dev-plan/test-runs/YYYYMMDD-HHMM-business
```

Exit criteria:

- Business readiness verifier passes, or Module J records `no-go`.

## Phase 3: Final Module J Handoff

Run after Phase 2 evidence lanes finish. Only one coordinator should own this phase.

### Worker 3A: Module J Final Aggregation

Goal:

- Create the final release handoff package and MVP go/no-go report.

Write scope:

- `codex-plus-dev-plan/test-runs/YYYYMMDD-HHMM-release/*`
- `codex-plus-dev-plan/07-integration-release/reports/coordinator-integration-release-verification.md`
- `codex-plus-dev-plan/IMPLEMENTATION-STATUS.md`
- `codex-plus-dev-plan/STAGE-GATE-LEDGER.md`
- `codex-plus-dev-plan/MULTI-SESSION-EXECUTION-TRACE.md`
- `codex-plus-dev-plan/FINAL-PLAN-COMPLETION-AUDIT.md`

Do not edit:

- Source code, unless a final verification defect is explicitly assigned back to the owning implementation worker.

Tasks:

- Run aggregate release evidence verifier.
- Generate release coverage summary.
- Generate release readiness summary.
- Draft Module J final report.
- Verify Module J final report.
- Verify final release handoff.
- Decide and record `go`, `go with accepted risks` or `no-go`.

Verification:

```powershell
powershell -ExecutionPolicy Bypass -File codex-plus-dev-plan/tools/verify-07-release-evidence.ps1 -E2EEvidenceDir codex-plus-dev-plan/test-runs/YYYYMMDD-HHMM-e2e -PackageEvidenceDir codex-plus-dev-plan/test-runs/YYYYMMDD-HHMM-package -CompatibilityEvidenceDir codex-plus-dev-plan/test-runs/YYYYMMDD-HHMM-compatibility -DocsEvidenceDir codex-plus-dev-plan/test-runs/YYYYMMDD-HHMM-docs

powershell -ExecutionPolicy Bypass -File codex-plus-dev-plan/tools/summarize-07-release-coverage.ps1 -E2EEvidenceDir codex-plus-dev-plan/test-runs/YYYYMMDD-HHMM-e2e -PackageEvidenceDir codex-plus-dev-plan/test-runs/YYYYMMDD-HHMM-package -CompatibilityEvidenceDir codex-plus-dev-plan/test-runs/YYYYMMDD-HHMM-compatibility -DocsEvidenceDir codex-plus-dev-plan/test-runs/YYYYMMDD-HHMM-docs -OutputFile codex-plus-dev-plan/test-runs/YYYYMMDD-HHMM-release/release-coverage-summary.md -FailOnIncomplete

powershell -ExecutionPolicy Bypass -File codex-plus-dev-plan/tools/summarize-07-release-readiness.ps1 -E2EEvidenceDir codex-plus-dev-plan/test-runs/YYYYMMDD-HHMM-e2e -PackageEvidenceDir codex-plus-dev-plan/test-runs/YYYYMMDD-HHMM-package -CompatibilityEvidenceDir codex-plus-dev-plan/test-runs/YYYYMMDD-HHMM-compatibility -DocsEvidenceDir codex-plus-dev-plan/test-runs/YYYYMMDD-HHMM-docs -BusinessEvidenceDir codex-plus-dev-plan/test-runs/YYYYMMDD-HHMM-business -CoverageSummaryFile codex-plus-dev-plan/test-runs/YYYYMMDD-HHMM-release/release-coverage-summary.md -OutputFile codex-plus-dev-plan/test-runs/YYYYMMDD-HHMM-release/release-readiness-summary.md -FailOnNoGo

powershell -ExecutionPolicy Bypass -File codex-plus-dev-plan/tools/verify-07-module-j-report.ps1 -ReportFile codex-plus-dev-plan/test-runs/YYYYMMDD-HHMM-release/module-j-final-report.md -CoverageSummaryFile codex-plus-dev-plan/test-runs/YYYYMMDD-HHMM-release/release-coverage-summary.md -ReadinessSummaryFile codex-plus-dev-plan/test-runs/YYYYMMDD-HHMM-release/release-readiness-summary.md

powershell -ExecutionPolicy Bypass -File codex-plus-dev-plan/tools/verify-07-release-handoff.ps1 -ReleaseDir codex-plus-dev-plan/test-runs/YYYYMMDD-HHMM-release
```

Exit criteria:

- Final handoff verifier passes.
- `STAGE-GATE-LEDGER.md` reflects the final 07 status.
- `IMPLEMENTATION-STATUS.md` has a concise MVP completion or no-go summary.
- `FINAL-PLAN-COMPLETION-AUDIT.md` records what is complete, what remains, and why.

## Recommended Run Order

1. Worker 0A: repair gates and scripts.
2. Worker 1A and 1B in parallel: environment readiness and docs evidence.
3. Workers 2A, 2B, 2C and 2D in parallel after the final run stamp is chosen.
4. Worker 3A: Module J aggregation.
5. If 3A finds defects, send them back to the owning worker with exact failing command and file path.

## File Ownership Matrix For Final Push

| Lane | Primary owner | Files/folders |
| --- | --- | --- |
| 0A Gate repair | Gate worker | `tools/validate-stage-gate.ps1`, `tools/verify-07-docs-product-copy-evidence.ps1`, 07 task metadata |
| 1A Runtime readiness | Env worker | local compose scripts, e2e-env test-runs, runtime verification notes |
| 1B Docs evidence | Docs worker | docs evidence folder, docs sync files, HTML product spec |
| 2A E2E evidence | E2E worker | final `*-e2e` folder and e2e helper scripts |
| 2B Package evidence | Package worker | final `*-package` folder and package evidence helpers |
| 2C Compatibility evidence | Compatibility worker | final `*-compatibility` folder and snapshot helpers |
| 2D Business readiness | Business worker | final `*-business` folder and readiness source docs |
| 3A Module J | Coordinator | final `*-release` folder, status ledgers and final audit |

## Stop And Ask The User

Workers must stop and ask before:

- Running real payment, refund, subscription or entitlement mutation outside a dedicated test environment.
- Sending real paid gateway requests if cost or model quota is not approved.
- Using production credentials, production customer data or real user accounts.
- Marking legal/privacy/payment/business approval as passed.
- Omitting Windows package targets from MVP.
- Accepting `go with accepted risks`.
- Publishing, uploading, distributing or announcing release artifacts.

## Copy-Paste Worker Prompts

### Prompt: Worker 0A Gate Repair

```text
You are Worker 0A for Codex++ MVP final completion. Work in F:\codex++\codex+++(2)\codex+++. Read codex-plus-dev-plan/MVP-FINAL-PARALLEL-COMPLETION-PLAN.md first. Your goal is to restore the 07 verification toolchain. Only edit the files listed under Worker 0A write scope. Fix PowerShell parse failures in validate-stage-gate.ps1 and verify-07-docs-product-copy-evidence.ps1, make verify-07-static.ps1 pass for legitimate current 07 state, and add required 07 task metadata sections. Do not weaken evidence gates. Run the three Worker 0A verification commands and report exact results.
```

### Prompt: Worker 1A Runtime Readiness

```text
You are Worker 1A for Codex++ MVP final completion. Work in F:\codex++\codex+++(2)\codex+++. Read codex-plus-dev-plan/MVP-FINAL-PARALLEL-COMPLETION-PLAN.md first. Your goal is to make the current local source runtime and toolchain readiness reproducible. Only edit Worker 1A files. Do not edit business logic. Boot or diagnose Docker/local source service, record route preflight, check Go/Node/npm/corepack pnpm/Rust/Cargo/Docker availability, and update release-local-verification.md with current facts. Do not print secrets.
```

### Prompt: Worker 1B Docs Evidence

```text
You are Worker 1B for Codex++ MVP final completion. Work in F:\codex++\codex+++(2)\codex+++. Read codex-plus-dev-plan/MVP-FINAL-PARALLEL-COMPLETION-PLAN.md first. Your goal is to make docs/product-copy evidence pass on the final run stamp. Only edit Worker 1B files. Do not claim E2E/package/compatibility/business completion. Run verify-07-docs-product-copy-evidence.ps1 against the docs evidence folder and report exact results.
```

### Prompt: Worker 2A E2E Evidence

```text
You are Worker 2A for Codex++ MVP final completion. Work in F:\codex++\codex+++(2)\codex+++. Read codex-plus-dev-plan/MVP-FINAL-PARALLEL-COMPLETION-PLAN.md first. Your goal is to produce real sanitized E2E evidence in codex-plus-dev-plan/test-runs/YYYYMMDD-HHMM-e2e. Only edit Worker 2A files. Do not run paid or production-impacting requests without explicit approval. Use the existing e2e helper scripts, collect the 13 required evidence files, run verify-07-evidence.ps1, and report pass/fail with blockers.
```

### Prompt: Worker 2B Package Evidence

```text
You are Worker 2B for Codex++ MVP final completion. Work in F:\codex++\codex+++(2)\codex+++. Read codex-plus-dev-plan/MVP-FINAL-PARALLEL-COMPLETION-PLAN.md first. Your goal is to produce Windows x64 package artifact and Windows install evidence in codex-plus-dev-plan/test-runs/YYYYMMDD-HHMM-package. Only edit Worker 2B files. Do not modify runtime source unless coordinator assigns a narrow packaging fix. Run inspect-07-package-artifacts.ps1 and verify-07-package-evidence.ps1 -WindowsOnlyMvp. Record explicit pass lines for Manager login, Manager install assistant, Manager diagnostics, Manager advanced configuration, and Missing-Codex first-run only after real proof. Record macOS as deferred post-MVP, not as a Windows-only MVP blocker.
```

### Prompt: Worker 2C Compatibility Evidence

```text
You are Worker 2C for Codex++ MVP final completion. Work in F:\codex++\codex+++(2)\codex+++. Read codex-plus-dev-plan/MVP-FINAL-PARALLEL-COMPLETION-PLAN.md first. Your goal is to produce compatibility and rollback evidence in codex-plus-dev-plan/test-runs/YYYYMMDD-HHMM-compatibility. Only edit Worker 2C files. Preserve manual providers. Do not print secrets. Run inspect-07-compatibility-snapshots.ps1 and verify-07-compatibility-evidence.ps1, and report exact blockers if real desktop snapshots are unavailable.
```

### Prompt: Worker 2D Business Readiness

```text
You are Worker 2D for Codex++ MVP final completion. Work in F:\codex++\codex+++(2)\codex+++. Read codex-plus-dev-plan/MVP-FINAL-PARALLEL-COMPLETION-PLAN.md first. Your goal is to produce owner-approved business readiness evidence in codex-plus-dev-plan/test-runs/YYYYMMDD-HHMM-business. Only edit Worker 2D files. Do not mark legal/privacy/payment/business approval as passed without explicit user confirmation. Run verify-07-business-readiness.ps1 and report blockers.
```

### Prompt: Worker 3A Module J Coordinator

```text
You are Worker 3A, the final Module J coordinator for Codex++ MVP completion. Work in F:\codex++\codex+++(2)\codex+++. Read codex-plus-dev-plan/MVP-FINAL-PARALLEL-COMPLETION-PLAN.md first. Do not start until Workers 0A, 1A, 1B, 2A, 2B, 2C and 2D have reported final status or explicit blockers. Only edit Worker 3A files. Aggregate final E2E/package/compatibility/docs/business evidence, generate release coverage and readiness summaries, draft module-j-final-report.md, run all Module J verification commands, and record go/no-go in status docs. Do not convert missing evidence into a go decision.
```

## Final MVP Checklist

- [ ] `verify-07-static.ps1` passes.
- [ ] `validate-stage-gate.ps1 -Stage 07-integration-release` passes.
- [ ] Docs product copy evidence passes on final run stamp.
- [ ] E2E evidence folder passes.
- [ ] Package evidence folder passes or accepted risk is explicitly approved.
- [ ] Compatibility evidence folder passes.
- [ ] Business readiness evidence passes.
- [ ] Aggregate release evidence passes.
- [ ] Release coverage summary is complete.
- [ ] Release readiness summary is generated from final evidence.
- [ ] Module J final report passes.
- [ ] Release handoff passes.
- [ ] `STAGE-GATE-LEDGER.md` and `IMPLEMENTATION-STATUS.md` record final MVP state.
