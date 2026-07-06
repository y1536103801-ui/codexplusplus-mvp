# Codex+++

Codex+++ is a Windows-first Codex launcher and operations platform described in `project-goals/`.

The current implementation contains:

1. A Go backend with admin APIs, client APIs, clean Codex+++ gateway APIs, PostgreSQL or local JSON persistence, token wallet logic, recharge confirmation, account pool routing, device controls, usage records, and audit records.
2. A browser-only administrator console served by the backend at `/admin`.
3. A static desktop client UI under `desktop-client/ui`, loaded by the Tauri shell in `desktop-client/src-tauri`.
4. PostgreSQL migrations and a Redis/PostgreSQL Docker Compose profile for the target deployment shape. PostgreSQL persists runtime state, Redis can enforce gateway request rate limits, and Docker deployment does not serve the ordinary-user client as a website.

## Reference Alignment

Codex+++ keeps two explicit reference boundaries:

- Codex++: use the same product boundary of a local Codex companion. The Windows client detects and launches local Codex, backs up and updates local Codex configuration, and keeps its own data outside the Codex install.
- sub2api: use the same operational boundary of an account-pool gateway. The backend owns upstream account import, platform API key generation, request forwarding, token-level usage records, recharge requests, and administrator confirmation.

Codex+++ intentionally does not expose OpenAI-compatible, Anthropic-compatible, sub2api-compatible, or public gateway product routes. The public surface stays unique and clean under `/api/admin`, `/api/client`, and the authenticated backend Codex adapter under `/api/codex/v1`; gateway execution remains an internal backend responsibility. The Windows desktop launch path does not use backend pool-account data as the local Codex account display source.

## Component Versioning

Backend and Windows client are managed as separate components. Routine backend changes belong to backend Git history. Routine Windows client changes belong to desktop-client Git history. The combined branch is an integration snapshot.

Client-backend API compatibility is controlled by `X-CodexPPP-Interop-Major`. Current value: `1`. The Windows client sends this header on every `/api/client/...` request. The backend rejects `/api/client/...` requests when the header is absent or different. Interop-major changes require paired backend and Windows-client changes in the same integration snapshot. Package patch versions such as the Tauri client version can move independently when `/api/client/...` request and response contracts stay on the same interop major.

## Run Backend

```powershell
cd backend
go run .
```

The backend defaults to `127.0.0.1:8787` for local Windows development, so it only accepts loopback traffic. For a deployed server that must accept network traffic, set `CODEXPPP_ADDR` explicitly, for example `0.0.0.0:8787` behind the intended firewall and reverse-proxy rules.

Cross-origin API access is not wildcarded. The backend allows same-origin admin traffic, the Windows Tauri desktop origins, and local development origins by default. Add deployment-specific desktop or webview origins with `CODEXPPP_CLIENT_ORIGINS`, as a comma-separated list of exact origins.

Open the admin console:

```text
http://localhost:8787/admin
```

The first admin setup is created from the admin console. The first admin must complete the first password-change step. Password content is intentionally unrestricted.

## Run PostgreSQL And Redis

```powershell
docker compose -f deploy/docker-compose.yml up -d postgres redis
```

The compose file starts:

- PostgreSQL on `localhost:54329`
- Redis on `localhost:63799`

The PostgreSQL container applies `backend/migrations/001_init.sql` on first initialization.

To run the backend against PostgreSQL from your host shell:

```powershell
$env:CODEXPPP_DATABASE_URL="postgres://codexppp:codexppp@localhost:54329/codexppp?sslmode=disable"
$env:CODEXPPP_SECRET="<set-a-long-random-secret>"
$env:CODEXPPP_REDIS_ADDR="localhost:63799"
cd backend
go run .
```

`CODEXPPP_SECRET` protects encrypted upstream credentials and session signing. It may be omitted for local JSON development, but when `CODEXPPP_DATABASE_URL` is set the backend refuses to start unless `CODEXPPP_SECRET` is explicitly set to a non-default value.

When `CODEXPPP_REDIS_ADDR` is set, the backend checks Redis at startup and uses it for per-user gateway request limiting. The default limit is 120 gateway requests per minute per user. Set `CODEXPPP_GATEWAY_RATE_LIMIT_PER_MINUTE=0` to disable Redis rate limiting while keeping the Redis connection available for later operational use.

Desktop update metadata is optional and served only through the authenticated client API. Set `CODEXPPP_DESKTOP_LATEST_VERSION`, `CODEXPPP_DESKTOP_DOWNLOAD_URL`, `CODEXPPP_DESKTOP_DOWNLOAD_SHA256`, and `CODEXPPP_DESKTOP_RELEASE_NOTES` when a Windows client release is available. The backend only returns `http` or `https` download URLs, and the ordinary user client shows the update check after login in advanced mode. These variables are passed through the Docker backend service; they do not turn the ordinary-user client into a web service.

## Run The Compose Backend

The compose file also includes a backend service. The backend image builds the Go service and installs the official Codex CLI package so the gateway can run `codex app-server --listen stdio://` inside the container.

Create a deployment env file from the example and set a non-default secret:

```powershell
Copy-Item deploy/.env.example deploy/.env
notepad deploy/.env
```

Then start the full local stack:

```powershell
docker compose --env-file deploy/.env -f deploy/docker-compose.yml up -d
```

The backend service listens inside the container on `0.0.0.0:8787` but is published to the host only as `127.0.0.1:8787`. For a remote deployment, put it behind the intended firewall and reverse proxy before exposing it beyond loopback.

The backend container healthcheck calls `GET /api/health`. That endpoint is only a liveness check and returns no account, route, database, Redis, API key, or credential detail.

The compose stack runs backend, admin console, PostgreSQL, and Redis only. The ordinary-user client is distributed and run as a Windows/Tauri software client; it is not served from the backend and is not started as a Docker web service.

To verify that the backend container can start the official Codex app-server protocol entry:

```powershell
go run .\scripts\verify-codex-app-server.go -docker codexppp-backend
```

Without `CODEXPPP_VERIFY_CODEX_ACCESS_TOKEN`, this command only checks that `codex app-server --listen stdio://` starts and completes `initialize`. With a real token in that environment variable, it also checks the authenticated account endpoints. Add `-turn-prompt "..."` only when you intentionally want a real Codex turn smoke test, because that can consume upstream quota.

To run the PostgreSQL round-trip test:

```powershell
$env:CODEXPPP_TEST_DATABASE_URL="postgres://codexppp:codexppp@localhost:54329/codexppp?sslmode=disable"
cd backend
go test ./...
```

## Develop Desktop Client UI

For quick local UI development, open:

```text
desktop-client/ui/index.html
```

The file is a development artifact for the Tauri webview, not a deployed ordinary-user website. It uses `http://localhost:8787/api` during local development. If it is opened directly as `file://`, set `CODEXPPP_CLIENT_ORIGINS=null` for that local development run. The Tauri shell reads `CODEXPPP_BACKEND_API_BASE` at build or runtime and normalizes it to an `/api` base; this deployment setting is not exposed as an ordinary-user editable endpoint.

The Windows client executable can be built from:

```powershell
.\scripts\build-windows-client.ps1
```

This produces `desktop-client/src-tauri/target/debug/codexppp-desktop.exe` for local validation. The Tauri config enables the Windows NSIS bundle target. When the Tauri CLI is installed, run `.\scripts\build-windows-client.ps1 -Bundle` to produce the configured Windows installer bundle. This remains a Windows software distribution path; it does not serve the ordinary-user client from the backend or Docker.

In the Tauri shell, the desktop client can:

- Detect local Codex installation and distinguish command-line Codex from Windows Store Codex.
- Back up `~/.codex/config.toml` and `~/.codex/auth.json` before Codex+++ managed writes.
- Remove stale Codex+++ provider/config keys with structured TOML editing.
- When local Codex has no ChatGPT login, request a backend generated Codex API key, write Codex `auth.json` with `auth_mode = "apikey"` and `OPENAI_API_KEY`, and configure the Codex+++ provider with `requires_openai_auth = true`.
- When local Codex already has ChatGPT login, preserve that login and remove stale Codex+++ API-key state.
- Launch local Codex through a verified Windows launch path.
- Check authenticated backend update metadata and open the Windows client update download through the desktop shell.

Windows Store Codex startup has recorded invalid paths:

- `cmd /C start` or equivalent shell-reparsed startup of `C:\Program Files\WindowsApps\OpenAI.Codex_...\app\Codex.exe` can surface a Windows “找不到文件 `\codex\`” or equivalent missing-application dialog.
- Direct `CreateProcess` execution of `C:\Program Files\WindowsApps\OpenAI.Codex_...\app\Codex.exe` can leave the desktop client at “启动失败”.

Do not use either path as the Windows Store Codex startup strategy, retry strategy, or fallback. A valid startup change must be checked on the deployment machine for Codex detection, window launch, running-state refresh, stop-state refresh, Codex+++ provider token availability, and local Codex account detection.

## Current Persistence

When `CODEXPPP_DATABASE_URL` is set, the backend applies `backend/migrations/*.sql`, loads state from PostgreSQL, and saves runtime changes back to PostgreSQL. When `CODEXPPP_DATABASE_URL` is not set, the backend stores data in `backend/data/codexppp.json` for local Windows development.

The PostgreSQL schema lives under `backend/migrations/`. The domain model is kept close to that schema: users, devices, token top-up entries, recharge requests, token ledger, upstream accounts, API keys, usage records, audit logs, sessions, and idempotency records are all separated.

## Clean Gateway Boundary

The backend intentionally exposes only Codex+++ API paths:

- `/api/admin/...`
- `/api/client/...`
- `/api/codex/v1/models`
- `/api/codex/v1/responses`
- `/api/health` for deployment liveness only

The gateway executor does not trust client-reported token usage. It sanitizes the request, selects an available Codex upstream account, starts `codex app-server --listen stdio://` with that account token, sends `thread/start` and `turn/start`, reads the returned usage, records that usage, and deducts the user's token balance from Codex-confirmed usage only. When Codex app-server does not return usage, the gateway fails closed and does not deduct balance.

`/api/codex/v1/responses` is not an old-interface compatibility fallback. It is a single authenticated backend adapter for controlled Codex-shaped execution, and it reuses the same internal billing, idempotency, route selection, and usage verification path as the gateway executor. The Windows desktop launch path is separate from this adapter.

Gateway executions support `Idempotency-Key`, `X-Request-ID`, or a body `requestId`. The backend reserves the user's currently available token balance internally before calling upstream, releases that reservation on failure, and records the actual debit only after upstream usage is verified. A completed request with the same request id returns the existing usage record and cannot charge twice.

When an upstream route returns account or route failures such as quota exhaustion, authorization failure, rate limiting, invalid route, or service unavailability, the gateway marks that upstream account unavailable for admin review and automatically tries the next available API key route. Codex+++ token shortage is not handled this way; it stops route supply instead of switching API keys.

Administrators can check an upstream Codex account from the account-pool page. The backend launches `codex app-server --listen stdio://`, passes the account token through `CODEX_ACCESS_TOKEN`, and reads `account/read`, `account/rateLimits/read`, and `account/usage/read` over JSON-RPC. Set `CODEXPPP_CODEX_COMMAND` when the Codex command is not on `PATH`.

It does not expose root-level `/v1/*`, `/api/gateway/`, Anthropic, sub2api, or old-interface compatibility routes.

## Recharge Model

The user-facing purchase action is a token top-up request, not a subscription plan. Current recharge entries are fixed RMB-denominated top-up options so the first operational version can run with manual administrator confirmation. Free entries are display-only. Paid entries can be requested repeatedly and only affect the user's token balance after administrator approval.

This model matches Codex pricing more closely than selling independent plans: OpenAI documents Codex usage as token-based and model-dependent, with separate rates for input, cached input, and output tokens. Codex+++ therefore treats the recharge entry as a wallet funding mechanism, while usage charging remains tied to upstream Codex usage data instead of client estimates.

The client advanced panel uses `/api/client/ledger/summary` for a 30-day UTC daily income/expense chart and keeps raw token ledger, usage, and recharge records as paginated tables. The admin usage and audit page also uses backend pagination directly, with independent controls for usage records and audit records.

Admin user management keeps manual corrections on the selected user detail view. The list is for search, pagination, status switching, and opening details; token adjustment, password reset, user token ledger, user recharge history, and user usage records are exposed through `/api/admin/users/{id}/...`.

The admin account-pool page supports upstream account checking, combined availability maintenance, API key generation, API key enable/disable, and paginated device management. Balance and risk availability stay as internal routing inputs. Account availability actions accept only combined `availabilityStatus`, and list/create/check responses avoid returning API key raw values, stable key prefixes, key hashes, upstream split status fields, or upstream credential fingerprints.

The Windows client prepares Codex by cleaning stale Codex+++ entries from user-level `~/.codex/config.toml` and stale Codex+++ API-key fields from `~/.codex/auth.json`. If local Codex already has ChatGPT tokens, preparation keeps that login and removes stale Codex+++ API-key state. If local Codex has no ChatGPT login, the client calls `/api/client/launch/prepare`, receives a backend generated Codex API key, writes Codex `auth.json` with `auth_mode = "apikey"` and `OPENAI_API_KEY`, and writes a Codex+++ custom provider with `model_provider = "codexppp"`, `[model_providers.codexppp]`, `wire_api = "responses"`, `base_url = "<backend>/codex/v1"`, and `requires_openai_auth = true`. The provider must not use `[model_providers.codexppp.auth]`, `env_key`, or `experimental_bearer_token`. The client does not inject `CODEX_HOME` into the Codex process and does not display backend pool-account data as the local Codex account. The desktop EXE does not start a loopback proxy; launching Codex does not require the client process to listen on a local port.
