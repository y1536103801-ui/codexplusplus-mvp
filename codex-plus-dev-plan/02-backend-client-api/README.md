# 02-backend-client-api 客户端 API 阶段

## 当前状态

- Status: passed
- Exit gate: `tools/verify-02-static.ps1`、`tools/verify-02-go.ps1` 和 `tools/validate-stage-gate.ps1 -Stage 02-backend-client-api` 已通过。
- Next stage: `03-client-cloud-core` active。

## 本阶段目标

在 Sub2API 中实现 Codex++ 客户端专用 API，把配置中心、用户权益、余额、用户侧 Key、设备和公告聚合成客户端可直接消费的快照。

## 工业级 v2 映射

- 架构层：`Data Plane / Client API Gateway`。
- 覆盖范围：bootstrap、usage、devices、redeem，以及客户端可见的最小运行快照。
- 关键原则：API 返回聚合后的用户态结果，不泄露完整控制面配置和真实上游凭证。

## 并行任务列表

- `task-client-bootstrap-api.md`
- `task-client-usage-api.md`
- `task-client-device-api.md`
- `task-client-redeem-api.md`

## 前置依赖

- `00-contract` 完成。
- `01-backend-config-center` 提供只读配置服务。

## 合并顺序

1. device API。
2. bootstrap API。
3. usage API。
4. redeem API。

## 阶段验收标准

- 客户端 JWT 登录后可调用所有 `/api/v1/client/*` 接口。
- bootstrap 不要求客户端理解套餐和计价细节。
- usage 状态与网关 enforcement 使用同一后台策略来源。
- API Key 创建和返回有脱敏日志。
- bootstrap 响应包含配置版本和快照版本，便于客户端判断刷新与回滚。
- 客户端 API 产生结构化访问事件，可接入审计、风控和可观测链路。

## 退出门禁摘要

- `/api/v1/client/bootstrap`、`/usage`、`/devices`、`/redeem` 已接入 authenticated client routes。
- client DTO 已包含 `message_key`、`commerce_action`、`action_copy_key`、`balance_summary`、`period_usage`、feature flags、announcements 和 version policy。
- usage 状态与网关 enforcement 共用后台配置来源：plan 由 `entitlement_sources` 解析，usage policy 由 `usage_policy_id` 解析，可见模型按 plan model groups 过滤。
- 结构化事件覆盖 bootstrap、usage、device 和 redeem，并保留 request context；有配置快照的接口保留 config version。
- Targeted Go tests 覆盖 managed key 复用、contract fields、policy source、device upsert 和 redeem status mapping。
