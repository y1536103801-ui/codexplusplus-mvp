# 客户端 API 契约

## 目标

定义 Codex++ 客户端专用 API，冻结请求、响应、鉴权、错误码和 mock 数据，供后台和客户端并行实现。

## 涉及项目

- sub2api-main：`backend/internal/server/routes`、`backend/internal/handler`、`backend/internal/service`
- CodexPlusPlus-main：后续由客户端 bootstrap consumer 消费本契约

## 输入契约

- 使用现有用户 JWT 鉴权。
- 所有 MVP 响应 fixture 使用双兼容 envelope：`code`、`status`、`message`、`reason`、`error_code`、`data`。成功响应为 `code=0`、`status=success`、`reason=null`、`error_code=null`；错误响应为 `status=error`，且 `reason` 必须等于 `error_code` 以兼容 legacy Sub2API unwrap。
- 客户端只消费聚合快照，不理解后台价格、倍率、模型分组和限流规则。
- 每个快照必须包含 `config_version`、`snapshot_version` 和兼容策略字段，便于灰度、回滚和客户端刷新。
- 客户端不得硬编码价格、套餐、模型倍率、额度阈值、限流策略、续费/购买文案或购买/续费按钮文案；这些只能来自 `bootstrap`/配置快照中的 display 字段、policy decision 和 `message_key` / `action_copy_key`。

## 输出行为

冻结以下接口：

- `POST /api/v1/auth/desktop/start`
- `POST /api/v1/auth/desktop/complete`
- `POST /api/v1/auth/desktop/poll`
- `GET /api/v1/client/bootstrap`
- `GET /api/v1/client/usage`
- `POST /api/v1/client/devices`
- `POST /api/v1/client/redeem`

`auth/desktop` 必须采用 browser handoff：

- `start` 由桌面端调用，返回 `session_token`、桌面私有 `poll_token`、`authorize_url`、6 位 `verification_code` 和过期时间。
- `complete` 必须由已登录的浏览器 Web JWT 调用，只返回 `status=completed`，不得返回桌面 access token 或 refresh token。
- `poll` 由桌面端携带 `session_token + poll_token` 轮询；完成后一次性消费 pending session，再返回桌面 token pair。
- `authorize_url` 只能包含 `session_token` 和确认码；不得包含 `poll_token`、JWT、API Key 或上游凭证。
- `poll` 完成态的 `user` 固定为最小对象：`id`、`username`、`email`、`display_name`、`role`。`user` 对象禁止嵌套 token、API key、poll token、session token 或任何其他 secret。

`bootstrap` 必须返回 `service`、`provider`、`plan`、`models`、`usage`、`feature_flags`、`announcements`、`version_policy`。

`feature_flags` 必须与配置 schema 的 user-safe 快照对齐，包含：

- `advanced_provider_config`
- `install_assistant`
- `new_user_tutorial`
- `model_selector`
- `diagnostic_export`
- `announcements`
- `force_update_prompt`
- `strict_device_enforcement`

`plan.commerce_action` 与 `usage.renew_action` 必须包含 `message_key` 和 `action_copy_key`，同时可带服务端按当前 locale 解析后的 `label`。客户端只能展示服务端返回的文案，不得内置续费、购买、充值或管理套餐按钮文案。

`POST /api/v1/client/devices` 的请求字段 `codex_version` 是必填 key。客户端检测到本机 Codex 时必须传版本字符串；本机 Codex 缺失或探测失败时可传 `null`，由后端映射到本地状态或诊断流程。

同时定义客户端 API 事件字段：

- `bootstrap_requested`
- `usage_viewed`
- `device_registered`
- `redeem_attempted`
- `desktop_login_started`
- `desktop_login_completed`
- `desktop_login_polled`

事件必须能关联用户、设备、配置版本、请求 ID 和错误码。

## 解耦要求

- `models.available` 来自后台模型配置和用户权益聚合结果。
- `usage.low_balance` 和 `service.message` 来自后台策略，不由客户端计算。
- `provider.api_key` 是 Sub2API 用户侧 Key，不是上游真实凭证。
- `poll_token` 只允许存入桌面本地 pending session，不允许进入 UI 状态、授权 URL、日志或诊断导出。
- `usage.low_balance`、`rate_limit_state`、`service.status`、`service.message`、`service.message_key`、`plan.commerce_action` 和 `usage.renew_action` 都是后台配置与策略聚合结果，客户端不得自行计算或替换。

## 禁止改动范围

- 不实现具体后台业务逻辑。
- 不改 Codex++ 客户端 UI。
- 不决定价格、模型倍率或套餐规则，只定义字段。

## 测试要求

- 提供每个接口的成功 mock、未登录 mock、套餐过期 mock、余额不足 mock、模型不可用 mock。
- mock 字段必须可被 TypeScript/Rust 生成静态类型。
- mock 必须覆盖配置版本变化、设备撤销、模型下架和网关策略拒绝。
- browser handoff mock 必须覆盖 start、complete、poll pending、poll completed，并验证 completed poll 前不会向浏览器返回桌面 token。
- `/usage`、`/devices`、`/redeem` 至少提供成功响应 fixture，错误响应走统一 envelope 与状态错误表。

## 预审决策

| Item | Decision | Evidence |
| --- | --- | --- |
| A1 | fixed | OpenAPI `EnvelopeBase` 和所有 client fixtures 固定双兼容 envelope，新 fixture 同时包含 `code/status/message/reason/error_code/data`。 |
| A2 | fixed | `DesktopLoginPollResult.user` 改为 `DesktopLoginUser` 最小对象，并禁止 additional properties。 |
| A3 | fixed | `FeatureFlagSnapshot` 暴露配置 schema 中的 user-safe flags：`announcements`、`force_update_prompt`、`strict_device_enforcement` 已加入必填字段。 |
| A4 | fixed | `DeviceRegisterRequest.codex_version` 改为必填 key；仅本机 Codex 缺失或探测失败时允许值为 `null`。 |
| A5 | fixed | `plan.commerce_action` 和 `usage.renew_action` 固定 `message_key` / `action_copy_key`，续费、购买、充值和管理套餐文案由快照提供。 |

## 交付物

- API 字段表。
- JSON mock 响应。
- 错误码引用表。
- 接口鉴权说明。
- 事件 schema 和兼容策略说明。
