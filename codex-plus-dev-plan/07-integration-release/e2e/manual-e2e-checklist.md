# Manual E2E Checklist: Buy, Login, Bootstrap, Launch

Status: scaffold only
Evidence status: E2E evidence pending

This checklist is prepared for the Module I / 07-integration-release E2E lane. It is not a pass result. Execute only in a production-equivalent test environment with Turnstile enabled, test users, test entitlement/payment setup, and safe upstream model accounts.

## Preconditions

- [ ] Module D client API final report is available.
- [ ] Module E gateway enforcement final report is available.
- [ ] Module F desktop runtime final report is available.
- [ ] Module G desktop UX final report is available.
- [ ] Module H admin operations final report is available.
- [ ] Coordinator provides backend, gateway, admin frontend, and desktop build versions.
- [ ] Test environment has Turnstile enabled.
- [ ] Test user accounts are available: `admin_test`, `user_active`, `user_not_purchased`, `user_expired`, `user_low_balance`, `user_device_revoked`, `user_model_denied`.
- [ ] Test plan/model policy exists and does not use production payment orders or real production credentials.
- [ ] Optional local env prep was generated with `tools/new-07-e2e-env-template.ps1`, then all required token, key and model placeholders were filled manually before use.
- [ ] `tools/verify-07-e2e-readiness.ps1` passes with `CODEXPLUS_07_E2E_*` variables or `-EnvFile <template.ps1>` for backend/admin URLs, Manager build path, test-account tokens, numeric test user IDs, test device ID and allowed/denied model names. If service-image mismatch is suspected, run `-EndpointPreflight` against the 07 client/admin/desktop/gateway routes.
- [ ] Evidence folder is created under `codex-plus-dev-plan/test-runs/YYYYMMDD-HHMM-e2e/`.
- [ ] Evidence folder was generated with `tools/new-07-evidence-run.ps1` or manually matches the same 13-file structure.
- [ ] Optional prep runner `sub2api-main/tools/e2e/codexplus/run-local-e2e.ps1` has been run for the client API subset, optionally with `-EnvFile`, or equivalent sanitized client API evidence has been supplied.
- [ ] Optional browser handoff prep runner `sub2api-main/tools/e2e/codexplus/run-browser-handoff-checks.ps1` has been run with `-AllowSessionStart`, and with `-AllowBrowserComplete` only after a real authenticated browser session token is available, or equivalent sanitized browser handoff evidence has been supplied.
- [ ] Optional gateway prep runner `sub2api-main/tools/e2e/codexplus/run-gateway-policy-checks.ps1` has been run with `-AllowGatewayRequests`, or equivalent sanitized gateway policy evidence has been supplied.
- [ ] If admin compliance acknowledgement is required in the local test stack, run `tools/accept-07-local-admin-compliance.ps1` only after explicit owner authorization and only against a localhost/127.0.0.1 admin URL.
- [ ] Optional admin audit prep runner `sub2api-main/tools/e2e/codexplus/run-admin-audit-checks.ps1` has been run with `-AllowAdminAuditReads` after gateway probes, or equivalent sanitized admin audit/event evidence has been supplied.
- [ ] Optional isolated desktop harness was generated with `tools/new-07-desktop-compatibility-harness.ps1` before Desktop Manager runtime testing. This prepares isolated `USERPROFILE`, `HOME`, `APPDATA`, `LOCALAPPDATA`, and `CODEX_HOME` values, but it does not launch Manager or prove runtime E2E by itself.
- [ ] Optional provider snapshots were captured with `tools/capture-07-desktop-provider-snapshot.ps1` or the harness-local `capture-snapshot.ps1`; snapshots store provider names and URL/key hashes only, not raw upstream secrets.
- [ ] Env template/checklist output and endpoint preflight output are treated as prep status only, not as release evidence.
- [ ] Files `02-contract-checks.md` through `10-rollback-notes.md` each include `Result: pass` or `Result: fail`.

## Required Evidence Files

- [ ] `00-environment.md`
- [ ] `01-test-accounts.md`
- [ ] `02-contract-checks.md`
- [ ] `03-admin-setup.md`
- [ ] `04-client-api-e2e.md`
- [ ] `05-gateway-policy-e2e.md`
- [ ] `06-desktop-manager-e2e.md`
- [ ] `07-package-install-check.md`
- [ ] `08-compatibility-migration.md`
- [ ] `09-usage-events-audit.md`
- [ ] `10-rollback-notes.md`
- [ ] `11-defects.md`
- [ ] `12-release-gate-report.md`
- [ ] `tools/verify-07-evidence.ps1` passes against the evidence folder.

## Happy Path

- [ ] Record backend, gateway, admin frontend, desktop build, contract, and config versions.
- [ ] Log in as `admin_test`.
- [ ] Confirm or create a safe test Codex++ plan.
- [ ] Confirm allowed model group and default model come from backend config.
- [ ] Confirm or grant active entitlement for `user_active`.
- [ ] Launch Codex++ Manager from the test desktop build.
- [ ] Start browser handoff login from Manager.
- [ ] Confirm desktop opens `/auth/desktop/authorize` from `/api/v1/auth/desktop/start`.
- [ ] Confirm the browser URL does not expose `poll_token`.
- [ ] Complete Web login, Turnstile, 2FA if required, and 6 digit authorization code confirmation.
- [ ] Confirm desktop polling `/api/v1/auth/desktop/poll` receives JWT only after authorization is complete.
- [ ] If using `run-browser-handoff-checks.ps1`, review that `-AllowSessionStart` was explicit, `poll_token` was absent from the redacted authorize URL, and `-AllowBrowserComplete` used only a browser-authenticated test token from environment variables.
- [ ] Register or refresh device through the client API.
- [ ] Fetch bootstrap.
- [ ] Confirm bootstrap status is `available`.
- [ ] Confirm bootstrap does not leak upstream credentials.
- [ ] If using `run-client-api-checks.ps1`, optionally pass `-EnvFile` and `-EndpointPreflight`, then review its client API subset table before continuing to browser, desktop and gateway steps.
- [ ] Confirm `Codex++ Cloud` provider is written or repaired.
- [ ] Confirm existing manual providers remain present.
- [ ] Capture a post-login/post-upgrade provider snapshot from the isolated profile.
- [ ] Launch Codex from Manager.
- [ ] Send one low-cost model request through the Sub2API gateway.
- [ ] Confirm the request succeeds.
- [ ] If using `run-gateway-policy-checks.ps1`, review its gateway policy subset table and confirm `user_active` succeeded while blocked personas were rejected with safe `request_id`, `GATEWAY_POLICY_*` and service status fields.
- [ ] If using `run-admin-audit-checks.ps1`, review `09-usage-events-audit.md` and confirm `usage_recorded` and `gateway_policy_rejected` rows include request IDs that match the gateway rows in `05-gateway-policy-e2e.md`, plus `config_version`, `GATEWAY_POLICY_*`, matching test device ID and `redaction_applied`.
- [ ] Confirm usage and audit/event rows are visible and redacted.
- [ ] Capture logout and rollback provider snapshots after those runtime actions complete.

Result: E2E evidence pending until executed.

## Compatibility Login Paths

- [ ] Verify `/api/v1/auth/login` remains compatibility-only.
- [ ] Verify `/api/v1/auth/login/2fa` remains compatibility-only.
- [ ] Confirm Turnstile-enabled production-equivalent login succeeds through browser handoff.
- [ ] If desktop password login is the only working path, mark release gate as no-go.

Result: E2E evidence pending until executed.

## Failure Paths

- [ ] `user_not_purchased`: bootstrap returns `not_purchased` or approved equivalent status.
- [ ] `user_not_purchased`: stale local provider cannot access paid gateway usage.
- [ ] `user_expired`: bootstrap and/or gateway blocks usage with expired status.
- [ ] `user_low_balance`: gateway blocks usage according to MVP balance policy.
- [ ] `user_device_revoked`: revoked device blocks bootstrap or normal launch path.
- [ ] `user_model_denied`: unauthorized model request is rejected server-side.
- [ ] Removed or disabled default model disappears after config refresh or is rejected server-side.
- [ ] Gateway unhealthy state shows an actionable desktop message.
- [ ] Missing local Codex or config write failure shows a local repair hint.

Result: E2E evidence pending until executed.

## Decoupling Checks

- [ ] Client does not hard-code prices.
- [ ] Client does not hard-code model list.
- [ ] Client does not hard-code quota or balance thresholds.
- [ ] Admin default model change is reflected in bootstrap without desktop release.
- [ ] Desktop consumes refreshed config snapshot.

Result: E2E evidence pending until executed.

## Redaction Requirements

- [ ] No real API Key is stored in evidence.
- [ ] No JWT or refresh token is stored in evidence.
- [ ] No `Authorization` header is stored in evidence.
- [ ] No upstream provider Key is stored in evidence.
- [ ] No desktop-private `poll_token` or `session_token` is stored in evidence.
- [ ] No `TODO`, `TBD`, `PLACEHOLDER`, `NOT_EXECUTED`, `FILL_ME`, or `pending` placeholder remains in evidence.
- [ ] Screenshots are reviewed and redacted before sharing.
- [ ] Logs are reviewed and redacted before sharing.

## Release Gate Notes

- Do not mark this checklist as passed without a real test environment.
- Do not bypass payment, entitlement, backend policy, or gateway enforcement.
- Do not manually fill API Keys for the happy path.
- Record blockers as blockers instead of substituting weaker checks.
