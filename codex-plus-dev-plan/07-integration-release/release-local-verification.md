# 07 Local Release Verification

Date: 2026-06-18
Worker: 1A Runtime Readiness
Run stamp: `20260618-2046-e2e-env`
Status: local source route readiness partially passed; full E2E release evidence still blocked
Release recommendation: no-go

## Current Environment Snapshot

This file records the current local runtime state for Worker 1A only. It is not a Module I E2E result, package result, compatibility result, business approval, or Module J release decision.

Evidence folder:

- `codex-plus-dev-plan/test-runs/20260618-2046-e2e-env/00-runtime-readiness.md`
- `codex-plus-dev-plan/test-runs/20260618-2046-e2e-env/local-source-service-probe.txt`
- `codex-plus-dev-plan/test-runs/20260618-2046-e2e-env/local-route-preflight.md`
- `codex-plus-dev-plan/test-runs/20260618-2046-e2e-env/rust-preflight-output.txt`
- `codex-plus-dev-plan/test-runs/20260618-2046-e2e-env/toolchain-status.json`
- `codex-plus-dev-plan/test-runs/20260618-2046-e2e-env/docker-containers.txt`
- `codex-plus-dev-plan/test-runs/20260618-2046-e2e-env/docker-version.json`

## Docker And Local Source Service

| Item | Current result |
| --- | --- |
| Docker Desktop initial state | Installed but not running; `com.docker.service` was stopped and daemon pipe was unavailable |
| Docker startup | `Docker Desktop.exe` started successfully from this session |
| Docker daemon | Reachable after startup |
| Docker version | Docker 29.3.1, build c2be9cc |
| Local source compose config | Passed |
| First local source build attempt | Failed at `go mod download` because `goproxy.cn` returned `unexpected EOF` for `github.com/smartwalle/ngx@v1.1.0` |
| Runtime readiness fix | `docker-compose.dev.yml`, `.env.codexplus-local.example`, and local start scripts now expose configurable `GOPROXY`/`GOSUMDB` build args |
| Successful local source start | Passed with `-GoProxy 'https://goproxy.cn|https://proxy.golang.org|direct'` |
| App endpoint | `http://127.0.0.1:8081` |
| App container | `sub2api-codexplus-local`, image `sub2api-codexplus-local:dev`, healthy |
| Postgres container | `sub2api-codexplus-postgres`, healthy |
| Redis container | `sub2api-codexplus-redis`, healthy |

## Local Route Results

| Probe | HTTP result | Interpretation |
| --- | --- | --- |
| `GET /` | 200 | root responds |
| `GET /health` | 200 | health responds |
| `GET /api/v1/client/bootstrap` | 401 | 07 client route exists and requires auth |
| `POST /api/v1/auth/desktop/poll` | 400 | 07 desktop route exists and validates request input |
| `GET /api/v1/admin/codex-plus/config` | 401 | 07 admin route exists and requires auth/admin state |
| `POST /v1/responses` | 401 | gateway route exists and requires auth |

These route results prove that the current workspace source is exposing the expected 07 route surface on the local service. They do not prove purchase, entitlement, browser handoff, desktop launch, managed provider write, gateway success, package install, compatibility migration, or business readiness.

## Toolchain Status

| Tool | Current result |
| --- | --- |
| `go version` | not found on PATH |
| `node --version` | v24.14.1 |
| `npm --version` | 11.11.0 |
| `corepack pnpm --version` | 11.4.0 |
| `rustc --version` | not found on PATH |
| `cargo --version` | not found on PATH |
| Docker | Docker version 29.3.1, build c2be9cc |

Go is available inside the Docker build image, so the local source container can build and run. Host-side Go verification still requires installing Go locally or using CI.

Rust desktop verification still requires a local or CI Rust toolchain plus linker setup.

## Required Command Results

| Command | Result | Details |
| --- | --- | --- |
| `powershell -ExecutionPolicy Bypass -File codex-plus-dev-plan/tools/verify-07-e2e-readiness.ps1 -EndpointPreflightOnly -OutputPath codex-plus-dev-plan/test-runs/20260618-2046-e2e-env/local-route-preflight.md` | failed, exit 1 | Environment URL and manager path checks passed. 07 route allowlist probes passed. The script-level base URL probes failed under Windows PowerShell 5.1 `Invoke-WebRequest` with `Object reference not set to an instance of an object`; manual `Invoke-WebRequest -UseBasicParsing` checks against `http://127.0.0.1:8081`, `/`, and `/health` returned HTTP 200. |
| `powershell -ExecutionPolicy Bypass -File codex-plus-dev-plan/tools/verify-07-rust-preflight.ps1` | failed, exit 1 | Missing `cargo`, `rustc`, `rustup`, `link.exe`, `dlltool.exe`, and workspace-local `work/rust-toolchain` / `work/w64devkit`. Disk space passed with 447.72 GB free on `F:\`. |

## Worker 2A Handoff

Worker 2A can use the local source service on `http://127.0.0.1:8081` for route-level prep, but cannot complete real E2E yet.

Required missing inputs and permissions:

- Admin token or test admin credentials.
- Active paid/entitled user token.
- Not-purchased, expired, low-balance, revoked-device and model-denied user tokens.
- Numeric test user IDs for admin audit correlation.
- Test device ID.
- Allowed and denied model names.
- Persona gateway keys.
- Permission to execute gateway test requests.
- Permission to start and complete browser handoff sessions.
- Permission to read admin audit rows for test users.
- Desktop/Rust-capable local or CI environment for desktop launch and provider-write evidence.

## Static Local Verification Index

This section is a static/local verification index for the 07 verifier. It does not convert pending release evidence into pass evidence.

Current static/local summary:

- local verification passed only for the local source route-surface checks and docs/html visual checks recorded in this file and `docs/html-sync-evidence.md`.
- release evidence pending remains the release posture for real E2E, package, compatibility, business readiness and aggregate handoff evidence.
- release recommendation no-go remains unchanged.

Host-side commands not proven as release-pass evidence in this worker:

- `go test ./...`
- `npm run typecheck`
- `npm run build`
- `npm run vite:build`
- `cargo fmt --check -p codex-plus-core`

HTML/docs evidence already available:

- `quotaProgress` is the current HTML runtime-neutral quota progress field, and no stale fixed quota CSS value is claimed.
- Local Chromium desktop/mobile screenshot evidence passed for `codex-plus-product-spec.html`.
- In-app browser HTTP preview evidence passed after direct local `file://` navigation was blocked by URL policy.
- Boundary file: `codex-plus-dev-plan/07-integration-release/docs/html-visual-evidence/in-app-browser-policy-boundary.md`.

E2E environment and runner boundary:

- `verify-07-e2e-readiness.ps1` was run for endpoint-preflight and produced partial route evidence, but the full test-environment readiness gate is still blocked by real credential and permission inputs.
- `new-07-e2e-env-template.ps1` and `CODEXPLUS_07_E2E` env names are part of the unblock/template path, not release-pass evidence by themselves.
- `run-client-api-checks.ps1` was not run as release E2E in this worker; client API subset remains unproven beyond route-existence probes.
- `run-browser-handoff-checks.ps1` requires explicit `AllowSessionStart` and `AllowBrowserComplete`; those approvals were not supplied.
- `run-gateway-policy-checks.ps1` requires explicit `AllowGatewayRequests`; that approval was not supplied.
- `run-admin-audit-checks.ps1` requires explicit `AllowAdminAuditReads`; that approval was not supplied.
- `run-local-e2e.ps1` was not run as real release E2E.
- `-SkipHandoff` is not release evidence and must not be used to claim browser handoff completion.
- `start-local-dev-compose.ps1`, `start-local-source-service.ps1`, `docker-compose.dev.yml`, `.env.codexplus-local.example`, `.codexplus-local`, `sub2api-codexplus-local` and `127.0.0.1:8081` document the local source route only.

Rust/toolchain boundary:

- `verify-07-rust-preflight.ps1` found missing `rust-toolchain`/Cargo/Rust/linker components.
- Disk space currently passed with 447.72 GB free on `F:\`. The stale static anchor `9.56GB` is not current host evidence and must not be treated as the present disk result.

Evidence tooling and aggregate boundary:

- `test-07-evidence-tooling.ps1` has a self-test log under `codex-plus-dev-plan/test-runs/evidence-tooling-full-20260618-192137.out.log`.
- `new-07-release-evidence-set.ps1` and `new-07-evidence-run.ps1` generate scaffold/evidence structures but do not replace real release evidence.
- `new-07-package-evidence.ps1`, `inspect-07-package-artifacts.ps1` and `verify-07-package-evidence.ps1` remain package-lane tooling; this worker did not edit package evidence.
- `new-07-compatibility-evidence.ps1`, `inspect-07-compatibility-snapshots.ps1` and `verify-07-compatibility-evidence.ps1` remain compatibility-lane tooling.
- `report-07-release-gaps.ps1`, `new-07-business-readiness-evidence.ps1`, `verify-07-business-readiness.ps1`, `verify-07-release-evidence.ps1`, `summarize-07-release-coverage.ps1`, `summarize-07-release-readiness.ps1` and `verify-07-release-handoff.ps1` currently support a no-go release handoff until real external evidence is available.

## Release Boundary

The release remains no-go. Worker 1A made the local source service bootable and diagnosable on this host, but the final MVP still requires real E2E, package, compatibility, docs/business aggregation, and Module J handoff evidence under the final release run stamp.
