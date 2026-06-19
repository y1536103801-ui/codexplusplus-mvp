# 01-backend-config-center 后台配置中心阶段

## 本阶段目标

在 Sub2API 中建立运营可调的配置中心，让管理员后续可以调套餐、价格、模型、用量策略和客户端功能开关。客户端不直接消费这些原始配置，只消费下一阶段聚合后的快照。

## 工业级 v2 映射

- 架构层：`Control Plane / Config Registry`。
- 覆盖范围：配置版本化、草稿、发布、灰度、回滚、校验和审计。
- 关键原则：配置中心是运营规则来源，但不是请求执行点；执行点在 `Data Plane`。

## 并行任务列表

- `task-plan-catalog.md`
- `task-model-catalog.md`
- `task-usage-policy.md`
- `task-feature-flags.md`

## 前置依赖

- `00-contract` 完成并冻结字段。
- 配置默认值、状态枚举和错误码已确认。

## 合并顺序

1. 套餐目录。
2. 模型目录。
3. 用量策略。
4. 功能开关。

## 阶段验收标准

- 管理员可通过后端配置接口读写四类配置。
- 配置校验能阻止明显无效值。
- 配置服务能为下一阶段客户端 API 提供只读查询能力。
- 配置不依赖 Codex++ 客户端内部页面或组件。
- 每次配置变更记录版本、操作者、发布时间、影响范围和回滚目标。
- 套餐、模型、用量策略和功能开关可被策略决策模块编译成运行策略。

## 当前执行状态

- P/M/U/F 四个并行 worker 已完成并提交 final report。
- Coordinator 已完成共享配置服务集成。
- Coordinator 已追加 targeted service tests，覆盖 registry 默认配置、跨 catalog 默认引用和 registry validator 接入。
- `tools/verify-01-static.ps1` 已通过离线配置中心审计。
- `tools/verify-01-go.ps1` 已通过 Go 1.26.4 门禁：`gofmt -l` 无输出，`go test ./internal/codexplus/configregistry ./internal/service -run CodexPlus` 通过。
- 本阶段状态：passed。
- 下一阶段：只允许启动 `02-backend-client-api` 的同级并行任务，`03-07` 继续 blocked。