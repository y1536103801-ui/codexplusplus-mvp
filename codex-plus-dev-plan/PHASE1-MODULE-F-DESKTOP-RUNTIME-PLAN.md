# Codex++ Phase 1 Module F Desktop Runtime Plan

本文档把 Module F 的桌面端 runtime 工作落到可实现的 Rust/Tauri 文件、命令、状态、缓存、托管供应商写入和日志脱敏边界。它解决的核心问题是：普通用户登录后，Codex++ Manager 能消费 Sub2API bootstrap 快照，并把 `Codex++ Cloud` 写成可直接启动 Codex 的本地托管供应商。

## Status

- State: ready to implement against Phase 0 fixtures; final backend integration waits for Module D
- Owner: Module F / desktop runtime worker
- Primary repo: `CodexPlusPlus-main`
- Dependency:
  - `codex-plus-contracts/api/client-openapi.yaml`
  - `codex-plus-contracts/test-fixtures/client/*.json`
  - `codex-plus-contracts/status-error/client-status-errors.md`
  - `codex-plus-dev-plan/PHASE1-MODULE-D-CLIENT-API-PLAN.md`

## Source Evidence

Current desktop runtime already provides these reusable foundations:

| Capability | Existing path | Module F decision |
| --- | --- | --- |
| Tauri command registration | `apps/codex-plus-manager/src-tauri/src/lib.rs` | Add cloud commands with minimal registration. |
| Manager command payload convention | `apps/codex-plus-manager/src-tauri/src/commands.rs` | Reuse `CommandResult<T>` shape for cloud commands. |
| Settings persistence | `crates/codex-plus-core/src/settings.rs` | Reuse `BackendSettings`, `RelayProfile`, `SettingsStore`; do not create a second provider settings system. |
| Relay profile switch/apply | `crates/codex-plus-core/src/relay_switch.rs`, `relay_config.rs` | Build `Codex++ Cloud` as a managed `RelayProfile` and apply through existing switch/apply helpers. |
| Provider sync | `crates/codex-plus-data/src/provider_sync.rs` | Optional post-apply sync only when user/settings enable it; never rewrite sessions during login. |
| Launch and diagnostics | `launcher.rs`, `diagnostic_log.rs`, existing manager commands | Reuse launcher; add cloud-specific redacted diagnostics around bootstrap/provider writes. |
| Current React shell | `apps/codex-plus-manager/src/App.tsx` | Module F does not own UX screens; expose command shapes for Module G. |

Important current constraint:

- `diagnostic_log::append_diagnostic_log` appends JSONL as provided. It is not a sanitizer. Module F must redact before calling it.

## Goal

Implement desktop runtime services that can:

1. Store Sub2API endpoint, user session, device identity and last bootstrap snapshot locally.
2. Call `/api/v1/client/bootstrap`, `/usage`, `/devices` and `/redeem` through authenticated HTTP.
3. Convert a successful bootstrap response into a managed `Codex++ Cloud` relay profile.
4. Apply that profile through existing settings/relay writer code without deleting manual providers.
5. Return stable local state and error categories to Module G.
6. Redact secrets in logs, command results, diagnostics and test snapshots.

Module F is runtime plumbing only. It does not design dashboard layout, pricing cards or tutorial screens.

## Non Goals

- Do not implement backend endpoints.
- Do not change `codex-plus-contracts/**`.
- Do not implement admin operations.
- Do not hard-code plan price, model list, quota, rate limits, renewal copy or model multipliers.
- Do not rewrite existing relay/profile architecture.
- Do not remove manual provider, official login, relay profile, provider sync, launcher or local session features.

## Target Runtime Shape

Add an additive cloud runtime namespace under `codex-plus-core`:

```text
CodexPlusPlus-main/
  crates/codex-plus-core/src/
    codexplus_cloud/
      mod.rs
      api.rs
      bootstrap.rs
      device.rs
      local_state.rs
      provider_writer.rs
      redaction.rs
      status.rs

  apps/codex-plus-manager/src-tauri/src/
    cloud_commands.rs
    lib.rs                    # minimal command registration only
```

If the implementation prefers fewer files, it may collapse modules, but the ownership boundaries must remain the same:

- `api.rs`: HTTP request/response DTOs and Sub2API auth/client API calls.
- `local_state.rs`: local endpoint/session/device/snapshot persistence.
- `provider_writer.rs`: bootstrap provider -> `RelayProfile` -> existing settings/relay apply.
- `redaction.rs`: shared redaction for logs, diagnostics and command payloads.
- `cloud_commands.rs`: Tauri command facade consumed by Module G.

## Local State Contract

Recommended local files:

```text
{app_state_dir}/codexplus-cloud/session.json
{app_state_dir}/codexplus-cloud/bootstrap_snapshot.json
{app_state_dir}/codexplus-cloud/device.json
```

Where `app_state_dir` is derived from the existing `codex_plus_core::paths` app-state directory.

### `session.json`

Stores:

- `base_url`
- `user_id`
- `access_token`
- `expires_at`
- `last_login_at`

Must not store:

- password
- upstream provider key
- admin token
- payment credentials

MVP may store the Sub2API user session token in app-state files with redacted diagnostics and strict file permissions where supported. If an OS keychain dependency is added, Module F owns the dependency and lockfile change, but must coordinate with the integration owner.

### `device.json`

Stores:

- stable local `device_id`
- `device_name`
- `platform`
- first seen timestamp
- last registered timestamp

Rules:

- Generate a stable random device id on first run.
- Never derive device id from machine username, disk serial, MAC address or other invasive identifiers.
- Use the same device id for `/api/v1/client/devices` and bootstrap context.

### `bootstrap_snapshot.json`

Stores the last successful bootstrap response and metadata:

- `snapshot_version`
- `config_version`
- `fetched_at`
- redacted status summary
- full provider material only if required to re-apply `Codex++ Cloud`

Rules:

- Never persist upstream real credentials.
- `provider.api_key` is a Sub2API user-side gateway key and may be required locally for Codex launch.
- Any UI-facing, log-facing or diagnostics-facing snapshot must redact the key.
- A stale snapshot can be shown as cached state, but gateway access remains server-enforced. The client must not treat a stale snapshot as proof of entitlement.

## Managed Provider Writer

Authoritative managed provider identity:

```text
managed_provider_id = "codex-plus-cloud"
display_name = "Codex++ Cloud"
```

The display name is not source of truth. The provider id is.

### Mapping Rules

From bootstrap provider:

| Bootstrap field | Relay profile destination |
| --- | --- |
| `provider.id` or managed provider id | `RelayProfile.id = "codex-plus-cloud"` |
| `provider.display_name` | `RelayProfile.name = "Codex++ Cloud"` |
| `provider.base_url` | profile base URL/config provider base URL |
| `provider.api_key` | bearer token used by existing relay writer |
| `defaults.model` | default model in profile config |
| `defaults.protocol` | `RelayProtocol` |
| feature/context settings | existing common/context config, not hard-coded in cloud writer |

Expected profile behavior:

- Use `RelayMode::PureApi` unless the backend contract explicitly returns a different supported mode.
- Use the existing `RelayProfile` normalization and `relay_config` apply helpers.
- Preserve `relay_common_config_contents` and `relay_context_config_contents`.
- Preserve all manual `relay_profiles` whose id is not `codex-plus-cloud`.
- Upsert the managed profile idempotently.
- Set `active_relay_id` to `codex-plus-cloud` only after bootstrap validation succeeds.
- Do not expose `apiKey` in serialized `settings.json` where existing settings rules omit derived fields.

### Apply Flow

```text
load local session
  -> register or refresh device
  -> GET /api/v1/client/bootstrap
  -> validate status and provider payload
  -> upsert Codex++ Cloud RelayProfile
  -> save BackendSettings
  -> apply selected relay profile through existing relay switch/apply helper
  -> optionally run provider sync if enabled
  -> return redacted CloudRuntimeState
```

If provider write fails:

- Keep previous settings/profile state where existing switch helper can roll back.
- Return local status `local_config_failed`.
- Keep server-side entitlement untouched.
- Write only redacted diagnostics.

## Tauri Command Surface For Module G

Module F should expose these commands. Names can be adjusted during implementation, but Module G must receive one stable list before UI work begins.

```text
codexplus_cloud_load_state()
codexplus_cloud_configure_endpoint(baseUrl)
codexplus_cloud_login(baseUrl, email, password)
codexplus_cloud_logout()
codexplus_cloud_refresh_bootstrap()
codexplus_cloud_register_device(deviceName)
codexplus_cloud_apply_managed_provider()
codexplus_cloud_repair_managed_provider()
codexplus_cloud_redeem(code)
codexplus_cloud_load_usage()
codexplus_cloud_read_redacted_diagnostics(lines)
```

Command result convention:

```json
{
  "status": "ok",
  "message": "Cloud state loaded.",
  "...": "payload fields"
}
```

Do not return raw:

- `provider.api_key`
- JWT/access token
- `Authorization`
- password
- upstream key
- full base URL query secrets

The only internal layer allowed to hold the full `provider.api_key` is the provider writer/local runtime layer needed to make Codex requests work.

## Runtime State Model

Module F should return a UI-safe state such as:

```text
CloudRuntimeState
  connection:
    base_url
    authenticated
    user_label
    token_expires_at
  device:
    device_id
    device_name
    status
  entitlement:
    status
    plan_name
    expires_at
    renewal_hint
  usage:
    period
    used
    limit
    unit
  provider:
    managed_provider_id
    display_name
    configured
    active
    base_url
    default_model
    has_api_key
  diagnostics:
    last_error_code
    last_error_message
    last_bootstrap_at
    config_version
    snapshot_version
```

UI-safe means:

- `has_api_key` is allowed.
- masked key fragments are discouraged unless product explicitly requires them.
- full key/token is forbidden.

## Status Mapping

Use frozen contract statuses and map them to desktop runtime categories:

| Contract/backend state | Runtime category | Runtime behavior |
| --- | --- | --- |
| authenticated and available | `ready` | Allow provider apply and launch. |
| unauthenticated or expired token | `needs_login` | Clear/ignore invalid session token, keep endpoint/device. |
| not purchased | `not_purchased` | Do not apply new provider; show server message from bootstrap. |
| expired | `expired` | Do not apply new provider; stale local provider may exist but server enforces denial. |
| low balance / quota exceeded | `limited` | Allow dashboard view; launch can remain available if backend status permits. |
| device revoked | `device_revoked` | Do not register silently under a new id; require user/admin action. |
| gateway unhealthy | `gateway_unhealthy` | Keep local settings; return repair/retry action. |
| local provider write failed | `local_config_failed` | Backend ok, local write failed; expose repair command. |
| network failure with cached snapshot | `stale_snapshot` | Show cached state and retry action; never claim entitlement is current. |

Module F must not invent new server statuses. Local-only statuses can be namespaced and documented for Module G.

## HTTP Client Requirements

- Reuse existing proxied HTTP client helper if appropriate.
- Apply timeouts to login/bootstrap/usage/device/redeem requests.
- Normalize `base_url` by trimming trailing slash.
- Never log request headers.
- Include the authenticated bearer header only in memory.
- Include `X-CodexPlus-Device-Id` where Module D/Module E expects device context.
- Treat non-2xx responses as structured contract errors when possible.
- Fall back to redacted transport errors when the response does not match contract shape.

## Redaction Requirements

Create one shared redaction utility for cloud runtime:

- redact JSON keys named like `api_key`, `apiKey`, `access_token`, `refresh_token`, `id_token`, `authorization`, `password`, `bearer`, `experimental_bearer_token`
- redact string patterns resembling long bearer tokens or `sk-...`
- redact URL query values named `key`, `token`, `access_token`, `api_key`
- preserve booleans such as `has_api_key`

Apply redaction before:

- `diagnostic_log::append_diagnostic_log`
- command responses
- copied diagnostics
- test snapshot files
- local error details shown to UI

## Parallel Boundaries

Module F may edit:

- `CodexPlusPlus-main/crates/codex-plus-core/src/codexplus_cloud/**`
- `CodexPlusPlus-main/crates/codex-plus-core/src/lib.rs` only to export the new module
- `CodexPlusPlus-main/apps/codex-plus-manager/src-tauri/src/cloud_commands.rs`
- `CodexPlusPlus-main/apps/codex-plus-manager/src-tauri/src/lib.rs` only to register cloud commands
- focused Rust tests beside the new modules

Module F may minimally touch:

- `settings.rs` only if a small managed-provider metadata field is unavoidable
- `relay_config.rs` or `relay_switch.rs` only to expose existing pure functions needed by the writer

Module F must not edit:

- `sub2api-main/**`
- `codex-plus-contracts/**`
- `apps/codex-plus-manager/src/App.tsx` and `styles.css` except for a pre-agreed command type stub
- admin UI
- payment/redeem backend
- gateway enforcement
- product HTML/doc pages unless the integration coordinator asks for a docs sync

If Module F needs a contract field that does not exist, stop and request a contract patch. Do not add a client-only field and ask Module D to follow later.

## Tests

Add focused Rust tests for:

- bootstrap fixture deserialization for all `codex-plus-contracts/test-fixtures/client/*.json`
- local state load/save when files are missing, corrupted or old-version
- stable device id generation and reuse
- redaction of API keys, JWT-like tokens, Authorization headers and URL query tokens
- managed provider upsert preserves manual providers
- repeated bootstrap reuses `codex-plus-cloud` profile instead of creating duplicates
- provider writer does not serialize derived `apiKey` into settings where existing rules omit it
- provider write failure returns `local_config_failed` and preserves previous active profile
- command payloads expose `has_api_key` but never full key/token
- stale snapshot behavior on transport failure

Suggested commands when Rust toolchain is available:

```powershell
cd CodexPlusPlus-main
cargo test -p codex-plus-core codexplus_cloud
cargo test -p codex-plus-manager
```

Current workspace audit showed Rust/Cargo missing, so the worker must report if these cannot run.

## Integration Notes For Module G

Module G should not parse settings files directly. It should consume only:

- `codexplus_cloud_load_state`
- `codexplus_cloud_login`
- `codexplus_cloud_refresh_bootstrap`
- `codexplus_cloud_apply_managed_provider`
- `codexplus_cloud_repair_managed_provider`
- `codexplus_cloud_redeem`
- `codexplus_cloud_load_usage`
- `codexplus_cloud_read_redacted_diagnostics`

Module G may use Phase 0 fixtures until Module F commands are implemented, but it must not invent fields outside the command payloads above.

## Exit Gate

Module F is complete only when:

- A user can authenticate against a test Sub2API endpoint or fixture client.
- Device id is stable and sent to client API calls.
- A successful bootstrap can upsert and activate `Codex++ Cloud`.
- Existing manual providers remain intact.
- `provider.api_key`, JWT and Authorization never appear in logs/command responses/diagnostic copies.
- Runtime state covers success, not purchased, expired, low balance, revoked device, gateway unhealthy, stale snapshot and local config failed.
- Module G has stable command names and UI-safe payload examples.
- Rust tests are run, or toolchain absence is explicitly reported.
