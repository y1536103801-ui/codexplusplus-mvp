# 设备管理

## 目标

实现管理员对 Codex++ 设备的查看、撤销和恢复，并让客户端 bootstrap 感知设备状态。

## 涉及项目

- sub2api-main：client device service
- sub2api-main：admin user/device handler
- CodexPlusPlus-main：bootstrap 错误状态消费

## 输入契约

- `DesktopDevice`。
- `POST /api/v1/client/devices`。
- `device_revoked` 状态。

## 输出行为

管理员可：

- 查看设备列表。
- 撤销设备。
- 恢复设备。
- 查看最后活跃时间、版本、平台。

设备被撤销后，bootstrap 返回不可用状态，客户端停止应用托管配置。

## 解耦要求

- MVP 不硬编码设备数限制。
- 设备状态由后台控制。

## 禁止改动范围

- 不改登录体系。
- 不实现复杂设备指纹。
- 不改支付权益。

## 测试要求

- 撤销后 bootstrap 拒绝。
- 恢复后可重新拉取配置。
- 管理员权限隔离。

## 交付物

- admin device API。
- 设备状态测试。
- 管理页面接入说明。

