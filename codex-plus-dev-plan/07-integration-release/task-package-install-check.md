# 安装包与入口检查

## 目标

检查 Windows 和 macOS 安装包、快捷方式、静默启动器、Manager 入口和安装辅助路径。

## 涉及项目

- CodexPlusPlus-main：Tauri build
- CodexPlusPlus-main：`crates/codex-plus-core/src/install`
- CodexPlusPlus-main：release assets workflow

## 输入契约

- 新版 Manager。
- 安装辅助 UI。
- 自动更新策略。

## 输出行为

确认：

- Windows 安装包创建桌面和开始菜单入口。
- macOS x64/arm64 DMG 可安装。
- 静默启动器仍只启动 Codex 并注入增强。
- Manager 可进入登录、安装辅助、诊断和高级配置。

## 解耦要求

- 安装包不内置共享 Key。
- 安装包不内置价格、套餐或固定模型策略。

## 禁止改动范围

- 不修改支付/后台。
- 不在安装脚本中写用户凭证。

## 测试要求

- 全新安装。
- 覆盖安装。
- 卸载后重装。
- 未安装 Codex 时的首启向导。

## 交付物

- 安装包检查清单。
- 平台测试记录。
- 发布前阻断问题列表。
- 可复跑的平台证据生成、artifact inspection 和校验脚本：`tools/new-07-package-evidence.ps1`、`tools/inspect-07-package-artifacts.ps1`、`tools/verify-07-package-evidence.ps1`。
