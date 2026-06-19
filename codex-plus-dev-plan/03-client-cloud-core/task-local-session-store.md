# 本地登录态与设备存储

## 目标

实现 Codex++ 本地云服务登录态、设备 ID、bootstrap 缓存和过期信息存储。

## 涉及项目

- CodexPlusPlus-main：`crates/codex-plus-core/src/settings.rs`
- CodexPlusPlus-main：`crates/codex-plus-core/src/paths.rs`
- CodexPlusPlus-main：`apps/codex-plus-manager/src-tauri/src/commands.rs`

## 输入契约

- 登录成功后的用户 JWT 或 refresh token。
- 设备 ID。
- bootstrap cache。

## 输出行为

客户端可：

- 保存和读取登录态。
- 生成稳定设备 ID。
- 记录最近一次 bootstrap 快照和更新时间。
- 退出登录时清除云服务登录态和托管 Key。

## 解耦要求

- 本地缓存只用于离线展示和故障提示，不作为权限来源。
- 额度、套餐、模型权限必须每次以后台为准。
- 客户端不得硬编码套餐、价格、模型倍率、额度阈值、限流阈值或续费文案。
- 本地缓存不得覆盖后台配置版本、权益状态、模型权限或续费入口。

## 禁止改动范围

- 不改 Codex 官方 auth.json 登录状态。
- 不保存上游真实账号凭证。
- 不实现支付或注册页面。

## 测试要求

- 登录态读写。
- 退出登录清理。
- 设备 ID 稳定。
- 缓存过期后强制刷新。

## 交付物

- 本地存储结构。
- Tauri 读写命令。
- 单元测试。
