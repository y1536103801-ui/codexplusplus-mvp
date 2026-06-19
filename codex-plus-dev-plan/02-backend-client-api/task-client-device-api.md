# Client Device API

## 目标

实现 `POST /api/v1/client/devices`，记录 Codex++ 桌面设备，用于后台查看、撤销、风控和售后定位。

## 涉及项目

- sub2api-main：backend entity/schema 或现有 user attribute 扩展
- sub2api-main：client handler/service
- sub2api-main：admin 后续查看接口

## 输入契约

- 用户 JWT。
- `device_id`、`platform`、`app_version`、`codex_version`、`last_seen_at`。

## 输出行为

同一用户同一 `device_id` 幂等 upsert，返回设备状态：`active`、`revoked`、`blocked`。

## 解耦要求

- MVP 不在客户端硬编码设备数量限制。
- 设备撤销由后台返回状态，客户端只展示和停止自动配置。

## 禁止改动范围

- 不实现管理员页面。
- 不改变登录鉴权机制。
- 不改 API Key 生成逻辑。

## 测试要求

- upsert 幂等。
- 设备归属用户隔离。
- revoked 设备调用 bootstrap 时返回 `device_revoked`。

## 交付物

- 设备存储或属性模型。
- client device API。
- 单元测试。

