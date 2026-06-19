# Codex++ Stage Gate Ledger

本文档是当前实现轮次的阶段门禁台账。它用于纠正“并行但不分阶段”或“单线推进但没有多会话”的执行偏差。

## Execution Contract

- 当前只能执行一个数字阶段；同一数字阶段内允许多会话并行。
- 下一数字阶段必须等待当前阶段的 coordinator 验收完成。
- worker 只拥有提示词中声明的写入范围；需要跨范围时必须停止并报告。
- coordinator 只做门禁、合并、轻量适配和状态记录，不抢占 worker 的任务文件。
- 未通过门禁的字段、状态、错误码、配置项和 mock 不得被后续源码硬编码。

## 2026-06-19 Module J Final Gate

| Stage | Status | Evidence | Rule |
| --- | --- | --- | --- |
| `07-integration-release` | passed / go with accepted risks | `codex-plus-dev-plan/test-runs/20260619-1940-release` | Owner-approved Windows-only local MVP evidence passed E2E, package, compatibility, docs, business readiness, coverage, readiness, Module J report and release handoff gates. Production release remains separately gated by production release verification and no-secret evidence. |

Gate details:

- `verify-07-static.ps1`: passed.
- `validate-stage-gate.ps1 -Stage 07-integration-release`: passed.
- `verify-07-evidence.ps1 -EvidenceDir codex-plus-dev-plan/test-runs/20260619-1940-e2e`: passed.
- `verify-07-package-evidence.ps1 -EvidenceDir codex-plus-dev-plan/test-runs/20260619-1940-package -WindowsOnlyMvp`: passed.
- `verify-07-compatibility-evidence.ps1 -EvidenceDir codex-plus-dev-plan/test-runs/20260619-1940-compatibility`: passed.
- `verify-07-docs-product-copy-evidence.ps1 -EvidenceDir codex-plus-dev-plan/test-runs/20260619-1940-docs`: passed.
- `verify-07-business-readiness.ps1 -EvidenceDir codex-plus-dev-plan/test-runs/20260619-1940-business`: passed.
- `verify-07-release-evidence.ps1 -WindowsOnlyMvp`: passed.
- `summarize-07-release-coverage.ps1 -WindowsOnlyMvp -FailOnIncomplete`: passed with coverage status complete, missing coverage 0 and nonrelease markers 0.
- `summarize-07-release-readiness.ps1 -WindowsOnlyMvp -AllowGoCandidate -FailOnNoGo`: passed with readiness posture `go-candidate-requires-module-j-review` and nonrelease markers 0.
- `verify-07-module-j-report.ps1`: passed.
- `verify-07-release-handoff.ps1 -WindowsOnlyMvp -AllowGoCandidate`: passed.
- Real user profile scan after isolated Desktop Manager/Codex E2E found no `Codex++ Cloud` managed provider markers in the real user profile.

## 2026-06-18 Module J Final Gate

| Stage | Status | Evidence | Rule |
| --- | --- | --- | --- |
| `07-integration-release` | blocked / no-go | `codex-plus-dev-plan/test-runs/20260618-2124-release` | Module J final report passed, but aggregate release evidence failed, coverage is incomplete, readiness posture is no-go, business readiness failed, and release handoff failed. |

Gate details:

- `verify-07-release-evidence.ps1`: failed, exit 1.
- `summarize-07-release-coverage.ps1 -FailOnIncomplete`: failed, exit 1; generated incomplete summary with 14 missing requirements and 6 nonrelease markers.
- `summarize-07-release-readiness.ps1 -FailOnNoGo`: failed, exit 1; generated no-go readiness summary.
- `verify-07-module-j-report.ps1`: passed, exit 0.
- `verify-07-release-handoff.ps1`: failed, exit 1, because the current package is no-go and the final evidence dirs are not same-stamp siblings for `20260618-2124`.
- Required before MVP go: real E2E pass, package artifact/install pass, compatibility runtime pass, business readiness pass, and a same-stamp final release evidence set.

## Current Stage

| Stage | Name | Status | Rule |
| --- | --- | --- | --- |
| `00-contract` | 契约冻结 | passed | 契约、schema、mock、错误状态和 final reports 已通过门禁 |
| `01-backend-config-center` | 后台配置中心 | passed | P/M/U/F 并行实现、coordinator 集成、静态门禁、gofmt 和 targeted Go tests 均已通过 |
| `02-backend-client-api` | 客户端 API | passed | device/bootstrap/usage/redeem 已接入，按 `00/01` 输出聚合快照，静态门禁、gofmt 和 targeted Go tests 均已通过 |
| `03-client-cloud-core` | 桌面云核心 | passed | runtime 对 02 快照字段已做静态/TS/Rust 对齐；bootstrap consumer、local session store、managed provider writer 和 redaction 门禁均已通过 |
| `04-client-user-experience` | 用户体验 | passed | Cloud home、登录绑定、安装辅助和新手教程已接入 03 runtime 状态模型，静态、TypeScript、构建和视觉门禁均已通过 |
| `05-admin-operations` | 管理后台 | passed | 管理员套餐、模型、用量策略、功能开关和用户权益视图已通过静态、Node、Go 与总门禁 |
| `06-commerce-and-enforcement` | 购买与强制执行 | passed | 支付权益、网关强制执行、设备管理和审计风控已通过静态、Go 与总门禁 |
| `07-integration-release` | 集成发布 | active | 当前阶段保留为 07 门禁复核入口；20260619 Module J final gate 已记录 Windows-only local MVP `go with accepted risks`，生产发布仍为独立门禁 |

## Completed Parallel Sessions

| Lane | Agent | Stage task | Write scope | Merge status |
| --- | --- | --- | --- | --- |
| A | Harvey | 客户端 API 契约冻结 | `codex-plus-contracts/api/client-openapi.yaml`, `codex-plus-contracts/test-fixtures/client/*.json`, `00-contract/task-client-api-contract.md`, `00-contract/reports/worker-a-client-api-final.md` | completed, gate passed |
| B | Pauli | 后台配置契约冻结 | `codex-plus-contracts/config/*.schema.json`, `00-contract/task-admin-config-contract.md`, `00-contract/reports/worker-b-admin-config-final.md` | completed, gate passed |
| C | Bernoulli | 状态、错误码与事件契约冻结 | `codex-plus-contracts/status-error/client-status-errors.md`, `codex-plus-contracts/events/client-events.schema.json`, `00-contract/task-status-and-error-model.md`, `00-contract/reports/worker-c-status-error-event-final.md` | completed, gate passed |
| Coordinator | main | 阶段门禁、执行记录、合并复核 | `STAGE-GATE-LEDGER.md`, `MULTI-SESSION-EXECUTION-TRACE.md`, `IMPLEMENTATION-STATUS.md` | active |

Previous A/B/C workers failed before final reports because of agent quota limits. The restarted A/B/C workers completed, and the stage gate script passed.

## Completed 01 Parallel Sessions

| Lane | Agent | Stage task | Write scope | Merge status |
| --- | --- | --- | --- | --- |
| P | Boyle | Plan Catalog config registry | `configregistry/plan_catalog.go`, `plan_catalog_test.go`, `task-plan-catalog.md`, `reports/worker-plan-catalog-final.md` | completed, coordinator integrated, static gate passed |
| M | Ohm | Model Catalog config registry | `configregistry/model_catalog.go`, `model_catalog_test.go`, `task-model-catalog.md`, `reports/worker-model-catalog-final.md` | completed, coordinator integrated, static gate passed |
| U | Kierkegaard | Usage Policy config registry | `configregistry/usage_policy.go`, `usage_policy_test.go`, `task-usage-policy.md`, `reports/worker-usage-policy-final.md` | completed, coordinator integrated, static gate passed |
| F | Rawls | Feature Flags config registry | `configregistry/feature_flags.go`, `feature_flags_test.go`, `task-feature-flags.md`, `reports/worker-feature-flags-final.md` | completed, coordinator integrated, static gate passed |

## 01-Backend Config Center Exit Gate

`01-backend-config-center` 已完成并通过退出门禁：

- P/M/U/F 四个 worker final report 均已返回并关闭会话。
- Coordinator 已把四个 registry 接入 `sub2api-main/backend/internal/service/codexplus_config_service.go`。
- `DefaultCodexPlusConfig` 由后台配置中心默认 catalog 合成，不在客户端或旧服务中散落运营规则。
- `ValidateCodexPlusConfig` 追加调用 Plan、Model、Usage Policy、Feature Flags 四个 registry validator。
- `codexplus_config_service_test.go` 覆盖 registry-backed defaults、跨 catalog 默认引用、Plan Catalog registry validation 和 Feature Flag exposure validation。
- `codex-plus-dev-plan/tools/verify-01-static.ps1` 已通过离线配置中心审计。
- `codex-plus-dev-plan/tools/verify-01-go.ps1` 已通过：`gofmt -l` 无输出，`go test ./internal/codexplus/configregistry ./internal/service -run CodexPlus` 在 Go 1.26.4 下通过。
- 只读复核 S/T 后已修：`display_price` 校验兜底、shared draft status 生命周期、执行轨迹表格残留。
- client/gateway/admin 深度消费风险已登记到 coordinator report，作为 `02/05/06` 阶段输入继续处理。

因此当前状态是：`01` passed，`02-backend-client-api` active；`03-07` 继续 blocked，等待 `02` 完成并通过门禁后逐阶段打开。

## 02-Backend Client API Exit Gate

`02-backend-client-api` 已完成并通过退出门禁：

- `/api/v1/client/bootstrap`、`/usage`、`/devices`、`/redeem` 已通过 authenticated client routes 接入。
- bootstrap/usage DTO 已补齐契约字段：`message_key`、`commerce_action`、`action_copy_key`、`balance_summary`、`period_usage`、`announcements`、`force_update_prompt` 和 `strict_device_enforcement`。
- Client API success envelope 已对齐 legacy-compatible 契约形态：`code`、`status`、`message`、`reason`、`error_code`、`data`。
- Client entitlement 不再依赖 `firstPlan` 或 `firstUsagePolicy`；套餐、usage policy 和可见模型由 `PlanCatalog.entitlement_sources`、`usage_policy_id` 和 plan model groups 解析。
- `bootstrap_requested`、`usage_requested`、`device_registered`、`redeem_attempted` 结构化事件已带上请求上下文，并在有配置快照的接口保留 config version。
- `codex-plus-dev-plan/tools/verify-02-static.ps1` 已通过。
- `codex-plus-dev-plan/tools/verify-02-go.ps1` 已通过：`gofmt -l` 无输出，`go test ./internal/service ./internal/handler/client ./internal/handler/dto ./internal/server/routes -run "CodexPlus|Client"` 和 `go test ./internal/service -run "CodexPlusClient|CodexPlusConfig|CodexPlusGateway"` 在 Go 1.26.4 下通过。
- `codex-plus-dev-plan/tools/validate-stage-gate.ps1 -Stage 02-backend-client-api` 已通过。

因此当前状态是：`02` passed，`03-client-cloud-core` active；`04-07` 继续 blocked，等待 `03` 完成并通过门禁后逐阶段打开。

## 03-Client Cloud Core Exit Gate

`03-client-cloud-core` 已完成并通过退出门禁：

- Desktop bootstrap/usage 类型已消费 `02` client API 输出字段：`message_key`、`commerce_action`、`action_type`、`action_copy_key`、`balance_display`、`usage_display`、`announcements`、`force_update_prompt` 和 `strict_device_enforcement`。
- 本地 runtime state 已把服务端 action metadata 投影到 entitlement/usage 状态，不在客户端生成套餐、价格、额度、倍率、模型权益或购买策略。
- 托管 provider 继续保持 `Codex++ Cloud` 的本地 helper 和真实后台 gateway 分离：写给 Codex 的 `base_url` 指向本地 helper，`upstream_base_url` 保留后台 gateway。
- provider key 只用于本地 helper 和 provider auth，不被 runtime state 序列化；`has_api_key` 仅暴露布尔状态，并覆盖 settings normalization 后的托管 profile。
- 日志脱敏覆盖 `sk-*`、Authorization/Bearer、JWT-like token、`poll_token`、`session_token` 和 URL token query。
- `codex-plus-dev-plan/tools/verify-03-static.ps1` 已通过。
- `codex-plus-dev-plan/tools/verify-03-node.ps1` 已通过：`npm run check` 在 `CodexPlusPlus-main/apps/codex-plus-manager` 下通过。
- `codex-plus-dev-plan/tools/verify-03-rust.ps1` 已通过：`cargo fmt --check -p codex-plus-core`、`cargo test -p codex-plus-core codexplus_cloud`、以及 `relay_config` / `protocol_proxy` 过滤器测试均通过。
- `codex-plus-dev-plan/tools/validate-stage-gate.ps1 -Stage 03-client-cloud-core` 可复核 03 证据与 04 打开状态。

因此当前状态是：`03` passed，`04-client-user-experience` active；`05-07` 继续 blocked，等待 `04` 完成并通过门禁后逐阶段打开。

## 04-Client User Experience Exit Gate

`04-client-user-experience` 已完成并通过退出门禁：

- 四个同阶段并行 worker 均已完成并由 coordinator 集成：Leibniz 登录绑定 UI、Bacon 首页/状态/用量 UI、Hegel 安装助手 UI、Newton 新手教程 UI。
- Cloud home 只消费 runtime/bootstrap/usage 状态，展示登录、套餐、到期、余额、用量、默认模型、公告、启动、诊断和修复入口，不在客户端硬编码价格、套餐、额度、模型权益或购买策略。
- Browser handoff 已作为主登录入口；密码登录保留为兼容路径；pending 2FA 不被视为 authenticated；2FA 输入限制为 6 位数字。
- 安装助手只做本地检测、路径选择和修复入口，不下载 Codex，也不替代云端 API。
- 新手教程支持任务模板、项目目录提示、结果确认、安全提示，以及远端教程文案/公告优先展示。
- 普通浏览器预览环境会使用 fixture，避免向用户暴露 Tauri IPC 技术错误；自动刷新不会在初始状态弹出遮挡性 toast。
- `codex-plus-dev-plan/tools/verify-04-static.ps1` 已通过。
- `codex-plus-dev-plan/tools/verify-04-node.ps1` 已通过：`npm run check` 在 `CodexPlusPlus-main/apps/codex-plus-manager` 下通过。
- `npm run vite:build` 已通过。
- Edge headless `1440x900` 和 `390x844` 视觉复验已通过，无技术错误、toast 遮挡或窄屏按钮裁切。
- `codex-plus-dev-plan/tools/validate-stage-gate.ps1 -Stage 04-client-user-experience` 可复核 04 证据。

因此当前状态是：`04` passed，`05-admin-operations` passed；`06-commerce-and-enforcement` active，`07` 继续 blocked，等待 `06` 完成并通过门禁后打开。

## 05-Admin Operations Exit Gate

`05-admin-operations` 已完成并通过退出门禁：

- 同阶段四个并行 worker 均已完成并由 coordinator 集成：Plan 管理、Model 管理、Usage Policy 管理和 User Entitlement 支持视图。
- 管理端 shared API 类型已覆盖后台控制面字段：价格、购买/续费 copy key、entitlement source、usage policy 关联、模型 rollout/deprecation、用量 quota/device policy 和 `strict_device_enforcement` server-only 标记。
- Plan catalog 面板支持套餐上下架、价格/币种/周期、购买/续费 URL、模型组、权益来源、usage policy 和 copy keys。
- Model catalog 面板支持 route model、模型组、上下架状态、rollout/quality tier、fallback、disabled replacement、context window、倍率和 operator tags。
- Usage policy 和 Feature flags 面板支持额度、并发/RPM/TPM、过期/宽限、设备策略、拒绝文案 copy keys，并明确 `strict_device_enforcement` 是 server/gateway-only 开关。
- User entitlement 面板可按用户查看计划/到期/余额、订阅/API key 分组、设备、托管 key 摘要、用量聚合、近期事件和集成状态。
- `codex-plus-dev-plan/tools/verify-05-static.ps1` 已通过。
- `codex-plus-dev-plan/tools/verify-05-node.ps1` 已通过：`npm run typecheck` 与 `npm run build` 在 `sub2api-main/frontend` 下通过。
- `codex-plus-dev-plan/tools/verify-05-go.ps1` 已通过：`gofmt -l` 清洁，`go test ./internal/service ./internal/handler/admin ./internal/server/routes -run "CodexPlus|Admin"` 通过。
- `codex-plus-dev-plan/tools/validate-stage-gate.ps1 -Stage 05-admin-operations` 已通过。

因此当前状态是：`05` passed，`06-commerce-and-enforcement` passed；`07-integration-release` active，等待集成发布证据完成后再做最终 go/no-go。

## 06-Commerce And Enforcement Exit Gate

`06-commerce-and-enforcement` 已完成并通过退出门禁：

- 同阶段四条执行线已完成或由 coordinator 接受落地结果：Payment entitlement、Gateway enforcement、Device management、Audit and risk control。
- 支付成功后的 Codex++ 权益开通已接入 `CodexPlusCommerceEntitlementService`，支持订阅订单解析、幂等 grant 记录和 expired-grace 恢复。
- 网关策略执行已通过 `CodexPlusGatewayPolicyService` 聚合套餐、模型、设备、用量策略和审计事件；拒绝路径可产生可追踪、可脱敏的 risk/audit payload。
- 设备管理已支持管理员按用户列出、撤销和恢复设备，并保持 revoked/blocked 状态不会被客户端心跳误复活。
- 审计风控已支持 user/device/request/config context、网关拒绝摘要、redaction 元数据和支持侧查询。
- `codex-plus-dev-plan/tools/verify-06-static.ps1` 已通过。
- `codex-plus-dev-plan/tools/verify-06-go.ps1` 已通过：`gofmt -l` 清洁，targeted Go tests 覆盖 service、handler、admin handler、repository 和 routes 相关 Codex++ payment/subscription/gateway/device/audit/risk 路径。
- `codex-plus-dev-plan/tools/validate-stage-gate.ps1 -Stage 06-commerce-and-enforcement` 可复核 06 final report、worker reports、代码符号和 07 顺序门禁状态。

因此当前状态是：`06` passed，`07-integration-release` active；第 8/8 阶段开始执行 E2E、兼容、安装包和文档发布验收。

## 00-Contract Exit Gate

`00-contract` 只有在以下条件全部满足后才能进入 `01-backend-config-center`：

- A/B/C 三个并行 worker 都返回 final report。
- 每个 worker 的实际改动都只落在声明的写入范围内。
- OpenAPI、JSON schema、event schema 和 mock fixture 至少完成本地可解析验证。
- `CONTRACT-GATE.md` 中的客户端 API、后台配置、状态错误和事件项均能追溯到具体文件。
- `compatibility-matrix.md` 记录新增/变更字段的兼容策略。
- `WORKER-PROMPTS.md` 或后续派发提示词引用最新契约文件，不再引用旧字段。
- 明确写入：客户端不得硬编码价格、套餐、模型、倍率、额度阈值、限流策略、续费/购买文案。
- [00-contract/COORDINATOR-PREAUDIT.md](00-contract/COORDINATOR-PREAUDIT.md) 的 A/B/C 项都被对应 worker fixed、deferred 或 rejected，并给出证据。
- `powershell -ExecutionPolicy Bypass -File codex-plus-dev-plan/tools/validate-stage-gate.ps1` passes.

Latest gate run:

- Command: `powershell -ExecutionPolicy Bypass -File codex-plus-dev-plan\tools\validate-stage-gate.ps1`
- Result: passed.
- Passing areas: required files, `01-07` blocked status during validation, restart/preaudit references, OpenAPI path presence, JSON parsing, approval-residue scan and A/B/C final report checks.
- After this successful validation, `00-contract` was marked passed and `01-backend-config-center` became active.

## Sequential Dispatch Order

1. `00-contract`: 冻结契约、mock、状态、错误码和配置 schema。
2. `01-backend-config-center`: 按冻结 schema 实现配置中心和校验，不新增客户端字段。
3. `02-backend-client-api`: 聚合权益、余额、设备和配置快照，只消费 `00/01` 输出。
4. `03-client-cloud-core`: 消费 bootstrap 快照，写入 `Codex++ Cloud` 托管供应商。
5. `04-client-user-experience`: 基于状态模型做首页、登录、安装辅助和教程。
6. `05-admin-operations`: 管理员调价格、模型、额度、功能开关和用户权益。
7. `06-commerce-and-enforcement`: 支付开通、网关强制执行、设备和风控。
8. `07-integration-release`: 购买到启动 Codex 的 E2E、兼容迁移、安装包和文案。

## Coordinator Merge Rules

- contract 文件优先于任何已写源码假设。
- 后台配置 schema 优先于客户端 UI 文案。
- 状态错误模型优先于散落的本地判断。
- worker 输出冲突时，先回到本阶段任务文件和 `CONTRACT-GATE.md`，不直接按源码现状定案。
- 阶段退出必须有可复跑门禁证据；工具链不可用时只能标记为 static verification，不能标记 complete。
