# 用户权益视图

## 目标

为管理员提供单个用户的 Codex++ 权益、设备、Key、套餐、余额、模型权限和异常状态视图。

## 涉及项目

- sub2api-main backend：admin user handler/service
- sub2api-main frontend：admin users view

## 输入契约

- client devices。
- user API keys。
- PlanCatalog/ModelCatalog/UsagePolicy 聚合结果。

## 输出行为

管理员可查看：

- 当前套餐、到期、余额。
- 允许模型。
- 设备列表和撤销状态。
- Codex++ 专用 API Key 摘要。
- 最近用量和错误状态。

## 解耦要求

- 视图展示后台聚合结果，不依赖客户端上报判断权益。
- 不显示完整 API Key。

## 禁止改动范围

- 不修改用户余额调整接口。
- 不实现支付。
- 不改客户端。

## 测试要求

- 权限隔离：非管理员不可访问。
- Key 脱敏。
- 设备撤销后 bootstrap 返回对应状态。

## 交付物

- 后台用户权益面板。
- admin API。
- 权限测试。

