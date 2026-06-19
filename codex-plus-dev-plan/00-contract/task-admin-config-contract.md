# 后台配置契约

## 目标

定义后台配置中心需要承载的套餐、模型、用量策略和功能开关字段，使管理员后续能动态调控价格、套餐、模型、默认模型、额度、倍率、限流、设备策略和客户端可见运营入口。

本任务只冻结契约，不实现后台服务、支付开通、网关扣费、客户端展示或管理端表单。

## 涉及项目

- sub2api-main：后续由配置中心、网关 enforcement 和客户端 API 聚合层消费。
- sub2api-main frontend：后续由管理员配置页面消费。
- CodexPlusPlus-main：后续只消费客户端 API 返回的配置快照。

## 输入契约

- 后台配置会被客户端 API 聚合为 bootstrap/config snapshot。
- 网关 enforcement 会读取同一组配置强制执行。
- 配置属于 Control Plane / Config Registry，必须支持版本、生命周期、发布、灰度、回滚和审计字段。

## 共享治理字段

四个配置文档必须使用同一组顶层治理字段：

- `config_version`
- `draft_status`
- `publish_scope`
- `rollback_from`
- `updated_by`
- `updated_at`
- `change_reason`

`config_version` 是客户端快照、网关 enforcement、审计日志和回滚引用的唯一版本锚点。`rollback_from` 在非回滚配置中显式为 `null`，不得省略。

## 配置模型

### PlanCatalog

`codex-plus-contracts/config/plan-catalog.schema.json` 冻结套餐、价格和权益字段：

- 套餐身份：`plan_id`、`name`、`description`、`status`、`is_listed`、`sort_order`。
- 价格：`billing_period`、`currency`、`price_amount_minor`、`display_price`、`external_billing_refs`。
- 权益：`entitlement_grant`、`entitlement_sources`、`model_groups`、`usage_policy_id`。
- 购买/续费入口：`purchase_url`、`renew_url`。
- 购买/续费文案：`copy_keys.purchase_action`、`copy_keys.renew_action`、`copy_keys.upgrade_action`、`copy_keys.not_purchased_message`、`copy_keys.expired_message`、`copy_keys.low_balance_message`。

`price_amount_minor` 只供后端计费和运营审计使用；客户端不得根据价格字段计算展示文案、折扣或套餐状态。客户端只消费聚合后的 display-safe 摘要。

`entitlement_sources` 是旧系统 subscription group、API key group 或 group name 映射到 Codex++ 套餐的唯一桥梁；缺失映射必须 fail closed，不得用第一个启用套餐兜底授权。

### ModelCatalog

`codex-plus-contracts/config/model-catalog.schema.json` 冻结模型目录和模型运营字段：

- 路由与展示：`model_id`、`display_name`、`route_model`、`model_group`、`context_window`。
- 计费策略：`billing_multiplier`。
- 默认模型：`is_default`，每个发布范围必须且只能有一个默认模型。
- 可用性：`is_enabled`、`is_hidden`、`disabled_reason`、`disabled_message_key`。
- 运营生命周期：`rollout_channel`、`quality_tier`、`fallback_model_id`、`deprecation_at`、`disabled_replacement_model_id`。

被禁用的默认模型是配置错误；默认模型必须启用且不可隐藏。`billing_multiplier`、fallback 和 disabled replacement 由后端和网关消费，客户端不得硬编码或自行推断。

### UsagePolicy

`codex-plus-contracts/config/usage-policy.schema.json` 冻结额度、限流、过期行为和设备策略：

- 适用范围：`applies_to.plan_ids`、`applies_to.model_groups`、`applies_to.user_segments`。
- 额度：`low_balance_threshold`、`daily_quota`、`monthly_quota`。
- 限流：`concurrency_limit`、`rpm_limit`、`tpm_limit`、`burst_limit`、`rate_limit_window_seconds`。
- 过期和超额：`expired_behavior`、`grace_period_hours`、`overage_behavior`。
- 客户端可见文案：`copy_keys.*`，全部是 message/action key。
- 设备策略：`device_policy`。

`device_policy` 是设备治理的一等配置位置，包含 `registration_required`、`max_devices_per_user`、`allow_self_service_replacement`、`replacement_cooldown_hours`、`strict_enforcement_default`、`revoke_reason_taxonomy`、`support_unlock_policy`、`revoked_behavior` 和设备相关 `message_keys`。

`FeatureFlags.strict_device_enforcement` 只作为全局 rollout 开关；设备数量、替换冷却、撤销原因和支持解锁规则不得藏在 feature flag 中。

### FeatureFlags

`codex-plus-contracts/config/feature-flags.schema.json` 冻结功能开关：

- `advanced_provider_config`
- `install_assistant`
- `new_user_tutorial`
- `model_selector`
- `diagnostic_export`
- `announcements`
- `force_update_prompt`
- `strict_device_enforcement`

`exposure.client_visible` 定义可进入 bootstrap snapshot 的 flag，`exposure.server_only` 定义仅供后端或网关使用的 flag。`copy_keys` 为强制更新、安装向导、新手教程、诊断导出和公告入口提供稳定文案引用键。

## 文案规则

客户端可见的运营文案采用 key-only 规则：

- schema 中的购买、续费、余额不足、限流、过期、设备撤销、强制更新和功能入口文案都用 `copy_keys` 或 `message_keys`。
- schema 不承载 raw localized text。
- 文案解析由后端 snapshot 聚合层或后续文案配置服务完成；客户端不得硬编码 fallback 文案来替代管理员配置。

## 解耦要求

- 客户端只消费配置快照，不保存或硬编码价格、套餐、模型、默认模型、模型倍率、额度阈值、限流策略、设备策略、续费文案、购买文案或运营提示。
- 后台配置不得依赖客户端组件名、页面结构或本地 Codex 配置路径。
- 配置中心只表达运营规则，不直接写 Codex 本地配置。
- 网关 enforcement 和客户端 usage/bootstrap API 必须读取同一份 policy decision 结果。

## COORDINATOR-PREAUDIT B 项处理

- B1: fixed。四个 schema 使用一致的 `config_version`、`draft_status`、`publish_scope`、`rollback_from`、`updated_by`、`updated_at`、`change_reason`。
- B2: fixed。设备策略冻结在 `UsagePolicy.device_policy`，feature flag 只保留严格执行 rollout 开关。
- B3: fixed。客户端可见运营文案冻结为 key-only，raw localized text 不进入这些 schema。
- B4: fixed。模型运营字段冻结为 rollout channel、quality tier、fallback model、deprecation time、disabled replacement 和 disabled message key。

## 禁止改动范围

- 不实现支付开通。
- 不实现网关扣费。
- 不改客户端展示。
- 不修改 `sub2api-main/**` 或 `CodexPlusPlus-main/**`。

## 测试要求

- 四个 JSON schema 必须可解析。
- 配置字段校验覆盖空模型组、禁用默认模型、负价格、无效倍率、无效限流值和缺失治理字段。
- 配置序列化兼容测试确保旧配置缺字段时由后端迁移或默认值补齐，客户端不自行补业务规则。
- 发布和回滚 schema 测试确保配置版本可追踪、可回滚。

## 交付物

- 配置 schema 文档。
- 默认配置样例。
- 管理端表单字段说明。
- 与 bootstrap 字段的映射表。
- 配置版本和发布治理说明。
