# 06-commerce-and-enforcement 商业闭环与强制执行阶段

## 当前状态

- Status: passed
- Previous gate: `05-admin-operations` passed。
- Current gate: 支付权益、网关策略、设备管理、审计风控已按本阶段任务文件实现并通过静态、Go 和总门禁。
- Next gate: `07-integration-release` active。
- Evidence: `reports/coordinator-commerce-enforcement-verification.md`、四个 worker final reports、`verify-06-static.ps1`、`verify-06-go.ps1`。

## 本阶段目标

打通支付成功后的权益开通，并确保模型权限、额度、限流、设备状态和风控在 Sub2API 网关侧强制执行。

## 工业级 v2 映射

- 架构层：`Control Plane + Data Plane`。
- 覆盖范围：支付权益、余额账本、计量事件、网关策略执行、模型路由、设备管理、审计和风控。
- 关键原则：控制面决定策略，数据面在请求路径中执行策略并生成可对账事件。

## 并行任务列表

- `task-payment-entitlement-flow.md`
- `task-gateway-policy-enforcement.md`
- `task-device-management.md`
- `task-audit-and-risk-control.md`

## 前置依赖

- `05-admin-operations` 已能配置套餐、模型和用量策略。
- `02-backend-client-api` 已能返回用户快照。

## 合并顺序

1. payment entitlement flow。
2. gateway policy enforcement。
3. device management。
4. audit and risk control。

## 阶段验收标准

- 支付成功后用户权益自动变化。
- 网关拒绝未授权模型、过期套餐、余额不足和被撤销设备。
- 风控和审计日志可支持售后定位。
- 请求计量支持预扣、实际结算、失败回补和支付对账。
- 策略拒绝事件包含用户、设备、模型、配置版本、错误码和请求 ID。
