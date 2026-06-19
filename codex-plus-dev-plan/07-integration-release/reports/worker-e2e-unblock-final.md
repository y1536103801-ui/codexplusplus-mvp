# Worker 4B E2E Credential And Authorization Checklist

Report status: final
Worker lane: E2E unblock
Run stamp: 20260618-2202-e2e-unblock
Release evidence: no
Final recommendation: no-go until real E2E inputs and authorizations are supplied

## Scope

This worker converted the Module J and Worker 2A no-go blockers into a safe credential and authorization package. It did not run real gateway, browser handoff, admin audit, desktop provider write, payment, entitlement, or redeem-code requests.

## Files Changed

- `codex-plus-dev-plan/test-runs/20260618-2202-e2e-unblock/00-unblock-summary.md`
- `codex-plus-dev-plan/test-runs/20260618-2202-e2e-unblock/01-e2e-credential-env.template.ps1`
- `codex-plus-dev-plan/test-runs/20260618-2202-e2e-unblock/02-env-var-inventory.md`
- `codex-plus-dev-plan/test-runs/20260618-2202-e2e-unblock/03-test-persona-matrix.md`
- `codex-plus-dev-plan/test-runs/20260618-2202-e2e-unblock/04-authorization-checklist.md`
- `codex-plus-dev-plan/test-runs/20260618-2202-e2e-unblock/05-safe-credential-handling.md`
- `codex-plus-dev-plan/test-runs/20260618-2202-e2e-unblock/06-static-checks.md`
- `codex-plus-dev-plan/test-runs/20260618-2202-e2e-unblock/parser-check-output.txt`
- `codex-plus-dev-plan/test-runs/20260618-2202-e2e-unblock/readiness-placeholder-output.txt`
- `codex-plus-dev-plan/test-runs/20260618-2202-e2e-unblock/readiness-placeholder-report.md`
- `codex-plus-dev-plan/test-runs/20260618-2202-e2e-unblock/secret-pattern-scan-output.txt`
- `sub2api-main/tools/e2e/codexplus/e2e-credentials.example.ps1`
- `sub2api-main/tools/e2e/codexplus/README.md`
- `codex-plus-dev-plan/07-integration-release/reports/worker-e2e-unblock-final.md`

## Script Inputs Extracted

Required non-secret inputs:

- `CODEXPLUS_07_E2E_BACKEND_BASE_URL`
- `CODEXPLUS_07_E2E_ADMIN_BASE_URL`
- `CODEXPLUS_07_E2E_GATEWAY_BASE_URL`
- `CODEXPLUS_07_E2E_MANAGER_BUILD_PATH`
- `CODEXPLUS_07_E2E_USER_ACTIVE_ID`
- `CODEXPLUS_07_E2E_USER_NOT_PURCHASED_ID`
- `CODEXPLUS_07_E2E_USER_EXPIRED_ID`
- `CODEXPLUS_07_E2E_USER_LOW_BALANCE_ID`
- `CODEXPLUS_07_E2E_USER_DEVICE_REVOKED_ID`
- `CODEXPLUS_07_E2E_USER_MODEL_DENIED_ID`
- `CODEXPLUS_07_E2E_TEST_DEVICE_ID`
- `CODEXPLUS_07_E2E_ALLOWED_TEST_MODEL`
- `CODEXPLUS_07_E2E_DENIED_TEST_MODEL`

Required secret inputs:

- `CODEXPLUS_07_E2E_ADMIN_TOKEN`
- `CODEXPLUS_07_E2E_USER_ACTIVE_TOKEN`
- `CODEXPLUS_07_E2E_USER_NOT_PURCHASED_TOKEN`
- `CODEXPLUS_07_E2E_USER_EXPIRED_TOKEN`
- `CODEXPLUS_07_E2E_USER_LOW_BALANCE_TOKEN`
- `CODEXPLUS_07_E2E_USER_DEVICE_REVOKED_TOKEN`
- `CODEXPLUS_07_E2E_USER_MODEL_DENIED_TOKEN`
- `CODEXPLUS_07_E2E_USER_ACTIVE_GATEWAY_KEY`
- `CODEXPLUS_07_E2E_USER_NOT_PURCHASED_GATEWAY_KEY`
- `CODEXPLUS_07_E2E_USER_EXPIRED_GATEWAY_KEY`
- `CODEXPLUS_07_E2E_USER_LOW_BALANCE_GATEWAY_KEY`
- `CODEXPLUS_07_E2E_USER_DEVICE_REVOKED_GATEWAY_KEY`
- `CODEXPLUS_07_E2E_USER_MODEL_DENIED_GATEWAY_KEY`

Optional or phase-specific secret inputs:

- `CODEXPLUS_07_E2E_BROWSER_AUTH_TOKEN`, only with browser handoff completion approval.
- `CODEXPLUS_07_E2E_REDEEM_CODE`, only with redeem mutation approval.

## Test Persona Matrix

The package defines six required personas:

- active: allowed model should pass gateway and produce `usage_recorded`.
- not-purchased: allowed model should reject with `GATEWAY_POLICY_ENTITLEMENT_NOT_PURCHASED`.
- expired: allowed model should reject with `GATEWAY_POLICY_ENTITLEMENT_EXPIRED`.
- low-balance: allowed model should reject with `GATEWAY_POLICY_BALANCE_INSUFFICIENT` or `GATEWAY_POLICY_QUOTA_EXCEEDED`.
- revoked-device: allowed model should reject with `GATEWAY_POLICY_DEVICE_REVOKED` or `GATEWAY_POLICY_DEVICE_BLOCKED`.
- model-denied: denied model should reject with `GATEWAY_POLICY_MODEL_NOT_ALLOWED`.

Each persona requires a client token, gateway key, numeric user ID, and matching admin audit expectation. Details are in `03-test-persona-matrix.md`.

## Authorization Required Before Running

The next worker must ask for explicit user authorization before:

- starting browser handoff sessions with `-AllowSessionStart`;
- completing browser handoff with `-AllowBrowserComplete` and `BROWSER_AUTH_TOKEN`;
- sending low-cost gateway requests with `-AllowGatewayRequests`;
- reading admin audit/event rows with `-AllowAdminAuditReads`;
- writing desktop provider/settings state;
- launching Codex through the managed provider if it can trigger model traffic;
- mutating payment, entitlement, redeem, or device state;
- using `-AllowProduction` for any production-looking URL.

The approval record format is in `04-authorization-checklist.md`.

## Safe Checks Run

- PowerShell parser check: pass, 0 parser errors across readiness, local runner, client API, browser handoff, gateway policy, admin audit, example env, and unblock env template.
- Placeholder readiness import: expected fail, exit 1, 26 failing checks, no HTTP probe, no endpoint preflight, release evidence `no`.
- Secret pattern scan: pass, 0 high-confidence secret patterns found.

## Still Missing Real Inputs

- Real nonproduction backend, admin, and gateway URLs.
- Existing Manager build path.
- Test admin token or approved test admin credentials.
- Six persona client tokens.
- Six persona gateway keys.
- Six numeric user IDs.
- Stable test device ID decision.
- Allowed and denied model names.
- Browser authenticated token if browser-complete is authorized.
- Desktop/Rust-capable runtime path for provider write and Codex launch evidence.
- Explicit operation approvals listed above.

## Safe Credential Handoff

The user should not paste secrets into chat. Use one of these methods:

- copy `codex-plus-dev-plan/test-runs/20260618-2202-e2e-unblock/01-e2e-credential-env.template.ps1` to a local uncommitted `e2e-env.local.ps1` and fill it locally;
- or copy `sub2api-main/tools/e2e/codexplus/e2e-credentials.example.ps1` to a local uncommitted env file;
- or set secrets only in the current PowerShell session.

Then run readiness with `-EnvFile` and no E2E action switches first.

## Next Worker

After credentials are filled locally and the user explicitly authorizes the requested operations, start Worker 2A Real E2E Evidence Retry. It should use this unblock package, run readiness first, then collect real sanitized E2E evidence under a new `YYYYMMDD-HHMM-e2e` folder without converting missing evidence into pass.
