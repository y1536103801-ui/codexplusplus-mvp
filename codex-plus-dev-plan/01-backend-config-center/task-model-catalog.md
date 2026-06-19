# 模型目录 Model Catalog

## Worker M 实现状态

- Status: implemented in `sub2api-main/backend/internal/codexplus/configregistry/model_catalog.go`.
- Tests: added in `sub2api-main/backend/internal/codexplus/configregistry/model_catalog_test.go`.
- Scope note: this worker only adds the config registry model catalog; shared service integration, bootstrap filtering, and gateway enforcement remain for the coordinator / later stage wiring.

## 目标

实现后台可配置的模型目录，支持管理员调整展示模型、真实路由模型、默认模型、倍率和上下文窗口。

## 涉及项目

- sub2api-main：`backend/internal/service/upstream_models.go`
- sub2api-main：`backend/resources/model-pricing`
- sub2api-main：admin setting/channel 相关 handler

## 输入契约

- 使用 `ModelCatalog` schema。
- bootstrap 只返回用户可用模型列表。
- 网关 enforcement 使用同一模型权限结果。

## 输出行为

管理员可配置：

- 模型 ID、展示名、badge。
- 真实路由模型。
- 所属模型组。
- 上下文窗口、倍率。
- 是否默认、是否开放、是否隐藏。

## 解耦要求

- 客户端不硬编码模型列表、默认模型、倍率或上下文窗口。
- 客户端也不得从 `model_id`、`route_model`、`quality_tier` 或任何本地列表推断可见模型、默认模型、计费倍率、上下文窗口、下架替代模型或运营文案；这些值必须来自后续客户端快照 API。
- 下架模型后客户端列表消失，网关同时拒绝请求。

## Worker M 校验输出

- 类型覆盖 `model_id`、`display_name`、`route_model`、`model_group`、`context_window`、`billing_multiplier`、`is_default`、`is_enabled`、`is_hidden`、`disabled_reason`、`rollout_channel`、`quality_tier`、`fallback_model_id`、`deprecation_at`、`disabled_replacement_model_id`、`disabled_message_key`、`sort_order` 和 `operator_tags`。
- 默认/样例构造：`DefaultModelCatalog()` 和 `SampleModelCatalog()`。
- 校验入口：`ValidateModelCatalog()`，包含重复 `model_id`、缺 route/model group、context 太小、倍率非正、默认模型唯一且 enabled / non-hidden、deprecated 必须带 `deprecation_at`、disabled 必须带 reason/message key，以及 fallback / disabled replacement 引用存在性。
- 模型组解析 helper：`ModelsByGroup()`；后续聚合可用 `DefaultModel()`、`ModelByID()`、`EnabledModels()`、`VisibleEnabledModels()`。

## 禁止改动范围

- 不改 Codex++ 本地模型 catalog。
- 不改支付套餐逻辑。
- 不改具体上游账号导入流程。

## 测试要求

- 默认模型被下架时必须返回配置错误。
- 用户请求未授权模型时网关返回 `GATEWAY_POLICY_MODEL_DENIED`。
- bootstrap 只返回当前用户套餐允许的模型。

## 交付物

- ModelCatalog 配置服务。
- 模型组解析逻辑。
- 默认模型校验。
- 单元测试和 mock 数据。
