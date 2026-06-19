# 错误映射与日志脱敏

## 目标

统一 Codex++ 云绑定相关错误映射，并确保日志、诊断导出和 UI notice 不泄露敏感凭证。

## 涉及项目

- CodexPlusPlus-main：`crates/codex-plus-core/src/diagnostic_log.rs`
- CodexPlusPlus-main：`apps/codex-plus-manager/src-tauri/src/commands.rs`
- CodexPlusPlus-main：Manager notice/actions

## 输入契约

- `00-contract` 错误码。
- bootstrap 和 usage API 错误响应。

## 输出行为

把错误映射为：

- 用户可读提示。
- 技术诊断摘要。
- 可恢复动作：重新登录、续费、重新检测、导出日志、联系支持。

## 解耦要求

- 运营文案优先以后端 `message` 为准。
- 客户端只补充本地安装和网络错误提示。
- 客户端不得硬编码套餐、价格、模型倍率、额度阈值、限流阈值或续费文案。
- 与购买、续费、用量和模型权限相关的提示必须来自 bootstrap、usage 或后台配置快照。
- 所有 token/key/header 脱敏。

## 禁止改动范围

- 不改后台错误码定义。
- 不改 UI 视觉布局。
- 不改支付逻辑。

## 测试要求

- 脱敏覆盖 `sk-*`、JWT、Authorization、URL token。
- 每个后端错误码都有本地 fallback。
- 诊断导出不含完整 Key。

## 交付物

- 错误映射表。
- redaction utility。
- 日志脱敏测试。
