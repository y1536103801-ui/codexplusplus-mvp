# Codex++ 工业级目标架构 v2

本文档用于把原有 `00` 到 `07` 多会话开发任务升级为工业级产品架构表达。升级不改变既有阶段目录和 `task-*.md` 分派方式，而是明确最终实现时的系统边界、治理能力和验收门槛。

## 分阶段执行覆盖层

本文档描述最终目标，不等于第一版全部实现，也不等于已经具备生产上线条件。项目拆成两个明确阶段：

### Phase 1：本地/测试 MVP

第一轮开发必须先遵守 [MVP-IMPLEMENTATION-PLAN.md](MVP-IMPLEMENTATION-PLAN.md)：

- MVP 目标是完成“后台开通/测试支付 -> 登录 -> bootstrap -> 写入 `Codex++ Cloud` -> 启动 Codex -> 完成一次请求”的闭环。
- 完整支付自动开通、复杂风控、团队套餐、白标、工单、发票和完整灰度平台不作为第一版阻塞项。
- `00-contract` 必须先按 [CONTRACT-GATE.md](CONTRACT-GATE.md) 产出真实 OpenAPI、schema、mock、状态错误和事件契约。
- 多会话实现必须按 [PARALLEL-DISPATCH-PLAN.md](PARALLEL-DISPATCH-PLAN.md) 和 [FILE-OWNERSHIP-MATRIX.md](FILE-OWNERSHIP-MATRIX.md) 派工。
- 任何长期架构能力如果没有进入 MVP scope，只能作为后续阶段预留，不得在 worker 会话中擅自扩大范围。

### Phase 2：生产上线

MVP 通过后，第二阶段必须遵守 [PRODUCTION-LAUNCH-PLAN.md](PRODUCTION-LAUNCH-PLAN.md)：

- Sub2API 从本地/测试环境升级为公网服务器上的后台和网关。
- 生产环境必须补齐域名、HTTPS、反向代理、PostgreSQL、Redis、密钥管理、备份恢复、监控告警和回滚。
- 真实支付回调、权益开通、退款/补偿和对账必须经过沙盒或小额真实验收。
- Codex++ Manager 必须使用生产 API base URL，并通过生产 smoke test。
- 正式上线前必须按 [QA-TESTING-ACCEPTANCE-PLAN.md](QA-TESTING-ACCEPTANCE-PLAN.md) 通过独立 QA/staging 验收；Codex 可按 [CODEX-AUTONOMOUS-TEST-RUNBOOK.md](CODEX-AUTONOMOUS-TEST-RUNBOOK.md) 自主执行大部分命令行、浏览器和桌面测试，但真实支付、生产配置修改和破坏性操作必须有明确授权。生产 smoke test 不能替代完整回归。生产 go/no-go 还必须通过安全、合规隐私、可观测 SLO、成本滥用和客服运营门禁。
- 没有通过生产 go/no-go checklist 前，不得把 Phase 1 MVP 当成可售卖产品发布。

## 架构原则

- 客户端不保存完整运营规则，只消费当前用户可用的最小运行快照。
- 价格、模型、套餐、额度、倍率、限流、续费文案和功能开关全部由控制面配置版本驱动。
- 策略决策和策略执行分离：控制面编译策略，数据面在请求路径中强制执行。
- 所有配置变更、权益变更、计量事件、网关拒绝和管理员操作都要可审计、可追踪、可回滚。
- 普通用户默认使用 `Codex++ Cloud` 托管供应商，高级供应商配置保留但默认隐藏。
- 后端和网关是权益、余额、模型权限、设备状态和用量策略的唯一执行来源；客户端只展示和发起请求。
- 契约文件是跨项目协作的唯一字段来源；UI、客户端和后端不得各自发明字段。

## 分层架构

### Control Plane

控制面负责运营、权益、策略决策和配置发布。它可以相对复杂，但必须有版本治理和回滚能力。

核心模块：

- `Admin Console`：套餐、模型、价格、功能开关、用户权益、设备状态和审计查询。
- `Config Registry`：版本化配置、草稿、发布、灰度、回滚和配置校验。
- `Entitlement`：订阅状态、余额账本、设备绑定、模型组绑定和有效期。
- `Billing & Metering`：支付回调、用量入账、支付对账、失败补偿和人工修正。
- `Policy Decision`：把套餐、模型、额度、限流和功能开关编译成可执行策略。
- `Audit`：管理员操作、配置变更、权益变更、风险处置和售后定位。

### Data Plane

数据面负责客户端 API、网关执行、模型路由、限流计量和风险拦截。它必须稳定、低延迟，并且不信任客户端。

核心模块：

- `Client API Gateway`：`bootstrap`、`usage`、`devices`、`redeem`。
- `Policy Enforcement`：模型权限、余额、套餐有效期、设备撤销、并发、RPM、TPM 和每日额度。
- `Model Router`：展示模型到真实供应商模型的映射、回退和下架处理。
- `Quota & Rate Limit`：请求预扣、实际结算、失败回补、速率限制和并发控制。
- `Risk Guard`：异常设备、共享账号、盗刷、异常请求频率和风险处置。
- `Usage Event`：每次请求的预估成本、实际成本、拒绝原因和计量事件。

### Client Runtime

客户端运行时负责登录、设备、快照缓存、托管供应商和用户体验。它不得成为运营规则的来源。

核心模块：

- `Auth & Device`：账号登录、设备绑定、退出登录、设备撤销感知。
- `Bootstrap Cache`：缓存当前用户运行快照，支持短期离线读取和版本刷新。
- `Managed Provider`：生成或更新 `Codex++ Cloud`，隐藏 API Key 和供应商细节。
- `UX Shell`：首页、安装辅助、新手教程、状态提示、续费入口和高级配置入口。
- `Local Privacy`：日志脱敏、错误映射、最小本地存储和敏感字段清理。

### Platform Ops

平台运维层负责生产环境可维护性，不直接参与业务流程，但决定产品是否能稳定运营。

核心模块：

- `Observability`：bootstrap 链路、网关请求、模型路由、策略拒绝、支付对账和用量指标。
- `Security & Risk Ops`：Token 轮换、设备撤销、异常账号处置、泄露响应和事故手册。
- `Release Gate`：契约测试、灰度发布、烟测、回滚计划和兼容矩阵。
- `Migration`：旧手动供应商兼容、旧登录态升级、配置备份和恢复。

## 关键链路

### 配置发布

`Admin Console -> Config Registry -> Policy Decision -> Bootstrap Snapshot -> Client Runtime`

要求：

- 每次配置发布都有版本号、操作者、发布时间、灰度范围和回滚点。
- bootstrap 返回的是当前用户的聚合快照，不返回完整后台配置。
- 客户端检测到快照版本变化后刷新托管供应商和 UI 状态。

### 请求执行

`Client Runtime -> Client API Gateway -> Policy Enforcement -> Model Router -> Model Provider`

要求：

- 客户端只能发起请求，不能决定模型是否可用、余额是否足够或设备是否有效。
- 数据面必须在请求路径中强制执行模型权限、余额、套餐、限流、设备和风控。
- 模型下架、套餐过期、设备撤销和余额不足必须无需客户端发版即可生效。

### 用量闭环

`Data Plane -> Usage Event -> Metering -> Billing / Audit / Risk`

要求：

- 请求前可做预扣，请求后按真实消耗结算，失败请求必须可回补。
- 用量事件需要支持用户账单、管理员查询、风控分析和支付对账。
- 每个拒绝事件必须包含结构化原因，便于售后定位。

## 最终代码结构建议

该结构用于指导实现边界，不要求一次性创建所有目录。

```text
codex-plus-contracts/
  api/
  config/
  events/
  status-error/
  test-fixtures/
  compatibility-matrix.md
  change-review-policy.md

sub2api-main/
  internal/codexplus/
    controlplane/
      admin_console/
      config_registry/
      entitlement/
      billing/
      metering/
      policy_decision/
      audit/
    dataplane/
      client_api/
      policy_enforcement/
      model_router/
      usage_event/
      risk_guard/

CodexPlusPlus-main/
  src-tauri/src/codexplus_runtime/
    auth_device/
    bootstrap_consumer/
    snapshot_cache/
    local_session_store/
    managed_provider_writer/
    error_mapping/
    log_redaction/
  src/features/codexplus-cloud/
    api/
    ui/
    state/
    rules/

platform-ops/
  observability/
  security-risk/
  release/
  migration/
  e2e/
```

## 阶段映射

| 阶段 | 工业级架构映射 | 说明 |
| --- | --- | --- |
| `00-contract` | `codex-plus-contracts` | 冻结 API、配置、事件、状态、错误码和兼容策略。 |
| `01-backend-config-center` | `Control Plane / Config Registry` | 建立可版本化、可灰度、可回滚的配置源。 |
| `02-backend-client-api` | `Data Plane / Client API Gateway` | 输出最小客户端快照和用户侧状态 API。 |
| `03-client-cloud-core` | `Client Runtime` | 消费 bootstrap，写入托管供应商，维护本地最小状态。 |
| `04-client-user-experience` | `Client Runtime / UX Shell` | 让普通用户买完即用，同时保留高级入口。 |
| `05-admin-operations` | `Control Plane / Admin Console` | 管理员运营套餐、模型、额度、开关、用户权益和审计查询。 |
| `06-commerce-and-enforcement` | `Control Plane + Data Plane` | 支付权益、计量账本、网关强制执行、设备和风控闭环。 |
| `07-integration-release` | `Platform Ops` | 契约测试、E2E、兼容迁移、安装包、文档、烟测和回滚。 |

## 生产验收门槛

- 契约测试覆盖 bootstrap、usage、devices、redeem、配置 schema、事件 schema 和错误码。
- 后台配置支持版本、灰度、发布、回滚和审计记录。
- 网关可拒绝未授权模型、余额不足、套餐过期、设备撤销和限流超限。
- 用量事件可追踪预扣、实际结算、失败回补和支付对账。
- 客户端不硬编码套餐、价格、模型倍率、额度阈值、限流阈值或续费文案。
- 观测面可按用户、设备、模型、请求 ID、配置版本和错误码定位问题。
- 发布前必须通过购买到启动 Codex 的 E2E，以及旧用户迁移和回滚演练。

## MVP 验收门槛

第一版不要求完整工业级能力全部上线，但必须满足：

- bootstrap、usage、devices、redeem 的契约和 mock 完整。
- 后台可手动开通或模拟测试订单状态，让测试用户获得 Codex++ 权益。
- 桌面端能写入 `Codex++ Cloud` provider，且不删除旧手动供应商。
- 网关至少强制拒绝未授权模型、套餐过期、余额不足和设备撤销。
- Admin 能调整默认模型/套餐/功能开关并查看用户权益摘要。
- E2E 能完成开通权益、登录、bootstrap、写 provider、启动、请求成功、usage/log 可查。
- 发布说明必须写明哪些工业级能力已实现，哪些能力仍是后续阶段。

## 生产上线门槛

第二阶段必须额外满足：

- Sub2API 已部署到生产服务器并通过 HTTPS 访问。
- [PRODUCTION-ENVIRONMENT-MATRIX.md](PRODUCTION-ENVIRONMENT-MATRIX.md) 中的生产服务器、域名、端口、密钥名称、备份和监控信息已确认。
- [BUSINESS-CONFIG-DECISION-TABLE.md](BUSINESS-CONFIG-DECISION-TABLE.md) 中的套餐、模型、额度、设备、支付、兑换码和成本上限已确认。
- [SERVER-SIZING-AND-SCALING-GUIDE.md](SERVER-SIZING-AND-SCALING-GUIDE.md) 中的服务器规格、升级触发条件、迁移路线和扩容路线已确认。
- [DEPLOYMENT-AUTOMATION-RUNBOOK.md](DEPLOYMENT-AUTOMATION-RUNBOOK.md) 中的部署、备份、健康检查和回滚路径已审阅。
- PostgreSQL/Redis 有生产配置、备份和恢复演练。
- 生产密钥、上游凭证、支付密钥和 JWT secret 不进入仓库、客户端或公开文档。
- 真实或沙盒支付回调能幂等开通权益。
- QA/staging 验收已覆盖主链路、失败路径、管理员操作、支付回调、日志脱敏和回滚演练。
- [SECURITY-REVIEW-PLAN.md](SECURITY-REVIEW-PLAN.md) 中的 P0/P1 安全问题全部关闭。
- [COMPLIANCE-PRIVACY-LEGAL-CHECKLIST.md](COMPLIANCE-PRIVACY-LEGAL-CHECKLIST.md) 中的隐私、条款、退款、支付和供应商义务已确认或明确风险接受。
- 监控和告警覆盖 API 可用性、网关错误、支付回调失败、数据库异常、上游错误率和磁盘空间。
- [OBSERVABILITY-SLO-ALERTING-PLAN.md](OBSERVABILITY-SLO-ALERTING-PLAN.md) 中的 SLO、dashboard、告警路由和事故证据要求已准备。
- [COST-CONTROL-AND-ABUSE-RUNBOOK.md](COST-CONTROL-AND-ABUSE-RUNBOOK.md) 中的成本上限、滥用检测、限额和紧急停机路径已准备。
- [SUPPORT-OPERATIONS-RUNBOOK.md](SUPPORT-OPERATIONS-RUNBOOK.md) 中的客服入口、工单分级、退款补偿和权益修复流程已准备。
- 生产 Codex++ Manager 能登录、bootstrap、写 provider、启动并完成请求。
- 发布、回滚、事故处理和管理员运营手册已准备好。
