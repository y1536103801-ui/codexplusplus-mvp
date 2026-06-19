# 管理员套餐管理

## 目标

实现管理员管理 Codex++ 套餐、价格、周期、额度和可用模型组。

## 涉及项目

- sub2api-main backend：admin setting/payment config handler
- sub2api-main frontend：admin settings/payment views

## 输入契约

- PlanCatalog 配置服务。
- Payment entitlement flow 后续消费套餐 ID。

## 输出行为

管理员可：

- 新增/编辑/下架套餐。
- 调整价格、币种、周期。
- 绑定模型组和默认续费入口。
- 绑定权益来源：subscription group、API key group 或 group name 到 Codex++ 套餐。
- 预览用户侧展示摘要。

## 解耦要求

- 客户端只展示当前用户 plan 快照。
- 价格展示由后台返回，不由客户端计算。
- 管理端只写 `entitlement_sources`，网关按该配置执行；不得通过客户端文案或套餐顺序推断授权。

## 禁止改动范围

- 不改网关扣费。
- 不改客户端 UI。
- 不实现支付回调。

## 测试要求

- 下架套餐不可购买但历史用户仍可识别。
- 修改价格后客户端 bootstrap 文案更新。
- 无权益来源映射时，托管 Key 不能被默认授予套餐。
- 修改 group 映射后，网关模型权限随配置版本变化。

## 交付物

- 管理接口。
- 管理页面字段。
- 配置测试。
