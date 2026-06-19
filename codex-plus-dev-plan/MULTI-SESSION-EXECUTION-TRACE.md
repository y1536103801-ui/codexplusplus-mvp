# Codex++ Multi-Session Execution Trace

本文档记录当前实现轮次如何把计划文档转成真实多会话执行。它不替代 `PARALLEL-DISPATCH-PLAN.md`，只记录本轮已经发生的任务拆分、代码落点和剩余阻塞。

## 2026-06-16 Correction Round

本轮根据用户反馈回到计划原点：不再继续用单一会话穿透多个阶段，而是先恢复 `00-contract` 的阶段门禁。

当前并行会话已完成 `00-contract`、`01-backend-config-center`、`02-backend-client-api`、`03-client-cloud-core`、`04-client-user-experience`、`05-admin-operations`、`06-commerce-and-enforcement` 和 `07-integration-release`。阶段门禁、当前 worker 和退出条件见 [STAGE-GATE-LEDGER.md](STAGE-GATE-LEDGER.md)。

## 2026-06-19 Final MVP Handoff

| Lane | Agent | Stage task | Write scope | Status |
| --- | --- | --- | --- | --- |
| 0A | gate/tooling worker | 07 verifier/tooling repair | 07 verifier scripts and task metadata | completed, static and stage gates passed |
| 2A | E2E worker | local Windows-only E2E evidence | `20260619-1940-e2e` and E2E helpers | completed, evidence verifier passed |
| 2B | package worker | Windows-only package evidence | `20260619-1940-package` | completed, Windows-only package verifier passed |
| 2C | compatibility worker | isolated provider compatibility evidence | `20260619-1940-compatibility` | completed, compatibility verifier passed |
| 2D | business worker | owner-approved business readiness | `20260619-1940-business` | completed, business readiness verifier passed |
| 3A | main coordinator | Module J release aggregation | `20260619-1940-release`, status docs and final audit | completed, release handoff verifier passed |

Final status: owner-approved Windows-only local MVP gate is `go with accepted risks` using `codex-plus-dev-plan/test-runs/20260619-1940-release`. Production launch, real user profile mutation, public paid traffic and macOS packages remain outside this local MVP gate.

| Lane | Agent | Stage task | Write scope | Status |
| --- | --- | --- | --- | --- |
| A | Harvey | 客户端 API 契约冻结 | `client-openapi.yaml`, `test-fixtures/client/*.json`, `task-client-api-contract.md`, worker A final report | completed, gate passed |
| B | Pauli | 后台配置契约冻结 | `config/*.schema.json`, `task-admin-config-contract.md`, worker B final report | completed, gate passed |
| C | Bernoulli | 状态、错误码与事件契约冻结 | `client-status-errors.md`, `client-events.schema.json`, `task-status-and-error-model.md`, worker C final report | completed, gate passed |
| Coordinator | main | 阶段门禁、合并复核、状态记录 | `STAGE-GATE-LEDGER.md`, this trace, `IMPLEMENTATION-STATUS.md` | active |

本轮状态：旧 A/B/C 在返回 final report 前因额度限制失败；当前 A/B/C 已重新派发并完成。`01` 到 `06` 的阶段门禁均已通过；`07-integration-release` 已启动；contract 文件优先于既有源码假设。

Worker B update: Pauli returned a final report, config schemas parse, and B1-B4 are answered as `fixed`. Full gate is still blocked on A/C final reports.

Worker C update: Bernoulli returned a final report, event schema parses, and C1-C4 are answered as `fixed`. Full gate is still blocked on Worker A final report and coordinator running-status residue.

Worker A update: Harvey returned a final report, client fixtures parse, and A1-A5 are answered as `fixed`. Coordinator running-status residue has been cleared; full stage gate verification is next.

Stage gate update: `powershell -ExecutionPolicy Bypass -File codex-plus-dev-plan\tools\validate-stage-gate.ps1` passed after A/B/C final reports and coordinator residue cleanup. `00-contract` is passed.

### 01-Backend Config Center Parallel Run

| Lane | Agent | Stage task | Write scope | Status |
| --- | --- | --- | --- | --- |
| P | Boyle | Plan Catalog config registry | `configregistry/plan_catalog.go`, `plan_catalog_test.go`, `task-plan-catalog.md`, plan report | completed, coordinator integrated, static gate passed |
| M | Ohm | Model Catalog config registry | `configregistry/model_catalog.go`, `model_catalog_test.go`, `task-model-catalog.md`, model report | completed, coordinator integrated, static gate passed |
| U | Kierkegaard | Usage Policy config registry | `configregistry/usage_policy.go`, `usage_policy_test.go`, `task-usage-policy.md`, usage report | completed, coordinator integrated, static gate passed |
| F | Rawls | Feature Flags config registry | `configregistry/feature_flags.go`, `feature_flags_test.go`, `task-feature-flags.md`, flags report | completed, coordinator integrated, static gate passed |

Coordinator created `sub2api-main/backend/internal/codexplus/configregistry/common.go` as shared read-only scaffolding for these workers. Shared service integration is reserved for coordinator after P/M/U/F return.

Worker U update: Kierkegaard returned a final report. Static file and field checks passed; Go/gofmt remain unavailable locally.

Worker M update: Ohm returned a final report. Static file and field checks passed; Go/gofmt remain unavailable locally.

Worker P update: Boyle returned a final report. Static file and field checks passed; Go/gofmt remain unavailable locally.

Worker F update: Rawls returned a final report. Static file and field checks passed; Go/gofmt remain unavailable locally.

Coordinator 01 integration update:

- P/M/U/F workers were closed after their final reports were captured.
- Main coordinator integrated `configregistry` into the existing shared config service instead of replacing downstream client/gateway/admin-facing types.
- The service now composes defaults from the four 01 registries and validates with the four registry validators.
- The coordinator added a default-reference alignment step because independently valid catalogs can still be incoherent when combined.
- The coordinator added targeted service tests for registry-backed defaults and registry validation handoff, and the current stage gate now checks those tests are present.
- At that point, `validate-stage-gate.ps1` supported `current`, `00-contract` and `01-backend-config-center`; later coordinator updates expanded it through the active `07-integration-release` gate.
- `verify-01-static.ps1` now provides a repeatable offline audit for registry integration constraints in environments without Go.
- `verify-01-go.ps1` now captures the exact Go/gofmt compile gate command for prepared environments.
- The 01 static gate passed both with explicit `-Stage 01-backend-config-center` and with default current-stage resolution.
- `verify-01-go.ps1` passed with local Go 1.26.4: `gofmt -l` is clean and targeted Go tests passed, so 01 is passed and 02 is active.
- Read-only review S found a service validation fallback that could hide missing `display_price`; coordinator removed that fallback and widened shared draft-status validation to the frozen lifecycle.
- Read-only review S also flagged downstream client/gateway/admin hardcoding or first-item behavior; these are recorded as later-stage gate risks, not treated as solved in 01.
- Read-only review T found the P/M/U/F table had stale pre-integration statuses; coordinator corrected the table.

Coordinator follow-up:

- Added [00-contract/COORDINATOR-PREAUDIT.md](00-contract/COORDINATOR-PREAUDIT.md) as restart input.
- Updated [00-contract/PARALLEL-RESTART-PACK.md](00-contract/PARALLEL-RESTART-PACK.md), [CONTRACT-GATE.md](CONTRACT-GATE.md) and [STAGE-GATE-LEDGER.md](STAGE-GATE-LEDGER.md) so A/B/C workers must answer the preaudit items before `00-contract` can pass.
- Added [tools/validate-stage-gate.ps1](tools/validate-stage-gate.ps1) and [00-contract/reports/README.md](00-contract/reports/README.md). The first run failed only on missing A/B/C final report files, which is the intended current gate behavior.
- Tightened final-report validation and added `.template.md` report templates. The latest run still fails only on the three missing final report files, so templates cannot accidentally pass the gate.

### 02-Backend Client API Dispatch Ready

下一轮只允许在 `02-backend-client-api` 内并行，不允许越级实现 03-07：

| Lane | Stage task | Write scope | Status |
| --- | --- | --- | --- |
| D1 | Client Device API | `/api/v1/client/devices` handler/service/repository seams and device tests | ready to dispatch |
| D2 | Client Bootstrap API | `/api/v1/client/bootstrap` aggregation, managed key reuse, snapshot tests | ready to dispatch |
| D3 | Client Usage API | `/api/v1/client/usage` status mapping, policy snapshot, usage tests | ready to dispatch |
| D4 | Client Redeem API | `/api/v1/client/redeem` flow, status mapping, redeem tests | ready to dispatch |

02 worker 必须消费 `00-contract` 和 `01-backend-config-center` 输出；不得在客户端或 API 层硬编码价格、套餐、模型倍率、额度阈值、限流策略或续费文案。

### 02-Backend Client API Exit Gate

`02-backend-client-api` 已通过 coordinator 集成门禁：

- `/api/v1/client/bootstrap`、`/usage`、`/devices`、`/redeem` 已接入 authenticated client routes。
- bootstrap/usage/device/redeem 的服务层和 DTO 层对齐客户端契约字段，并保留 legacy-compatible success envelope。
- usage 状态与网关 enforcement 共用后台配置来源；client API 不再使用 `firstPlan` / `firstUsagePolicy` 位置兜底。
- 结构化事件包含 request context，并在有配置快照的接口保留 config version。
- `tools/verify-02-static.ps1` passed。
- `tools/verify-02-go.ps1` passed。
- `tools/validate-stage-gate.ps1 -Stage 02-backend-client-api` passed。
- 当前顺序门禁状态：`00` passed，`01` passed，`02` passed，`03-client-cloud-core` passed，`04-client-user-experience` active，`05-07` blocked。

### 03-Client Cloud Core Exit Gate

`03-client-cloud-core` 已通过 coordinator 退出门禁：

- 桌面 runtime 已消费 `02` bootstrap/usage/device/redeem 输出字段，并把 `message_key`、`commerce_action`、`action_copy_key`、feature flags 和 usage display 字段投影到本地状态。
- 本地 state 只暴露 `has_api_key` 布尔值，不序列化 provider key/JWT；归一化后的 `Codex++ Cloud` profile 也能正确识别 key 已配置。
- `Codex++ Cloud` provider 继续通过本地 helper 转发主网关请求，并保持后台 gateway URL 与写给 Codex 的 helper URL 分离。
- `tools/verify-03-static.ps1` passed。
- `tools/verify-03-node.ps1` passed。
- `tools/verify-03-rust.ps1` passed after preparing a local workspace Rust/MinGW toolchain; `codexplus_cloud` Rust tests passed 22/22, and `relay_config` / `protocol_proxy` filtered tests passed.
- 当前顺序门禁状态：`00` passed，`01` passed，`02` passed，`03` passed，`04-client-user-experience` active，`05-07` blocked。

### 04-Client User Experience Exit Gate

`04-client-user-experience` 已通过 coordinator 退出门禁：

- 同阶段四个并行 worker 已完成并被 coordinator 集成：Leibniz 登录绑定 UI、Bacon 首页/状态/用量 UI、Hegel 安装助手 UI、Newton 新手教程 UI。
- Cloud home、登录绑定、安装辅助、新手教程、用量/模型、诊断和高级配置入口均基于 03 runtime/bootstrap 状态模型展示，不硬编码价格、套餐、额度、倍率、模型权益或续费策略。
- 普通浏览器预览已降级到 fixture 状态，不再展示 Tauri IPC 技术错误；自动刷新不再弹出初始遮挡 toast。
- 窄屏样式已复验，390px 下顶栏、Cloud 卡片和操作按钮无裁切。
- `tools/verify-04-static.ps1` passed。
- `tools/verify-04-node.ps1` passed。
- `npm run vite:build` passed。
- Edge headless `1440x900` 和 `390x844` 视觉复验 passed。
- 当前顺序门禁状态：`00` passed，`01` passed，`02` passed，`03` passed，`04` passed，`05-admin-operations` active，`06-07` blocked。

### 05-Admin Operations Exit Gate

`05-admin-operations` 已通过 coordinator 退出门禁：

- 同阶段四个并行 worker 已完成并被 coordinator 集成：Singer 套餐管理、Hilbert 模型管理、Maxwell 用量策略/功能开关、Bernoulli 用户权益视图。
- 后台 shared admin API 类型已覆盖 05 所需字段：价格、entitlement source、usage policy link、copy keys、模型 rollout/deprecation、usage quota/device policy 和 server-only feature flags。
- 管理员页面可维护套餐、模型、用量策略和功能开关，并可查看单个用户权益、设备、托管 key、用量摘要、近期事件和 integration status。
- `tools/verify-05-static.ps1` passed。
- `tools/verify-05-node.ps1` passed。
- `tools/verify-05-go.ps1` passed。
- `tools/validate-stage-gate.ps1 -Stage 05-admin-operations` passed。
- 当前顺序门禁状态：`00` passed，`01` passed，`02` passed，`03` passed，`04` passed，`05` passed，`06-commerce-and-enforcement` active，`07` blocked。

### 06-Commerce And Enforcement Exit Gate

`06-commerce-and-enforcement` 已通过 coordinator 退出门禁：

- 同阶段四条并行线已完成或被 coordinator 以落地代码和测试证据接受：Zeno 支付权益、Hubble 网关强制执行、Gauss 设备管理、Dewey 审计风控。
- Payment entitlement flow 接入 Codex++ subscription order -> Plan Catalog entitlement grant，覆盖幂等 grant、expired-grace 恢复和 client state refresh。
- Gateway enforcement 现在通过 `CodexPlusGatewayPolicyService` 强制执行套餐/模型/设备/用量策略，并生成可追踪、脱敏的拒绝事件。
- Device management 支持管理员列出、撤销和恢复设备，repository 层保持 revoked/blocked 状态不被客户端 upsert 覆盖。
- Audit and risk control 支持 user/device/request/config context、gateway rejection summary、redaction 和支持侧查询。
- `tools/verify-06-static.ps1` passed。
- `tools/verify-06-go.ps1` passed。
- `tools/validate-stage-gate.ps1 -Stage 06-commerce-and-enforcement` passed。
- 当前顺序门禁状态：`00` passed，`01` passed，`02` passed，`03` passed，`04` passed，`05` passed，`06` passed，`07-integration-release` active。

### 07-Integration Release Dispatch Ready

下一轮只允许在 `07-integration-release` 内并行，不再新增 00-06 的大功能：

| Lane | Stage task | Write scope | Status |
| --- | --- | --- | --- |
| R1 | E2E buy/login/launch | `07-integration-release/**` E2E evidence, approved E2E tools/checklists | ready to dispatch |
| R2 | Compatibility and migration | migration notes, compatibility test evidence, rollback notes | ready to dispatch |
| R3 | Package install check | installer/package checklist and platform evidence | ready to dispatch |
| R4 | Docs and product copy | README/product docs/HTML/release notes sync | ready to dispatch |

07 worker 必须消费 `INTEGRATION-VERIFICATION-CHECKLIST.md`、`PHASE1-MODULE-I-E2E-RELEASE-GATE-PLAN.md` 和 `PHASE1-MODULE-J-INTEGRATION-COORDINATOR-PLAN.md`；不得绕过支付/权益、不得写入真实凭证、不得删除旧手动供应商。

### 07-Integration Release Material Gate

`07-integration-release` 已完成第一轮材料门禁，但尚未完成真实发布验收：

- E2E lane：原 Planck 会话 shutdown 且未落文件；替代 Locke 会话完成 `manual-e2e-checklist.md`、`evidence-template/README.md` 和 E2E final report，状态为 `E2E evidence pending`。
- Compatibility lane：原 Beauvoir 会话 shutdown 且未落文件；替代 Poincare 会话完成 compatibility checklist、provider settings evidence template、rollback notes 和 final report，状态为 `compatibility evidence pending`。
- Package lane：Bohr 完成 package install checklist、platform evidence template、local command evidence、pre-release blockers 和 final report，状态为 `package evidence pending`。
- Docs lane：Chandrasekhar 完成产品方案书、产品说明书、07 docs 和 final report；coordinator 后续完成 HTML 展示页 backend-configured 文案和四层架构术语同步。
- Read-only HTML drift audit 发现 demo quota bar 仍有固定 `65%` 宽度且把后台快照文案当 CSS 宽度；coordinator 已改为 `quotaProgress` 与显示文案分离。
- Coordinator 用本地 Chromium 生成并目检 HTML 桌面 `1440x900` 与移动 `390x844` 截图；移动端标题/说明裁切已修复。本机 in-app browser 仍因 runtime setup 失败保持 pending。
- Coordinator 新增 `07-integration-release/release-local-verification.md`，记录本地 release-readiness 命令，但保持 release recommendation 为 no-go。
- Backend full local verification 通过：`GOTOOLCHAIN=local go test ./...`。
- Sub2API frontend verification 通过：`npm run typecheck` 和 `npm run build`。
- Desktop manager local frontend build 通过：`npm run vite:build`；package lane 先前已通过 `npm run check`。
- Coordinator 后续复用 workspace-local Rust toolchain，完成 `cargo fmt --check -p codex-plus-core`、`cargo test -p codex-plus-core codexplus_cloud`、`relay_config` 和 `protocol_proxy`。
- Broader `cargo test --workspace` 已尝试但未通过：默认 MSVC 缺 `link.exe`；GNU 加本地 `w64devkit` 后推进到链接/写入阶段，但因本机磁盘空间耗尽失败。Coordinator 已清理生成的 `target` 构建产物。
- `tools/verify-07-static.ps1` passed。
- `tools/validate-stage-gate.ps1 -Stage 07-integration-release` passed。
- Current `tools/validate-stage-gate.ps1` passed and resolves to `07-integration-release`。
- 当前 release recommendation: no-go，直到真实 E2E、兼容迁移和平台安装包证据完成。

## Execution Rule

- 按 `PARALLEL-DISPATCH-PLAN.md` 的 DAG 执行，而不是把所有阶段同时开跑。
- `00-contract` 和存储决策作为硬门禁；当前实现只在已冻结或已有 mock 的范围内推进。
- 同一时间并行的会话必须拥有不同写入范围。
- 中央路由、wire、gateway hook 和全局入口由主集成会话统一收口。
- 并行检查会话默认只读，除非明确分配了互不重叠的代码所有权。

## Historical Prior Round

下表是纠偏前的历史并行/复核记录，不代表当前阶段已允许继续向后执行。当前有效轮次见本文顶部 `2026-06-16 Correction Round` 和 [STAGE-GATE-LEDGER.md](STAGE-GATE-LEDGER.md)。

| Lane | Session | Responsibility | Write scope | Result |
| --- | --- | --- | --- | --- |
| Coordinator | main | 集成补线、wire/router/hook 收口、状态文档 | integration files and docs | active |
| Backend verification | Bohr | 复核 D/E/H 后端实现是否按计划落地 | none, read-only | completed |
| Desktop verification | Kuhn | 复核 F/G 桌面端 runtime/UX 与设备 header 闭环 | none, read-only | completed |
| Desktop auth verification | Godel | 复核桌面 `/auth/login` 与 `/auth/login/2fa` runtime/Tauri/UI 链路 | none, read-only | completed |
| Plan consistency verification | Locke | 复核多会话计划文档、执行记录和 2FA 状态是否一致 | none, read-only | completed |
| Backend handoff verification | Volta | 复核 pending auth / desktop handoff 应如何避免 Turnstile 绕过 | none, read-only | completed |
| Desktop handoff verification | Hooke | 复核 desktop handoff 最小 runtime/Tauri/UI 落点 | none, read-only | completed |

## Parallel Verification Findings

### Backend Verification / Bohr

- `/api/v1/client/*` 已进入中央 router/wire。
- `CodexPlusGatewayPolicyService` 已进入 Claude/OpenAI/Gemini 主网关热路径。
- admin entitlement 已聚合真实用户、订阅、API key、usage、device 和 event 数据。
- 发现并修复：`codexplus_events` 是 append-only 表，没有 `deleted_at` 字段，repo 查询不能带 `deleted_at IS NULL`。
- 发现并修复：设备心跳 upsert 不能把 `revoked` / `blocked` 设备复活为 `active`。
- 发现并修复：真实用户 plan/model group 到 gateway entitlement 不能靠默认套餐兜底；已加入 `PlanCatalog.entitlement_sources`，网关按 subscription group、API key group 或 group name 映射套餐，无映射时 fail closed。
- 发现并修复：严格设备 enforcement 需要后台开关；已加入 `FeatureFlags.strict_device_enforcement`，开启后网关要求设备上下文。
- 追加同步：`codex-plus-contracts` schema 和 `151_codexplus_foundation.sql` 默认配置已补齐同名字段，防止代码、文档、seed 三处漂移。
- 追加自查：订阅 group object fallback 已修正为读取 `subscription_group_ids`，并补充 subscription group 映射单元测试。
- 仍缺：HTTP/WS 协议级测试和真实 Go 编译验证。

### Desktop Verification / Kuhn

- desktop client API 调用已对 login/bootstrap/usage/devices/redeem 链路使用本地 device，并在 client API 请求中发送 `X-CodexPlus-Device-Id`。
- provider writer 使用 bootstrap 快照中的 gateway base URL、API key 和 default model，没有硬编码套餐、价格、额度或倍率。
- 发现并修复：Codex CLI 主网关请求原先不能证明会携带设备 header；现在 `Codex++ Cloud` provider 写给 Codex 的 `base_url` 指向本地 helper，helper 再转发到后台 gateway 并补 `X-CodexPlus-Device-Id`。
- 追加修复：`relay_config` 归一化时会保留 `Codex++ Cloud` 的真实后台 gateway base URL，同时保持写给 Codex 的 `config.toml` 使用本地 helper base URL，避免后续 provider switch 或 storage normalize 把两者混淆。
- 追加修复：本地 helper 转发不再只依赖被序列化隐藏的 `api_key` 字段；重启后可从 `auth_contents.OPENAI_API_KEY` 恢复托管 key。
- 仍缺：端到端启动 Codex 后抓包/网关日志证明 header 到达 sub2api，Rust 编译验证和 desktop runtime fixture 测试。
- 追加复核：Godel 确认桌面 `/api/v1/auth/login` 与 `/api/v1/auth/login/2fa` 已接通，pending 2FA 不会被误判为 authenticated；同时发现生产 Turnstile 会拦截当前桌面密码登录，因为桌面端未提交 `turnstile_token`。
- 追加复核：Volta/Hooke 确认生产默认应走 browser handoff，不应新增无 Turnstile 的桌面密码登录；建议保存独立 pending handoff、只暴露 authorize URL/确认码、把 `poll_token` 留在本地。

## Implemented By Plan Stage

### Phase 1: Backend Foundations

- `codexplus_config_v1` 配置服务、默认配置、校验和 additive migration 已加入。
- `codexplus_devices`、`codexplus_managed_provider_keys`、`codexplus_events` 的基础 schema/repository/service seam 已加入。
- 管理端和网关所需的 `GetByAPIKeyID`、`ListByUser`、`GetByUserAndDevice` seam 已形成。

### Phase 2: Backend API And Gateway

- `/api/v1/client/bootstrap`、`usage`、`devices`、`redeem` 已接入中央 router/wire。
- client API 只从鉴权上下文取用户身份，不接受客户端传入的 `user_id`。
- `CodexPlusGatewayPolicyService` 已注入 Claude-compatible、OpenAI-compatible、Gemini-compatible 主路径。
- 共享 handler helper 已覆盖模型权限、计费检查入口和设备 header 传递。
- 网关 policy 现在通过后台配置的 `entitlement_sources` 映射旧系统分组到 Codex++ 套餐；无映射不再默认放行。
- `strict_device_enforcement` 已成为后台配置项，可将设备上下文从“有则校验”升级为“必须存在”。
- device repository upsert 已保留服务端 `revoked` / `blocked` 状态，客户端心跳不再复活被撤销设备。
- event repository 已按 append-only schema 查询，不再引用不存在的 `deleted_at` 字段。

### Phase 3: Desktop Runtime And Admin Operations

- 桌面 runtime 已消费 bootstrap/usage/device/redeem，并把 `Codex++ Cloud` 写入托管 provider。
- provider writer 使用后台快照中的 base URL、Key 和默认模型，不在客户端硬编码套餐、价格、额度或倍率。
- `Codex++ Cloud` 现在通过本地 helper 转发 Codex 主请求到后台 gateway，并由 helper 注入 `X-CodexPlus-Device-Id`。
- `relay_config` 已保护托管 provider 的 upstream/base URL 分离：运行时转发使用真实后台 gateway，Codex 配置文件使用本地 helper。
- 本地 helper 转发的认证 key 已支持从 `auth_contents` 恢复，覆盖应用重启后的托管 profile。
- 桌面登录已从不存在的 `/api/v1/client/login` 切换到现有 `/api/v1/auth/login`；当后端返回 `requires_2fa=true` 时，本地只保存 pending 2FA session，并通过 `/api/v1/auth/login/2fa` 换取正式 JWT。
- 新增 browser handoff 初始闭环：
  - 后端 `/api/v1/auth/desktop/start` 创建短期 pending session，返回 `session_token`、桌面私有 `poll_token`、浏览器授权 URL、6 位确认码和过期时间。
  - Web 前端 `/auth/desktop/authorize` 使用当前已登录浏览器 JWT 调用 `/api/v1/auth/desktop/complete`，不向浏览器返回桌面 token。
  - 桌面端 `codexplus_cloud_start_browser_handoff` / `poll_browser_handoff` / `cancel_browser_handoff` 管理 pending 状态，完成后保存正式 session 并刷新 bootstrap。
  - Manager 登录面板把 browser handoff 作为默认入口，邮箱密码登录折叠为兼容路径。
- browser handoff 已回补 Phase 0 契约：
  - OpenAPI 冻结 `/api/v1/auth/desktop/start`、`/complete`、`/poll`。
  - mock fixtures 覆盖 start、complete、poll pending、poll completed。
  - 错误表和事件 schema 增加 `CLIENT_AUTH_DESKTOP_*` 与 `desktop_login_*`。
  - Rust redaction 规则补充 `poll_token`/`session_token` JSON 字段和 URL `poll_token`。
- 后端 handoff 已复用 `codexplus_events` 记录 `desktop_login_started`、`desktop_login_completed` 和 terminal `desktop_login_polled`，payload 不包含 `session_token`、`poll_token`、access token 或 refresh token。
- admin service 已从真实 device/event repo 聚合用户权益视图，不再返回空 stub 列表。

### Phase 4: Desktop UX And E2E Prep

- 桌面 UX 文件已观察到 Cloud home、登录绑定、用量/模型、安装辅助、新手教学和诊断入口。
- 登录绑定 UI 已接入 runtime 的 `connection.pendingTwoFactor` 状态，展示动态验证码输入并调用 Tauri `codexplus_cloud_login_2fa` 命令完成登录。
- 2FA 输入已限制为 6 位数字，减少无效请求打到后端。
- Locke 复核发现的文档漂移已修：网页登录/OAuth handoff 不再写成已完成能力，C/D 复核状态和下一阶段门禁已同步。
- E2E 仍未完成真实环境执行；当前状态只能算本地集成候选。

## Current Gaps

- 07 本地 release-readiness 已覆盖 backend `go test ./...`、Sub2API frontend `npm run typecheck` / `npm run build`、desktop manager `npm run check` / `npm run vite:build`、以及 targeted desktop Rust `cargo fmt` / `codexplus_cloud` / `relay_config` / `protocol_proxy` 检查；`verify-07-static.ps1` 和 `validate-stage-gate.ps1 -Stage 07-integration-release` 在 2026-06-18 复跑通过。
- 2026-06-18 continuation 新增并加固 `tools/verify-07-evidence.ps1`，用于最终 E2E 证据目录的 13 文件结构、关键 `Result: pass` / `Result: fail` 标记、核心 E2E 场景覆盖、release recommendation、rollback/audit/defect notes 和文本泄密扫描；模板目录按预期失败，临时脱敏 fixture 通过后已清理。
- 2026-06-18 continuation 新增 `tools/new-07-evidence-run.ps1`，用于生成时间戳 E2E 证据骨架；生成的 TODO / `Result: pending` scaffold 会被 verifier 按预期拒绝，直到替换为脱敏真实执行证据。
- 2026-06-18 continuation 新增 `tools/new-07-package-evidence.ps1` 和 `tools/verify-07-package-evidence.ps1`，用于 Windows/macOS 安装包证据骨架与校验；生成的 TODO/pending scaffold 会被 verifier 拒绝，脱敏 package fixture 已验证可通过。
- 2026-06-18 continuation 新增 `tools/inspect-07-package-artifacts.ps1`，用于对已生成的 Windows setup 和 macOS x64/arm64 DMG 做只读 artifact inspection：记录文件名、SHA256、三平台覆盖、高置信 secret/policy 扫描和 installer script 凭据写入风险，不打印命中值。脱敏 fixture 通过，缺失 artifact 负例按预期失败；它不替代平台安装证据。
- 2026-06-18 continuation 加强 `tools/verify-07-package-evidence.ps1` 对 artifact inspector 输出的绑定：artifact metadata 必须 `Result: pass` 并记录 Windows/macOS x64/macOS arm64 expected coverage；artifact inspection 必须 `Result: pass`、记录 `inspect-07-package-artifacts.ps1` 命令、scanner findings 为 none、无 shared key、无 user credentials、无 fixed commercial policy，且 installer-script credential scan 为 pass。`tools/test-07-evidence-tooling.ps1` 已覆盖 `package-metadata-result-fail-fails` 和 `package-artifact-coverage-missing-fails`。
- 2026-06-18 continuation 新增 `tools/new-07-compatibility-evidence.ps1` 和 `tools/verify-07-compatibility-evidence.ps1`，用于旧手动供应商、Cloud 升级/退出、provider sync 和 rollback 证据骨架与校验；生成的 TODO/pending scaffold 会被 verifier 拒绝，脱敏 compatibility fixture 已验证可通过。
- 2026-06-18 continuation 新增 `tools/inspect-07-compatibility-snapshots.ps1`，用于对 pre-upgrade、post-upgrade、logout 和 rollback provider 快照做只读对比：检查手动 provider 保留、`Codex++ Cloud` 托管 provider 存在、logout 后 token 字段清空、rollback 后手动 provider 保留，以及本地快照未写入套餐/价格/倍率/权益/用量策略。脱敏 fixture 通过并能通过 compatibility verifier，缺失快照负例按预期失败；它不替代真实桌面运行兼容证据。
- 2026-06-18 continuation 加强 `tools/verify-07-compatibility-evidence.ps1` 对 snapshot inspector 输出的绑定：snapshot context 必须 `Result: pass`，四类快照均 parsed，missing inputs 和 parse failures 均为 none；pre-upgrade 必须有手动 provider；post-upgrade 必须保留手动 provider、包含 managed `Codex++ Cloud` 且未写入本地商业策略；logout 必须保留手动 provider 且 token-field scan clear；rollback 后不得缺失手动 provider；gate report 必须记录 `inspect-07-compatibility-snapshots.ps1`。`tools/test-07-evidence-tooling.ps1` 已覆盖 `compatibility-context-result-fail-fails` 和 `compatibility-missing-manual-provider-fails`。
- 2026-06-18 continuation 新增 `tools/new-07-business-readiness-evidence.ps1` 和 `tools/verify-07-business-readiness.ps1`，用于 Phase 9 business readiness 证据骨架与校验；覆盖生产环境值、业务配置决策、安全/合规/隐私/法务、可观测性、成本滥用、付费用户支持和人工决策 owner。生成 scaffold 会被 TODO/pending 拒绝，owner-approved 脱敏 fixture 可通过。
- 2026-06-18 continuation 新增 `tools/verify-07-release-evidence.ps1`，用于 Module J 将 E2E/package/compatibility 三类证据一起跑聚合卫生门禁；缺少 package 证据的负例按预期失败，三份脱敏 fixture 的正例通过后已清理。
- 2026-06-18 continuation 新增 `07-integration-release/reports/module-j-final-report-template.md` 和 `tools/verify-07-module-j-report.ps1`，用于 Module J final report 的元数据、必需章节、Module A-I report input 和 merge order 字段、verification command/skipped/unavailable 处置字段、release evidence hygiene 字段、冲突 file/module/rule/result 字段、contract drift/change-review 字段、go/no-go 裁决值、聚合证据边界、coverage summary 一致性、named go-policy 信号、风险/回滚字段、accepted-risk impact 和脱敏卫生校验；模板负例按预期失败，脱敏 final-report fixture 正例通过后已清理。
- 2026-06-18 continuation 新增 `tools/new-07-release-evidence-set.ps1`，用于一次创建同一时间戳的 E2E、package、compatibility、business readiness、coverage/readiness summary 和 Module J report draft scaffold；生成物按预期会被聚合证据、business readiness 和 Module J report verifier 拒绝，直到真实脱敏证据填入。
- 2026-06-18 continuation 新增 `tools/summarize-07-release-coverage.ps1`，用于把 E2E/package/compatibility 证据映射成 release 场景覆盖矩阵；生成 scaffold 会保持 incomplete，内部一致的脱敏 handoff candidate 可生成 complete coverage。
- 2026-06-18 continuation 新增并加强 `tools/summarize-07-release-readiness.ps1`，用于在 Module J final report 前生成保守 readiness summary：聚合 verifier、coverage summary verification、coverage summary 输入路径一致性或 business readiness verifier 失败都会保持 no-go，聚合通过但 coverage incomplete、missing coverage count 非 0、nonrelease marker count 非 0，或含 fixture/scaffold/subset/pending/缺外部证据标记也会保持 no-go；`-AllowGoCandidate` 只能在真实生产等价技术证据、complete coverage summary 和 business readiness 证据齐备后用于 Module J 候选评审。下一步 hardening 会把 generated readiness、`Allow go candidate` 与 `Nonrelease markers` 作为一致性字段，避免手写或陈旧 summary 被误当成候选输入。
- 2026-06-18 continuation 加强 `tools/verify-07-module-j-report.ps1`：`go` / `go with accepted risks` final report 必须提供 `-CoverageSummaryFile` 和 `-ReadinessSummaryFile`，并拒绝 coverage incomplete、missing coverage count 非 0、nonrelease marker count 非 0、readiness summary coverage verification 缺失/失败、readiness 未显式记录 `Allow go candidate: true`、报告记录的 summary/evidence 路径与生成摘要不一致、summary 为 no-go、summary 缺少 passed business readiness、报告缺少 business evidence hygiene 字段或报告内 business readiness verification 不是 passed、缺少 Module A-I input 或 merge order、存在 unapproved/pending/unreviewed contract drift、缺少 named go-policy 信号、缺少 skipped/unavailable 处置字段、缺少 conflict rule/result 字段或缺少 accepted impact 的报告，防止人工报告绕开 coverage/readiness summary 边界。下一步 hardening 会要求 Module J 使用 generated readiness，并且 `go` / `go with accepted risks` 不允许残留 readiness markers；`E2E Level 3 pass` 会成为 named go-policy 的机器化输入。
- 2026-06-18 continuation 新增并加强 `tools/verify-07-release-handoff.ps1`：对 timestamped `*-release` 交付目录做最终一致性校验，按 run stamp 推导 E2E/package/compatibility/business 目录，重跑聚合 verifier 和 business readiness verifier，重生成 coverage/readiness summary 做一致性比对，比较存档与重生成的 coverage/readiness 输入路径和计数，要求 handoff index 记录最终 verification results 并与 summary/report 推荐一致，并调用 Module J final report verifier。下一步 hardening 会绑定 handoff index 的 run stamp 与 release 目录名，按字段校验 E2E/package/compatibility/business/summary/report evidence paths，并比对 readiness `Generated`、`Allow go candidate`、`Nonrelease markers` 字段。生成 scaffold 负例失败，缺少最终结果字段的 index 负例失败，存档 coverage/readiness 输入不一致负例失败，内部一致的 marker-free synthetic 候选 fixture 正例仅用于 verifier 覆盖。
- 2026-06-18 continuation 重试 in-app Browser HTML visual evidence：browser runtime 连接成功，但本地 `file://` HTML 目标被 URL policy 阻止；未绕过策略，边界记录在 `07-integration-release/docs/html-visual-evidence/in-app-browser-policy-boundary.md`。
- 2026-06-18 continuation 新增并通过 `tools/test-07-evidence-tooling.ps1`，把 07 证据工具链的 scaffold 负例和脱敏 fixture 正例收成可复跑自检脚本。
- 2026-06-18 continuation 新增并运行 `tools/verify-07-rust-preflight.ps1`；当前主机 Rust workspace 预检失败，因为 Rust toolchain/linker 命令缺失、此前 workspace-local toolchain 已不存在，且 `C:\` 可用空间为 9.56GB、低于 20GB 阈值。这是环境准备阻塞，不是 Rust 测试失败。
- 2026-06-18 continuation 新增 `tools/verify-07-e2e-readiness.ps1`；它在 E2E 执行前检查 `CODEXPLUS_07_E2E_*` 后端/后台 URL、Manager build path、测试账号 token、测试 device id 和 allowed/denied model，不打印 token 值。当前主机因缺真实测试环境变量按预期失败，脱敏临时 fixture 通过，并已纳入 `tools/test-07-evidence-tooling.ps1`。
- 2026-06-18 continuation 新增 `sub2api-main/tools/e2e/codexplus/run-client-api-checks.ps1` 和 `run-local-e2e.ps1`；fixture 运行已通过，前者能写入脱敏 `02/04/09/11` client API subset 证据，后者能生成标准 13 文件 E2E scaffold 后调用前者。这仍只是执行辅助，不等于真实 browser handoff、desktop launch、gateway、package 或 compatibility 证据。
- 2026-06-18 continuation 新增 `sub2api-main/tools/e2e/codexplus/run-gateway-policy-checks.ps1`；fixture 运行覆盖 active success 与 not-purchased/expired/low-balance/revoked-device/model-denied rejection，默认不带 `-AllowGatewayRequests` 会按预期失败，避免误发真实低成本模型请求。
- 2026-06-18 continuation 加强 E2E、package、compatibility 和 business readiness 单 lane verifier：结构完整但最终 result 为 fail 的证据会在 lane 层被拒绝，`tools/test-07-evidence-tooling.ps1` 已覆盖四类 failed-result 负例。
- 2026-06-18 continuation 修正 package artifact inspection 的 readiness 边界措辞：runner 仍只证明 package hygiene、不替代平台安装证据，但不再使用会触发 readiness `runtime-evidence-required` marker 的 `platform install ... required` 文案；marker-free synthetic handoff 正例已重新通过。
- 桌面密码登录端点已切到现有 `/api/v1/auth/login` 并解析后端 `AuthResponse`；TOTP-style 2FA 已通过 `/api/v1/auth/login/2fa` 接通。Browser handoff 已作为生产默认路径落地，但仍需真实 Turnstile-enabled 环境 E2E 证明。
- 后端 legacy `code/message/reason/data` envelope 已在 OpenAPI 中标注为兼容形态；桌面端继续兼容 `status/error_code` fixtures 与 legacy code envelopes。
- Codex CLI 主网关请求已改为通过本地 helper 注入 `X-CodexPlus-Device-Id`，但仍需真实启动 Codex 后用网关日志或抓包证明 header 到达 sub2api。
- Windows/macOS installer artifact build/install、旧手动供应商迁移回滚、以及生产等价 E2E 仍缺外部环境证据，因此 release recommendation 必须保持 no-go。

## 2026-06-18 Multi-Session Continuation

| Session | Scope | Write scope | Status |
| --- | --- | --- | --- |
| Main coordinator | Implemented E2E env template, readiness `-EnvFile` / `-EndpointPreflight`, runner plumbing, docs/status updates and verification reruns | release tooling and status docs | active |
| Anscombe | Read-only investigation of readiness preflight HTTP status reporting | none, read-only | parallel audit |
| Lagrange | Read-only investigation of whether the current local service can expose the 07 Codex++ routes or needs a different build/image | none, read-only | parallel audit |

Current local service finding: base `http://localhost:8080` health and gateway reachability are available, but 07 Codex++ client/admin/desktop routes return HTTP 404 under endpoint preflight. A source-built local image was then built and started as `sub2api-codexplus-local` on `http://127.0.0.1:8081`; that container returns auth/validation responses instead of 404 for the 07 routes. This keeps the original upstream `8080` service running while making the current workspace source visible locally on `8081`. The repeatable local-source entry is now `sub2api-main/deploy/docker-compose.dev.yml` with `.env.codexplus-local.example`, git-ignored `.env.codexplus-local` and isolated `.codexplus-local/` data; `sub2api-main/tools/e2e/codexplus/start-local-source-service.ps1` provides the build/probe route preflight helper.

Continuation follow-up: parallel audit identified local-source reproducibility risks and the next non-external 07 gap. The env template now uses bootable local-only 64-hex JWT/TOTP key shapes, dev docs cover port conflict/lifecycle commands, and the probe helper confirms the target container plus explicit route status allowlists. A real docs/product-copy evidence folder now exists at `test-runs/20260618-1403-docs` and passes `verify-07-docs-product-copy-evidence.ps1`; release remains no-go until E2E, package, compatibility runtime and business readiness evidence are complete.

E2E readiness follow-up: parallel audit confirmed that local route preflight can be a prep diagnostic but not release evidence. `verify-07-e2e-readiness.ps1` now has `-EndpointPreflightOnly` and `-OutputPath`; `test-runs/20260618-1425-e2e-env/8081-local-preflight.md` records a passing `8081` local route diagnostic, while `full-readiness-placeholder-failure.md` records the expected full-readiness failure until real test tokens, model names, gateway keys and personas are supplied.

## 2026-06-18 Module J Final Aggregation

| Session | Scope | Write scope | Status |
| --- | --- | --- | --- |
| Worker 3A Module J | Final aggregation, coverage/readiness summaries, Module J report, handoff verification, final status docs | release folder and allowed status docs only | completed / no-go |

- Release run stamp: `20260618-2124-release`.
- Evidence inputs: `20260618-2103-e2e`, `20260618-2103-package`, `20260618-2108-compatibility`, `20260618-1403-docs`, `20260618-2110-business`.
- Final report: `codex-plus-dev-plan/test-runs/20260618-2124-release/module-j-final-report.md`.
- Final recommendation: no-go.
- Verifier results: aggregate exit 1, coverage summary exit 1, readiness summary exit 1, Module J report verifier exit 0, release handoff verifier exit 1.
- The no-go is caused by missing real E2E, package, compatibility, and business readiness evidence/approvals. Docs pass is recorded only as docs/product-copy hygiene.
- No source code was edited, no verifier was weakened, no blocked lane evidence folder was edited, and no new worker was started.

## Next Sequential Gate

在完成 `07-integration-release` 前，必须完成：

- 购买、browser handoff 登录、bootstrap、`Codex++ Cloud` provider 写入、启动 Codex 和一次 gateway 请求的 E2E 证据。
- 旧手动供应商、云登录退出、provider sync 和迁移回滚证据。
- Windows/macOS 安装包、入口、安装辅助和覆盖安装/重装证据。
- 文档、HTML、管理员指南、发布说明、已知风险和回滚路径同步。
- 更广范围 Go/Wire/backend、Rust/Cargo desktop、Node/frontend 和 release package checks 在可用工具链或 CI 中通过。
