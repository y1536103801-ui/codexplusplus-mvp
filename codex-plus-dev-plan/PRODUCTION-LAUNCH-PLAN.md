# Codex++ Production Launch Plan

本文档定义第二阶段：把第一阶段本地/测试环境 MVP 推进到可真实上线售卖的生产方案。第一阶段目标是跑通产品闭环；第二阶段目标是让这个闭环能在公网、真实用户、真实支付和可运维环境下稳定运行。测试验收标准见 [QA-TESTING-ACCEPTANCE-PLAN.md](QA-TESTING-ACCEPTANCE-PLAN.md)，Codex 自主测试操作步骤见 [CODEX-AUTONOMOUS-TEST-RUNBOOK.md](CODEX-AUTONOMOUS-TEST-RUNBOOK.md)。

## Status

- State: planning
- Owner: Codex coordinator
- Created: 2026-06-16
- Last updated: 2026-06-16
- Depends on: [MVP-IMPLEMENTATION-PLAN.md](MVP-IMPLEMENTATION-PLAN.md)

## Phase Definition

### Phase 1: Local/Test MVP

目标：

- 在本地或测试环境跑通“开通权益 -> 登录 -> bootstrap -> 写入 `Codex++ Cloud` -> 启动 Codex -> 完成一次请求”。
- 证明产品核心链路成立。
- 证明客户端不硬编码价格、模型、额度和 Key。
- 证明网关能在服务端执行最小权限和用量拦截。

交付物：

- 可运行的 `sub2api-main` 后台/网关。
- 可运行的 `CodexPlusPlus-main` 桌面客户端。
- 契约、mock、测试和 E2E 清单。
- MVP 验收报告。

### Phase 2: Production Launch

目标：

- 把 Sub2API 部署到公网服务器。
- 给 Codex++ Manager 配置生产 bootstrap/API 地址。
- 接入真实域名、HTTPS、数据库、Redis、密钥、支付回调、备份、监控和回滚。
- 让真实用户可以下载客户端、登录、购买或被开通权益，并稳定使用。

交付物：

- 生产部署架构。
- 生产环境信息表：[PRODUCTION-ENVIRONMENT-MATRIX.md](PRODUCTION-ENVIRONMENT-MATRIX.md)。
- 业务配置决策表：[BUSINESS-CONFIG-DECISION-TABLE.md](BUSINESS-CONFIG-DECISION-TABLE.md)。
- 服务器规格和扩容路线：[SERVER-SIZING-AND-SCALING-GUIDE.md](SERVER-SIZING-AND-SCALING-GUIDE.md)。
- 服务器部署脚本或 Docker Compose/systemd 方案。
- 部署自动化手册和脚本模板：[DEPLOYMENT-AUTOMATION-RUNBOOK.md](DEPLOYMENT-AUTOMATION-RUNBOOK.md)。
- 生产环境变量和密钥管理方案。
- 域名、HTTPS、CORS、反向代理配置。
- PostgreSQL/Redis 生产配置、备份和恢复演练。
- 真实支付回调与权益开通验收。
- 独立 QA / staging 验收报告。
- 监控、日志、告警和事故手册。
- 客户端安装包、更新/回滚策略和用户安装说明。
- 安全评审、合规隐私、成本控制和客服运营上线门禁。
- 生产上线验收报告。

## What Users Get After Phase 2

普通用户：

- 下载 Codex++ Manager 安装包。
- 登录账号。
- 购买套餐或被管理员开通。
- 客户端自动连接你的生产 Sub2API 服务。
- 自动写入 `Codex++ Cloud` provider。
- 点击启动 Codex 即可使用。
- 过期、余额不足、设备撤销、模型不可用时看到明确提示。

管理员：

- 通过生产后台管理套餐、模型、用量策略、功能开关和用户权益。
- 能查看订单、支付状态、用户设备、Key 摘要、用量和异常。
- 能处理退款/补偿/手动开通/设备撤销等运营动作。
- 能查看日志、监控、告警和审计记录。

系统：

- Sub2API 在服务器上作为后台和网关运行。
- PostgreSQL/Redis 在生产环境中持久化和缓存。
- 域名和 HTTPS 对外提供服务。
- 网关转发到上游模型服务。
- 日志、备份、告警和回滚路径可用。

## Production Topology

```text
User desktop
  Codex++ Manager
  Codex App
      |
      | HTTPS login / bootstrap / usage / model request
      v
Production domain
  reverse proxy / TLS
      |
      v
Sub2API backend and gateway
  admin API
  client API
  OpenAI-compatible gateway
  payment callback handlers
      |
      | read/write
      v
PostgreSQL
Redis
      |
      | upstream API calls
      v
Model providers
  OpenAI / compatible providers
```

## Scope

第二阶段要做：

- 生产部署拓扑和服务器规格建议。
- 服务器容量、升级、迁移和多实例扩容路线。
- 生产环境信息表、域名计划、端口矩阵和环境变量矩阵。
- 套餐、模型、额度、设备、支付、兑换码和成本上限决策表。
- Docker Compose 或 systemd 部署方式。
- 域名、HTTPS、反向代理、CORS 和安全 header。
- PostgreSQL/Redis 生产配置、迁移、备份、恢复。
- 环境变量、密钥、上游凭证、支付凭证管理。
- 真实支付 provider callback 配置和幂等验收。
- 生产版 Codex++ Manager 的 API base URL 策略。
- 安装包构建、签名/分发、版本回滚说明。
- 日志脱敏、监控指标、告警规则和事故排查手册。
- 独立 QA / staging 验收、上线前回归和生产 smoke test。
- 合规隐私、成本控制、客服运营和安全评审。

第二阶段不做：

- 不重新定义 MVP 产品功能。
- 不在生产上线阶段临时扩大业务范围到团队套餐、白标、发票、工单。
- 不绕过 Phase 1 的契约和 E2E 验收。
- 不把真实密钥写入客户端、仓库或公开文档。

## Acceptance Criteria

- 生产服务器能通过 HTTPS 访问 Sub2API API 和管理后台。
- [PRODUCTION-ENVIRONMENT-MATRIX.md](PRODUCTION-ENVIRONMENT-MATRIX.md) 已填完所有生产必填项或明确延期项。
- [BUSINESS-CONFIG-DECISION-TABLE.md](BUSINESS-CONFIG-DECISION-TABLE.md) 已确定第一版套餐、模型、额度、设备、支付和成本上限。
- [SERVER-SIZING-AND-SCALING-GUIDE.md](SERVER-SIZING-AND-SCALING-GUIDE.md) 已确定当前阶段服务器规格和后续升级/迁移触发条件。
- [DEPLOYMENT-AUTOMATION-RUNBOOK.md](DEPLOYMENT-AUTOMATION-RUNBOOK.md) 的部署、备份、健康检查和回滚流程已审阅。
- 生产 PostgreSQL/Redis 可重启恢复，备份和恢复流程已演练。
- 真实或沙盒支付回调能幂等开通/续费权益。
- 生产 Codex++ Manager 指向生产 API 地址，能登录、bootstrap、写 provider、启动 Codex、完成请求。
- 网关在生产环境拒绝未授权模型、过期、余额不足、设备撤销和限流状态。
- 管理员能在生产后台修改模型/套餐/功能开关，并被客户端 bootstrap 承接。
- 日志不泄露 API Key、JWT、Authorization、上游凭证和支付密钥。
- 监控能看到 API 可用性、网关错误率、支付回调失败、用量异常、Redis/Postgres 状态。
- 告警能覆盖服务不可用、支付回调失败、数据库连接失败、上游错误率异常、磁盘空间不足。
- 发布说明包含版本、迁移、配置变更、回滚路径和已知风险。
- 安全、合规隐私、成本控制、客服运营门禁均已通过。

## Risk Level

高风险。

原因：

- 真实用户、真实支付、真实凭证、生产数据库和公网暴露都会放大错误影响。
- 部署、密钥、支付回调、日志、备份和回滚任何一处缺失，都可能导致用户无法使用、权益错误、成本失控或安全事故。

## Production Risk Review

| 风险域 | 必须解决 | 生产要求 |
| --- | --- | --- |
| 数据一致性 | 是 | 订单、权益、余额、用量和设备状态必须可追踪、可回滚。 |
| 并发/幂等 | 是 | 支付回调、兑换码、Key 创建、设备 upsert 必须幂等。 |
| 权限/安全 | 是 | Admin、用户 API、网关 API 权限隔离；生产密钥不进客户端和仓库。 |
| 支付/订单 | 是 | 回调签名校验、重复回调、失败补偿、退款/撤销策略明确。 |
| 数据库迁移 | 是 | 迁移前备份，迁移后验证，失败有回滚或人工修复方案。 |
| 外部服务 | 是 | 上游超时、错误率、限流、Key 失效、支付 provider 异常要可观测。 |
| 监控/日志 | 是 | 指标、结构化日志、请求 ID、用户/设备上下文和脱敏规则完整。 |
| 备份/恢复 | 是 | PostgreSQL 定时备份、恢复演练、Redis 可重建策略。 |
| 发布/回滚 | 是 | 服务端可回滚，客户端旧版本兼容，配置可回滚。 |
| 成本控制 | 是 | 上游模型成本、用户余额、异常用量、限流和告警必须闭环。 |

## Phase 2 Workstreams

### 2.0 Production Readiness Audit

Goal:

- 对 Phase 1 MVP 做上线差距审计。

Deliverables:

- `production-readiness-report.md`
- 未满足上线条件列表。
- 阻断项、可延期项、手动运营项分类。

Acceptance:

- 所有 Phase 1 E2E 已通过或阻断原因明确。
- 所有生产缺口都有 owner 和解决阶段。

### 2.1 Infrastructure and Deployment

Goal:

- 确定服务器、部署方式和基础运行环境。

Deliverables:

- 服务器规格建议。
- [SERVER-SIZING-AND-SCALING-GUIDE.md](SERVER-SIZING-AND-SCALING-GUIDE.md) 中的规格选择、扩容路线、迁移流程和升级触发指标。
- [PRODUCTION-ENVIRONMENT-MATRIX.md](PRODUCTION-ENVIRONMENT-MATRIX.md) 中的服务器、域名、端口、服务、变量、备份和监控信息。
- [DEPLOYMENT-AUTOMATION-RUNBOOK.md](DEPLOYMENT-AUTOMATION-RUNBOOK.md) 中的部署自动化流程。
- Docker Compose 或 systemd 部署方案。
- [deployment/README.md](deployment/README.md) 中的 Compose、Caddy、env 和脚本模板。
- 目录结构、日志路径、数据路径。
- 启停、重启、健康检查命令。

Acceptance:

- 新服务器可从零部署 Sub2API。
- 服务重启后后台、网关、管理前端可恢复。
- 当前阶段服务器规格有明确结论：staging、私测、付费验证或正式上线。
- 部署脚本、备份脚本、回滚脚本和健康检查脚本已完成 dry-run 或人工审阅。

### 2.1a Business Configuration Baseline

Goal:

- 在真实支付或正式售卖前冻结第一版业务配置，避免开发完成后才临时决定套餐、额度和成本边界。

Deliverables:

- [BUSINESS-CONFIG-DECISION-TABLE.md](BUSINESS-CONFIG-DECISION-TABLE.md) 已填入第一版套餐、模型、额度、设备、手动开通、兑换码、支付和成本策略。
- Admin 后台需要支持的配置项列表。
- Gateway 必须强制执行的额度、模型、设备和成本规则。

Acceptance:

- 第一版至少支持 `starter`、`manual_admin` 和 `redeem_code` 三类权益入口。
- 真实支付上线前，支付回调、重复回调、退款/撤销、人工补偿路径已定义。
- 客户端不硬编码套餐、价格、模型、额度和续费文案。
- 成本上限和紧急停机策略已定义。

### 2.2 Domain, TLS, Reverse Proxy and CORS

Goal:

- 让公网用户通过安全域名访问生产服务。

Deliverables:

- 域名和子域规划。
- HTTPS 证书配置。
- Nginx/Caddy/Traefik 反向代理配置。
- CORS、安全 header、body size、超时配置。

Acceptance:

- API 和管理后台均通过 HTTPS 访问。
- Codex++ Manager 能访问生产 client API。
- 浏览器和客户端请求没有 CORS/证书错误。

### 2.3 PostgreSQL, Redis, Migration and Backup

Goal:

- 建立可靠数据层。

Deliverables:

- PostgreSQL 生产参数。
- Redis 生产参数。
- migration 执行流程。
- 备份计划。
- 恢复演练记录。

Acceptance:

- 数据库重启后数据完整。
- 备份可恢复到临时实例。
- 迁移失败有回退或人工修复步骤。

### 2.4 Secrets and Environment Management

Goal:

- 保护生产密钥和环境变量。

Deliverables:

- `.env.example` 或部署变量清单，不含真实值。
- 生产密钥存储方式。
- 密钥轮换流程。
- 上游 Key、支付密钥、JWT secret、数据库密码管理说明。

Acceptance:

- 仓库、客户端、文档中没有真实 secrets。
- 生产服务可通过受控环境变量启动。
- 泄露响应和轮换步骤明确。

### 2.5 Payment Callback and Entitlement Automation

Goal:

- 把真实支付状态可靠转成用户权益。

Deliverables:

- 支付 provider 生产配置。
- 回调 URL 和签名校验配置。
- 支付成功、重复回调、失败回调、退款/撤销处理说明。
- 对账和补偿手册。

Acceptance:

- 沙盒或小额真实支付能自动开通权益。
- 重复回调不重复开通或重复充值。
- 回调失败可重试或人工补偿。

### 2.6 Observability, Alerts and Incident Response

Goal:

- 让生产问题可发现、可定位、可处理。

Deliverables:

- 指标列表。
- 结构化日志字段。
- 告警规则。
- 事故处理手册。
- 常见问题排查 runbook。

Acceptance:

- 能定位登录失败、bootstrap 失败、网关拒绝、支付回调失败、上游错误、数据库异常。
- 告警能通知到负责人。
- 日志脱敏验证通过。

### 2.7 Client Packaging, Distribution and Update

Goal:

- 让真实用户获得可安装客户端。

Deliverables:

- Windows/macOS 构建命令。
- 安装包检查清单。
- 生产 API base URL 配置策略。
- 下载分发页面或渠道说明。
- 客户端回滚和兼容策略。

Acceptance:

- 新机器安装后可登录生产服务。
- 覆盖安装不删除旧手动供应商。
- 旧客户端对服务端配置变更有兼容策略。

### 2.8 QA and Staging Acceptance

Goal:

- 在正式上线前，用接近生产的 staging 环境完成独立验收，确认不是“本地能跑”，而是“上线前可接受”。
- 按 [QA-TESTING-ACCEPTANCE-PLAN.md](QA-TESTING-ACCEPTANCE-PLAN.md) 执行 staging QA regression。

Deliverables:

- QA 测试计划。
- staging 环境配置说明。
- staging E2E 报告。
- 回归测试记录。
- 支付沙盒或小额真实支付验收记录。
- 安全/密钥/日志脱敏检查记录。
- 上线阻断问题列表。

Acceptance:

- staging 环境使用生产级部署方式，但不使用生产真实用户数据。
- 用户登录、购买/开通、bootstrap、provider 写入、启动 Codex、完成请求链路通过。
- 未开通、过期、余额不足、设备撤销、模型下架、网关异常、支付回调失败等失败路径通过。
- 管理员套餐、模型、功能开关和用户权益操作通过。
- 日志、错误提示、告警和审计记录可定位问题且不泄露 secrets。
- 所有 P0/P1 阻断问题关闭；P2/P3 问题有上线接受结论。

### 2.9 Business Readiness Gates

Goal:

- 在生产发布前确认技术上线以外的商业运营门禁已经准备好。

Deliverables:

- Security review report, based on [SECURITY-REVIEW-PLAN.md](SECURITY-REVIEW-PLAN.md).
- Compliance/privacy/legal checklist, based on [COMPLIANCE-PRIVACY-LEGAL-CHECKLIST.md](COMPLIANCE-PRIVACY-LEGAL-CHECKLIST.md).
- Observability/SLO/alerting readiness, based on [OBSERVABILITY-SLO-ALERTING-PLAN.md](OBSERVABILITY-SLO-ALERTING-PLAN.md).
- Cost/abuse readiness, based on [COST-CONTROL-AND-ABUSE-RUNBOOK.md](COST-CONTROL-AND-ABUSE-RUNBOOK.md).
- Support operations readiness, based on [SUPPORT-OPERATIONS-RUNBOOK.md](SUPPORT-OPERATIONS-RUNBOOK.md).

Acceptance:

- No open P0/P1 security issue.
- Public policy documents have owner approval or launch is explicitly limited to private beta.
- Cost caps, abuse controls and emergency switches are defined.
- P0/P1 alerts route to an owner.
- Support channel, refund process and entitlement recovery process are ready.

### 2.10 Production Smoke and Launch

Goal:

- 在 QA/staging 验收通过后执行生产上线和生产冒烟。

Deliverables:

- 生产上线步骤。
- 生产 smoke test。
- 回滚指令。
- 上线窗口和负责人。
- 上线后观察清单。

Acceptance:

- 生产上线后 smoke test 通过。
- 回滚路径可执行。
- 上线后观察窗口内没有 P0/P1 问题。

### 2.11 Post-Launch Monitoring and Support

Goal:

- 上线后持续观察真实用户链路，及时发现支付、登录、网关、成本和客户端问题。

Deliverables:

- 上线后 24-72 小时观察报告。
- 支付/订单/权益抽样核对。
- 上游模型成本和用户用量核对。
- 用户反馈收集和分级处理表。
- 第一轮热修/补丁发布策略。

Acceptance:

- 关键指标稳定。
- 支付成功率、bootstrap 成功率、网关成功率、客户端启动成功率在可接受范围。
- 发现的问题按 P0/P1/P2/P3 分级处理。
- 有明确热修、回滚或继续放量结论。

## Production Test Plan

本节是测试类别总览，具体步骤、测试账号、测试矩阵、报告模板和 go/no-go 规则见 [QA-TESTING-ACCEPTANCE-PLAN.md](QA-TESTING-ACCEPTANCE-PLAN.md)。

### Build and Deployment Tests

- Backend production build.
- Frontend production build.
- Docker Compose/systemd cold start.
- Service restart and health check.
- Reverse proxy reload.
- Deployment script dry run or manual review.
- Rollback script dry run or manual review.

### Data and Migration Tests

- Database migration dry run.
- Old-data compatibility check.
- Backup and restore test.
- Redis restart behavior check.

### Security and Config Tests

- HTTPS certificate check.
- CORS check from Codex++ Manager.
- Secret scan.
- Log redaction scan.
- Admin permission check.
- User isolation check.

### Business Flow Tests

- Payment callback sandbox/real low-value test.
- Duplicate payment callback idempotency test.
- Admin manual entitlement test.
- Redeem code activation test.
- User login/bootstrap/provider write/launch/request E2E.
- Usage and balance update check.
- Refund/compensation/manual correction procedure check.

### Gateway and Failure Path Tests

- Unauthorized model rejection.
- Expired entitlement rejection.
- Insufficient balance rejection.
- Revoked device rejection.
- Rate limit/concurrency rejection.
- Upstream provider failure behavior.
- Gateway unhealthy behavior.

### Release Tests

- Staging full regression.
- Production smoke test.
- Rollback drill.
- Post-launch monitoring check.

## Production Go/No-Go Checklist

- [ ] Phase 1 MVP E2E passed.
- [ ] Production environment matrix completed.
- [ ] Business config decision table completed.
- [ ] Server sizing and scaling guide reviewed.
- [ ] Production deployment scripts reviewed.
- [ ] Domain and HTTPS ready.
- [ ] Database migration and backup verified.
- [ ] Redis restart behavior understood.
- [ ] Secrets are outside repo/client/docs.
- [ ] Payment callback verified.
- [ ] QA/staging acceptance passed.
- [ ] P0/P1 blockers closed.
- [ ] Security review passed.
- [ ] Compliance/privacy/legal checklist accepted.
- [ ] Observability/SLO/alerting gate passed.
- [ ] Cost control and abuse gate passed.
- [ ] Support operations gate passed.
- [ ] Admin operations verified.
- [ ] User client E2E verified.
- [ ] Monitoring and alerting enabled.
- [ ] Rollback instructions tested or reviewed.
- [ ] Support/admin runbook ready.
- [ ] Known risks accepted by owner.

## Relationship to Existing Docs

- `MVP-IMPLEMENTATION-PLAN.md` owns Phase 1 local/test MVP scope.
- `PRODUCTION-LAUNCH-PLAN.md` owns Phase 2 production launch scope.
- `CONTRACT-GATE.md` remains valid for both phases.
- `PARALLEL-DISPATCH-PLAN.md` should be extended before Phase 2 parallel implementation starts.
- `INTEGRATION-VERIFICATION-CHECKLIST.md` verifies Phase 1; Phase 2 adds independent QA/staging acceptance, production smoke and post-launch monitoring in this file.

## Next Step

Do not start Phase 2 implementation until Phase 1 MVP E2E has passed or the coordinator explicitly marks which Phase 2 prep tasks are safe to run in parallel, such as deployment docs, server inventory, domain planning, and secret-management design.
