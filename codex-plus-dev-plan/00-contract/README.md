# 00-contract 契约冻结阶段

## 当前状态

Status: `passed`

当前阶段因多会话执行纠偏重新打开后，A/B/C 三个并行 worker 已按 [PARALLEL-RESTART-PACK.md](PARALLEL-RESTART-PACK.md) 重新派发并返回 final report。coordinator 已运行阶段门禁并通过，`01-backend-config-center` 可以启动。

本阶段门禁验证命令：

```powershell
powershell -ExecutionPolicy Bypass -File codex-plus-dev-plan/tools/validate-stage-gate.ps1
```

该命令在 00 通过前用于验证门禁。通过后，阶段状态已转入 [STAGE-GATE-LEDGER.md](../STAGE-GATE-LEDGER.md)。

## 本阶段目标

冻结 Codex++ 客户端与 Sub2API 后台之间的客户端 API、后台配置模型、状态枚举、错误码和 mock 数据。后续所有实现必须按本阶段输出开发。

本阶段是整个 MVP 的硬门禁，执行细则见 [CONTRACT-GATE.md](../CONTRACT-GATE.md)。除只读调研外，后端、客户端、管理后台和网关 worker 不得在本阶段完成前实现跨层字段。

## 工业级 v2 映射

- 架构层：`codex-plus-contracts`。
- 覆盖范围：API schema、配置 schema、事件 schema、状态错误模型、mock fixtures 和兼容策略。
- 关键原则：契约必须支持版本化和兼容检查，不能只为当前 UI 字段服务。

## 并行任务列表

- `task-client-api-contract.md`
- `task-admin-config-contract.md`
- `task-status-and-error-model.md`
- 存储/迁移决策：记录配置、设备、权益、用量事件和幂等键的落点；可作为 coordinator 或独立 Module B 执行。

## 前置依赖

- 已确认产品采用登录绑定。
- 已确认客户端不硬编码价格、模型、套餐、额度、倍率、限流和续费文案。
- 已确认普通用户默认走 `Codex++ Cloud` 托管供应商。

## 合并顺序

1. 客户端 API 契约。
2. 后台配置契约。
3. 状态和错误模型。
4. 事件 schema、mock fixtures、类型生成/兼容矩阵。
5. 存储/迁移决策和变更审查策略。

## 阶段验收标准

- `/api/v1/auth/desktop/start`、`complete`、`poll` 的 browser handoff 鉴权边界明确。
- `/api/v1/client/bootstrap`、`usage`、`devices`、`redeem` 的请求和响应字段明确。
- 后台配置中心的套餐、模型、用量策略、功能开关字段明确。
- 所有服务状态、错误码和用户可读提示有统一枚举。
- mock JSON 可供客户端和后台并行开发。
- 用量、计费、审计、风控事件的字段边界明确，供后续 `Control Plane` 和 `Data Plane` 对接。
- 契约变更必须有版本号、兼容性说明和回滚影响说明。
- OpenAPI、JSON schema、status/error、events、fixtures、compatibility matrix 和 change policy 均已落地到可引用文件。
- Go、TypeScript、Rust 消费方的类型来源或手动类型 owner 已明确。
- worker prompts 已更新到最新契约版本。
