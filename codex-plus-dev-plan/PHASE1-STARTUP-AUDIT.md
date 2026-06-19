# Codex++ Phase 1 Startup Audit

本文档是 Phase 1 本地/测试 MVP 开发启动审计。目标不是继续扩大产品范围，而是把当前真实源码和开发计划对齐，明确下一步从哪里开工、哪些能力可以复用、哪些缺口必须先补。

## Audit Scope

Workspace:

- `C:\Users\23293\Desktop\codex+++`

Source projects:

- `C:\Users\23293\Desktop\codex+++\sub2api-main`
- `C:\Users\23293\Desktop\codex+++\CodexPlusPlus-main`
- `C:\Users\23293\Desktop\codex+++\codex-plus-dev-plan`

Audit date:

- 2026-06-16

Method:

- 从 `codex+++.zip` 只解压 `sub2api-main` 和 `CodexPlusPlus-main`，没有覆盖已更新的 `codex-plus-dev-plan`。
- 读取项目清单、路由注册、Ent schema、前端/桌面 package、Rust workspace、MVP 计划和契约门禁。
- 只做只读源码审计和文档补充，不改业务源码。

## Executive Summary

结论：可以开始 Phase 1，但第一步必须是 `00-contract` 和存储/迁移决策，不应该直接写后端或桌面 UI。

原因：

- `sub2api-main` 已经具备大量可复用底座：用户、登录、API Key、分组、订阅、兑换码、支付、用量、Admin、网关、运维监控。
- `CodexPlusPlus-main` 已经具备大量本地能力：Tauri Manager、Rust core、provider/relay 配置写入、provider sync、启动 Codex、日志和本地集成。
- 当前缺失的是把两者连接起来的 Codex++ Cloud 契约和运行时闭环：`登录 Sub2API -> bootstrap -> 设备状态 -> user-side gateway key -> 写入 Codex++ Cloud provider -> 网关执行权益策略`。
- 本机工具链暂时不完整，不能直接跑完整后端/桌面测试：缺 Go、Rust/Cargo、全局 pnpm；但 `corepack pnpm` 可用，Node 和 Docker 可用。
- 两个源码目录是从 zip 解压的普通目录，不是 git repo。要做多会话/可回滚开发，必须先建立版本基线或重新 clone 正式仓库。

## Source Inventory

| Project | Files found | Repo state | Primary stack | Notes |
| --- | ---: | --- | --- | --- |
| `sub2api-main` | 2337 | no `.git` | Go backend, Vue frontend, Ent, Gin, PostgreSQL/Redis, Docker | Mature backend/admin/gateway project. |
| `CodexPlusPlus-main` | 160 | no `.git` | Rust workspace, Tauri, React/Vite | Mature desktop manager and local Codex integration. |
| `codex-plus-dev-plan` | docs only | no `.git` at workspace root | Markdown plans and templates | Current planning source of truth. |

## Toolchain Probe

| Tool | Result | Impact |
| --- | --- | --- |
| `node --version` | `v24.14.1` | Available. |
| `npm --version` | `11.11.0` | Available. |
| `corepack pnpm --version` | `11.4.0` | Frontend can likely use Corepack even without global pnpm. |
| `pnpm --version` | not found | Use `corepack pnpm ...` or install pnpm. |
| `go version` | not found | Blocks Sub2API backend tests/build. |
| `cargo --version` | not found | Blocks Codex++ Rust tests/build. |
| `rustc --version` | not found | Blocks Codex++ Rust tests/build. |
| `docker --version` | `29.3.1` | Docker available for later local services/deploy rehearsal. |

Minimum environment fixes before implementation verification:

1. Install Go compatible with `sub2api-main/backend/go.mod` (`go 1.26.4` declared).
2. Install Rust toolchain and Cargo.
3. Enable or install pnpm. `corepack pnpm` currently works.
4. Install frontend dependencies in both frontends before typecheck/build.
5. Decide whether to initialize git baselines for extracted directories or re-clone real upstream repos.

## Sub2API Current Capabilities

Important backend entry points:

- Router setup: `sub2api-main/backend/internal/server/router.go`
- User routes: `sub2api-main/backend/internal/server/routes/user.go`
- Gateway routes: `sub2api-main/backend/internal/server/routes/gateway.go`
- Payment routes: `sub2api-main/backend/internal/server/routes/payment.go`
- Admin routes: `sub2api-main/backend/internal/server/routes/admin.go`

Existing route groups:

- `/api/v1/auth/*`: login/register/session/OAuth.
- `/api/v1/user/*`: profile, API key usage, quotas, TOTP.
- `/api/v1/keys/*`: user API key CRUD.
- `/api/v1/groups/*`: user-visible groups and rates.
- `/api/v1/usage/*`: usage list/stats/dashboard.
- `/api/v1/redeem/*`: user redeem and history.
- `/api/v1/subscriptions/*`: user subscription list/active/progress/summary.
- `/api/v1/payment/*`: plans, order create/verify/cancel/refund request.
- `/api/v1/payment/webhook/*`: EasyPay, Alipay, WeChat Pay, Stripe, Airwallex.
- `/api/v1/admin/*`: users, groups, accounts, redeem codes, subscriptions, payment, settings, ops, channel monitor, risk control.
- `/v1/*`, `/responses`, `/backend-api/codex/*`: OpenAI/Claude/Gemini/Codex-compatible gateway paths.

Existing storage primitives that likely map to Codex++ MVP:

- `subscription_plans`: saleable plans tied to groups.
- `user_subscriptions`: user entitlement period, status, usage windows.
- `redeem_codes`: one-time activation codes with group mapping.
- `api_keys`: user-side gateway keys, status, quota, rate-limit fields.
- `groups`: model/platform/rate/subscription grouping and many gateway policy fields.
- `payment_orders` and `payment_provider_instances`: payment integration.
- `idempotency_records`: likely useful for callback and resource creation idempotency.
- `usage_logs`: request/account/user usage visibility.

Assessment:

- Do not build a separate mini billing system for MVP.
- Reuse existing subscription, group, API key, redeem, payment and usage foundations where safe.
- Add only the Codex++-specific layer that existing Sub2API does not own: client bootstrap snapshot, desktop devices, Codex++ Cloud provider policy, and contract-shaped responses.

## CodexPlusPlus Current Capabilities

Important desktop entry points:

- Tauri app bootstrap: `CodexPlusPlus-main/apps/codex-plus-manager/src-tauri/src/lib.rs`
- Tauri commands: `CodexPlusPlus-main/apps/codex-plus-manager/src-tauri/src/commands.rs`
- React UI shell: `CodexPlusPlus-main/apps/codex-plus-manager/src/App.tsx`
- Rust settings/provider write logic: `CodexPlusPlus-main/crates/codex-plus-core/src/settings.rs`
- Relay/profile config: `CodexPlusPlus-main/crates/codex-plus-core/src/relay_config.rs`
- Provider sync: `CodexPlusPlus-main/crates/codex-plus-data/src/provider_sync.rs`
- Launcher: `CodexPlusPlus-main/crates/codex-plus-core/src/launcher.rs`

Existing useful capability:

- Can write/inspect Codex config and provider profiles.
- Can sync provider metadata in local Codex state.
- Has tests around provider sync rollback, config writes, launcher behavior and relay profiles.
- Manager UI already has local settings, relay mode, provider presets and launch controls.

Current gap:

- No Sub2API account login/session UI for ordinary users.
- No `/api/v1/client/bootstrap` consumer.
- No device registration/refresh path.
- No Codex++ Cloud managed-provider abstraction sourced from backend bootstrap.
- No user-visible plan/usage/status dashboard tied to Sub2API entitlement.
- No redacted diagnostics path specifically for cloud bootstrap/provider write failures.

Assessment:

- Desktop work should be additive: new `codexplus-cloud` runtime/UI modules, not a rewrite of existing local relay controls.
- Existing manual provider and relay profile features must remain intact.
- The managed provider writer should reuse existing config-writing code and tests rather than inventing a second writer.

## Contract Gate Status

Required by `CONTRACT-GATE.md`, now present as Phase 0 contract artifacts:

- `codex-plus-contracts/api/client-openapi.yaml`
- `codex-plus-contracts/config/*.schema.json`
- `codex-plus-contracts/status-error/client-status-errors.md`
- `codex-plus-contracts/events/*.schema.json`
- `codex-plus-contracts/test-fixtures/client/*.json`
- `codex-plus-contracts/compatibility-matrix.md`
- `codex-plus-contracts/change-review-policy.md`
- `codex-plus-contracts/storage-decision.md`

Required API not currently present:

- `GET /api/v1/client/bootstrap`
- `GET /api/v1/client/usage`
- `POST /api/v1/client/devices`
- `POST /api/v1/client/redeem`

The closest existing routes are `/api/v1/user/*`, `/api/v1/redeem`, `/api/v1/subscriptions`, `/api/v1/payment/*`, and `/v1/*` gateway routes. The MVP should add a client-oriented facade instead of forcing the desktop client to compose many existing web/admin endpoints.

Implementation workers may now consume the contracts, but any field/status/config change must follow `codex-plus-contracts/change-review-policy.md`.

## Storage Decision Recommendation

Recommended MVP storage shape:

| Domain | Recommendation | Reason |
| --- | --- | --- |
| Entitlement | Reuse `user_subscriptions`, `subscription_plans`, `groups` | Existing subscription model already maps user to group and expiry/usage windows. |
| User-side API key | Reuse `api_keys` for the secret and gateway auth; add `codexplus_managed_provider_keys` mapping | `api_keys.name` is not unique and has no metadata field, so Codex++ idempotency needs its own owner mapping. |
| Redeem | Reuse `redeem_codes` and current redeem service | Existing one-time code and group mapping already exists. |
| Payment | Reuse existing payment order/provider/callback flow | MVP can test later; do not duplicate. |
| Model catalog | Map from `groups`, `models_list_config`, account/channel capabilities, plus Codex++ config metadata | Avoid parallel model truth in client. |
| Usage summary | Reuse `usage_logs`, API key usage windows and subscription usage fields | Existing usage surfaces are broad enough for MVP summaries. |
| Devices | Add a small typed `codexplus_devices` Ent schema/table | Device revoke/upsert/user isolation needs clean unique constraints. |
| Bootstrap snapshot/version | Store current Codex++ config in `settings.key = "codexplus_config_v1"` | Client needs `config_version` and `snapshot_version`, and the existing settings table supports MVP key/value JSON. |
| Events | Add `codexplus_events` unless a general-purpose append-only audit table is confirmed before migration work | Payment audit logs are payment-specific and should not become a runtime/gateway event sink. |

Frozen decisions before implementation:

- API key display name is `Codex++ Cloud`, but source of truth is `codexplus_managed_provider_keys`.
- Bootstrap returns the full user-side gateway key for successful authenticated bootstrap responses; logs/events/admin views must redact it.
- Device unique constraint is `(user_id, device_id)`.
- Feature flags live in the `codexplus_config_v1` settings JSON for MVP.
- `config_version` increments on every validated admin write; rollback writes a previous validated document as a new version with `rollback_from`.

## Risk Review

Must solve now:

- Contract freeze before implementation.
- Storage/idempotency decision before migrations.
- User isolation for `/api/v1/client/*`.
- Idempotent key creation and device upsert.
- Gateway policy source of truth.
- Redaction of user-side key, JWT and Authorization.
- Git/version baseline before multi-agent development.
- Local toolchain setup before claiming tests pass.

Simplify now:

- Use existing Sub2API subscription/payment/redeem/admin foundations.
- Make `/api/v1/client/*` a stable facade over existing services.
- Start with admin/manual entitlement and redeem; keep real payment in MVP support path but not the first blocker.
- Use a single managed provider ID such as `codex-plus-cloud`.

Defer:

- Full production deployment execution.
- Complex anti-abuse scoring.
- Team plans, white-label, invoices and support ticket productization.
- Full gray release platform; keep config version and rollback notes.

## Phase 1 Startup Plan

### Step 0: Establish Source Baseline

Required before broad edits:

- Decide whether to initialize git repositories in extracted directories or re-clone from upstream.
- If using extracted directories, create an initial baseline commit before any implementation.
- Keep `codex-plus-dev-plan` as planning source of truth.

Recommended command after user approval:

```powershell
cd C:\Users\23293\Desktop\codex+++\sub2api-main
git init
git add .
git commit -m "baseline: extracted sub2api source"

cd C:\Users\23293\Desktop\codex+++\CodexPlusPlus-main
git init
git add .
git commit -m "baseline: extracted CodexPlusPlus source"
```

### Step 1: Prepare Toolchain

Required:

- Go matching backend `go.mod`.
- Rust/Cargo and Tauri prerequisites.
- pnpm through Corepack or global install.
- Frontend dependencies.

Suggested verification commands:

```powershell
go version
rustc --version
cargo --version
corepack pnpm --version
docker --version
```

### Step 2: Run Phase 0 Contract Work

Create:

- `codex-plus-contracts/api/client-openapi.yaml`
- `codex-plus-contracts/config/plan-catalog.schema.json`
- `codex-plus-contracts/config/model-catalog.schema.json`
- `codex-plus-contracts/config/usage-policy.schema.json`
- `codex-plus-contracts/config/feature-flags.schema.json`
- `codex-plus-contracts/status-error/client-status-errors.md`
- `codex-plus-contracts/events/client-events.schema.json`
- `codex-plus-contracts/test-fixtures/client/*.json`
- `codex-plus-contracts/compatibility-matrix.md`
- `codex-plus-contracts/change-review-policy.md`
- `codex-plus-contracts/storage-decision.md`

Exit gate:

- `CONTRACT-GATE.md` checklist can be marked complete.
- Worker prompts updated if actual contract path differs.

### Step 3: Backend Foundation

Implement after contract/storage approval:

- Codex++ config read service.
- Codex++ device repository/service.
- Codex++ user-side API key ensure/reuse service.
- Bootstrap snapshot assembler.
- Targeted tests for idempotency, user isolation and redaction.

### Step 4: Backend Client API and Gateway

Implement:

- `/api/v1/client/bootstrap`
- `/api/v1/client/usage`
- `/api/v1/client/devices`
- `/api/v1/client/redeem`
- Gateway policy checks for model entitlement, expiry, balance/usage, revoked device where applicable.

### Step 5: Desktop Runtime and Admin UI

Implement:

- Sub2API login/session in Manager.
- Bootstrap API client and cache.
- Managed provider writer for `Codex++ Cloud`.
- Status/plan/usage model display.
- Admin minimal Codex++ config/entitlement surface, reusing existing payment/subscription/group UI where possible.

### Step 6: MVP E2E

Run:

- Admin grants or redeem activates test user.
- User logs in from Manager.
- Manager registers device and calls bootstrap.
- Manager writes `Codex++ Cloud`.
- Manager launches Codex.
- One low-cost request goes through Sub2API gateway.
- Usage/log/audit record exists.

## First Worker Dispatch Recommendation

Phase 0 Module A/B is complete. Do not dispatch all modules blindly; start with
the implementation modules that consume the frozen contracts while keeping
global write areas single-owned:

1. Module C: Config and storage foundations
   - Implement `codexplus_config_v1`, `codexplus_devices`,
     `codexplus_managed_provider_keys` and `codexplus_events`.
   - Own migration/schema-affecting work; other modules must not edit migrations.

2. Module D: Client API skeleton and contract tests
   - Implement `/api/v1/client/*` routes against fixtures and frozen OpenAPI.
   - Own route registration only if the integration owner assigns that file.

3. Module F/G/H: Desktop runtime, desktop UX and admin MVP
   - May proceed in parallel from fixtures and schemas.
   - Must not modify contract artifacts or backend migrations.
   - Module F must follow `PHASE1-MODULE-F-DESKTOP-RUNTIME-PLAN.md` and expose stable Tauri command payloads before Module G final UI integration.
   - Module G must follow `PHASE1-MODULE-G-DESKTOP-UX-PLAN.md` and keep ordinary-user cloud UX separate from advanced provider editing.
   - Module H must follow `PHASE1-MODULE-H-ADMIN-OPERATIONS-PLAN.md` and compose existing payment/group/subscription APIs instead of duplicating billing or entitlement.

4. Module I/J: E2E gate and integration coordinator
   - Module I must follow `PHASE1-MODULE-I-E2E-RELEASE-GATE-PLAN.md`; it may prepare checklists early but final pass/fail waits for D/E/F/G/H.
   - Module J must follow `PHASE1-MODULE-J-INTEGRATION-COORDINATOR-PLAN.md`; it merges, verifies and syncs docs/HTML after implementation workers report.

Module E gateway enforcement should begin after Module C exposes the policy
read interface and after Module D can produce bootstrap/device context.

## Remaining External Readiness Items

- Source directories are not git repositories.
- Go is not installed.
- Rust/Cargo is not installed.
- Global pnpm is not installed, though `corepack pnpm` works.

Cleared items:

- Contract artifacts exist and passed light validation.
- Storage decisions are frozen for MVP implementation.

## Suggested Next Action

Start Phase 1 worker dispatch using the frozen Phase 0 artifacts:

```text
按 PARALLEL-DISPATCH-PLAN.md 分配 Module C/D/E/F/G/H。

优先顺序：
- Module C: 先按 PHASE1-MODULE-C-BACKEND-FOUNDATION-PLAN.md 实现 codexplus_config_v1、codexplus_devices、codexplus_managed_provider_keys、codexplus_events 的后端基础。
- Module D: 按 PHASE1-MODULE-D-CLIENT-API-PLAN.md 实现 /api/v1/client/* skeleton、facade service 与 contract tests。
- Module E: 按 PHASE1-MODULE-E-GATEWAY-ENFORCEMENT-PLAN.md 实现 gateway MVP policy resolver，只消费已冻结的状态/事件/配置。
- Module F: 按 PHASE1-MODULE-F-DESKTOP-RUNTIME-PLAN.md 实现 bootstrap consumer、本地 state、`Codex++ Cloud` managed provider writer 和日志脱敏。
- Module G: 按 PHASE1-MODULE-G-DESKTOP-UX-PLAN.md 实现普通用户云首页、登录绑定、安装辅助、新手教学和高级配置隐藏。
- Module H: 按 PHASE1-MODULE-H-ADMIN-OPERATIONS-PLAN.md 实现 admin Codex++ 配置发布、计划/模型/用量/开关管理和用户权益视图。
- Module I: 按 PHASE1-MODULE-I-E2E-RELEASE-GATE-PLAN.md 准备 E2E 证据目录、测试账号矩阵、失败路径和 go/no-go 输入，最终执行等待 D/E/F/G/H。
- Module J: 按 PHASE1-MODULE-J-INTEGRATION-COORDINATOR-PLAN.md 负责合并、冲突处理、最终验证、HTML/文档同步和发布裁决。
```
