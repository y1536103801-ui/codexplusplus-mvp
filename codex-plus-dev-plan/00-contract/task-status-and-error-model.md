# 状态与错误模型

## 目标

统一客户端、后台 API、网关 enforcement 和管理后台使用的状态枚举、错误码、用户可读提示和日志脱敏规则。

## 涉及项目

- sub2api-main：handler/service/gateway 错误响应
- CodexPlusPlus-main：Tauri 命令结果、Manager UI 状态显示、日志

## 输入契约

- 所有客户端 API 返回 `service.available` 和 `error_code`。
- 客户端只展示后台下发的用户可读提示或本地安装检测提示。

## 输出行为

定义状态：

- `available`
- `not_authenticated`
- `not_purchased`
- `expired`
- `low_balance`
- `disabled`
- `device_revoked`
- `model_unavailable`
- `rate_limited`
- `gateway_unhealthy`
- `local_codex_missing`
- `local_config_failed`

冻结 browser handoff 状态：

- `created`
- `poll_pending`
- `browser_approved`
- `redeemed`
- `expired`
- `consumed`

定义错误码前缀：

- `CLIENT_AUTH_*`
- `CLIENT_ENTITLEMENT_*`
- `CLIENT_DEVICE_*`
- `CLIENT_PROVIDER_*`
- `CLIENT_LOCAL_*`
- `GATEWAY_POLICY_*`

冻结客户端动作枚举：

- `none`
- `sign_in`
- `open_browser_handoff`
- `wait_for_browser_approval`
- `restart_desktop_handoff`
- `purchase`
- `renew`
- `recharge`
- `redeem_code`
- `choose_available_model`
- `retry_request`
- `retry_later`
- `contact_support`
- `install_codex`
- `repair_local_config`

冻结用户消息来源：

- `admin_config_message_key`：购买、续费、充值、低余额、限流、套餐禁用、模型不可用、设备撤销等运营/策略文案由后台配置或管理员配置的 message key/action copy key 驱动，客户端不得硬编码。
- `contract_message_key`：登录、browser handoff、网关临时异常等安全或通用状态使用契约固定 key，客户端可带固定 fallback。
- `desktop_local_diagnostic`：仅用于本机安装、Codex 缺失、本地 provider 写入失败等本机诊断，不得描述套餐、价格、额度或权益规则。
- `support_only_diagnostic`：只进入脱敏日志或客服工具，不得展示为普通用户可见文案。

## 解耦要求

- 用户可读提示由后台配置或错误模型提供，客户端不硬编码运营文案。
- 客户端本地错误只描述本机环境，不描述套餐规则。
- `client_action` 是唯一客户端行为选择器；OpenAPI 中历史字段 `action_hint` 视为同一枚举的别名。
- 错误码必须冻结 HTTP 状态、retryability、`client_action`、`user_message_source` 和日志字段 profile。
- 所有 Key、token、authorization header 必须脱敏。
- 事件和日志不得记录 `access_token`、`refresh_token`、`session_token`、`poll_token`、托管 API key、用户侧 API key、原始 prompt、原始 response、可能包含 prompt/code/output 的 request/response body。
- 事件 metadata 必须使用 allowlist；禁止键包括 `access_token`、`refresh_token`、`session_token`、`poll_token`、`api_key`、`managed_api_key`、`upstream_api_key`、`authorization`、`prompt`、`response`、`raw_prompt`、`raw_response`、`request_body`、`response_body`。

## 覆盖要求

- 购买/未购买：`not_purchased` + `CLIENT_ENTITLEMENT_NOT_PURCHASED` + `purchase`。
- 未登录：`not_authenticated` + `CLIENT_AUTH_NOT_AUTHENTICATED` / `CLIENT_AUTH_TOKEN_EXPIRED` + `sign_in`。
- 过期：`expired` + `CLIENT_ENTITLEMENT_EXPIRED` / `GATEWAY_POLICY_ENTITLEMENT_EXPIRED` + `renew`。
- 余额不足：`low_balance` + `CLIENT_ENTITLEMENT_LOW_BALANCE` / `GATEWAY_POLICY_BALANCE_INSUFFICIENT` + `recharge`。
- 模型不可用：`model_unavailable` + `GATEWAY_POLICY_MODEL_NOT_ALLOWED` + `choose_available_model`。
- 设备撤销：`device_revoked` + `CLIENT_DEVICE_REVOKED` / `CLIENT_DEVICE_BLOCKED` / `GATEWAY_POLICY_DEVICE_REVOKED` + `contact_support`。
- 网关异常：`gateway_unhealthy` + `GATEWAY_POLICY_CONFIG_UNAVAILABLE` / `GATEWAY_POLICY_UPSTREAM_UNHEALTHY` / provider key failures + retry action。
- Browser handoff：`created`、`poll_pending`、`browser_approved`、`redeemed`、`expired`、`consumed`，且事件不得记录 handoff token 或桌面 token。

## 禁止改动范围

- 不改具体 API 实现。
- 不改支付或套餐逻辑。
- 不改 UI 视觉样式。

## 测试要求

- 每个错误码有对应 HTTP 状态和用户提示。
- 日志脱敏测试覆盖 `sk-*`、JWT、Authorization、Base URL query token。

## 交付物

- 状态枚举表。
- 错误码表。
- 用户提示示例。
- 日志脱敏规则。
- 事件 schema metadata allowlist。
- COORDINATOR-PREAUDIT C1-C4 的 fixed/deferred/rejected 决策。

## Worker C 冻结决策

| 预审项 | 决策 | 证据 |
| --- | --- | --- |
| C1 | fixed | `client-status-errors.md` 增加 `handoff_state` 状态机；`client-events.schema.json` 对 `desktop_login_started/completed/polled` 增加条件约束。 |
| C2 | fixed | `client-events.schema.json` 的 `metadata` 改为 `additionalProperties: false` allowlist，并显式禁止 token、API key、authorization、prompt/response/body 键。 |
| C3 | fixed | `client-status-errors.md` 为每个错误码冻结 `client_action`；本任务文档声明 OpenAPI `action_hint` 是同一枚举别名。 |
| C4 | fixed | `client-status-errors.md` 和本任务文档冻结 `user_message_source` 四类边界，区分 admin config message key、契约固定 key、本地诊断和 support-only 诊断。 |
