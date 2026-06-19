# 功能开关 Feature Flags

## 目标

实现后台可配置的客户端功能开关，用于控制 Codex++ 普通用户首页、高级配置、安装向导、新手教程和公告入口。

## 涉及项目

- sub2api-main：`backend/internal/service/setting_service.go`
- sub2api-main：`backend/internal/handler/setting_handler.go`
- CodexPlusPlus-main：后续 bootstrap consumer 只消费聚合结果

## 输入契约

- 使用 `FeatureFlags` schema。
- bootstrap 返回当前用户可用的功能开关。

## 输出行为

管理员可配置：

- `advanced_provider_config`
- `install_assistant`
- `new_user_tutorial`
- `model_selector`
- `diagnostic_export`
- `announcements`
- `force_update_prompt`
- `strict_device_enforcement`

## 解耦要求

- 客户端只根据 feature flag 显示或隐藏入口。
- 功能开关不包含价格、模型、额度和权限判断。
- `strict_device_enforcement` 是服务端网关执行开关，不是 UI 开关；客户端不得通过本地状态关闭设备校验。
- Feature Flags 只表达入口可见性和服务端 rollout 状态；价格由 PlanCatalog，模型由 ModelCatalog，额度/设备策略由 UsagePolicy，权限/权益判断由后端聚合和网关执行。
- 客户端消费聚合后的只读快照时只能显示或隐藏入口，不得用本地 feature flag 计算购买状态、模型可用性、额度阈值、套餐权限或设备放行结果。
- `diagnostic_export=true` 只有在导出路径已满足 redaction-ready 语义时才允许发布；导出内容必须先完成敏感信息脱敏，再进入用户可复制或支持可见路径。
- `strict_device_enforcement` 仅用于服务端/网关将设备策略从兼容模式切到严格执行；开启后缺失、撤销或未知设备的拒绝由后续网关执行层处理。

## 禁止改动范围

- 不实现客户端 UI。
- 不实现版本更新逻辑。
- 不改支付。
- 不在客户端实现网关拒绝逻辑。

## 测试要求

- 缺省配置时返回安全默认值：高级配置隐藏、安装向导开启、新手教程开启。
- `strict_device_enforcement=false` 时保持兼容；开启后缺失、撤销或未知设备必须被网关拒绝。
- 用户组或套餐覆盖优先级测试。

## 交付物

- FeatureFlags 配置服务。
- 默认开关配置。
- bootstrap 映射说明。

## Worker F 实现记录

- Added additive config registry types in `sub2api-main/backend/internal/codexplus/configregistry/feature_flags.go`.
- `DefaultFeatureFlags` uses safe defaults: `advanced_provider_config=false`, `install_assistant=true`, `new_user_tutorial=true`, `force_update_prompt=false`, `strict_device_enforcement=false`.
- `SampleFeatureFlags` provides a reviewable draft-like example that enables advanced provider config, force update prompt, and strict device enforcement while keeping exposure boundaries valid.
- `ValidateFeatureFlags` validates schema metadata, the eight frozen flag names, complete exposure partitioning, copy key presence/patterns, redaction-ready diagnostics semantics, and server-only strict device enforcement.
- Tests in `feature_flags_test.go` cover all eight flags, exposure overlap/missing/unknown values, copy key missing/invalid cases, diagnostic export redaction readiness, strict device server-only behavior, and required JSON flag fields.
