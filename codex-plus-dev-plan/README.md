# Codex++ 解耦式多会话开发任务集

本目录用于把 Codex++ 买完即用产品拆成可并行执行的开发任务。目录序号代表执行顺序，同一目录内的 `task-*.md` 可以分配给多个会话并行开发。

## 工业级 v2 架构入口

当前文档已升级为工业级 v2 口径。阶段目录仍保持 `00` 到 `07` 不变，但最终实现必须映射到以下四层：

- `Control Plane`：后台运营、配置版本、权益、计费、策略决策和审计。
- `Data Plane`：客户端 API、策略执行、模型路由、限流、计量和风控。
- `Client Runtime`：桌面端登录、设备、bootstrap 快照、托管供应商和用户体验。
- `Platform Ops`：可观测、安全风险、发布门禁、迁移、烟测和回滚。

详细边界、关键链路和最终代码结构见 [ARCHITECTURE.md](ARCHITECTURE.md)。

## 执行入口

本目录现在分成“长期架构、Phase 1 MVP、Phase 2 生产上线”三层。开工时按以下顺序阅读：

1. [MVP-IMPLEMENTATION-PLAN.md](MVP-IMPLEMENTATION-PLAN.md)：第一版范围、验收标准、风险分级、阶段计划和测试计划。
2. [PHASE1-STARTUP-AUDIT.md](PHASE1-STARTUP-AUDIT.md)：真实源码启动审计、已有能力、缺口、工具链阻断和 Phase 0 开工建议。
3. [PRODUCTION-LAUNCH-PLAN.md](PRODUCTION-LAUNCH-PLAN.md)：第二阶段生产上线范围、服务器部署、域名 HTTPS、数据库、密钥、支付回调、QA/staging 验收、生产 smoke、监控和回滚。
4. [PRODUCTION-ENVIRONMENT-MATRIX.md](PRODUCTION-ENVIRONMENT-MATRIX.md)：生产服务器、域名、端口、环境变量、密钥名称、备份和监控信息表。
5. [BUSINESS-CONFIG-DECISION-TABLE.md](BUSINESS-CONFIG-DECISION-TABLE.md)：套餐、模型、额度、设备、手动开通、兑换码、真实支付和成本上限决策表。
6. [SERVER-SIZING-AND-SCALING-GUIDE.md](SERVER-SIZING-AND-SCALING-GUIDE.md)：服务器规格、容量判断、纵向升级、迁移和多服务器扩容路线。
7. [DEPLOYMENT-AUTOMATION-RUNBOOK.md](DEPLOYMENT-AUTOMATION-RUNBOOK.md)：单服务器 Docker Compose 部署、回滚、备份、健康检查和脚本模板说明。
8. [QA-TESTING-ACCEPTANCE-PLAN.md](QA-TESTING-ACCEPTANCE-PLAN.md)：MVP、staging、生产 smoke 和上线后观察的具体测试执行流程。
9. [CODEX-AUTONOMOUS-TEST-RUNBOOK.md](CODEX-AUTONOMOUS-TEST-RUNBOOK.md)：让 Codex 自主执行命令行、浏览器、桌面端和生产 smoke 测试的操作手册。
10. [SECURITY-REVIEW-PLAN.md](SECURITY-REVIEW-PLAN.md)：威胁模型、鉴权、支付回调、网关、密钥、桌面端和部署安全评审。
11. [COMPLIANCE-PRIVACY-LEGAL-CHECKLIST.md](COMPLIANCE-PRIVACY-LEGAL-CHECKLIST.md)：隐私政策、用户协议、退款政策、数据保留和上游服务条款检查。
12. [OBSERVABILITY-SLO-ALERTING-PLAN.md](OBSERVABILITY-SLO-ALERTING-PLAN.md)：监控指标、SLO、告警、仪表盘和事故排查。
13. [COST-CONTROL-AND-ABUSE-RUNBOOK.md](COST-CONTROL-AND-ABUSE-RUNBOOK.md)：上游模型成本、限流、熔断、滥用识别和人工处置。
14. [SUPPORT-OPERATIONS-RUNBOOK.md](SUPPORT-OPERATIONS-RUNBOOK.md)：客服、售后、退款、用户权益、设备和故障处理流程。
15. [CONTRACT-GATE.md](CONTRACT-GATE.md)：`00-contract` 必须产出的 OpenAPI、schema、mock、状态错误和事件契约。
16. [PARALLEL-DISPATCH-PLAN.md](PARALLEL-DISPATCH-PLAN.md)：模块 DAG、阶段并行规则、worktree/分支建议和集成顺序。
17. [FILE-OWNERSHIP-MATRIX.md](FILE-OWNERSHIP-MATRIX.md)：多会话开发时的文件归属、冲突矩阵和 stop conditions。
18. [PHASE1-MODULE-C-BACKEND-FOUNDATION-PLAN.md](PHASE1-MODULE-C-BACKEND-FOUNDATION-PLAN.md)：Module C 后端配置、设备、托管 Key、事件基础的可执行实现蓝图。
19. [PHASE1-MODULE-D-CLIENT-API-PLAN.md](PHASE1-MODULE-D-CLIENT-API-PLAN.md)：Module D `/api/v1/client/*` 客户端 API facade 的可执行实现蓝图。
20. [PHASE1-MODULE-E-GATEWAY-ENFORCEMENT-PLAN.md](PHASE1-MODULE-E-GATEWAY-ENFORCEMENT-PLAN.md)：Module E 网关策略强制执行、拒绝事件和协议错误映射的可执行实现蓝图。
21. [PHASE1-MODULE-F-DESKTOP-RUNTIME-PLAN.md](PHASE1-MODULE-F-DESKTOP-RUNTIME-PLAN.md)：Module F 桌面 runtime、bootstrap 消费、本地快照、托管供应商写入和日志脱敏的可执行实现蓝图。
22. [PHASE1-MODULE-G-DESKTOP-UX-PLAN.md](PHASE1-MODULE-G-DESKTOP-UX-PLAN.md)：Module G 普通用户云首页、登录绑定、安装辅助、新手教学、高级配置隐藏和有意义动效的可执行实现蓝图。
23. [PHASE1-MODULE-H-ADMIN-OPERATIONS-PLAN.md](PHASE1-MODULE-H-ADMIN-OPERATIONS-PLAN.md)：Module H 后台运营、配置发布、价格/模型/额度/功能开关管理、用户权益和设备视图的可执行实现蓝图。
24. [PHASE1-MODULE-I-E2E-RELEASE-GATE-PLAN.md](PHASE1-MODULE-I-E2E-RELEASE-GATE-PLAN.md)：Module I E2E 发布门禁、证据目录、测试账号矩阵、失败路径和 go/no-go 裁决的可执行计划。
25. [PHASE1-MODULE-J-INTEGRATION-COORDINATOR-PLAN.md](PHASE1-MODULE-J-INTEGRATION-COORDINATOR-PLAN.md)：Module J 最终集成、冲突处理、验证分层、文档同步和发布报告的可执行计划。
26. [FINAL-PLAN-COMPLETION-AUDIT.md](FINAL-PLAN-COMPLETION-AUDIT.md)：本轮计划文档完成度、结构校验、解耦要求和 HTML 同步审计记录。
27. [MODEL-QUALITY-POLICY.md](MODEL-QUALITY-POLICY.md)：防止核心 workflow/subagent 使用低能力模型开发高风险模块。
28. [WORKER-PROMPTS.md](WORKER-PROMPTS.md)：可复制给独立 Codex 会话的模块提示词。
29. [INTEGRATION-VERIFICATION-CHECKLIST.md](INTEGRATION-VERIFICATION-CHECKLIST.md)：最终合并、验证、E2E 和发布门禁。
30. [MULTI-SESSION-EXECUTION-TRACE.md](MULTI-SESSION-EXECUTION-TRACE.md)：当前实现轮次的真实多会话分工、代码落点、验收证据和阻塞项记录。

07 release evidence note: final evidence workspaces are timestamped. The upcoming Docs product copy lane adds `test-runs/YYYYMMDD-HHMM-docs`, verified by `tools/verify-07-docs-product-copy-evidence.ps1`, alongside E2E, package, compatibility, business readiness and `YYYYMMDD-HHMM-release` handoff artifacts. Use `tools/report-07-release-gaps.ps1 -RunStamp YYYYMMDD-HHMM` as a read-only sibling-lane/key-file gap report before handoff. Aggregate evidence, release coverage, readiness, handoff and Module J report checks are expected to require this Docs product copy evidence before a final release recommendation. Current static docs still include draft/pending boundaries; real release docs evidence remains to be finalized.

`ARCHITECTURE.md` 表达最终工业级目标；Phase 1 开发以 `MVP-IMPLEMENTATION-PLAN.md` 的范围为准，Phase 2 上线以 `PRODUCTION-LAUNCH-PLAN.md` 的范围为准。任何 worker 不得用长期目标扩大当前阶段范围。

## 两阶段路线

### Phase 1：本地/测试 MVP

目标是先在本地或测试环境跑通产品闭环：

`后台开通/测试支付 -> 登录 -> bootstrap -> 写入 Codex++ Cloud -> 启动 Codex -> 完成一次请求`

Phase 1 完成后，得到的是可运行、可验证的产品底座，但不等于已经生产上线。

### Phase 2：生产上线

目标是把 Phase 1 的产品底座部署到可真实售卖的生产环境：

`生产环境信息表 -> 业务配置决策表 -> 服务器部署 -> 域名 HTTPS -> PostgreSQL/Redis -> 密钥管理 -> 真实支付回调 -> 安全/合规/成本/客服门禁 -> QA/staging 验收 -> 生产 smoke test -> 监控告警 -> 安装包分发`

Phase 2 完成后，真实用户才能通过公网服务和客户端稳定使用。

## 总原则

- 客户端不内置价格、模型、套餐、额度、倍率、限流、续费文案或供应商规则。
- 管理员在 Sub2API 后台调整价格、模型、用量策略、功能开关后，Codex++ 客户端通过 bootstrap 快照自动承接。
- Codex++ 客户端只保存运行必要的登录态、设备信息和 Sub2API 用户侧 Key，不暴露上游真实凭证。
- Sub2API 网关必须强制执行模型权限、额度、限流和风控，不能依赖客户端隐藏按钮。
- 普通用户默认使用 `Codex++ Cloud` 托管供应商，高级供应商配置保留但默认隐藏。
- 所有配置变更、权益变更、用量事件、网关拒绝和管理员操作必须可审计、可追踪、可回滚。
- 策略决策属于 `Control Plane`，策略执行属于 `Data Plane`，客户端不得承担策略来源职责。
- 第一阶段先证明“后台开通/测试支付 -> 登录 -> bootstrap -> 写入 `Codex++ Cloud` -> 启动 -> 完成请求”的闭环。
- 第二阶段再补齐服务器部署、域名 HTTPS、生产数据库、密钥管理、真实支付回调、QA/staging 验收、监控告警、备份恢复和发布回滚。

## 阶段顺序

1. `00-contract`：冻结接口、配置、事件、状态、错误码、mock 响应。
2. `01-backend-config-center`：建设 `Control Plane / Config Registry`。
3. `02-backend-client-api`：实现 `Data Plane / Client API Gateway`。
4. `03-client-cloud-core`：Codex++ `Client Runtime` 消费 bootstrap 并写入托管供应商。
5. `04-client-user-experience`：普通用户首页、登录绑定、安装辅助和新手教学。
6. `05-admin-operations`：建设 `Control Plane / Admin Console`。
7. `06-commerce-and-enforcement`：支付、计量、网关强制执行、设备和风控闭环。
8. `07-integration-release`：`Platform Ops` 端到端验收、兼容迁移、安装包、烟测和回滚。

## 并行规则

- 默认按 [PARALLEL-DISPATCH-PLAN.md](PARALLEL-DISPATCH-PLAN.md) 的模块 DAG 分阶段并行，不再简单理解为“同一数字目录全部同时开跑”。
- `00-contract` 是硬门禁；除只读调研外，后端、客户端、管理后台 worker 必须等待对应契约冻结。
- 同一阶段内只有写入范围互不重叠的任务可以并行。
- 中央路由、schema/migration、setting service、gateway hooks、lockfile、全局 UI 入口必须有单一 owner，详见 [FILE-OWNERSHIP-MATRIX.md](FILE-OWNERSHIP-MATRIX.md)。
- 高风险模块必须按 [MODEL-QUALITY-POLICY.md](MODEL-QUALITY-POLICY.md) 指定模型质量层级；只读探索可以用轻量 worker，核心实现和集成不允许降级。
- 后续阶段可以提前阅读前序文档，也可以基于冻结 mock 做 UI/客户端原型，但不得提前实现未冻结的跨层字段。
- 每个任务必须遵守本任务文件的禁止改动范围和 worker prompt 中的 owned/avoid 列表。
- 每轮完成后由集成会话合并、跑测试、更新阶段 README 和执行文档，再进入下一轮。

## MVP 内部门禁

- MVP-0 完成条件：`CONTRACT-GATE.md` checklist 完成，存储/迁移决策明确，worker prompts 更新。
- MVP-1 完成条件：配置、设备、Key、权益基础能力可测，且没有路由/UI 字段漂移。
- MVP-2 完成条件：`/api/v1/client/*` 和网关 MVP enforcement 通过后端测试。
- MVP-3 完成条件：桌面 runtime 和后台运营能力分别通过类型检查/单元测试，且互不写入对方文件。
- MVP-4 完成条件：桌面 UX 覆盖主要状态，E2E 清单可在测试环境执行。
- MVP-5 完成条件：按 [INTEGRATION-VERIFICATION-CHECKLIST.md](INTEGRATION-VERIFICATION-CHECKLIST.md) 完成最终验证和发布说明。
- MVP-5 执行必须同时参考 [PHASE1-MODULE-I-E2E-RELEASE-GATE-PLAN.md](PHASE1-MODULE-I-E2E-RELEASE-GATE-PLAN.md) 和 [PHASE1-MODULE-J-INTEGRATION-COORDINATOR-PLAN.md](PHASE1-MODULE-J-INTEGRATION-COORDINATOR-PLAN.md)，分别负责 E2E 证据和最终集成裁决。

以上 MVP-0 到 MVP-5 属于第一阶段本地/测试 MVP 开发。生产上线前还必须通过 [PRODUCTION-LAUNCH-PLAN.md](PRODUCTION-LAUNCH-PLAN.md) 的 go/no-go checklist。

## 交付约定

每个任务完成时需要留下：

- 代码或配置变更说明。
- 测试命令和结果。
- 影响的接口、字段、状态或功能开关。
- 未完成项和风险。
- 与其他任务的接口对接备注。

## HTML 展示页

`index.html` 是本任务集的可视化索引。它必须与本文档和 [ARCHITECTURE.md](ARCHITECTURE.md) 保持一致：

- 阶段、任务数量和任务链接以目录中的 Markdown 文件为准。
- 工业级架构表达以 `ARCHITECTURE.md` 为准。
- HTML 只表达结构、边界和流向，不替代任务文档。
