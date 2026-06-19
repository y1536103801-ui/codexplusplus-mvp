# Codex++ QA Testing and Acceptance Plan

本文档是 Codex++ 的独立测试验收流程。它定义“测什么、何时测、怎样判断能否上线”。如果需要让 Codex 具体执行命令行、浏览器、桌面应用和生产 smoke 测试，请使用 [CODEX-AUTONOMOUS-TEST-RUNBOOK.md](CODEX-AUTONOMOUS-TEST-RUNBOOK.md)。

它覆盖两个阶段：

- Phase 1：本地/测试 MVP 验收。
- Phase 2：生产上线前 QA / staging 验收、生产 smoke test 和上线后观察。

原则：开发完成不等于可上线。必须按本文件完成测试、记录结果、关闭阻断问题后，才能进入下一阶段。

## Testing Goals

测试要回答四个问题：

- 产品链路是否真的跑通：开通权益、登录、bootstrap、写 provider、启动 Codex、完成请求。
- 权限和用量是否由后端/网关执行，而不是客户端假装限制。
- 生产上线风险是否可控：部署、域名、HTTPS、数据库、密钥、支付、备份、监控、回滚。
- 出问题时是否能定位、修复、补偿和回滚。

## Environments

| Environment | Purpose | Required before |
| --- | --- | --- |
| Local dev | 开发者本机验证单模块功能 | Module worker final report |
| MVP test | 本地或测试环境跑完整 MVP 链路 | Phase 1 MVP acceptance |
| Staging | 接近生产的上线前验收环境 | Phase 2 go/no-go |
| Production | 正式用户环境 | Production smoke and launch |

### Local dev

- 可以使用本地 PostgreSQL/Redis。
- 可以使用 mock payment 或手动权益开通。
- 用于模块测试、接口测试、客户端本地调试。

### MVP test

- 可以部署在本地、局域网或测试服务器。
- 要跑完整用户链路。
- 不要求生产域名、真实 HTTPS 和真实支付全部就绪。

### Staging

- 必须尽量接近生产部署方式。
- 使用独立数据库和 Redis。
- 不使用生产真实用户数据。
- 使用支付沙盒或小额真实支付。
- 使用接近生产的域名、HTTPS、反向代理和客户端配置。

### Production

- 只做 smoke test、监控观察和必要抽样验证。
- 不在生产环境做破坏性测试。
- 不用真实用户账号做高风险测试，优先使用测试账号。

## Test Accounts and Test Data

上线前至少准备：

- `admin_test`: 管理员测试账号。
- `user_active`: 已开通套餐的普通用户。
- `user_not_purchased`: 未开通用户。
- `user_expired`: 套餐过期用户。
- `user_low_balance`: 余额不足或接近不足用户。
- `user_device_revoked`: 设备被撤销用户。
- `user_model_denied`: 无指定模型权限用户。

测试数据要求：

- 每个用户有明确套餐、余额、模型权限和设备状态。
- 测试订单和支付回调有唯一编号。
- 测试 API Key 必须是测试环境 Key。
- 不把真实密钥、真实支付凭证、真实用户隐私写入测试报告。

## Severity Levels

| Level | Meaning | Release decision |
| --- | --- | --- |
| P0 | 核心链路不可用、数据错乱、密钥泄露、支付/权益严重错误 | 必须阻断上线 |
| P1 | 主要功能失败、权限绕过、网关错误拒绝/放行、无法回滚 | 必须修复或明确降级方案 |
| P2 | 非核心功能缺陷、提示不清楚、部分边缘状态不佳 | 可评估延期，但需记录 |
| P3 | 文案、样式、低风险体验问题 | 可延期 |

任何 P0/P1 未关闭时，不允许进入生产发布。

## Overall Test Flow

```text
1. Contract verification
2. Module verification
3. MVP integration test
4. Staging QA regression
5. Production release readiness review
6. Production smoke test
7. Post-launch monitoring
```

## 1. Contract Verification

Purpose:

- 确认前后端、桌面端、网关、管理后台都按同一套契约开发。

Entry criteria:

- `CONTRACT-GATE.md` checklist 完成。
- OpenAPI、schema、mock、错误码、事件定义存在。

Steps:

1. 检查 `GET /api/v1/client/bootstrap` 是否定义完整。
2. 检查 `GET /api/v1/client/usage` 是否定义完整。
3. 检查 `POST /api/v1/client/devices` 是否定义完整。
4. 检查 `POST /api/v1/client/redeem` 是否定义完整。
5. 检查 PlanCatalog、ModelCatalog、UsagePolicy、FeatureFlags schema。
6. 检查状态和错误码是否覆盖成功、未登录、未购买、过期、余额不足、设备撤销、模型不可用、网关异常、本地配置失败。
7. 检查 mock fixtures 能被桌面端和 UI worker 使用。
8. 检查事件 schema 是否覆盖 bootstrap、device、usage、redeem、gateway rejection、local write failure。

Pass criteria:

- 没有消费者使用契约外字段。
- 没有接口字段只有 UI 文档、没有后端契约。
- 所有错误状态都有用户提示和日志字段。

## 2. Module Verification

Purpose:

- 每个 worker 先在自己的模块内自测，不能把明显失败交给集成阶段。

Required report from each worker:

- Changed files.
- Contract inputs consumed.
- Contract outputs produced.
- Test commands and results.
- Known risks.
- Forbidden file pressure or conflict notes.

Suggested commands:

Backend:

```powershell
cd sub2api-main/backend
go test ./...
```

Backend full gate:

```powershell
cd sub2api-main
make test-backend
```

Admin frontend:

```powershell
cd sub2api-main/frontend
pnpm run lint:check
pnpm run typecheck
pnpm run test:run
```

Desktop:

```powershell
cd CodexPlusPlus-main
cargo test --workspace
```

Manager:

```powershell
cd CodexPlusPlus-main/apps/codex-plus-manager
npm run check
npm run build
```

Pass criteria:

- Worker-owned tests pass or failure is explained.
- No secrets added.
- No forbidden files edited.
- No contract drift.

## 3. MVP Integration Test

Purpose:

- 验证 Phase 1 本地/测试 MVP 是否得到你想要的第一版产品底座。

Entry criteria:

- Module A-J 已按 `PARALLEL-DISPATCH-PLAN.md` 合并。
- `INTEGRATION-VERIFICATION-CHECKLIST.md` 中的基础检查可执行。
- 测试环境能启动 Sub2API backend、admin frontend、PostgreSQL、Redis、Codex++ Manager。

### 3.1 Happy Path

Steps:

1. 管理员登录 Sub2API 后台。
2. 创建或确认一个套餐。
3. 配置可用模型和默认模型。
4. 配置 UsagePolicy 和 FeatureFlags。
5. 给 `user_active` 手动开通权益或使用测试订单开通。
6. 在 Codex++ Manager 登录 `user_active`。
7. 客户端注册/刷新设备。
8. 客户端调用 bootstrap。
9. 检查 bootstrap 返回 `available`、provider、plan、models、usage、feature flags、config version、snapshot version。
10. 客户端写入或更新 `Codex++ Cloud` provider。
11. 启动 Codex。
12. 发起一次模型请求。
13. 检查请求经过 Sub2API 网关。
14. 检查 usage/log/event 有记录。

Expected result:

- 用户无需手动填写 API Key、Base URL、模型倍率或套餐规则。
- Codex 请求成功。
- 后台能看到用量或事件。
- 日志没有完整 API Key/JWT/Authorization。

### 3.2 Not Purchased

Steps:

1. 使用 `user_not_purchased` 登录。
2. 调用 bootstrap。
3. 尝试启动或请求。

Expected result:

- bootstrap 返回 `not_purchased`。
- 客户端显示购买/开通提示。
- 网关不允许绕过。

### 3.3 Expired

Steps:

1. 使用 `user_expired` 登录。
2. 调用 bootstrap。
3. 尝试请求。

Expected result:

- bootstrap 返回 `expired`。
- 客户端显示续费或联系管理员提示。
- 网关拒绝请求。

### 3.4 Low Balance / Insufficient Balance

Steps:

1. 使用 `user_low_balance` 登录。
2. 调用 usage/bootstrap。
3. 发起请求。

Expected result:

- usage/bootstrap 返回低余额或余额不足状态。
- 客户端不自己计算余额阈值。
- 网关按后端策略拒绝或提示。

### 3.5 Device Revoked

Steps:

1. 管理员撤销 `user_device_revoked` 的测试设备。
2. 客户端刷新 bootstrap。
3. 尝试启动或请求。

Expected result:

- bootstrap 返回 `device_revoked`。
- 客户端停止自动配置或给出设备撤销提示。
- 网关或 client API 不允许继续使用被撤销设备。

### 3.6 Model Denied / Model Removed

Steps:

1. 管理员下架某模型或移除用户模型组权限。
2. 客户端刷新 bootstrap。
3. 尝试请求被下架/未授权模型。

Expected result:

- 客户端可用模型列表更新。
- 请求未授权模型时网关拒绝。
- 拒绝事件可查。

### 3.7 Local Failure

Steps:

1. 模拟 Codex 未安装。
2. 模拟本地 provider 写入失败。
3. 导出诊断日志。

Expected result:

- 客户端显示 `local_codex_missing` 或 `local_config_failed`。
- 提示可执行修复动作。
- 诊断日志脱敏。

MVP pass criteria:

- Happy path 必须通过。
- not purchased、expired、low balance、device revoked、model denied 至少覆盖。
- 没有 P0/P1。
- 所有未测项写入风险清单。

## 4. Staging QA Regression

Purpose:

- 在接近生产的环境证明系统可以上线，而不是只在本地能跑。

Entry criteria:

- MVP integration test 通过。
- Staging 环境部署完成。
- Staging 使用生产级部署方式、HTTPS、独立 PostgreSQL/Redis、独立测试密钥。
- Codex++ Manager staging build 指向 staging API。

### 4.1 Deployment Regression

Steps:

1. 从零部署 staging。
2. 执行数据库迁移。
3. 启动 backend、frontend、Redis、PostgreSQL。
4. 检查健康接口。
5. 重启服务。
6. 检查数据是否保留。

Expected result:

- 服务冷启动和重启可恢复。
- migration 无失败。
- 日志路径、数据路径、环境变量正确。

### 4.2 HTTPS and Client Connectivity

Steps:

1. 检查 staging 域名证书。
2. 检查反向代理。
3. 从 Codex++ Manager 调用 login/bootstrap。
4. 检查 CORS 和超时。

Expected result:

- 无证书错误。
- 无 CORS 错误。
- 客户端能稳定访问 API。

### 4.3 Payment Regression

Steps:

1. 创建测试订单。
2. 使用支付沙盒或小额真实支付。
3. 触发支付回调。
4. 重放同一回调。
5. 查询用户权益。
6. 检查对账/事件。

Expected result:

- 支付成功后权益开通。
- 重复回调不重复开通或重复充值。
- 回调签名校验有效。
- 失败回调可重试或人工补偿。

### 4.4 Admin Regression

Steps:

1. 管理员修改套餐。
2. 管理员修改默认模型。
3. 管理员下架模型。
4. 管理员修改 feature flags。
5. 管理员查看用户权益、设备、Key 摘要。

Expected result:

- 后端校验生效。
- 客户端 bootstrap 能承接变更。
- 不显示完整 secrets。

### 4.5 Security and Redaction

Steps:

1. 搜索日志中的 API Key、JWT、Authorization。
2. 搜索构建产物中的真实密钥。
3. 检查 `.env.example` 不含真实值。
4. 检查客户端不包含上游真实凭证。
5. 检查 admin API 不能被普通用户调用。

Expected result:

- 无 secret 泄露。
- 权限隔离正确。

### 4.6 Backup and Restore

Steps:

1. 创建测试数据。
2. 执行 PostgreSQL 备份。
3. 恢复到临时实例。
4. 验证关键表和测试用户状态。

Expected result:

- 备份可恢复。
- 恢复后关键业务数据一致。

### 4.7 Rollback Drill

Steps:

1. 记录当前版本。
2. 部署新版本。
3. 执行 smoke test。
4. 模拟失败。
5. 回滚服务端。
6. 回滚配置。
7. 验证旧客户端兼容。

Expected result:

- 回滚步骤可执行。
- 数据不丢失。
- 手动 provider 不被删除。

Staging pass criteria:

- Happy path 和关键失败路径全部通过。
- 支付、权限、密钥、备份、回滚通过。
- P0/P1 关闭。
- P2/P3 有接受结论。

## 5. Production Release Readiness Review

Purpose:

- 正式上线前做 go/no-go 决策。

Required inputs:

- MVP integration report.
- Staging QA report.
- Production environment matrix from [PRODUCTION-ENVIRONMENT-MATRIX.md](PRODUCTION-ENVIRONMENT-MATRIX.md).
- Business config baseline from [BUSINESS-CONFIG-DECISION-TABLE.md](BUSINESS-CONFIG-DECISION-TABLE.md).
- Server sizing and scaling decision from [SERVER-SIZING-AND-SCALING-GUIDE.md](SERVER-SIZING-AND-SCALING-GUIDE.md).
- Deployment automation review from [DEPLOYMENT-AUTOMATION-RUNBOOK.md](DEPLOYMENT-AUTOMATION-RUNBOOK.md).
- Payment callback report.
- Backup/restore report.
- Security/redaction report from [SECURITY-REVIEW-PLAN.md](SECURITY-REVIEW-PLAN.md).
- Privacy/legal readiness report from [COMPLIANCE-PRIVACY-LEGAL-CHECKLIST.md](COMPLIANCE-PRIVACY-LEGAL-CHECKLIST.md).
- Observability/SLO readiness report from [OBSERVABILITY-SLO-ALERTING-PLAN.md](OBSERVABILITY-SLO-ALERTING-PLAN.md).
- Cost and abuse readiness report from [COST-CONTROL-AND-ABUSE-RUNBOOK.md](COST-CONTROL-AND-ABUSE-RUNBOOK.md).
- Support operations readiness report from [SUPPORT-OPERATIONS-RUNBOOK.md](SUPPORT-OPERATIONS-RUNBOOK.md).
- Rollback drill report.
- Known risks list.

Go criteria:

- No open P0/P1.
- Staging QA passed.
- Production environment matrix has no unresolved required production value.
- First-launch business config is frozen or explicitly marked as private beta.
- Selected server spec matches the selected release stage.
- Deployment, backup, healthcheck and rollback scripts are reviewed.
- Production secrets configured outside repo.
- Security review passed with no unresolved P0/P1.
- Privacy/legal launch checklist accepted or explicitly risk-approved.
- Observability dashboards, SLOs and alert routing are enabled.
- Cost caps, quota controls, abuse controls and emergency shutoff are ready.
- Monitoring and alerts enabled.
- Rollback owner and steps confirmed.
- Support/admin runbook ready.
- Refund, entitlement correction and user support paths have owners.

No-go criteria:

- Payment callback not verified.
- Production environment values are incomplete.
- First-launch package/model/quota/cost decisions are missing.
- Server spec is too small for the selected paid launch stage and no mitigation exists.
- Deployment or rollback path is unreviewed.
- Bootstrap happy path unstable.
- Gateway can be bypassed.
- Logs leak secrets.
- Security review has open P0/P1.
- Upstream model provider terms, payment terms or privacy obligations are not reviewed.
- No cost emergency stop or abuse response path exists.
- Backup restore unverified.
- Rollback path unknown.
- Admin cannot recover user entitlement issues.
- No refund/support process exists for paid users.

## 6. Production Smoke Test

Purpose:

- 生产上线后做最小非破坏性验证。

Entry criteria:

- Release readiness review gives go.
- Production deployment completed.
- Test user and admin user ready.

Steps:

1. 打开生产后台健康检查。
2. 管理员登录生产后台。
3. 检查套餐、模型、feature flags。
4. 用生产测试用户登录 Codex++ Manager。
5. 调用 bootstrap。
6. 写入 `Codex++ Cloud` provider。
7. 启动 Codex。
8. 发起一次低成本模型请求。
9. 检查 usage/log/event。
10. 检查监控面板和告警状态。

Expected result:

- 主链路成功。
- 没有异常告警。
- 日志脱敏。
- 用量记录正确。

Production smoke should not:

- 使用真实普通用户账号做破坏性测试。
- 删除真实用户设备。
- 执行批量退款或批量权益修改。
- 关闭网关或数据库做故障演练。

## 7. Post-Launch Monitoring

Purpose:

- 上线后 24-72 小时观察真实链路。

Monitor:

- 登录成功率。
- bootstrap 成功率。
- 网关请求成功率。
- gateway rejection 分布。
- 支付成功率和回调失败数。
- 上游 provider 错误率。
- PostgreSQL/Redis 健康。
- API latency。
- 用户投诉和支持请求。
- 上游成本和用户余额变动。

Actions:

- 每 2-4 小时记录一次关键指标。
- P0/P1 立即处理或回滚。
- P2/P3 进入补丁计划。
- 支付/权益异常优先人工核对。

Exit criteria:

- 观察窗口内无持续 P0/P1。
- 关键指标稳定。
- 已知问题有 owner 和处理计划。

## Test Report Template

```text
Test phase:
Environment:
Build/version:
Tester:
Date:

Scope:
- 

Test data/accounts:
- 

Commands/checks run:
- command:
  result:

Passed cases:
- 

Failed cases:
- ID:
  severity:
  steps:
  expected:
  actual:
  owner:
  decision:

Unrun cases:
- case:
  reason:
  risk:

Go/no-go:
- 

Notes:
- 
```

## Minimum Test Case Matrix

| Area | Must test before MVP acceptance | Must test before production |
| --- | --- | --- |
| Contract shape | yes | yes |
| Bootstrap happy path | yes | yes |
| Provider write | yes | yes |
| Manual provider preservation | yes | yes |
| Not purchased | yes | yes |
| Expired | yes | yes |
| Low balance | yes | yes |
| Device revoked | yes | yes |
| Model denied | yes | yes |
| Admin plan/model/flags | basic | full |
| Payment callback | optional/test order | required |
| HTTPS/CORS | no | required |
| Backup/restore | no | required |
| Secret scan | yes | required |
| Monitoring/alerting | basic logs | required |
| Rollback drill | documented | required |
| Production smoke | no | required |
| Post-launch monitoring | no | required |
