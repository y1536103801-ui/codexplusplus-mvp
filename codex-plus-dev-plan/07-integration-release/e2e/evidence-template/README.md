# E2E Evidence Template

Status: scaffold only
Evidence status: E2E evidence pending

Use this template when the Module I E2E gate is executed in a production-equivalent test environment. This directory is a scaffold and does not contain executed evidence.

## Suggested Run Folder

Create one timestamped folder per execution:

```text
codex-plus-dev-plan/test-runs/YYYYMMDD-HHMM-e2e/
```

You can generate the standard file set with:

```text
powershell -ExecutionPolicy Bypass -File codex-plus-dev-plan/tools/new-07-evidence-run.ps1
```

The generated files intentionally contain `TODO` placeholders and `Result: pending` markers. Replace every placeholder with sanitized execution evidence and set each key result to `Result: pass` or `Result: fail` before treating the folder as a candidate release evidence package.

Recommended files:

```text
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

## Minimum Content Per File

`00-environment.md`
- Backend, gateway, admin frontend, desktop build, contract, and config versions.
- Base URLs with secrets removed.
- Test date, executor, and environment owner.

`01-test-accounts.md`
- Sanitized account matrix.
- Entitlement state for each test user.
- Missing account setup blockers, if any.

`02-contract-checks.md`
- `Result: pass` or `Result: fail`.
- Client API, auth handoff, bootstrap, usage, device, and redeem contract checks.
- Any contract mismatch.

`03-admin-setup.md`
- `Result: pass` or `Result: fail`.
- Test plan, model policy, default model, usage policy, and feature flag setup.
- Admin-visible entitlement/device/Key summaries with secrets removed.

`04-client-api-e2e.md`
- `Result: pass` or `Result: fail`.
- Browser handoff start, complete, and poll behavior.
- Exact `/api/v1/auth/desktop/start`, `/api/v1/auth/desktop/complete`, and `/api/v1/auth/desktop/poll` route observations.
- Confirmation that `poll_token` was not present in `authorize_url` and the 6 digit verification code was confirmed.
- Bootstrap status for each user state.
- Device registration or refresh behavior.

`05-gateway-policy-e2e.md`
- `Result: pass` or `Result: fail`.
- Successful active-user request.
- Rejection evidence for no entitlement, expired, insufficient balance, revoked device, and unauthorized model.
- Safe structured fields for each gateway row: `request_id`, `GATEWAY_POLICY_*` error code, service status, body parse status, and audit-correlation readiness.

`06-desktop-manager-e2e.md`
- `Result: pass` or `Result: fail`.
- Manager login, bootstrap fetch, `Codex++ Cloud` write or repair, and launch evidence.
- Manual provider preservation checks.

`07-package-install-check.md`
- `Result: pass` or `Result: fail`.
- Installer, first launch, missing Codex detection, and repair path checks.

`08-compatibility-migration.md`
- `Result: pass` or `Result: fail`.
- Old user and manual provider compatibility checks.
- Confirmation that password login is compatibility-only for Turnstile-enabled production-equivalent flow.

`09-usage-events-audit.md`
- `Result: pass` or `Result: fail`.
- Usage rows and admin/audit events for success and rejection paths.
- Admin audit/event rows for `usage_recorded` and `gateway_policy_rejected`, including gateway-matched `request_id`, `config_version`, `GATEWAY_POLICY_*`, service status, matching test device ID where supplied, and `redaction_applied`.
- Secret redaction confirmation.

`10-rollback-notes.md`
- `Result: pass` or `Result: fail`.
- Config rollback, backend rollback, desktop rollback, entitlement correction, leaked user-side Key response, and failed provider write recovery.

`11-defects.md`
- Defects grouped by P0, P1, P2, and P3.
- Owner, impact, mitigation, and release decision effect.

`12-release-gate-report.md`
- Final recommendation: `go`, `go with accepted risks`, or `no-go`.
- Commands executed.
- Evidence links.
- Remaining risks and rollback notes.

## Redaction Rules

- Never include real API Keys.
- Never include full JWTs or refresh tokens.
- Never include `Authorization` headers.
- Never include upstream provider credentials.
- Never include desktop-private `poll_token`.
- Redact screenshots and logs before committing or sharing.

## Local Evidence Verification

Before executing E2E, optionally generate a local env template for the selected machine:

```text
powershell -ExecutionPolicy Bypass -File codex-plus-dev-plan/tools/new-07-e2e-env-template.ps1
```

This creates `test-runs/YYYYMMDD-HHMM-e2e-env/e2e-env.template.ps1` and `e2e-env-checklist.md`, fills detected local URLs and Manager build candidate paths, and marks token, key and model values that must be completed manually. These files are execution prep aids only, not release evidence.

Validate the selected test environment without printing token values:

```text
powershell -ExecutionPolicy Bypass -File codex-plus-dev-plan/tools/verify-07-e2e-readiness.ps1
```

Set the `CODEXPLUS_07_E2E_*` variables for backend/admin URLs, Manager build path, test-account tokens, numeric test user IDs, test device ID and allowed/denied model names before running the readiness check, or load them with `-EnvFile codex-plus-dev-plan/test-runs/YYYYMMDD-HHMM-e2e-env/e2e-env.template.ps1`. Use `-EndpointPreflight` when you need to probe the 07 client/admin/desktop/gateway routes and catch a service image that is not the Codex++ 07 version. The readiness check and endpoint preflight only prove that execution inputs and selected routes are plausible; they do not execute E2E or create release evidence.

For local route diagnostics only, use `-EndpointPreflightOnly -OutputPath <report.md>`. That mode can pass without real token/model/persona values because it only proves the selected local/dev service exposes plausible 07 routes; it must not be attached as final release evidence.

To create the standard scaffold and run the selected client API, gateway and admin audit subsets:

```text
powershell -ExecutionPolicy Bypass -File sub2api-main/tools/e2e/codexplus/run-local-e2e.ps1 -EnvFile codex-plus-dev-plan/test-runs/YYYYMMDD-HHMM-e2e-env/e2e-env.template.ps1 -RunGatewayPolicy -AllowGatewayRequests -RunAdminAudit -AllowAdminAuditReads
```

To run only the client API subset against an existing evidence folder:

```text
powershell -ExecutionPolicy Bypass -File sub2api-main/tools/e2e/codexplus/run-client-api-checks.ps1 -EvidenceDir codex-plus-dev-plan/test-runs/YYYYMMDD-HHMM-e2e -EnvFile codex-plus-dev-plan/test-runs/YYYYMMDD-HHMM-e2e-env/e2e-env.template.ps1 -EndpointPreflight
```

The client API runner covers bootstrap, usage and idempotent test-device refresh only. `-EndpointPreflight` is diagnostic route probing only. Redeem remains opt-in, and the runner does not execute browser handoff completion, desktop launch, gateway model requests, package install, compatibility migration or payment flows.

To run the browser handoff subset against an existing evidence folder:

```text
powershell -ExecutionPolicy Bypass -File sub2api-main/tools/e2e/codexplus/run-browser-handoff-checks.ps1 -EvidenceDir codex-plus-dev-plan/test-runs/YYYYMMDD-HHMM-e2e -EnvFile codex-plus-dev-plan/test-runs/YYYYMMDD-HHMM-e2e-env/e2e-env.template.ps1 -AllowSessionStart
```

When a real browser login has completed and an authenticated browser JWT has been placed in `CODEXPLUS_07_E2E_BROWSER_AUTH_TOKEN`, rerun with `-AllowBrowserComplete` to approve the pending desktop session and poll for desktop token issuance. The runner records only status, HTTP code, token-presence booleans and redaction notes. It must not print session token, poll token, browser JWT, desktop access token, refresh token or Authorization values. Fixture-mode output is tooling self-test only and does not satisfy real Turnstile/Web login evidence.

To run the gateway policy subset against an existing evidence folder:

```text
powershell -ExecutionPolicy Bypass -File sub2api-main/tools/e2e/codexplus/run-gateway-policy-checks.ps1 -EvidenceDir codex-plus-dev-plan/test-runs/YYYYMMDD-HHMM-e2e -AllowGatewayRequests
```

The gateway runner requires a configured test gateway URL, user-side gateway keys for each persona, allowed/denied test model names and a test device ID. It refuses to send requests without `-AllowGatewayRequests`, keeps response bodies summarized, does not print gateway key values, and requires safe `request_id`, `GATEWAY_POLICY_*` and service status fields for rejection evidence.

To read admin-visible audit/event rows against an existing evidence folder after gateway probes:

```text
powershell -ExecutionPolicy Bypass -File sub2api-main/tools/e2e/codexplus/run-admin-audit-checks.ps1 -EvidenceDir codex-plus-dev-plan/test-runs/YYYYMMDD-HHMM-e2e -AllowAdminAuditReads
```

The admin audit runner requires admin URL/token, numeric test user IDs and the test device ID. It refuses to read rows without `-AllowAdminAuditReads`, omits raw event payloads, and only records counts, event types, gateway-matched `request_id`, `config_version`, `GATEWAY_POLICY_*`, service status and `redaction_applied` signals.

After a real E2E run creates a timestamped folder, run:

```text
powershell -ExecutionPolicy Bypass -File codex-plus-dev-plan/tools/verify-07-evidence.ps1 -EvidenceDir codex-plus-dev-plan/test-runs/YYYYMMDD-HHMM-e2e
```

The verifier checks the required 13 evidence files, key `Result: pass` / `Result: fail` markers, release-gate report shape, rollback/audit notes, and text evidence for obvious secret leakage or unfinished `pending` markers. A passing verifier result is necessary evidence hygiene, not a release `go` decision.

After E2E, package, compatibility and Docs product copy evidence folders all exist, run the aggregate release evidence gate:

```text
powershell -ExecutionPolicy Bypass -File codex-plus-dev-plan/tools/verify-07-release-evidence.ps1 -E2EEvidenceDir codex-plus-dev-plan/test-runs/YYYYMMDD-HHMM-e2e -PackageEvidenceDir codex-plus-dev-plan/test-runs/YYYYMMDD-HHMM-package -CompatibilityEvidenceDir codex-plus-dev-plan/test-runs/YYYYMMDD-HHMM-compatibility -DocsEvidenceDir codex-plus-dev-plan/test-runs/YYYYMMDD-HHMM-docs
```

The aggregate gate still proves evidence hygiene only; Module J owns the final recommendation.

## Current State

No E2E run has been executed from this scaffold. All evidence remains pending until a real test environment is provided.
