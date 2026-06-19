# Codex++ MVP Final Execution Board

Date: 2026-06-19
Workspace: `F:\codex++\codex+++(2)\codex+++`
Current release candidate: `codex-plus-dev-plan/test-runs/20260619-1314-release`
Current posture: no-go

This board is the short execution view for the remaining MVP work. It does not approve launch, does not weaken any verifier, and does not replace the detailed authorization runbook.

## Current Gate State

| Lane | Current verifier | Result | Blocking count | Owner |
| --- | --- | --- | ---: | --- |
| Static/stage gates | `verify-07-static.ps1`, `validate-stage-gate.ps1` | pass | 0 | Coordinator |
| E2E | `verify-07-evidence.ps1` | fail | 12 | E2E worker after authorization |
| Windows package | `verify-07-package-evidence.ps1 -WindowsOnlyMvp` | pass | 0 | Package worker |
| Compatibility | `verify-07-compatibility-evidence.ps1` | fail | 34 | Compatibility worker after authorization |
| Docs/product copy | `verify-07-docs-product-copy-evidence.ps1` | pass | 0 | Docs worker |
| Business readiness | `verify-07-business-readiness.ps1` | fail | 18 | Owner/business/legal/security/support/cost |
| Module J report | `verify-07-module-j-report.ps1` | pass | 0 | Coordinator |
| Release handoff | `verify-07-release-handoff.ps1 -WindowsOnlyMvp` | fail | 12 | Coordinator after failed lanes pass |

## Exact Remaining Technical Blockers

### E2E

These are the current failing checks from `verify-07-evidence.ps1` against `20260619-1314-e2e`:

- `release-report:final-recommendation-pass`
- `level3:pass-recorded`
- `admin-setup:result-pass`
- `desktop-manager:result-pass`
- `compatibility-migration:result-pass`
- `usage-events-audit:result-pass`
- `rollback-notes:result-pass`
- `desktop-manager:login`
- `desktop-manager:cloud-provider`
- `desktop-manager:manual-provider-preservation`
- `desktop-manager:codex-launch`
- `audit:gateway-request-id-correlation`

Resolution path:

1. Receive exact authorization phrase `授权本地测试合规确认` if local admin compliance accept must be executed.
2. Receive exact authorization phrase `授权在隔离 profile 下执行 Desktop Manager/Codex 本地 E2E` before Desktop Manager/Codex, browser handoff, provider write, gateway/admin audit, or runtime compatibility work.
3. Run `accept-07-local-admin-compliance.ps1` with `-EvidenceDir <e2e evidence folder>` after the exact local compliance authorization phrase; the helper writes sanitized `03-admin-setup.md` evidence with `Run folder:` and still does not print token values or raw API responses.
4. Fill real sanitized evidence in `06-desktop-manager-e2e.md`, `08-compatibility-migration.md`, `09-usage-events-audit.md`, `10-rollback-notes.md`, and `12-release-gate-report.md`.
5. Re-run `verify-07-evidence.ps1` until Level 3 records pass.

### Compatibility

These current failures are real evidence gaps, not just formatting gaps:

- no parsed pre-upgrade/post-upgrade/logout/rollback snapshot set
- no verified legacy relay profiles/settings parse
- no proven nonzero manual provider count
- no proven manual provider preservation after Cloud upgrade
- no proven managed `Codex++ Cloud` write without overwriting manual providers
- no proven logout boundary preserving manual providers and clearing token fields
- no runtime manual provider selection/request pass
- no provider sync log review pass and redaction proof
- no runtime rollback rehearsal pass
- no final compatibility gate pass

Resolution path:

1. Use only the isolated desktop harness created by `new-07-desktop-compatibility-harness.ps1`.
2. Capture sanitized `pre-upgrade`, `post-upgrade`, `logout`, and `rollback` snapshots after the authorized runtime sequence.
3. Run `inspect-07-compatibility-snapshots.ps1` with all four snapshots.
4. Add manual runtime results for login/logout, provider switch, manual request, provider sync log redaction, and rollback rehearsal.
5. Re-run `verify-07-compatibility-evidence.ps1` until pass.

### Business Readiness

Current `verify-07-business-readiness.ps1` failures are owner approval gaps:

- production environment values
- business config decisions
- deployment automation, backup, rollback, healthcheck
- security review P0/P1/P2
- compliance, privacy, legal, payment provider terms, refund policy
- observability SLO, dashboards, alert routing
- cost control, abuse, spend caps, emergency shutoff
- support operations, paid-user support, refund/compensation/admin recovery
- human business or legal decisions
- no-go scan items for open P0/P1, production values, legal terms, observability, cost emergency, support
- unresolved placeholder in `11-business-readiness.md`

Resolution path:

1. Owner completes `codex-plus-dev-plan/07-integration-release/reports/business-owner-approval-packet.md`.
2. Business worker updates `20260619-1314-business` or a new same-stamp business evidence folder from real approvals only.
3. Re-run `verify-07-business-readiness.ps1` until pass.

## Work That Is Already Done

- 07 static verifier passes.
- 07 stage gate passes.
- E2E env readiness passes with local generated env file, without printing secret values.
- Evidence tooling self-test passes.
- Local backend and mock upstream health checks were previously 200 at check time.
- Windows-only package lane passes.
- Windows-only MVP release summaries now record macOS package evidence as `deferred-post-mvp`; current MVP coverage has 4 missing E2E items and 6 nonrelease markers.
- Docs/product-copy lane passes.
- Module J report verifier passes for the current no-go package.
- Same-stamp `20260619-1314-release` index is final and internally consistent.
- E2E and compatibility verifiers now reject mismatched `Run folder:` declarations when a file declares one.
- Local compliance accept helper has self-test coverage proving `-EvidenceDir` cannot bypass the opt-in gate and denied runs do not write `03-admin-setup.md`.

## Parallel Dispatch From Here

Only these workstreams are safe to run in parallel:

| Worker | Start condition | Write scope |
| --- | --- | --- |
| E2E worker | after required exact authorization phrases and local env inputs are available | one fresh `*-e2e` folder and E2E helper logs |
| Compatibility worker | after Desktop Manager/Codex isolated-profile authorization is available | one fresh `*-compatibility` folder and isolated harness snapshots |
| Business worker | after owner approval packet is completed | one fresh `*-business` folder |
| Coordinator | after all three lanes report pass or explicit no-go | one fresh `*-release` folder and status ledgers |

Do not start E2E and compatibility workers against the real user profile. Do not let any worker edit provider configs outside the isolated harness. Do not let business readiness pass without explicit owner approval.

## Stop Conditions

- Missing exact authorization phrase for a gated action.
- Missing local env file values needed for E2E.
- Any evidence output contains raw token, JWT, Authorization header, provider key, password, or provider config secret.
- Any worker needs production endpoints, production payment, production gateway traffic, or `-AllowProduction`.
- Owner approval is not explicit enough to satisfy the business readiness verifier.
