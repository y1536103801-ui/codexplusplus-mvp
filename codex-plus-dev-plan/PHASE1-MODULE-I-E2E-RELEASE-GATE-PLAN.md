# Codex++ Phase 1 Module I E2E Release Gate Plan

本文档把 Module I 从“准备 E2E 清单”升级为可执行的发布门禁计划。它不实现后端、桌面端或管理后台功能，只定义如何在测试环境证明 Codex++ MVP 闭环真的可用、可拒绝、可追踪、可回滚。

## Status

- State: ready for read-only prep; final execution waits for Modules D/E/F/G/H merged
- Owner: Module I / E2E release gate worker
- Primary scope: `codex-plus-dev-plan/07-integration-release/**`, E2E evidence, release gate scripts/checklists
- Dependency:
  - `codex-plus-contracts/api/client-openapi.yaml`
  - `codex-plus-contracts/test-fixtures/client/*.json`
  - `codex-plus-contracts/status-error/client-status-errors.md`
  - `codex-plus-dev-plan/INTEGRATION-VERIFICATION-CHECKLIST.md`
  - `codex-plus-dev-plan/CODEX-AUTONOMOUS-TEST-RUNBOOK.md`
  - final reports from Modules D/E/F/G/H

## Source Evidence

Existing release and test documents already define the outer shape:

| Source | Module I usage |
| --- | --- |
| `07-integration-release/README.md` | Stage objective and merge order. |
| `07-integration-release/task-e2e-buy-login-launch.md` | Happy-path E2E task. |
| `07-integration-release/task-compatibility-and-migration.md` | Manual provider and old-user compatibility checks. |
| `07-integration-release/task-package-install-check.md` | Installer and first-run checks. |
| `INTEGRATION-VERIFICATION-CHECKLIST.md` | Final Module J gate that consumes Module I evidence. |
| `QA-TESTING-ACCEPTANCE-PLAN.md` | Broader local/staging/production acceptance strategy. |
| `CODEX-AUTONOMOUS-TEST-RUNBOOK.md` | Autonomous command/browser/desktop test execution flow. |
| `FILE-OWNERSHIP-MATRIX.md` | Confirms Module I is read-only for implementation code. |

Important constraint:

- Module I can prepare checklists, scripts and report templates early, but final pass/fail judgment is valid only after D/E/F/G/H are merged into an integration branch or equivalent test build.

## Goal

Module I must produce an executable E2E gate that proves:

1. A test user with entitlement can log in, bootstrap, write `Codex++ Cloud`, launch Codex and complete one gateway request.
2. A user without entitlement cannot use paid models.
3. Expired, insufficient balance, unauthorized model and revoked device states are enforced server-side.
4. Admin config changes affect bootstrap without desktop release.
5. Usage, rejection and admin events can be inspected without revealing secrets.
6. Manual provider settings survive login, refresh, logout and repair flows.
7. Install/package checks cover first launch for non-Codex users.
8. Rollback notes exist for bad config, bad build, bad entitlement and failed provider write.

## Non Goals

- Do not implement `/api/v1/client/*`.
- Do not implement gateway enforcement.
- Do not edit desktop runtime or React UI.
- Do not edit admin feature code.
- Do not create or mutate real production payment orders.
- Do not run destructive device deletion, refund, database reset or production smoke tests without explicit authorization.
- Do not include real credentials, real JWT, full user-side API Key or upstream provider Key in evidence.

## Execution Modes

### Prep Mode

Can start while modules are still in development.

Allowed output:

- manual E2E checklist
- scripted E2E skeleton or pseudocode
- test account matrix
- evidence directory template
- package/install check list
- release-readiness report template

Prep mode must not mark the release as pass or fail. It can only state whether the future test is executable.

### Final Execution Mode

Starts only when:

- Module D client API final report exists
- Module E gateway enforcement final report exists
- Module F desktop runtime final report exists
- Module G desktop UX final report exists
- Module H admin operations final report exists
- Module J or the coordinator confirms the build/test environment to use

Final execution can produce a `go`, `go with accepted risks` or `no-go` recommendation.

## Target Evidence Structure

Module I should create a timestamped evidence folder when running the gate:

```text
codex-plus-dev-plan/
  test-runs/
    YYYYMMDD-HHMM-e2e/
      00-environment.md
      01-test-accounts.md
      02-contract-checks.md
      03-admin-setup.md
      04-client-api-e2e.md
      05-gateway-policy-e2e.md
      06-desktop-manager-e2e.md
      07-package-install-check.md
      08-compatibility-migration.md
      09-usage-events-audit.md
      10-rollback-notes.md
      11-defects.md
      12-release-gate-report.md
```

Evidence files may be Markdown reports, sanitized command output, screenshots or links to CI artifacts. Screenshots and logs must be redacted before being committed or shared.

## Test Account Matrix

Use test accounts only:

| Account | Purpose | Required setup |
| --- | --- | --- |
| `admin_test` | Admin config, entitlement and device operations | Admin role, no production authority. |
| `user_active` | Happy path | Active Codex++ entitlement, safe model group, test balance. |
| `user_not_purchased` | Purchase required state | No active entitlement. |
| `user_expired` | Expired entitlement | Expired subscription or manually expired grant. |
| `user_low_balance` | Insufficient/low balance behavior | Entitlement exists but balance/policy blocks request. |
| `user_device_revoked` | Device block path | Device registered then revoked by admin/test setup. |
| `user_model_denied` | Unauthorized model rejection | Entitlement active but requested model outside allowed set. |

If any account cannot be created safely, Module I records the missing setup as a blocker instead of substituting a weaker check.

## Scenario Matrix

### Happy Path

Steps:

1. Confirm backend, gateway, admin frontend and desktop build versions.
2. Log in as `admin_test`.
3. Confirm or create a safe test Codex++ plan and model policy.
4. Grant or confirm `user_active` entitlement.
5. Launch Codex++ Manager.
6. Log in as `user_active`.
7. Register or refresh device.
8. Call or trigger bootstrap.
9. Confirm `service.status = "available"`.
10. Apply or repair `Codex++ Cloud`.
11. Launch Codex from Manager.
12. Send one low-cost test request through Sub2API gateway.
13. Confirm usage and audit event visibility.

Pass evidence:

- sanitized bootstrap response shape
- `Codex++ Cloud` provider exists
- manual providers still exist
- one request succeeds
- usage/event row visible
- no upstream credential appears

### No Entitlement

Expected:

- bootstrap returns `not_purchased` or approved equivalent status
- UI shows a purchase/activation action from backend copy/config
- gateway does not allow paid usage if a stale local provider exists

### Expired Entitlement

Expected:

- bootstrap and/or usage returns expired state
- gateway rejects usage server-side
- event is visible and redacted

### Low or Insufficient Balance

Expected:

- status and gateway behavior match MVP policy
- UI does not invent quota thresholds
- admin can inspect why the user is blocked

### Unauthorized or Removed Model

Expected:

- removed/disabled model does not appear as selectable when config is refreshed
- direct request to unauthorized model is rejected by gateway
- rejection event identifies policy reason without leaking secrets

### Revoked Device

Expected:

- revoked device cannot continue normal launch path
- repair/relogin behavior is actionable
- manual providers are not deleted

### Admin Config Change

Expected:

- admin changes default model or feature flag
- config version changes
- bootstrap returns new version
- desktop consumes new snapshot without app release

### Package and First-Run Check

Expected:

- install package starts Manager
- first-run route lands on ordinary-user cloud UX
- install assistant detects missing local Codex or broken path
- no manual API setup is required for the happy path

## Automation Guidance

Module I may add scripts only under an approved E2E/tools path, for example:

```text
sub2api-main/tools/e2e/codexplus/
  README.md
  run-local-e2e.ps1
  run-client-api-checks.ps1
  run-browser-handoff-checks.ps1
```

The current prep-mode scaffold, readiness and evidence hygiene tools are:

```text
codex-plus-dev-plan/tools/new-07-e2e-env-template.ps1
codex-plus-dev-plan/tools/new-07-release-evidence-set.ps1
codex-plus-dev-plan/tools/new-07-evidence-run.ps1
codex-plus-dev-plan/tools/verify-07-e2e-readiness.ps1
codex-plus-dev-plan/tools/verify-07-evidence.ps1
```

Use `new-07-release-evidence-set.ps1` when preparing a full Module I/J release evidence run. It creates matching E2E, package and compatibility evidence scaffolds plus a Module J report draft under a shared timestamp.

Use `new-07-e2e-env-template.ps1` to prepare local execution inputs. It creates `test-runs/YYYYMMDD-HHMM-e2e-env/e2e-env.template.ps1` and `e2e-env-checklist.md`, fills detected local URLs and Manager build candidate paths, and marks token, key and model values that must be completed manually. The generated env template and checklist are prep aids only, not release evidence.

Before executing E2E, run `verify-07-e2e-readiness.ps1` with the `CODEXPLUS_07_E2E_*` environment variables set for the selected test environment, or pass `-EnvFile <template.ps1>` to load them from a local env file. It validates local/dev/test/staging/sandbox target URLs, required test-account tokens, numeric test user IDs for admin audit correlation, test model names, device ID and Manager build path without printing token values. Non-test or production-looking URLs require the explicit `-AllowProduction` override, HTTP probing is opt-in with `-ProbeHttp`, and `-EndpointPreflight` can probe the 07 client/admin/desktop/gateway routes to catch a service image that is not the Codex++ 07 version. Use `-EndpointPreflightOnly -OutputPath <report.md>` only for non-release local route diagnostics; this mode skips token/model/persona checks and cannot be used as release evidence. Endpoint preflight is diagnostic only and does not replace executed release evidence.

The current prep-mode client API runners are:

```text
sub2api-main/tools/e2e/codexplus/run-client-api-checks.ps1
sub2api-main/tools/e2e/codexplus/run-browser-handoff-checks.ps1
sub2api-main/tools/e2e/codexplus/run-gateway-policy-checks.ps1
sub2api-main/tools/e2e/codexplus/run-admin-audit-checks.ps1
sub2api-main/tools/e2e/codexplus/run-local-e2e.ps1
```

`run-client-api-checks.ps1` runs the non-destructive client API subset for bootstrap, usage and idempotent test-device refresh, then writes sanitized observations into the E2E evidence folder. `run-browser-handoff-checks.ps1` runs the browser handoff subset for desktop start, pending poll, optional authenticated browser complete and completed poll; real session creation requires `-AllowSessionStart`, and real browser approval requires `-AllowBrowserComplete` plus `CODEXPLUS_07_E2E_BROWSER_AUTH_TOKEN` from an already authenticated browser session. `run-gateway-policy-checks.ps1` runs the gateway policy subset for one low-cost active-user request and required rejection probes, but refuses to send requests unless `-AllowGatewayRequests` is supplied; it must retain safe `request_id`, `GATEWAY_POLICY_*` and service-status fields for every rejection. `run-admin-audit-checks.ps1` reads admin-visible Codex++ usage/audit events after gateway probes, but refuses to read rows unless `-AllowAdminAuditReads` is supplied; it omits raw payloads and records only safe correlation fields such as `usage_recorded`, `gateway_policy_rejected`, gateway-matched `request_id`, `config_version`, `GATEWAY_POLICY_*` and `redaction_applied`. `run-local-e2e.ps1` creates the standard 13-file E2E scaffold and then calls the selected subset runners. `run-client-api-checks.ps1` and `run-local-e2e.ps1` can load the same local env file with `-EnvFile`; `run-client-api-checks.ps1` also accepts `-EndpointPreflight` for the same route diagnostic before client API checks. These scripts do not execute desktop Manager UI confirmation, Codex launch, package install, compatibility migration or payment flows.

Script requirements:

- Read base URLs and test tokens from environment variables, `-EnvFile`, or a local ignored file.
- Never hard-code secrets or test credentials.
- Print redacted output by default.
- Fail fast on contract mismatch, leaked secret pattern or unexpected HTTP success for blocked users.
- Make destructive or payment-like actions opt-in with an explicit flag.
- For evidence-folder verification, report only file names, line numbers and rule names for suspected leaks; do not echo matched secret values.
- Evidence scaffolds may contain TODO placeholders and `Result: pending` markers, but final evidence verification must fail until every placeholder is replaced with sanitized execution evidence.
- Final evidence files `02-contract-checks.md` through `10-rollback-notes.md` must each include `Result: pass` or `Result: fail`; `pending` is not acceptable in a candidate release evidence folder.
- E2E readiness verification, env template generation and endpoint preflight are only test-environment prerequisites or diagnostics. They do not execute login, gateway, desktop, payment, package install or compatibility scenarios, and they do not override no-go release criteria.
- Client API runners must keep redeem opt-in, must not print token values, and must label fixture-mode output as tooling self-test only.
- Browser handoff runners must keep session creation opt-in through `-AllowSessionStart`, keep browser completion opt-in through `-AllowBrowserComplete`, use browser JWT only from environment variables, verify `poll_token` is not present in `authorize_url`, and never print desktop session, poll, browser, access or refresh token values.
- Gateway policy runners must keep real model requests opt-in through `-AllowGatewayRequests`, use user-side gateway keys only from environment variables, require safe structured rejection fields, and never print gateway key values.
- Admin audit runners must keep real admin event reads opt-in through `-AllowAdminAuditReads`, require numeric test user IDs, require `usage_recorded` and `gateway_policy_rejected` correlation fields, require admin event `request_id` values to match the gateway evidence, and never print admin tokens or raw event payloads.

If implementation code is not yet available, keep scripts as documented pseudocode rather than brittle partial automation.

## Release Gate Decisions

Use exactly one of:

- `go`: all P0/P1 checks pass, P2 risks accepted with owners.
- `go with accepted risks`: no P0/P1, but known P2/P3 items have owner, impact and mitigation.
- `no-go`: any P0/P1 remains open, a required happy path fails, unpaid user can use, paid user cannot use, secret leakage occurs, or rollback path is missing.

P0 examples:

- unpaid user can access paid model
- active paid/test user cannot use happy path
- full API Key/JWT/upstream Key appears in logs/evidence/UI
- payment or entitlement duplication cannot be reconciled
- provider writer deletes manual providers

## Owned Scope

Module I may write:

- `codex-plus-dev-plan/07-integration-release/**`
- `codex-plus-dev-plan/test-runs/**`
- E2E scripts under an approved tools path
- release gate templates and sanitized evidence

## Forbidden Scope

Module I must not write:

- backend handler/service/gateway implementation
- Ent schema or migrations
- desktop Rust runtime or provider writer
- desktop React feature code
- admin Vue feature code
- contract artifacts unless Module J approves a contract patch flow
- lockfiles, package manifests or production deployment files

## Verification

Prep-mode verification:

- every E2E step names its required owner module
- every status/error maps to contract table
- every expected API path maps to OpenAPI
- every failure case states whether enforcement is backend, gateway, desktop local or admin-visible

Final-mode verification:

- run `INTEGRATION-VERIFICATION-CHECKLIST.md`
- run the local/MVP E2E phase from `CODEX-AUTONOMOUS-TEST-RUNBOOK.md`
- record commands, screenshots and sanitized evidence
- produce `12-release-gate-report.md`

## Handoff To Module J

Module I hands Module J:

- evidence folder path
- release gate decision
- defect list grouped by P0/P1/P2/P3
- environment/build versions
- config version and contract version tested
- commands executed and unavailable commands
- rollback notes and unresolved blockers
