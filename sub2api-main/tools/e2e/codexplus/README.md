# Codex++ 07 E2E Runners

These scripts are Module I prep-mode helpers for the `07-integration-release` gate. They make the E2E run more executable, but they do not replace the production-equivalent browser, desktop, gateway, package, compatibility or Module J release decision evidence.

## Scripts

- `run-client-api-checks.ps1`: runs the non-destructive client API subset against the selected test backend and writes sanitized observations into a 07 E2E evidence folder. It accepts `-EnvFile` and can run `-EndpointPreflight` before the API checks.
- `run-browser-handoff-checks.ps1`: runs the desktop browser handoff subset for `/auth/desktop/start`, pending poll, optional authenticated browser complete, and completed poll. Real session creation requires `-AllowSessionStart`; real browser approval requires `-AllowBrowserComplete` plus `CODEXPLUS_07_E2E_BROWSER_AUTH_TOKEN`.
- `run-local-e2e.ps1`: creates the standard 07 E2E evidence scaffold, then runs the selected client API, browser handoff, gateway and admin audit subset runners against that folder. It accepts `-EnvFile`.
- `run-gateway-policy-checks.ps1`: runs the gateway policy subset for one active-user request and required rejection probes, but only when `-AllowGatewayRequests` is supplied. It retains safe `request_id`, `GATEWAY_POLICY_*`, service status and body-parse fields while redacting response bodies.
- `run-admin-audit-checks.ps1`: reads admin-visible Codex++ event rows for success/rejection personas and writes sanitized usage/audit correlation evidence, including matched gateway `request_id` values from `05-gateway-policy-e2e.md`, but only when `-AllowAdminAuditReads` is supplied.
- `start-local-dev-compose.ps1`: starts the isolated `docker-compose.dev.yml` local source stack with the fixed `sub2api-codexplus-local` project name, then probes the 07 routes.
- `start-local-source-service.ps1`: builds or probes a current-source local Sub2API image and checks that the 07 client/admin/desktop/gateway routes return auth or validation responses instead of 404.

## Local Source Runtime

For repeatable local inspection without replacing an existing upstream `8080` deployment, use the isolated dev compose entry:

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File sub2api-main/tools/e2e/codexplus/start-local-dev-compose.ps1 -Root (Resolve-Path .) -InitEnv
```

The wrapper creates `.env.codexplus-local` from the example when needed, starts the compose stack with the fixed `sub2api-codexplus-local` project name, verifies the app container is compose-owned, and then runs the 07 route preflight.

```powershell
cd sub2api-main/deploy
Copy-Item .env.codexplus-local.example .env.codexplus-local
# Edit .env.codexplus-local and replace placeholder passwords.
# Generate local 64-hex secrets when you need fresh values:
# [Convert]::ToHexString([Security.Cryptography.RandomNumberGenerator]::GetBytes(32)).ToLowerInvariant()
docker compose --env-file .env.codexplus-local -p sub2api-codexplus-local -f docker-compose.dev.yml up -d --build
```

The default local source URL is `http://127.0.0.1:8081`, with app/postgres/redis data isolated under `deploy/.codexplus-local`. The example env file contains valid local-only 64-hex `JWT_SECRET` and `TOTP_ENCRYPTION_KEY` values so a copied file can boot; replace both before any shared, long-lived, or production-like use. The generated `.env.codexplus-local` and `.codexplus-local/` directory are ignored by git.

After the service is running, probe the 07 routes without rebuilding:

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File sub2api-main/tools/e2e/codexplus/start-local-source-service.ps1 -Root (Resolve-Path .) -SkipBuild -ProbeOnly -HostPort 8081
```

If `8081` is already occupied, set `SUB2API_DEV_HOST_PORT=8082` in `.env.codexplus-local` and pass `-HostPort 8082` to the wrapper and probe. Use `start-local-source-service.ps1 -ProbeOnly` for route preflight of a running local source service; its non-`ProbeOnly` mode is a compatibility helper for building/running against an existing `sub2api` Docker stack. Prefer `start-local-dev-compose.ps1` or `docker-compose.dev.yml` for a fresh isolated local-source stack.

## Required Environment

Optionally run `codex-plus-dev-plan/tools/new-07-e2e-env-template.ps1` first. It creates `test-runs/YYYYMMDD-HHMM-e2e-env/e2e-env.template.ps1` and `e2e-env-checklist.md`, fills detected local URLs and Manager build candidate paths, and marks token, key and model values that must be filled manually.

For the isolated local source service on `8081`, generate a template with explicit local URLs:

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File codex-plus-dev-plan/tools/new-07-e2e-env-template.ps1 -Root (Resolve-Path .) -BackendBaseUrl http://127.0.0.1:8081 -AdminBaseUrl http://127.0.0.1:8081 -GatewayBaseUrl http://127.0.0.1:8081 -ProbeHttp
```

Run `codex-plus-dev-plan/tools/verify-07-e2e-readiness.ps1` before the runners. The default input prefix is `CODEXPLUS_07_E2E_` and includes backend/admin URLs, Manager build path, test-account tokens, numeric test user IDs for admin audit correlation, test device ID and allowed/denied model names. Use `-EnvFile <template.ps1>` to load a local env file instead, and use `-EndpointPreflight` to probe the 07 client/admin/desktop/gateway routes when checking for a non-07 service image.

When you only need a local route diagnostic, use `-EndpointPreflightOnly` with `-OutputPath`. This mode skips token/model/persona checks, writes a prep report, and is not release evidence:

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File codex-plus-dev-plan/tools/verify-07-e2e-readiness.ps1 -Root (Resolve-Path .) -EnvFile codex-plus-dev-plan/test-runs/YYYYMMDD-HHMM-e2e-env/e2e-env.template.ps1 -EndpointPreflightOnly -OutputPath codex-plus-dev-plan/test-runs/YYYYMMDD-HHMM-e2e-env/8081-local-preflight.md
```

Run full readiness without `-EndpointPreflightOnly` when real test tokens, model names and gateway keys have been filled. With generated placeholder values, full readiness must fail; use `-OutputPath` if you want that failure captured as a prep diagnostic.

The runner checks token presence but does not print token values. Production-looking URLs require the explicit `-AllowProduction` override. HTTP probing remains opt-in.

## Boundary

The client API runner covers bootstrap, usage and idempotent test-device refresh. Redeem is skipped unless `-AllowRedeem` is supplied with a test redeem code.

The browser handoff runner requires `CODEXPLUS_07_E2E_BACKEND_BASE_URL` and `CODEXPLUS_07_E2E_TEST_DEVICE_ID` for real session creation. It refuses to create a pending desktop session unless `-AllowSessionStart` is supplied, and it refuses to call browser completion unless `-AllowBrowserComplete` is supplied with `CODEXPLUS_07_E2E_BROWSER_AUTH_TOKEN` from an already authenticated browser session. It records only status, HTTP code, token presence booleans, and redaction notes; it never prints session token, poll token, browser JWT, desktop access token, refresh token, or Authorization header values.

The gateway policy runner requires `CODEXPLUS_07_E2E_GATEWAY_BASE_URL` and per-persona user-side gateway keys such as `CODEXPLUS_07_E2E_USER_ACTIVE_GATEWAY_KEY`. It never prints key values. It refuses to send requests unless `-AllowGatewayRequests` is supplied in an approved low-cost test environment, and it requires each rejection to expose safe structured policy fields such as `request_id`, `GATEWAY_POLICY_*` and service status.

The admin audit runner requires `CODEXPLUS_07_E2E_ADMIN_BASE_URL`, `CODEXPLUS_07_E2E_ADMIN_TOKEN`, `CODEXPLUS_07_E2E_TEST_DEVICE_ID` and numeric user IDs such as `CODEXPLUS_07_E2E_USER_ACTIVE_ID`. It never prints token values or raw event payloads. It refuses to read admin event rows unless `-AllowAdminAuditReads` is supplied in an approved test environment. Run it after gateway policy probes when you want `09-usage-events-audit.md` to prove `usage_recorded` and `gateway_policy_rejected` rows include gateway-matched `request_id`, `config_version`, `GATEWAY_POLICY_*`, matching test device ID and `redaction_applied`.

The env template/checklist, endpoint preflight and fixture-mode runner output are prep diagnostics only. Browser handoff completion from a real Web session, desktop launch, package install and compatibility migration remain separate release evidence requirements, and no prep diagnostic overrides no-go release criteria.
