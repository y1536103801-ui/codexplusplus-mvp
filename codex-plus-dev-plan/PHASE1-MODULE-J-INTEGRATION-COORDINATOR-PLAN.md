# Codex++ Phase 1 Module J Integration Coordinator Plan

本文档把 Module J 的“最终集成”职责落到可执行的合并、冲突处理、验证、文档同步和发布裁决流程。Module J 是 Phase 1 MVP 的收口角色，不是新的功能开发角色。

## Status

- State: ready as coordinator checklist; execution waits for module final reports
- Owner: Module J / integration coordinator
- Primary scope: merge coordination, final verification, docs sync, release-readiness report
- Dependency:
  - `codex-plus-dev-plan/PARALLEL-DISPATCH-PLAN.md`
  - `codex-plus-dev-plan/FILE-OWNERSHIP-MATRIX.md`
  - `codex-plus-dev-plan/INTEGRATION-VERIFICATION-CHECKLIST.md`
  - `codex-plus-dev-plan/PHASE1-MODULE-I-E2E-RELEASE-GATE-PLAN.md`
  - final reports from Modules A/B/C/D/E/F/G/H/I

## Source Evidence

| Source | Module J usage |
| --- | --- |
| `PARALLEL-DISPATCH-PLAN.md` | Merge order and concurrency rules. |
| `FILE-OWNERSHIP-MATRIX.md` | Conflict authority and forbidden file checks. |
| `INTEGRATION-VERIFICATION-CHECKLIST.md` | Final integrated acceptance checklist. |
| `MVP-IMPLEMENTATION-PLAN.md` | Phase 1 scope and exit gates. |
| `CODEX-AUTONOMOUS-TEST-RUNBOOK.md` | Command/browser/desktop evidence flow. |
| `PHASE1-MODULE-C-H-*.md` | Module-specific implementation blueprints and expected outputs. |
| `PHASE1-MODULE-I-E2E-RELEASE-GATE-PLAN.md` | E2E evidence and release decision input. |

Important constraint:

- Module J may resolve small integration mismatches, but it must not silently rewrite module intent or expand scope. If a field, route, event or storage decision conflicts with frozen contracts, Module J stops and records a contract drift issue.

## Goal

Module J must produce an integrated Phase 1 result that can answer:

1. Which modules were merged and in what order?
2. Which conflicts happened and which ownership rule resolved them?
3. Which contract assumptions changed from the original plan?
4. Which tests were run, skipped or unavailable, and why?
5. Does the MVP happy path work in the selected test environment?
6. Are negative paths enforced server-side?
7. Are docs and HTML aligned with the implemented scope?
8. Is the result `go`, `go with accepted risks` or `no-go` for the next stage?

## Non Goals

- Do not build new product features.
- Do not redesign contracts after implementation unless using the contract change policy.
- Do not bypass tests by narrowing acceptance criteria.
- Do not perform production payment, refund, entitlement mutation or smoke test without explicit authorization.
- Do not use destructive git commands.
- Do not revert unrelated user changes.

## Integration Inputs

Module J must collect one final report from each module:

| Module | Required report evidence |
| --- | --- |
| A Contracts | final OpenAPI/schema/status/event/mock paths and validation result |
| B Storage | frozen storage/migration decision and conflict notes |
| C Backend Foundation | schema/migration/service files, tests, defaults, redaction notes |
| D Client API | route/handler/service files, contract tests, auth behavior |
| E Gateway Enforcement | policy hooks, rejection events, usage/rate behavior |
| F Desktop Runtime | Tauri commands, local state, provider writer, redaction tests |
| G Desktop UX | cloud route, user states, install/tutorial flows, UI checks |
| H Admin Operations | config publish/rollback, user entitlement/device/admin tests |
| I E2E Release Gate | evidence folder, E2E result, defects, release gate decision |

Missing final report is a blocker for broad integration unless the coordinator explicitly marks the module out of scope for the current build.

## Merge Order

Use this order unless a final report gives a documented reason to adjust:

1. A contracts and B storage decision.
2. C backend foundations.
3. D client API.
4. E gateway enforcement.
5. F desktop runtime.
6. H admin operations.
7. G desktop UX.
8. I E2E release artifacts.
9. Docs and HTML sync.
10. Final verification and report.

Reasoning:

- Contracts and storage decisions must precede implementation.
- Backend producers must precede runtime/UX final integration.
- Gateway enforcement must be integrated before E2E is considered meaningful.
- Admin operations precede UX final validation because admin changes must affect bootstrap.
- E2E artifacts are merged after they can point to real commands/builds.

## Pre-Merge Checklist

Before each module merge:

- inspect status/diff for the module worktree or patch
- confirm module only touched owned or approved secondary files
- scan for secrets, JWT, Authorization headers, full user-side API Key and upstream Key
- confirm any contract change followed `change-review-policy.md`
- confirm lockfiles/package manifests were changed only by approved owner
- read the module final report before resolving conflicts
- run the smallest meaningful targeted test after high-risk merges

If a module edited a forbidden file, Module J must not auto-accept it. Record the pressure, identify the owning module and ask for an approved integration patch.

## Conflict Resolution Rules

Follow the priority order from `FILE-OWNERSHIP-MATRIX.md`:

1. Frozen contracts win over consumer assumptions.
2. Storage/migration decision wins over ad hoc state.
3. Backend truth wins over frontend-only checks.
4. Gateway enforcement wins over client hidden UI.
5. Existing manual provider data must be preserved.
6. Secrets and credentials must be redacted even if this reduces debugging detail.

Additional Module J rules:

- Prefer additive route files over broad central router rewrites.
- Prefer typed adapters over duplicating response shaping in UI.
- Prefer admin config facade over raw settings JSON writes.
- Prefer integration patch files over modifying multiple module-owned files.
- If a field rename affects more than one consumer, stop and do a contract patch.

## Verification Levels

### Level 0: Documentation and Contract Static Check

Required before code merge begins:

- Markdown links valid.
- JSON schemas/fixtures parse.
- OpenAPI validates if tool is available.
- No stale rejected contract wording remains.
- No secret-like sample values appear.

### Level 1: Targeted Module Tests

After each module:

- run module-owned unit or type checks
- record unavailable commands and missing tools
- do not mark unavailable checks as pass

### Level 2: Integrated Backend/Desktop/Admin Checks

At integration end:

```powershell
cd sub2api-main
make test

cd sub2api-main/frontend
pnpm run lint:check
pnpm run typecheck
pnpm run test:run

cd CodexPlusPlus-main
cargo test --workspace

cd CodexPlusPlus-main/apps/codex-plus-manager
npm run check
npm run build
```

If commands are unavailable in the local toolchain, Module J records:

- command
- reason unavailable
- replacement narrower check, if any
- owner needed to clear the blocker

### Level 3: E2E Gate

Consume Module I evidence:

- happy path
- not purchased
- expired
- low balance
- model denied
- device revoked
- admin config change reflected in bootstrap
- package first run
- manual provider preservation
- usage/event/audit redaction

No Level 3 pass means no Phase 1 go decision.

After the individual E2E, package, compatibility and Docs product copy evidence folders exist, run the aggregate hygiene gate:

To create a matched scaffold set before execution:

```powershell
powershell -ExecutionPolicy Bypass -File codex-plus-dev-plan/tools/new-07-release-evidence-set.ps1
```

```powershell
powershell -ExecutionPolicy Bypass -File codex-plus-dev-plan/tools/verify-07-release-evidence.ps1 `
  -E2EEvidenceDir codex-plus-dev-plan/test-runs/YYYYMMDD-HHMM-e2e `
  -PackageEvidenceDir codex-plus-dev-plan/test-runs/YYYYMMDD-HHMM-package `
  -CompatibilityEvidenceDir codex-plus-dev-plan/test-runs/YYYYMMDD-HHMM-compatibility `
  -DocsEvidenceDir codex-plus-dev-plan/test-runs/YYYYMMDD-HHMM-docs
```

This aggregate verifier only proves that the release evidence folders satisfy their hygiene contracts. It does not replace Module J's go/no-go recommendation.

For Docs product copy evidence, the final release evidence set must include the sibling folder `codex-plus-dev-plan/test-runs/YYYYMMDD-HHMM-docs` and verify it with:

```powershell
powershell -ExecutionPolicy Bypass -File codex-plus-dev-plan/tools/verify-07-docs-product-copy-evidence.ps1 `
  -EvidenceDir codex-plus-dev-plan/test-runs/YYYYMMDD-HHMM-docs
```

The Docs product copy verifier is the release evidence boundary for public README/user guide/admin guide/release notes/HTML copy alignment. Current static docs can still contain draft or pending boundaries; they are not final release docs evidence until this folder is finalized and verified.

For business readiness, create and verify the Phase 9 business evidence:

```powershell
powershell -ExecutionPolicy Bypass -File codex-plus-dev-plan/tools/new-07-business-readiness-evidence.ps1
powershell -ExecutionPolicy Bypass -File codex-plus-dev-plan/tools/verify-07-business-readiness.ps1 `
  -EvidenceDir codex-plus-dev-plan/test-runs/YYYYMMDD-HHMM-business
```

The business readiness verifier checks production environment decisions, package/model/quota/payment/cost decisions, server sizing, deployment/backup/rollback/healthcheck paths, security, compliance/privacy/legal/payment terms, observability, cost/abuse emergency stop, paid-user support and human decision ownership. It does not execute technical release tests or make the technical go/no-go decision.

Before writing the final report, generate a release coverage matrix:

```powershell
powershell -ExecutionPolicy Bypass -File codex-plus-dev-plan/tools/summarize-07-release-coverage.ps1 `
  -E2EEvidenceDir codex-plus-dev-plan/test-runs/YYYYMMDD-HHMM-e2e `
  -PackageEvidenceDir codex-plus-dev-plan/test-runs/YYYYMMDD-HHMM-package `
  -CompatibilityEvidenceDir codex-plus-dev-plan/test-runs/YYYYMMDD-HHMM-compatibility `
  -DocsEvidenceDir codex-plus-dev-plan/test-runs/YYYYMMDD-HHMM-docs `
  -OutputFile codex-plus-dev-plan/test-runs/YYYYMMDD-HHMM-release/release-coverage-summary.md `
  -FailOnIncomplete
```

The coverage summary maps the E2E, package, compatibility and Docs product copy evidence files to the required release scenarios such as browser handoff, bootstrap, gateway allow/deny paths, package install paths, provider migration, rollback and public docs/HTML implemented-scope alignment. It fails on missing coverage or obvious fixture/scaffold/pending markers, but it still does not execute release scenarios or make a go/no-go decision.

Then generate a conservative readiness summary:

```powershell
powershell -ExecutionPolicy Bypass -File codex-plus-dev-plan/tools/summarize-07-release-readiness.ps1 `
  -E2EEvidenceDir codex-plus-dev-plan/test-runs/YYYYMMDD-HHMM-e2e `
  -PackageEvidenceDir codex-plus-dev-plan/test-runs/YYYYMMDD-HHMM-package `
  -CompatibilityEvidenceDir codex-plus-dev-plan/test-runs/YYYYMMDD-HHMM-compatibility `
  -DocsEvidenceDir codex-plus-dev-plan/test-runs/YYYYMMDD-HHMM-docs `
  -BusinessEvidenceDir codex-plus-dev-plan/test-runs/YYYYMMDD-HHMM-business `
  -CoverageSummaryFile codex-plus-dev-plan/test-runs/YYYYMMDD-HHMM-release/release-coverage-summary.md `
  -OutputFile codex-plus-dev-plan/07-integration-release/reports/release-readiness-summary.md `
  -FailOnNoGo
```

The readiness summary is not the final Module J report. It combines aggregate verifier output, coverage summary verification, Docs product copy evidence, business readiness verification and obvious nonrelease markers such as fixture, scaffold, subset, pending or missing external evidence. It defaults to `no-go`; `-AllowGoCandidate` may only be used during Module J review after real production-equivalent E2E, package, compatibility, Docs product copy and business readiness evidence exists, and only when the generated coverage summary is complete with zero missing coverage, zero nonrelease markers and the same E2E/package/compatibility/docs input paths as the readiness summary.

## Documentation Sync

After integration, Module J must sync:

- `README.md`
- `PARALLEL-DISPATCH-PLAN.md`
- `INTEGRATION-VERIFICATION-CHECKLIST.md`
- `WORKER-PROMPTS.md`
- `index.html`
- any affected `07-integration-release/**` docs

Sync rules:

- Docs must describe implemented behavior, not hoped-for behavior.
- HTML must not promise features outside MVP unless explicitly marked later phase.
- If implementation intentionally deviates from the plan, record the deviation and reason.
- Prices, model names, quotas, rate limits and renewal copy still must be described as admin/config-driven, not hard-coded.

## Final Report Structure

Module J writes or updates a final report using this structure:

```text
Modules merged:
- module reports:
- merge order:
- out of scope modules:

Builds and versions:
- backend:
- admin frontend:
- desktop manager:
- contract version:
- config version:

Conflicts resolved:
- file:
  modules:
  rule used:
  result:

Contract changes from original plan:
- drift status:
- affected surface:
- change review evidence:
- owner:
- impact:

Verification commands:
- command:
  result:
  evidence:
  skipped or unavailable:
  reason unavailable:
  replacement narrower check:
  owner needed:

Release evidence hygiene:
- verifier:
- E2E evidence folder:
- Package evidence folder:
- Compatibility evidence folder:
- Docs product copy evidence folder:
- Docs product copy verification:
- Business readiness folder:
- Release coverage summary:
- Release readiness summary:
- Aggregate evidence result:
- Coverage summary status:
- Business readiness verification:

E2E result:
- decision:
- evidence folder:
- happy path:
- failure paths:
- admin config bootstrap:

Docs and HTML sync:
-

Remaining risks:
- severity:
  owner:
  impact:
  mitigation:
  target date:

Rollback notes:
- config rollback:
- backend rollback:
- desktop rollback:
- manual provider recovery:
```

Use `07-integration-release/reports/module-j-final-report-template.md` as the fillable structure. Before treating the report as final, run:

```powershell
powershell -ExecutionPolicy Bypass -File codex-plus-dev-plan/tools/verify-07-module-j-report.ps1 `
  -ReportFile codex-plus-dev-plan/07-integration-release/reports/module-j-final-report.md `
  -CoverageSummaryFile codex-plus-dev-plan/07-integration-release/reports/release-coverage-summary.md `
  -ReadinessSummaryFile codex-plus-dev-plan/07-integration-release/reports/release-readiness-summary.md
```

The report verifier checks the required sections, module report input and merge-order fields, verification command/skipped/unavailable disposition fields, release evidence hygiene fields, conflict file/module/rule/result fields, contract drift/change-review fields, final recommendation value, aggregate evidence result boundary, coverage summary consistency, readiness summary consistency, readiness coverage verification, readiness summary coverage path, report summary path and evidence input consistency, Docs product copy evidence signal, business readiness summary signal, named go-policy evidence signals, rollback fields, remaining-risk ownership/impact fields and obvious secret or placeholder leakage. A `go` or `go with accepted risks` report must include `-CoverageSummaryFile` and `-ReadinessSummaryFile`, and it cannot override incomplete coverage, missing coverage, nonrelease coverage markers, mismatched summary/evidence paths, missing/failed readiness coverage verification, readiness summary `Allow go candidate: false`, a readiness summary posture of `no-go`, missing/failed Docs product copy evidence, or a missing/failed business readiness verification. A `go` or `go with accepted risks` report must explicitly record final reports for Modules A through I, no out-of-scope modules, merge order A/B/C/D/E/F/H/G/I, no unapproved/pending/unreviewed contract drift, coverage summary status `complete`, Docs product copy verification `passed` and business readiness verification `passed` in the report's `Release evidence hygiene` section, plus happy-path, not-purchased/unpaid, expired, revoked-device, unauthorized-model, admin-config/bootstrap, manual-provider and docs/HTML implemented-scope signals. A `go with accepted risks` report must also record accepted impact for remaining risks.

Before handing off a release evidence workspace, also run the handoff verifier against the timestamped `*-release` folder:

```powershell
powershell -ExecutionPolicy Bypass -File codex-plus-dev-plan/tools/verify-07-release-handoff.ps1 `
  -ReleaseDir codex-plus-dev-plan/test-runs/YYYYMMDD-HHMM-release `
  -AllowGoCandidate
```

Use `-AllowGoCandidate` only for a final Module J candidate handoff after real external evidence exists; omit it for scaffold or no-go handoff checks. The handoff verifier derives the matching E2E, package, compatibility, Docs product copy and business readiness folders from the release folder's run stamp, reruns aggregate evidence verification, verifies Docs product copy and business readiness evidence, regenerates the coverage and readiness summaries for consistency checks, compares stored and regenerated coverage/readiness input paths and counts, verifies that the readiness summary records passed coverage verification, verifies the Module J final report with `-CoverageSummaryFile` and `-ReadinessSummaryFile`, and requires the final handoff index to record matching final verification results for aggregate evidence, Docs product copy evidence, business readiness, coverage status, readiness posture, Module J report verification and final recommendation. It proves handoff consistency only; it does not execute E2E, build packages, run compatibility migration, finalize public docs, or make the release go/no-go decision.

## Go/No-Go Policy

Module J can recommend `go` only when:

- Module I E2E happy path passes.
- No open P0/P1 remains.
- No secret leak is found.
- Unpaid or expired user cannot use paid access.
- Gateway rejects unauthorized model and revoked device cases.
- Admin config changes reflect in bootstrap.
- Desktop preserves manual providers.
- Docs product copy evidence passes and Docs/HTML match implemented scope.
- Rollback notes are actionable.

Recommend `no-go` when any of the above fails or is unverified.

Recommend `go with accepted risks` only when all P0/P1 are clear and every remaining P2/P3 has owner, mitigation and accepted impact.

## Owned Scope

Module J may write:

- integration reports and release notes
- `codex-plus-dev-plan/**`
- final docs/HTML sync
- small merge-resolution patches across module-owned files only after reading owner reports
- package manifests/lockfiles only when an approved dependency change requires final reconciliation

## Forbidden Scope

Module J must not:

- introduce new product scope
- change frozen contracts without contract review
- hide failed tests by deleting checks
- bypass server-side enforcement for UI convenience
- delete manual providers or migration safeguards
- commit secrets, local `.env` files or unredacted logs
- run production calls without explicit authorization

## Handoff

At the end, Module J hands the project owner:

- final report
- evidence folder path
- Docs product copy evidence folder path
- go/no-go recommendation
- exact commands run and results
- unresolved decisions requiring human approval
- rollback path
- list of docs synced
