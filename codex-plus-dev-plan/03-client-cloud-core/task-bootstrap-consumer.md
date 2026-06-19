# Bootstrap Consumer

## 目标

在 Codex++ Rust/Tauri 层实现 bootstrap 调用，把 Sub2API 返回的快照转成客户端内部状态。

## 涉及项目

- CodexPlusPlus-main：`crates/codex-plus-core/src/http_client.rs`
- CodexPlusPlus-main：新增 cloud/bootstrap 模块
- CodexPlusPlus-main：`apps/codex-plus-manager/src-tauri/src/commands.rs`

## 输入契约

- `GET /api/v1/client/bootstrap`。
- 本地登录态。
- 设备 ID。

## 输出行为

提供 Tauri 命令：

- `cloud_bootstrap_status`
- `cloud_refresh_bootstrap`
- `cloud_apply_managed_provider`

命令返回服务状态、用户提示、可用模型、余额摘要和 feature flags。

## 解耦要求

- 不解析价格、倍率、套餐规则。
- 只消费 bootstrap 聚合快照。
- 模型列表和默认模型完全以后端返回为准。
- 客户端不得硬编码套餐、价格、模型倍率、额度阈值、限流阈值或续费文案。
- feature flags、可用模型、余额摘要和用户行动提示必须由 bootstrap/usage 快照驱动。

## 禁止改动范围

- 不改 Manager UI 页面结构。
- 不改现有 relay profile 手动编辑逻辑。
- 不改 Sub2API。

## 测试要求

- mock bootstrap 成功、401、过期、余额不足、设备撤销、网关异常。
- 响应缺可选字段时有安全降级。

## 交付物

- Rust 类型。
- Tauri 命令。
- mock 测试。
