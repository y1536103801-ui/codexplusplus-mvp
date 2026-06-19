# 网关策略强制执行

## 目标

在 Sub2API 网关侧强制执行 Codex++ 用户的模型权限、余额、套餐、并发、RPM、TPM 和每日额度。

## 涉及项目

- sub2api-main：`backend/internal/service/gateway_service.go`
- sub2api-main：`backend/internal/service/openai_gateway_service.go`
- sub2api-main：billing、ratelimit、concurrency 相关服务

## 输入契约

- ModelCatalog。
- UsagePolicy。
- PlanCatalog.entitlement_sources。
- FeatureFlags.strict_device_enforcement。
- 用户权益。
- 用户侧 API Key。
- Control Plane 编译出的策略决策结果。
- Data Plane 当前请求上下文：用户、设备、模型、请求 ID、配置版本和预估 token。

## 输出行为

网关请求前检查：

- 用户 Key 是否有效。
- 用户套餐是否可用。
- 当前订阅或 API Key 分组是否能映射到 Codex++ 套餐。
- 请求模型是否授权。
- 余额和用量是否足够。
- 设备是否存在、可用且未撤销。
- 并发和速率是否超限。

失败时返回 `GATEWAY_POLICY_*` 错误码。

成功或失败都要产生 `usage_event`：

- 成功请求记录预扣、真实消耗、模型路由和结算结果。
- 失败请求记录拒绝原因、错误码、配置版本和是否需要回补。
- 限流、余额不足、设备撤销、模型下架都要进入可观测和审计链路。

## 解耦要求

- enforcement 不依赖客户端隐藏 UI。
- 模型和用量策略以后台配置为唯一来源。
- 套餐授权必须来自后台配置的 entitlement source mapping；无映射时不得默认授予任意套餐。
- 严格设备校验由后台功能开关控制；客户端只发送设备上下文，不能决定 enforcement 强度。
- 策略执行只消费控制面编译后的策略，不在请求路径中临时解释管理员表单。

## 禁止改动范围

- 不改客户端 bootstrap。
- 不改管理员页面。
- 不改变无关平台的现有路由行为。
- 不在客户端或 UI 层实现真实拒绝逻辑。

## 测试要求

- 未授权模型请求被拒绝。
- 托管 Key 无套餐映射时被拒绝为未购买或未授权。
- API key group / subscription group 映射到套餐后，只能使用该套餐模型组。
- `strict_device_enforcement` 开启后，缺失设备 ID、未知设备、撤销设备都被拒绝。
- 下架模型立即生效。
- 余额不足、过期、并发超限、RPM 超限。
- 合法请求继续转发并计费。
- 预扣、实际结算、失败回补和拒绝事件可对账。
- 配置回滚后策略执行结果跟随配置版本变化。

## 交付物

- policy resolver。
- gateway enforcement hooks。
- 网关接口测试。
- usage_event 和 policy_rejection 事件定义。
