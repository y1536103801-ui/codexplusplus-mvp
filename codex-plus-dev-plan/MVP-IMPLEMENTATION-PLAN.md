# Codex++ MVP Implementation Plan

本文档用 `pm-codex-workflow` 口径把现有工业级蓝图收敛成第一阶段可交付闭环。`ARCHITECTURE.md` 仍是长期目标；本文件决定第一阶段本地/测试 MVP 只做什么、先不做什么、如何验收。生产服务器、真实域名、HTTPS、密钥、备份、监控、真实支付回调和上线回滚由 [PRODUCTION-LAUNCH-PLAN.md](PRODUCTION-LAUNCH-PLAN.md) 负责。

## Status

- State: planning
- Owner: Codex coordinator
- Created: 2026-06-16
- Last updated: 2026-06-16
- Source projects: `sub2api-main/`, `CodexPlusPlus-main/`

## Goal

第一版只证明一条商业闭环：用户被后台开通权益后，登录 Codex++，客户端自动拉取托管配置，写入 `Codex++ Cloud` provider，一键启动 Codex，并完成一次通过 Sub2API 网关的模型请求。

本阶段默认运行在本地或测试环境。即使本阶段完成，也只能说明产品核心链路可运行，不代表已经具备正式上线售卖条件。

## Requirement Restatement

要解决的问题：

- 普通用户不需要理解 Base URL、API Key、模型倍率、套餐规则，也能买完或被开通后直接使用 Codex。
- 管理员能在后台控制套餐、模型、默认模型、余额/额度和功能开关。
- 网关必须在服务端强制执行权益，不能依赖客户端隐藏按钮。

本轮范围：

- 冻结客户端 API、配置 schema、状态错误模型和 mock。
- 建立最小后台配置与权益读取能力。
- 实现 `GET /api/v1/client/bootstrap` 作为 MVP 主接口。
- 实现最小设备登记、usage 摘要和兑换码接口框架，允许部分能力先返回受控占位。
- 客户端登录后消费 bootstrap，写入或更新 `Codex++ Cloud` 托管供应商。
- 客户端展示服务状态、套餐/余额摘要、可用模型、一键启动和可执行错误提示。
- 网关至少强制拒绝：未授权模型、套餐过期、余额不足、设备撤销。
- 用手动开通或测试订单完成 E2E 验收。

本轮不做：

- 不做生产服务器部署、域名 HTTPS、生产数据库、生产 Redis、生产密钥、备份恢复、监控告警和发布回滚。
- 不做团队套餐、发票、工单、白标、多租户销售体系。
- 不做完整自动支付闭环作为 MVP 阻塞项；支付可作为后续阶段接入，MVP 可用后台手动开通或测试订单。
- 不做复杂风控模型、盗刷画像、自动封禁策略。
- 不做完整灰度发布平台；只保留配置版本、发布记录和回滚说明字段。
- 不重构 Codex 官方文件或删除用户已有手动供应商配置。

## Acceptance Criteria

- 新测试用户登录后可调用 `GET /api/v1/client/bootstrap`，响应包含服务状态、用户侧 Key、网关地址、当前套餐摘要、可用模型、usage 摘要、feature flags、配置版本和快照版本。
- 同一用户重复调用 bootstrap 不重复创建 Key，不泄露上游真实凭证。
- 客户端根据 bootstrap 写入 `Codex++ Cloud` provider，旧手动供应商不丢失。
- 客户端能展示 available、not_purchased、expired、low_balance、device_revoked、model_unavailable、gateway_unhealthy、local_config_failed 等状态中的 MVP 子集。
- 修改后台默认模型或下架模型后，客户端无需发版即可在下一次 bootstrap/刷新后体现。
- 网关请求未授权模型、余额不足、套餐过期、设备撤销时在服务端拒绝，并产生可排查日志或事件。
- E2E 清单能走通：开通权益 -> 登录 -> bootstrap -> 写 provider -> 启动 Codex -> 完成一次请求 -> 查看 usage/日志。
- 所有新增日志对 API Key、JWT、Authorization header、Base URL query token 脱敏。

## Risk Level

高风险。

原因：

- 涉及登录鉴权、用户隔离、API Key、套餐权益、余额、网关拒绝、桌面本地配置和潜在支付/计费。
- 多项目联动，且存在后端、管理后台、桌面端、网关请求路径的跨层契约。
- 若并行开发没有文件归属和契约门禁，容易出现 API/UI 字段漂移、重复创建 Key、错误扣费或泄露凭证。

## Professional Risk Review

| 风险域 | 当前决策 | MVP 处理方式 |
| --- | --- | --- |
| 数据一致性 | 必须现在解决 | Key 创建、设备 upsert、权益读取必须幂等；bootstrap 不能重复创建资源。 |
| 并发/幂等 | 必须现在解决 | 设备登记、Key 创建、兑换码兑换、支付回调后续接入都必须有幂等键或唯一约束。 |
| 权限/安全 | 必须现在解决 | 所有 `/api/v1/client/*` 使用用户 JWT；管理员接口继续走 admin 权限；用户只能读取自己的状态。 |
| 计费/订单/credits | 先简化 | MVP 支持后台手动开通或测试订单；真实支付自动开通放入 `06-commerce-and-enforcement`。 |
| 数据库迁移/兼容 | 必须现在解决 | 契约阶段先决定新增表/旧表扩展/setting JSON；迁移需有旧数据默认值和回滚说明。 |
| 外部服务 | 必须现在解决 | 网关请求路径不能调用不稳定控制面表单解析；策略应预编译或缓存。 |
| 日志/可排查性 | 必须现在解决 | bootstrap、gateway rejection、provider write、local config failure 都要有 request ID 或可定位上下文。 |
| 测试/回归 | 必须现在解决 | 每阶段必须运行最小后端/前端/客户端检查，最终跑 E2E 手动或脚本验收。 |
| 上线/回滚 | 先简化 | MVP 需要配置版本和回滚说明，但不要求完整灰度平台。 |
| 风控 | 推迟 | 先记录设备和异常事件；自动处置、风险评分、封禁策略后续做。 |

## MVP Architecture Plan

### Files and modules to inspect first

- `sub2api-main/backend/internal/server/routes/user.go`
- `sub2api-main/backend/internal/server/routes/gateway.go`
- `sub2api-main/backend/internal/server/router.go`
- `sub2api-main/backend/internal/service/setting_service.go`
- `sub2api-main/backend/internal/service/api_key_service.go`
- `sub2api-main/backend/internal/service/redeem_service.go`
- `sub2api-main/backend/internal/service/payment_*`
- `sub2api-main/backend/internal/service/billing_*`
- `sub2api-main/backend/internal/service/ratelimit_service.go`
- `sub2api-main/frontend/src/views/admin/SettingsView.vue`
- `sub2api-main/frontend/src/views/admin/orders/AdminPaymentPlansView.vue`
- `CodexPlusPlus-main/crates/codex-plus-core/src/settings.rs`
- `CodexPlusPlus-main/crates/codex-plus-core/src/relay_config.rs`
- `CodexPlusPlus-main/crates/codex-plus-core/src/launcher.rs`
- `CodexPlusPlus-main/apps/codex-plus-manager/src/App.tsx`
- `CodexPlusPlus-main/apps/codex-plus-manager/src-tauri/src/commands.rs`

### Expected change areas

- `codex-plus-contracts/` or equivalent docs/contracts folder for OpenAPI, schema, mocks, events, error codes and generated types.
- `sub2api-main/backend/internal/codexplus/` for additive Codex++ modules when practical.
- `sub2api-main/backend/internal/server/routes/` only for route registration with a single owner.
- `sub2api-main/frontend/src/views/admin/` or new Codex++ admin pages for config and entitlement views.
- `CodexPlusPlus-main/crates/codex-plus-core/src/codexplus_runtime/` or equivalent additive runtime modules.
- `CodexPlusPlus-main/apps/codex-plus-manager/src/features/codexplus-cloud/` or equivalent additive UI modules.

### Areas that should not change in MVP

- Do not rewrite existing payment provider signature verification.
- Do not remove existing manual provider configuration.
- Do not store upstream real API keys in the desktop client.
- Do not change unrelated gateway behavior for non-Codex++ users.
- Do not edit package manifests or lockfiles unless a phase explicitly owns dependency changes.

## Data Flow

```text
Admin config/manual entitlement
  -> Config Registry / Entitlement
  -> Client Bootstrap API
  -> Codex++ desktop bootstrap consumer
  -> Managed Provider writer
  -> Codex launcher
  -> Sub2API gateway policy enforcement
  -> Usage event / logs / usage summary
```

## API and Data Shape Changes

The exact fields are frozen by `CONTRACT-GATE.md`. MVP must at minimum define:

- `GET /api/v1/client/bootstrap`
- `GET /api/v1/client/usage`
- `POST /api/v1/client/devices`
- `POST /api/v1/client/redeem`
- `PlanCatalog`, `ModelCatalog`, `UsagePolicy`, `FeatureFlags`
- `ClientServiceStatus`, `ClientErrorCode`
- `bootstrap_requested`, `device_registered`, `gateway_policy_rejected`, `usage_recorded`

## Phase Plan

### MVP Phase 0: Discovery and Contract Gate

- Goal: freeze MVP contracts before broad implementation.
- Deliverables: OpenAPI/schema/mock/error/events/types plan, data storage decision, hotspot file ownership.
- Acceptance: `CONTRACT-GATE.md` checklist is complete; no backend/client worker starts without explicit contract inputs.
- Status: complete for MVP implementation; contract artifacts are in `codex-plus-contracts/`, storage decisions are coordinator-reviewed, and later changes must follow `change-review-policy.md`.

### MVP Phase 1: Backend Contract Producers

- Goal: implement minimal config/entitlement/device/Key foundations behind stable APIs.
- Deliverables: config schema/defaults, device upsert, Key reuse/create contract, bootstrap service skeleton.
- Acceptance: backend tests cover validation, auth, idempotent Key/device behavior, redaction.
- Status: not started.

### MVP Phase 2: Backend Consumers and Gateway Enforcement

- Goal: expose `/api/v1/client/*` and enforce MVP policy in gateway.
- Deliverables: client routes/handlers, usage summary, gateway policy resolver, rejection events.
- Acceptance: unauthorized model, expired, low balance, revoked device and valid request paths are tested.
- Status: not started.

### MVP Phase 3: Desktop Client Runtime and UX

- Goal: consume bootstrap and make ordinary users able to launch.
- Deliverables: login/session store, bootstrap cache, managed provider writer, status UI, install assistant shell.
- Acceptance: provider write does not delete manual providers; logs are redacted; local failure states are actionable.
- Status: not started.

### MVP Phase 4: Admin Operations

- Goal: give operators enough controls for MVP support.
- Deliverables: minimal plan/model/default model/feature flag configuration and user entitlement view.
- Acceptance: admin can manually open/expire/restore a test user and change default model without client release.
- Status: not started.

### MVP Phase 5: Integration and Release Gate

- Goal: verify the full path and prepare release notes.
- Deliverables: E2E script or manual checklist, package install check, compatibility migration notes, rollback notes.
- Acceptance: E2E passes in a test environment; docs and HTML index match implemented scope.
- Status: not started.

## Test Plan

- Backend: `cd sub2api-main/backend && go test ./...`
- Backend full gate: `cd sub2api-main/backend && make test`
- Sub2API aggregate: `cd sub2api-main && make test`
- Frontend admin: `cd sub2api-main/frontend && pnpm run lint:check && pnpm run typecheck && pnpm run test:run`
- Desktop manager typecheck: `cd CodexPlusPlus-main/apps/codex-plus-manager && npm run check`
- Desktop manager build: `cd CodexPlusPlus-main/apps/codex-plus-manager && npm run build`
- Rust core: `cd CodexPlusPlus-main && cargo test --workspace`
- E2E: manual or scripted buy/open entitlement -> login -> bootstrap -> provider write -> launch -> request -> usage/log check.

## Open Questions Before Coding

- Codex++ MVP 权益复用现有 `subscription_plans`、`user_subscriptions`、`groups`、用户余额和 API key quota/rate-limit 字段，不新增独立 billing 系统。
- `provider.api_key` 在 successful bootstrap 中返回完整用户侧网关 Key，用于客户端自动写入/修复 `Codex++ Cloud` provider；该 Key 不是上游真实凭证，日志、事件、后台视图和测试证据必须脱敏。
- MVP 允许无真实支付先跑通后台手动开通或测试订单；真实支付自动开通进入后续 commerce/enforcement 阶段。

## Frozen Phase 0 Decisions

- Codex++ 当前配置写入 `settings.key = "codexplus_config_v1"`，值为通过 schema 校验的版本化 JSON。
- 用户侧网关 Key 的实际 secret 复用 `api_keys`，但 Codex++ 托管供应商身份和幂等性使用 `codexplus_managed_provider_keys`，唯一键为 `(user_id, managed_provider_id)`。
- 设备状态新增 typed Ent schema/table：`codexplus_devices`，唯一键为 `(user_id, device_id)`。
- Codex++ runtime/gateway 事件新增 `codexplus_events`，除非实现前确认已有通用 append-only audit table。
- Phase 1 worker 可以消费 `codex-plus-contracts/`，但不得私自改动契约字段；发现字段缺口时必须按 `change-review-policy.md` 提交契约 patch。

## Next Step

按 `PARALLEL-DISPATCH-PLAN.md` 分阶段派工进入 Phase 1。建议先启动 Module C 的配置/存储基础和 Module D 的客户端 API skeleton，但 route 注册、migration、gateway hook 和 lockfile 仍必须保持单一 owner。任何 worker 需要新增/修改跨层字段时，先提交契约 patch，不得后端和客户端自行对齐。
