# Codex+++

Codex+++ is a Windows and macOS Codex launcher and operations platform described in `project-goals/`.

The current implementation contains:

1. A Go backend with admin APIs, client APIs, clean Codex+++ gateway APIs, PostgreSQL or local JSON persistence, token wallet logic, recharge confirmation, account pool routing, device controls, usage records, and audit records.
2. A public product website served at `/` for project information, Windows and macOS client downloads, paid token-plan display, and manually reviewed account-purchase requests.
3. A browser-only administrator console served by the backend at `/admin`.
4. A static desktop client UI under `desktop-client/ui`, loaded by the Tauri shell in `desktop-client/src-tauri`.
5. PostgreSQL migrations and a Redis/PostgreSQL Docker Compose profile for the target deployment shape. PostgreSQL owns durable business state and atomic billing; Redis coordinates request limiting, session affinity, per-upstream concurrency leases, and distributed scheduled-task locks. Docker serves the public website and admin console, but never serves the desktop client UI as a web application.

## Reference Alignment

Codex+++ keeps two explicit reference boundaries:

- Codex++: use the same product boundary of a local Codex companion. The Windows and macOS clients detect and launch local Codex, back up and update local Codex configuration, and keep their own data outside the Codex install.
- sub2api: use the same operational boundary of an account-pool gateway. The backend owns upstream account import, platform API key generation, request forwarding, token-level usage records, recharge requests, and administrator confirmation.

Codex+++ intentionally does not expose OpenAI-compatible, Anthropic-compatible, sub2api-compatible, or public gateway product routes. The public surface stays unique and clean under `/api/admin`, `/api/client`, and the authenticated backend Codex adapter under `/api/codex/v1`; gateway execution remains an internal backend responsibility. The desktop service-status `Codex account` field uses only local Codex authentication detection results.

The account-pool-to-client chain is recorded in `docs/account-pool-client-flow.md`.

## Component Versioning

Backend and desktop client are managed as separate components. Routine backend changes belong to backend Git history. Routine Windows/macOS client changes belong to desktop-client Git history. The combined branch is an integration snapshot.

Client-backend API compatibility is controlled by `X-CodexPPP-Interop-Major`. Current value: `1`. Both desktop platforms send this header on every `/api/client/...` request. The backend rejects `/api/client/...` requests when the header is absent or different. Interop-major changes require paired backend and desktop-client changes in the same integration snapshot. Package patch versions such as the Tauri client version can move independently when `/api/client/...` request and response contracts stay on the same interop major.

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

Open the public website:

```text
http://localhost:8787/
```

The public website reads paid, enabled token plans from `/api/site/config` and submits account-purchase requests to `/api/site/orders`. It does not provide public registration, user login, payment confirmation, balance lookup, or a browser account center. Buyer contact details are encrypted at rest and can only be decrypted in the authenticated admin console. An administrator must confirm payment, create or select an active user, and fulfill the order before its token snapshot is credited once.

The first admin setup is created from the admin console. The first admin must complete the first password-change step. Password content is intentionally unrestricted.

## Run PostgreSQL And Redis

```powershell
docker compose -f deploy/docker-compose.yml up -d postgres redis
```

The compose file starts:

- PostgreSQL on `localhost:54329`
- Redis on `localhost:63799`

The PostgreSQL container applies the ordered SQL files under `backend/migrations/` on first initialization. The backend records each migration and its SHA-256 checksum in `schema_migrations`, applies missing migrations transactionally at startup, and refuses a changed checksum instead of silently mutating an already-applied migration.

To run the backend against PostgreSQL from your host shell:

```powershell
$env:CODEXPPP_DATABASE_URL="postgres://codexppp:codexppp@localhost:54329/codexppp?sslmode=disable"
$env:CODEXPPP_SECRET="<set-a-long-random-secret>"
$env:CODEXPPP_REDIS_ADDR="localhost:63799"
cd backend
go run .
```

`CODEXPPP_SECRET` protects encrypted upstream credentials and session signing. It may be omitted for local JSON development, but when `CODEXPPP_DATABASE_URL` is set the backend refuses to start unless `CODEXPPP_SECRET` is explicitly set to a non-default value.

When `CODEXPPP_REDIS_ADDR` is set, the backend checks Redis at startup and uses it for per-user gateway request limiting, cross-instance session affinity, per-upstream concurrency leases, and distributed scheduled-task locks. The default limit is 120 gateway requests per minute per user. Set `CODEXPPP_GATEWAY_RATE_LIMIT_PER_MINUTE=0` to disable only request limiting. `CODEXPPP_GATEWAY_UPSTREAM_CONCURRENCY` controls the maximum simultaneous requests admitted to one upstream account and defaults to `2`.

Gateway task affinity is persisted for 30 days using a hash of the Codex session identifier and mirrored into Redis, so restarts and multiple backend instances keep an existing task on its selected account while that account remains available. Reservations left by an interrupted backend process are marked failed and released during startup; they do not continue reducing the user's available token balance.

Desktop update metadata is optional and platform-specific. Windows uses `CODEXPPP_DESKTOP_LATEST_VERSION`, `CODEXPPP_DESKTOP_DOWNLOAD_URL`, `CODEXPPP_DESKTOP_DOWNLOAD_SHA256`, and `CODEXPPP_DESKTOP_RELEASE_NOTES`. macOS uses the corresponding `CODEXPPP_MACOS_DESKTOP_*` variables. The client sends `platform=windows|macos` to `/api/client/desktop/update`; old clients without the parameter remain on the Windows metadata. The public website exposes a separate safe download entry for each platform. These variables are passed through the Docker backend service and do not turn the desktop client UI into a web service.

## Run The Compose Backend

The compose file also includes a backend service. The backend image builds the Go service and installs the official Codex CLI package for account authorization, credential refresh, and account checks. User Codex turns are forwarded by the gateway to the official Codex Responses endpoint.

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

The backend container healthcheck calls `GET /api/ready`, which succeeds only when PostgreSQL and configured Redis are reachable and returns no dependency or credential detail. `GET /api/health` remains a process-only liveness check.

The compose stack runs the backend-hosted public website, admin console, PostgreSQL, and Redis. The ordinary-user client is distributed and run as Windows/Tauri software; its UI is not served from the backend and it is not started as a Docker web application.

To verify that the backend container can start the official Codex app-server account-maintenance protocol entry:

```powershell
go run .\scripts\verify-codex-app-server.go -docker codexppp-backend
```

Without `CODEXPPP_VERIFY_CODEX_ACCESS_TOKEN`, this command only checks that `codex app-server --listen stdio://` starts and completes `initialize`. With a real token in that environment variable, it also checks the authenticated account endpoints. The script's optional `-turn-prompt` mode is a standalone protocol diagnostic and is not the gateway runtime path; it can consume upstream quota.

To run the PostgreSQL round-trip test:

```powershell
$env:CODEXPPP_TEST_DATABASE_URL="postgres://codexppp:codexppp@localhost:54329/codexppp?sslmode=disable"
cd backend
go test ./...
```

To exercise the atomic PostgreSQL gateway transactions and Redis coordination tests against isolated test services, set both `CODEXPPP_TEST_DATABASE_URL` and `CODEXPPP_TEST_REDIS_ADDR`. Tests never select the production database implicitly.

## Develop Desktop Client UI

For quick local UI development, open:

```text
desktop-client/ui/index.html
```

The file is a development artifact for the Tauri webview, not a deployed ordinary-user website. Its checked-in fallback and packaged desktop clients use `https://codex.52cx.top/api`. Packaged release builds keep that production address fixed and send backend calls through the native Tauri HTTP layer, avoiding WebView cross-origin/network-policy differences. Debug builds may still read `CODEXPPP_BACKEND_API_BASE` at build or runtime and normalize it to an `/api` base; this setting is not exposed as an ordinary-user editable endpoint. To work against a local backend, start the Tauri development process with `CODEXPPP_BACKEND_API_BASE=http://localhost:8787` and keep the corresponding development origin in the backend CORS allowlist.

The Windows client executable can be built from:

```powershell
.\scripts\build-windows-client.ps1
```

This produces `desktop-client/src-tauri/target/debug/codexppp-desktop.exe` for local validation. The Tauri config enables the Windows NSIS bundle target. When the Tauri CLI is installed, run `.\scripts\build-windows-client.ps1 -Bundle` to produce the configured Windows installer bundle. This remains a Windows software distribution path; it does not serve the ordinary-user client from the backend or Docker.

The macOS client must be built on macOS 14+ running on Apple hardware (a physical Mac, an Apple-hosted CI runner, or a VM whose host is Apple hardware):

```sh
sh scripts/build-macos-client.sh
```

The script runs Rust tests and creates the configured application/DMG bundles. A public release additionally requires `APPLE_SIGNING_IDENTITY` plus Apple notarization credentials; ad-hoc signing is development-only and must not be published. The project does not use VMware unlockers or non-Apple macOS guests.

For repeatable VM validation, `.github/workflows/macos-client-test.yml` runs the same script on GitHub's Apple-hosted `macos-14` Apple Silicon runner, verifies the generated app identifier and code signature, and retains only a short-lived ad-hoc test artifact. It does not publish that artifact as a customer download.

In the Tauri shell, the desktop client can:

- Detect the official ChatGPT desktop app used by Codex on Windows through its `OpenAI.Codex` Store/AppX AppID; the Windows client ignores standalone Codex CLI installations. The Windows app is branded `ChatGPT`, while Codex is its native coding-agent workflow.
- On macOS, detect only OpenAI-signed `ChatGPT.app` bundles that pass Gatekeeper. Install the current official DMG into `~/Applications` from inside Codex+++, keep installation separate from launch, and compare the live official DMG ETag with the recorded installer identity instead of hardcoding a Codex version.
- Back up `~/.codex/config.toml` and `~/.codex/auth.json` before Codex+++ managed writes.
- Remove stale Codex+++ provider/config keys with structured TOML editing.
- Before launch, send only the locally recognized ChatGPT account identifier to the backend for account-pool ownership matching; never upload local ChatGPT tokens or `auth.json`.
- When there is no local ChatGPT login, or the local login matches an account owned by this backend pool, request a user-and-device-scoped Codex+++ gateway access key with a short runtime lease, back up the local Codex login state, switch Codex to the Codex+++ provider, and restore the original state when Codex is stopped. Account-pool route keys remain internal to the backend.
- Default the official Codex desktop UI to Simplified Chinese through `[desktop].localeOverride = "zh-CN"` when the user has not already selected a language; preserve any explicit language choice.
- When the local ChatGPT login does not match this backend pool, do not modify local authentication or configuration and ask the user to sign out of the personal account first.
- Install the official ChatGPT desktop app for Codex from inside the Codex+++ client, detect only its `OpenAI.Codex` AppID, and launch it through that AppID. Running-state detection does not require access to the protected WindowsApps directory: its primary check uses Windows `tasklist /APPS` and requires the returned package full name to be the official `OpenAI.Codex...__2p2nqsd0c76g0` package. Normal process-path and live package-family checks remain secondary fallbacks. A same-named standalone process is never accepted. Command-line Codex is not an accepted Windows installation or fallback.
- Keep installation and launch as separate state transitions. An install or Store update must leave Codex stopped even when the Microsoft installer activates it automatically; the later explicit launch performs backend account preparation and managed login injection. After that launch succeeds, Codex+++ immediately hides to the tray while continuing presence and launch heartbeats.
- Check authenticated platform-specific backend update metadata after login; when a newer client is required, replace the simple-mode launch action with a client-update action. The client downloads only an HTTPS `.exe` on Windows or `.dmg` on macOS, enforces a bounded size, verifies the published SHA-256, applies the platform package after the current process exits, and restarts itself. A macOS update is accepted only when the app inside the DMG also passes code-signing and Gatekeeper checks.

Windows Store Codex startup has recorded invalid paths:

- `cmd /C start` or equivalent shell-reparsed startup of `C:\Program Files\WindowsApps\OpenAI.Codex_...\app\Codex.exe` can surface a Windows “找不到文件 `\codex\`” or equivalent missing-application dialog.
- Direct `CreateProcess` execution of `C:\Program Files\WindowsApps\OpenAI.Codex_...\app\Codex.exe` can leave the desktop client at “启动失败”.

Do not use either path as the Windows Store Codex startup strategy, retry strategy, or fallback. A valid startup change must be checked on the deployment machine for Codex detection, window launch, running-state refresh, stop-state refresh, Codex+++ provider token availability, and local Codex account detection.

The client launches the exact installed `OpenAI.Codex` AppUserModelId through Windows `IApplicationActivationManager`, not through `explorer.exe`. It displays `已启动` and records the ordinary-user launch heartbeat only after Windows returns a nonzero application process id. That process id remains the primary running-state and stop reference; protected-process enumeration is only a secondary confirmation.

## Current Persistence

When `CODEXPPP_DATABASE_URL` is set, the backend applies `backend/migrations/*.sql`, loads state from PostgreSQL, and saves runtime changes back to PostgreSQL. When `CODEXPPP_DATABASE_URL` is not set, the backend stores data in `backend/data/codexppp.json` for local Windows development.

The PostgreSQL schema lives under `backend/migrations/`. The domain model is kept close to that schema: users, devices, token top-up entries, recharge requests, token ledger, upstream accounts, internal account-pool route keys, user-scoped client access keys, usage records, audit logs, sessions, and idempotency records are all separated.

## Clean Gateway Boundary

The backend intentionally exposes only Codex+++ API paths:

- `/api/admin/...`
- `/api/client/...`
- `/api/codex/v1/models`
- `/api/codex/v1/responses`
- `/api/health` for deployment liveness only
- `/api/ready` for dependency readiness without operational detail

The gateway executor does not trust client-reported token usage. It sanitizes the request, selects an available Codex upstream account, sends the original Responses request to `https://chatgpt.com/backend-api/codex/responses` with that account's access token and ChatGPT account ID, preserves the upstream SSE body and Codex response headers, records verified upstream usage, and deducts the user's token balance from that usage only. When the completed upstream response does not contain verifiable usage, the gateway fails closed and does not deduct balance. Tool calls are returned to local Codex and are not executed inside the backend container.

`/api/codex/v1/responses` is not an old-interface compatibility fallback. It is the single authenticated adapter used by local Codex and reuses the same internal billing, idempotency, route selection, and usage verification path as the gateway executor. Responses tool events and `x-codex-turn-state` remain intact across the adapter.

Official Codex may zstd-compress larger Responses request bodies. The Codex Responses route disables Nginx request buffering and streams the body to the authenticated backend. The backend admits at most two concurrent uploads globally and one per user by default, incrementally decodes zstd, validates and sanitizes top-level gateway fields, and spools the replayable JSON to a private temporary file instead of retaining the request in memory. Encoded and decoded bodies each have a 1 GiB product safety ceiling; this is an engineering guardrail, not an OpenAI-published generic Responses byte limit. Unsupported encodings, malformed bodies, exhausted upload capacity, storage failures, and oversized decoded bodies return structured errors before routing or billing.

Gateway executions support `Idempotency-Key`, `X-Request-ID`, or a body `requestId`. PostgreSQL serializable transactions reserve the user's currently available token balance before upstream execution, release that reservation on failure, and atomically persist the actual debit, usage, ledger, route, and idempotency completion only after upstream usage is verified. SSE responses are flushed to the client as upstream events arrive rather than buffered to completion. PostgreSQL retains an exact idempotent response replay for six hours and up to 2 MiB, reads that payload only when the same request is retried, and keeps completion, billing, and usage metadata for 30 days without allowing a second charge. Larger or older responses use the structured completion fallback.

When an upstream route returns account or route failures such as quota exhaustion, authorization failure, rate limiting, invalid route, or service unavailability, the gateway marks that upstream account unavailable for admin review and automatically tries the next available API key route. Codex+++ token shortage is not handled this way; it stops route supply instead of switching API keys.

Administrators can check an upstream Codex account from the account-pool page. The backend launches `codex app-server --listen stdio://`, passes the account token through `CODEX_ACCESS_TOKEN`, and reads `account/read`, `account/rateLimits/read`, and `account/usage/read` over JSON-RPC. Set `CODEXPPP_CODEX_COMMAND` when the Codex command is not on `PATH`.

Administrators can add Codex pool accounts from ChatGPT session JSON, email/password text or JSON, Codex `auth.json`, token JSON, Sub2API account backups, and existing plain-token text. A ChatGPT session import extracts only the access token, account id, email, plan, and earliest known expiry, creates a minimal encrypted auth snapshot, and discards unrelated session profile fields. Because the session response has no refresh token, the account is marked as short-lived and can be renewed by importing a newer session for the same account. Email/password input creates an encrypted pending candidate; it receives no platform API key and cannot be selected by the gateway. The account-pool page can bind that candidate to Codex app-server browser authorization or device-code authorization. Successful authorization updates the same account, clears the stored password cipher, and creates its platform API key.

The administrator can export the account pool as a round-trippable JSON backup. The export contains account credentials because it is intended for restoration, but excludes internal platform API keys, key hashes, public prefixes, and credential fingerprints. Export is administrator-only, audited, and requires an administrator login no older than 15 minutes. Gateway usage records retain their upstream account and internal API-key ownership so the account-pool rows can prioritize in-flight users and running-client leases derived from real Gateway ownership, then the latest user after stop or heartbeat timeout, today's routed token usage, and the remaining upstream allowance. A separate short-lived client runtime lease shows users whose Codex desktop is running; before the first routed task they are shown as waiting for assignment and are not falsely attached to a pool account. Closing a browser-login or device-code authorization panel cancels and cleans up the backend authorization session; closing the add-account panel only discards the current unsaved form.

Both authorization paths use an isolated `codex app-server --listen stdio://` and read the generated `auth.json` after login. Browser authorization uses `account/login/start` with `chatgpt`; a remote administrator can paste the final localhost callback URL into the admin console, where the backend validates its loopback host, callback path, port, and OAuth state before forwarding it to the app-server listener. Device-code authorization uses `chatgptDeviceCode` and displays the verification URL and user code. The backend does not store OpenAI OAuth client ids, token endpoints, or authorization-code exchange code.

It does not expose root-level `/v1/*`, `/api/gateway/`, Anthropic, sub2api, or old-interface compatibility routes.

## Recharge Model

The user-facing purchase action is a token top-up request, not a subscription plan. Current recharge entries are fixed RMB-denominated top-up options so the first operational version can run with manual administrator confirmation. Free entries are display-only. Paid entries can be requested repeatedly and only affect the user's token balance after administrator approval.

This model matches Codex pricing more closely than selling independent plans: OpenAI documents Codex usage as token-based and model-dependent, with separate rates for input, cached input, and output tokens. Codex+++ therefore treats the recharge entry as a wallet funding mechanism, while usage charging remains tied to upstream Codex usage data instead of client estimates.

The client advanced panel uses `/api/client/ledger/summary` for a 30-day UTC daily income/expense chart and keeps raw token ledger, usage, and recharge records as paginated tables. The admin usage and audit page also uses backend pagination directly, with independent controls for usage records and audit records.

Admin user management keeps manual corrections on the selected user detail view. The list is for search, pagination, status switching, and opening details; token adjustment, password reset, user token ledger, user recharge history, and user usage records are exposed through `/api/admin/users/{id}/...`.

The admin account-pool page supports multi-format account ingestion, audited account-pool export, cancellable pending-candidate authorization, upstream account checking, usage-priority account rows, combined availability maintenance, API key generation, API key enable/disable, and paginated device management. Balance and risk availability stay as internal routing inputs. Account availability actions accept only combined `availabilityStatus`, and list/create/check responses avoid returning passwords, token material, API key raw values, stable key prefixes, key hashes, upstream split status fields, or upstream credential fingerprints. Desktop clients have no account-pool import or upstream-authorization surface.

Both desktop platforms prepare Codex by cleaning stale Codex+++ entries from user-level `~/.codex/config.toml` and stale Codex+++ API-key fields from `~/.codex/auth.json`. They call `/api/client/launch/prepare` with only the locally recognized account identifier, auth-mode label, and a managed-runtime flag. If a local ChatGPT login exists, the backend must match that identifier to a stored pool account by ChatGPT account ID or email before returning a user-and-device-scoped Codex+++ gateway access key. An unmatched or unrecognized ChatGPT login is left untouched and the user is asked to sign out first. With no ChatGPT login, or after a successful pool match, the client snapshots the original `config.toml` and `auth.json`, writes `auth_mode = "apikey"` and `OPENAI_API_KEY`, writes `[desktop].localeOverride = "zh-CN"` only when no explicit locale is present, and writes the Codex+++ custom provider. The returned key remains scoped to the user, device, and live runtime lease; routing stays dynamic and server-side. Windows validates Store/AppX identity and the live Store row. macOS validates the OpenAI application signature, Gatekeeper, bundle version, and live official DMG identity. Neither platform accepts the standalone CLI as a desktop fallback. Stopping Codex restores the original snapshot, and tray/lease behavior is identical on both platforms.
