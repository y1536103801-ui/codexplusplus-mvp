# 00-Contract Parallel Restart Pack

本文件用于在额度恢复或开启新会话后，重新派发 `00-contract` 阶段。三个 worker 必须并行运行，但写入范围互不重叠。所有 worker 返回 final report 之前，不得进入 `01-backend-config-center`。

## Coordinator Rule

- 当前阶段：`00-contract`。
- 后续阶段：`01` 到 `07` 全部 blocked。
- worker 不得修改源码目录：`sub2api-main/**`、`CodexPlusPlus-main/**`。
- worker 不得把已有实现当作契约事实来源；契约文件优先。
- 如果发现契约与已有实现冲突，只记录冲突，不直接修改实现。
- 如果需要编辑未授权文件，停止并报告。
- 开工前读取 [COORDINATOR-PREAUDIT.md](COORDINATOR-PREAUDIT.md)，并在 final report 中逐项回答属于自己 lane 的预审问题。
- 可复制 [reports/](reports/) 中对应 `.template.md` 编写 final report，但必须改为固定 final 文件名并把 `Report status` 改为 `final`。

## Worker A: Client API Contract

```text
你是 00-contract 并行 worker A，负责“客户端 API 契约冻结”。

你不是单独在代码库里工作，其他 worker 和主集成会话会并行修改不同文件；不要 revert 任何非你所做的改动。

阶段约束：
- 只允许处理 00-contract。
- 不进入 01/02/03 等后续阶段。
- 不修改源码。

唯一写入范围：
- codex-plus-contracts/api/client-openapi.yaml
- codex-plus-contracts/test-fixtures/client/*.json
- codex-plus-dev-plan/00-contract/task-client-api-contract.md

目标：
1. 对照 codex-plus-dev-plan/00-contract/README.md、CONTRACT-GATE.md 和 task-client-api-contract.md。
2. 冻结 /api/v1/auth/desktop/start、/complete、/poll。
3. 冻结 /api/v1/client/bootstrap、/usage、/devices、/redeem。
4. 补齐缺失 response schema、错误 envelope、mock fixture 或任务验收清单。
5. 明确客户端不得硬编码价格、套餐、模型倍率、额度阈值、限流策略、续费/购买文案；全部来自 bootstrap/config snapshot。
6. 回答 COORDINATOR-PREAUDIT.md 中 A1-A5，每项必须标记为 fixed、deferred 或 rejected，并说明理由。

验证：
- JSON fixtures 必须可解析。
- OpenAPI 至少完成结构自查：路径存在、schema 可追溯、错误 envelope 一致。

最终报告：
- 改动文件。
- 补齐内容。
- 未解决契约风险。
- 验证命令和结果。
```

## Worker B: Admin Config Contract

```text
你是 00-contract 并行 worker B，负责“后台配置契约冻结”。

你不是单独在代码库里工作，其他 worker 和主集成会话会并行修改不同文件；不要 revert 任何非你所做的改动。

阶段约束：
- 只允许处理 00-contract。
- 不进入 01/02/03 等后续阶段。
- 不修改源码。

唯一写入范围：
- codex-plus-contracts/config/plan-catalog.schema.json
- codex-plus-contracts/config/model-catalog.schema.json
- codex-plus-contracts/config/usage-policy.schema.json
- codex-plus-contracts/config/feature-flags.schema.json
- codex-plus-dev-plan/00-contract/task-admin-config-contract.md

目标：
1. 对照 codex-plus-dev-plan/00-contract/README.md、CONTRACT-GATE.md 和 task-admin-config-contract.md。
2. schema 必须支持管理员调控价格、套餐、模型、默认模型、额度、倍率、限流、设备策略、功能开关、续费/购买文案引用键。
3. 明确客户端只消费配置快照，不得硬编码上述值。
4. 明确本任务只冻结契约，不实现后台服务。
5. 回答 COORDINATOR-PREAUDIT.md 中 B1-B4，每项必须标记为 fixed、deferred 或 rejected，并说明理由。

验证：
- 四个 JSON schema 必须可解析。
- schema 字段与任务文档中的解耦要求一致。

最终报告：
- 改动文件。
- 补齐内容。
- 未解决契约风险。
- 验证命令和结果。
```

## Worker C: Status Error Event Contract

```text
你是 00-contract 并行 worker C，负责“状态、错误码与事件契约冻结”。

你不是单独在代码库里工作，其他 worker 和主集成会话会并行修改不同文件；不要 revert 任何非你所做的改动。

阶段约束：
- 只允许处理 00-contract。
- 不进入 01/02/03 等后续阶段。
- 不修改源码。

唯一写入范围：
- codex-plus-contracts/status-error/client-status-errors.md
- codex-plus-contracts/events/client-events.schema.json
- codex-plus-dev-plan/00-contract/task-status-and-error-model.md

目标：
1. 对照 codex-plus-dev-plan/00-contract/README.md、CONTRACT-GATE.md 和 task-status-and-error-model.md。
2. 覆盖购买/未登录/未购买/过期/余额不足/模型不可用/设备撤销/网关异常/browser handoff pending/expired/approved/redeemed 等状态。
3. 冻结错误码、HTTP 状态、retryability、client_action、user_message_source 和 log fields。
4. 明确事件不得记录 access token、refresh token、session_token、poll_token、托管 API key、原始 prompt/response 正文。
5. 回答 COORDINATOR-PREAUDIT.md 中 C1-C4，每项必须标记为 fixed、deferred 或 rejected，并说明理由。

验证：
- event JSON schema 必须可解析。
- 状态表中的错误码必须能覆盖 mock fixtures 和 OpenAPI 错误响应。

最终报告：
- 改动文件。
- 补齐内容。
- 未解决契约风险。
- 验证命令和结果。
```

## Coordinator Merge Checklist

- [ ] A/B/C 都返回 final report。
- [ ] A/B/C final report 写入 [reports/](reports/) 中的固定文件名。
- [ ] 三个 worker 都没有越权编辑文件。
- [ ] JSON fixtures 和 JSON schemas 可解析。
- [ ] OpenAPI 路径和 schema 与任务文档一致。
- [ ] `CONTRACT-GATE.md` 的 checklist 与实际文件逐项对应。
- [ ] `compatibility-matrix.md` 记录变更影响。
- [ ] `WORKER-PROMPTS.md` 引用最新 `00-contract` 产物。
- [ ] [COORDINATOR-PREAUDIT.md](COORDINATOR-PREAUDIT.md) 中的 A/B/C 项均已由对应 worker fixed、deferred 或 rejected。
- [ ] `powershell -ExecutionPolicy Bypass -File codex-plus-dev-plan/tools/validate-stage-gate.ps1` passes.
- [ ] `STAGE-GATE-LEDGER.md` 将 `00-contract` 标记为 passed 后，才允许启动 `01`。
