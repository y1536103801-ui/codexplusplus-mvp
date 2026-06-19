# 03-client-cloud-core 客户端云绑定核心阶段

## 当前状态

- Status: passed
- Static gate: `tools/verify-03-static.ps1` passed。
- Node/TypeScript gate: `tools/verify-03-node.ps1` passed。
- Rust gate: `tools/verify-03-rust.ps1` passed。
- Next stage: `04-client-user-experience` is active；`05-07` remain blocked。

## 本阶段目标

让 Codex++ 客户端消费 Sub2API bootstrap 快照，自动生成 `Codex++ Cloud` 托管供应商并写入 Codex 本地配置。客户端只保存运行所需信息，不保存后台运营规则。

## 工业级 v2 映射

- 架构层：`Client Runtime`。
- 覆盖范围：认证设备、bootstrap consumer、snapshot cache、本地会话、托管供应商写入、错误映射和日志脱敏。
- 关键原则：客户端只执行快照，不生成价格、模型、额度、倍率、限流或续费策略。

## 并行任务列表

- `task-bootstrap-consumer.md`
- `task-managed-provider-writer.md`
- `task-local-session-store.md`
- `task-error-and-log-redaction.md`

## 前置依赖

- `02-backend-client-api` 提供可用 API 或 mock server。
- `00-contract` 的状态和错误码已冻结。

## 合并顺序

1. local session store。
2. bootstrap consumer。
3. managed provider writer。
4. error and log redaction。

## 阶段验收标准

- Codex++ 可登录并拉取 bootstrap。
- 可生成并应用 `Codex++ Cloud`。
- 日志不泄露 Key、JWT 或上游凭证。
- 网络失败、未购买、过期、余额不足有明确本地状态。
- 本地快照包含版本和过期策略，支持服务端回滚后的刷新。
- 设备撤销、模型下架和套餐过期由服务端状态驱动，客户端只能展示和引导。
