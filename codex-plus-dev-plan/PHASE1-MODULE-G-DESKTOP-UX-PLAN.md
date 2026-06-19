# Codex++ Phase 1 Module G Desktop UX Plan

本文档把 Module G 的桌面端用户体验工作落到可实现的 React/Tauri UI 文件、状态页面、安装辅助、新手教学、错误呈现和动效边界。它解决的核心问题是：普通用户不理解 Codex、API Key、供应商、模型倍率和配置文件时，打开 Codex++ Manager 仍能完成购买后登录、检查安装、写入 `Codex++ Cloud`、启动 Codex 的闭环。

## Status

- State: ready to prototype against Phase 0 fixtures; final command integration waits for Module F
- Owner: Module G / desktop UX worker
- Primary repo: `CodexPlusPlus-main`
- Dependency:
  - `codex-plus-contracts/test-fixtures/client/*.json`
  - `codex-plus-contracts/status-error/client-status-errors.md`
  - `codex-plus-dev-plan/PHASE1-MODULE-F-DESKTOP-RUNTIME-PLAN.md`

## Source Evidence

Current Manager UI already provides these reusable foundations:

| Capability | Existing path | Module G decision |
| --- | --- | --- |
| React shell and route switch | `apps/codex-plus-manager/src/App.tsx` | Add a cloud route and minimal wiring; avoid a broad rewrite. |
| Navigation | `Route` union and `routes` array in `App.tsx` | Add `cloud` as the ordinary-user first route; keep advanced routes available. |
| UI primitives | `components/ui/*`, local `Panel`, `CardHead`, `Toolbar`, `StatusRow` | Reuse existing primitives for consistency. |
| Icons | `lucide-react` already used | Use lucide icons for actions and states. |
| Existing install maintenance | `MaintenanceScreen` | Surface only necessary checks in cloud onboarding; deep maintenance stays in its page. |
| Existing relay/provider UI | `RelayScreen` | Keep it as advanced configuration; Module G must not duplicate provider editor. |
| Existing motion system | `styles.css` with enter animations and reduced-motion handling | Use motion only for progress/state transitions; no decorative animation. |

Important current constraint:

- `App.tsx` is already very large. Module G should add cloud-specific code in separate files and keep `App.tsx` changes to route registration, command actions and prop wiring.

## Goal

Implement a normal-user desktop experience that can:

1. Show a clean `Codex++ Cloud` home dashboard as the primary entry.
2. Guide first-time users through endpoint/login, entitlement status, installation check and provider apply.
3. Let users redeem activation codes without visiting advanced backend pages.
4. Explain status and next action without exposing API complexity.
5. Keep advanced provider configuration present but visually secondary.
6. Consume only Module F command payloads or Phase 0 fixtures; never invent backend fields.
7. Avoid hard-coded price, model catalog, quota, rate limit, renewal wording or model multipliers.

## Non Goals

- Do not implement Tauri/Rust commands.
- Do not change Sub2API backend/admin code.
- Do not edit `codex-plus-contracts/**`.
- Do not build a marketing landing page.
- Do not replace `RelayScreen` or remove manual providers.
- Do not show raw API Key, JWT, authorization header or upstream credential.
- Do not hard-code business configuration that belongs in Sub2API admin/config center.

## Target UI Structure

Recommended additive structure:

```text
CodexPlusPlus-main/
  apps/codex-plus-manager/src/
    cloud/
      types.ts
      fixtures.ts
      cloudCommands.ts
      useCloudRuntime.ts
      CloudHomeScreen.tsx
      CloudLoginPanel.tsx
      CloudStatusPanel.tsx
      CloudInstallAssistant.tsx
      CloudUsagePanel.tsx
      CloudTutorialPanel.tsx
      CloudDiagnosticsPanel.tsx
      cloud.css

    App.tsx                  # route/action wiring only
```

If the repo style later moves all screens into a common `screens/` directory, Module G may follow that style. The boundary remains:

- cloud UI logic lives outside the current monolithic `App.tsx`
- `App.tsx` only registers `cloud`, invokes commands and passes existing actions like launch/maintenance navigation
- no backend or runtime implementation in React files

## Route and Navigation

Add:

```text
Route = "cloud" | existing routes...
```

Recommended navigation order for ordinary users:

1. `Codex++ Cloud`
2. `会话管理`
3. `工具与插件`
4. `安装维护`
5. `高级供应商`
6. other advanced routes

The exact labels can follow product copy, but the intent must hold:

- `Codex++ Cloud` is the default first route after startup.
- `供应商配置` remains accessible as advanced configuration.
- The overview route can be merged into cloud or kept as diagnostics-oriented overview, but ordinary users should not start from JOJO/model/relay marketing content.

`loadInitialRoute()` should prefer `cloud` unless:

- the user explicitly selected another route in local storage
- a startup query parameter requests update/about
- a command-line mode intentionally opens maintenance

## Cloud Home Layout

The first screen should be a working dashboard, not a landing page:

```text
CloudHomeScreen
  Header:
    account/entitlement status
    refresh
    launch Codex

  Primary action strip:
    Login / Refresh / Repair / Apply Codex++ Cloud / Redeem

  State panels:
    Connection
    Entitlement and usage
    Installation readiness
    Codex++ Cloud provider
    New user tutorial
    Redacted diagnostics
```

No panel should repeat backend jargon. User-facing copy should explain what action to take:

- "登录后自动配置 Codex++ Cloud"
- "需要开通后才能使用"
- "本机设备已被停用，请联系管理员"
- "本地写入失败，可尝试修复配置"

Do not expose:

- raw API Key
- JWT/access token
- upstream provider name if not needed
- model multiplier
- price calculation
- quota formulas

## UX State Machine

Consume Module F `CloudRuntimeState` and map it to screens:

| Runtime category | Primary UI state | Main action |
| --- | --- | --- |
| `needs_login` | Login required | Login form and endpoint check. |
| `not_purchased` | Account valid, entitlement missing | Redeem code or open purchase URL from backend payload/config. |
| `expired` | Entitlement expired | Renewal action from backend payload/config. |
| `limited` | Usage/balance limited | Show usage summary and backend-provided action. |
| `device_revoked` | Device blocked | Explain device state and contact/admin action. |
| `gateway_unhealthy` | Server/gateway issue | Retry bootstrap and show redacted diagnostics. |
| `local_config_failed` | Backend ok, local write failed | Repair managed provider. |
| `stale_snapshot` | Offline/cached state | Show cached timestamp and retry. |
| `ready` | Ready to use | Launch Codex and refresh usage. |

Module G must not decide entitlement locally. It only reflects runtime/backend state.

## Onboarding Flow

First-time user flow:

```text
Open Manager
  -> Cloud route loads
  -> check local install status
  -> configure endpoint if missing
  -> login
  -> register device
  -> fetch bootstrap
  -> apply Codex++ Cloud provider
  -> launch Codex
  -> show first-run tutorial checklist
```

The UI can collapse these into a stepper:

1. 登录账号
2. 检查权益
3. 检查安装
4. 配置 Codex++ Cloud
5. 启动 Codex

Step animation is allowed only to show real progress or a running command. Decorative looping loaders should not run after a command finishes.

## Install Assistant

Module G should expose a simple install assistant inside the cloud route by reusing existing `MaintenanceScreen` actions:

- Codex app detected or missing
- saved app path
- silent shortcut installed
- management shortcut installed
- watcher state if relevant
- one-click repair shortcuts/backend
- choose Codex app folder/file

Rules:

- Do not teach users to manually edit `config.toml`.
- Do not show debug/helper ports in normal flow.
- Keep deep maintenance controls in `安装维护`.
- Use concise status rows and action buttons; no long explanatory copy blocks.

## Tutorial Panel

Tutorial should be operational, not a marketing tour:

- "登录并配置云服务"
- "点击启动 Codex"
- "在 Codex 中开始一个任务"
- "用量在这里刷新"
- "遇到失败先点修复，再看诊断"

Rules:

- Tutorial progress is local UI state only.
- It must not unlock features or bypass backend entitlement.
- It should be dismissible and restorable.
- It should not contain hard-coded pricing/model claims.

## Advanced Configuration Hiding

Default ordinary-user behavior:

- Show `Codex++ Cloud` and installation/usage first.
- Treat `供应商配置` as advanced.
- Hide raw config editors from cloud route.
- Do not show `apiKey` inputs in cloud route.
- Do not encourage users to create a manual provider unless cloud runtime explicitly fails and they choose advanced mode.

Advanced users can still use existing Relay/Provider pages. Module G must not remove them.

## Motion And Interaction Rules

Allowed purposeful motion:

- screen enter transition already present in `styles.css`
- progress step transitions while login/bootstrap/apply commands are running
- status change highlight when a panel moves from failed to ready
- progress bar for provider write or session repair

Not allowed:

- decorative blobs/orbs/background motion
- looping animation after a command has completed
- motion that hides error text
- animation that changes layout height unpredictably

Must support:

- existing `prefers-reduced-motion: reduce`
- stable panel dimensions for status/action areas
- no text overlap at narrow widths
- no nested card stacks inside cards

## Command Integration

Module G consumes commands defined by Module F:

```text
codexplus_cloud_load_state
codexplus_cloud_configure_endpoint
codexplus_cloud_login
codexplus_cloud_logout
codexplus_cloud_refresh_bootstrap
codexplus_cloud_register_device
codexplus_cloud_apply_managed_provider
codexplus_cloud_repair_managed_provider
codexplus_cloud_redeem
codexplus_cloud_load_usage
codexplus_cloud_read_redacted_diagnostics
```

Until Module F lands, Module G may use local fixture adapters:

- fixtures must be sourced from `codex-plus-contracts/test-fixtures/client/*.json`
- fixture adapter must be clearly replaceable with Tauri command calls
- fixture fields cannot exceed the frozen contract or Module F payload plan

## UI Data Rules

Can display:

- `plan_name` or backend-provided plan label
- backend-provided status text
- backend-provided renewal/purchase/redeem action URL or label
- `config_version`
- `snapshot_version`
- usage period and values returned by runtime/backend
- `has_api_key`
- `Codex++ Cloud` provider active/configured state

Must not display:

- raw `provider.api_key`
- JWT/access token
- authorization header
- password
- upstream key
- hard-coded prices
- hard-coded model availability
- hard-coded quota thresholds
- hard-coded renewal copy
- hard-coded rate-limit/multiplier math

If a field is missing from runtime payload, show a neutral unknown state instead of inventing a value.

## Error Copy Rules

Use short action-oriented messages:

| State | Copy intent |
| --- | --- |
| login failed | account/password/session issue; allow retry |
| not purchased | account is valid but service is not open |
| expired | service period ended; use backend-provided renewal path |
| low balance | usage or balance limit reached; show backend-provided action |
| device revoked | this device is disabled; do not auto-create a new identity |
| gateway unhealthy | cloud service is reachable but model gateway is unhealthy |
| local config failed | cloud account is okay; local Codex config write failed |
| stale snapshot | showing cached info; retry to confirm current state |

Avoid:

- stack traces in the main UI
- raw JSON in normal panels
- blaming the user for configuration details they did not choose

Raw details can appear only in the redacted diagnostics panel.

## Parallel Boundaries

Module G may edit:

- `apps/codex-plus-manager/src/cloud/**`
- `apps/codex-plus-manager/src/App.tsx` only for route/action/type wiring
- `apps/codex-plus-manager/src/styles.css` or `src/cloud/cloud.css` only for cloud UI styles
- focused UI tests or story fixtures if the project has test harness support

Module G must not edit:

- `CodexPlusPlus-main/crates/**`
- `apps/codex-plus-manager/src-tauri/**`
- `sub2api-main/**`
- `codex-plus-contracts/**`
- payment/admin/backend files
- existing provider editor internals except for adding an "advanced" entry point or read-only state indicator

If Module G needs a new runtime field, it must request a Module F payload update. If it needs a new backend field, it must request a contract patch.

## Tests

Add or update tests/checks where available:

- TypeScript typecheck for cloud payload types.
- Cloud route renders every runtime category from fixture data.
- Login form never renders token/API key values.
- Ready state shows launch/apply/refresh actions.
- Not purchased/expired/limited states show backend-provided action copy only.
- Device revoked state does not offer silent device reset.
- Local config failed state shows repair action.
- Install assistant hides debug/helper ports in normal flow.
- Reduced-motion CSS keeps UI usable.
- Narrow viewport has no overlapping action text.

Suggested commands:

```powershell
cd CodexPlusPlus-main/apps/codex-plus-manager
corepack pnpm install
corepack pnpm run check
corepack pnpm run build
```

If dependencies or Rust/Tauri prerequisites are unavailable, report exactly which checks could not run.

## Exit Gate

Module G is complete only when:

- Ordinary users can start from `Codex++ Cloud` without opening provider settings.
- All major runtime states have a visible action and no raw secrets.
- Install assistant guides missing Codex app/shortcut/backend issues.
- Tutorial explains first use without teaching API configuration.
- Advanced supplier configuration remains available but not required.
- UI contains no hard-coded price/model/quota/multiplier/renewal business configuration.
- Module F command names and payloads are consumed through a typed adapter.
- Typecheck/build are run, or toolchain/dependency blockers are explicitly reported.
