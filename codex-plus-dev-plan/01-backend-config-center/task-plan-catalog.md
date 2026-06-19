# 套餐目录 Plan Catalog

## 目标

实现可后台配置的套餐目录，用于定义价格、周期、权益、续费入口和可用模型组。

## 涉及项目

- sub2api-main：`backend/internal/service/setting_service.go`
- sub2api-main：`backend/internal/handler/admin/setting_handler.go`
- sub2api-main frontend：管理员设置页面后续扩展

## 输入契约

- 使用 `PlanCatalog` schema。
- 套餐结果会被 payment entitlement 和 bootstrap 聚合消费。
- 套餐目录属于 `Control Plane / Config Registry`，发布后由策略决策模块编译为可执行权益策略。

## 输出行为

管理员可配置：

- 套餐 ID、名称、描述。
- 价格、币种、周期。
- 初始余额或额度。
- 可用模型组。
- 权益来源映射：subscription group、API key group 或 group name 到 Codex++ 套餐。
- 续费 URL 或购买入口。
- 是否上架。
- 配置版本、发布范围、灰度比例、回滚来源和变更原因。

## 解耦要求

- 客户端只展示 bootstrap 下发的当前套餐名称、到期时间和续费入口。
- 价格、折扣、周期计算只在后台和支付侧处理。
- 套餐变更不得直接写客户端状态，必须通过配置版本和 bootstrap 快照生效。
- 网关模型权限必须通过 `entitlement_sources` 映射得到套餐，不允许在无映射时自动授予默认套餐。

## 禁止改动范围

- 不改支付订单流程。
- 不改网关扣费。
- 不改客户端页面。

## 测试要求

- 无效价格、重复套餐 ID、空模型组、下架套餐不可购买。
- 老配置缺字段时返回后端默认值。
- 无 `entitlement_sources` 映射的托管 Key 请求必须进入 `not_purchased` 或等价未授权状态。
- 套餐发布、灰度、回滚和下架后 bootstrap 变化可验证。

## 交付物

- PlanCatalog 存储与读写服务。
- 管理接口或设置接口。
- 默认套餐配置样例。
- 单元测试。
- 配置版本审计记录。

## Worker P 实现记录

- 已在 `sub2api-main/backend/internal/codexplus/configregistry/plan_catalog.go` 新增 additive Plan Catalog 类型、默认样例构造 `DefaultPlanCatalog` 与 `ValidatePlanCatalog`。
- 校验覆盖 `price_amount_minor`/`display_price`、`billing_period`、`entitlement_grant`、`entitlement_sources`、`usage_policy_id`、`model_groups`、购买/续费 URL、`copy_keys`、`status`/`is_listed`。
- 单元测试覆盖重复 `plan_id`、空 `model_groups`、负权益、无效 entitlement source、无效 status、缺 copy key、缺 usage policy id、无效价格/展示价/周期/购买 URL 和下架套餐不可购买。
- 客户端不得硬编码价格、套餐、计费周期、续费入口、购买入口、续费/购买按钮文案或未购买/过期/低余额提示文案；这些值必须来自后端发布的 Plan Catalog 经后续 bootstrap 聚合后的快照，客户端只消费 copy key 和后端展示字段。
- 本 worker 未接入 `codexplus_config_service.go`，后续由主会话统一完成配置中心服务聚合。
