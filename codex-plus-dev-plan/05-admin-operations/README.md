# 05-admin-operations 管理员运营阶段

## 当前状态

- Status: passed
- Previous gate: `04-client-user-experience` passed。
- Exit gate: 管理员套餐、模型、用量策略、功能开关和用户权益视图已按本阶段任务文件实现并验证。
- Next gate: `06-commerce-and-enforcement` can become active。
- Blocked stages: `07-integration-release` remains blocked until `06-commerce-and-enforcement` passes。

## 本阶段目标

让管理员可以在 Sub2API 后台调控 Codex++ 产品的套餐、价格、模型、用量策略、功能开关和用户权益，同时查看用户服务状态和设备。

## 工业级 v2 映射

- 架构层：`Control Plane / Admin Console`。
- 覆盖范围：运营配置管理、权益查看、配置发布、审计查询和用户售后定位。
- 关键原则：后台页面不直接写客户端状态，只写控制面配置和权益数据。

## 并行任务列表

- `task-admin-plan-management.md`
- `task-admin-model-management.md`
- `task-admin-usage-policy-management.md`
- `task-admin-user-entitlement-view.md`

## 前置依赖

- `01-backend-config-center` 提供配置服务。
- `02-backend-client-api` 已能返回客户端快照。

## 合并顺序

1. plan management。
2. model management。
3. usage policy management。
4. user entitlement view。

## 阶段验收标准

- 管理员能改价格、模型、默认模型、额度、限流和功能开关。
- 改动后 bootstrap 输出变化。
- 管理员能查看单个用户的 Codex++ 可用状态。
- 重要配置变更必须进入配置版本流，支持草稿、发布、灰度和回滚。
- 管理员能看到配置版本、用户权益、设备状态、网关拒绝和用量摘要之间的关联。

## 验证结果

- 四个并行 worker final report 已归档。
- `tools/verify-05-static.ps1` passed。
- `tools/verify-05-node.ps1` passed：`npm run typecheck` 和 `npm run build` 均通过。
- `tools/verify-05-go.ps1` passed：`gofmt -l` 清洁，targeted Go tests 通过。
- `tools/validate-stage-gate.ps1 -Stage 05-admin-operations` passed。
