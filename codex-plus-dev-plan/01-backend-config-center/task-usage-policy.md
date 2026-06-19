# 用量策略 Usage Policy

Status: implemented by worker U for 01-backend-config-center.

## 目标

实现后台可配置的用量策略，让管理员动态调整余额提醒、每日额度、并发、RPM、TPM 和套餐过期行为。

## 涉及项目

- sub2api-main：`backend/internal/service/billing*`
- sub2api-main：`backend/internal/service/ratelimit_service.go`
- sub2api-main：`backend/internal/service/concurrency*`
- sub2api-main：`backend/internal/service/user_group_rate*`

## 输入契约

- 使用 `UsagePolicy` schema。
- usage API 和 gateway enforcement 消费同一策略结果。

## 输出行为

管理员可配置：

- 余额低阈值。
- 每日 token 或金额上限。
- 并发上限。
- RPM、TPM。
- 套餐过期后策略。
- 余额不足时用户提示。

## 解耦要求

- 客户端不计算 low balance、不判断过期策略、不限制模型请求。
- 网关必须强制执行策略。

## Worker U 实现记录

- 新增 additive `configregistry` Usage Policy 类型，不改现有共享入口。
- `DefaultUsagePolicyCatalog` 和 `SampleUsagePolicyCatalog` 均按冻结 schema 构造完整治理字段、策略字段、copy keys 和 device policy。
- `ValidateUsagePolicyCatalog` 覆盖 `policy_id` 唯一性、治理字段、适用范围 ID、low balance、daily/monthly quota、concurrency、RPM、TPM、burst/window limits、expired/overage behavior、copy keys、device policy 和 message keys。
- 客户端仍只消费后续聚合快照：不计算 low balance、不判断过期策略、不限制模型请求、不把 copy key 替换为硬编码文案。
- 网关 enforcement 后续必须消费同一份 policy decision，并强制执行额度、并发、RPM、TPM、burst/window、过期、超额和设备撤销策略。

## 禁止改动范围

- 不改套餐目录字段。
- 不改客户端状态 UI。
- 不改支付回调。

## 测试要求

- 策略变更后 gateway enforcement 立即使用新值。
- usage API 返回后台策略下的可读状态。
- 余额不足和过期策略覆盖测试。

## 交付物

- UsagePolicy 配置服务。
- 与现有限流/计费服务的读取接口。
- 策略校验测试。

## Worker U 交付物

- `sub2api-main/backend/internal/codexplus/configregistry/usage_policy.go`
- `sub2api-main/backend/internal/codexplus/configregistry/usage_policy_test.go`
- `codex-plus-dev-plan/01-backend-config-center/reports/worker-usage-policy-final.md`
