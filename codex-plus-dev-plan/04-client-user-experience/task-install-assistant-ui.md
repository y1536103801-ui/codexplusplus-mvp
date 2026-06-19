# 安装辅助 UI

## 目标

为不会安装 Codex 的用户提供首启检测、下载指引、安装说明、权限修复和重新检测。

## 涉及项目

- CodexPlusPlus-main：`apps/codex-plus-manager/src/App.tsx`
- CodexPlusPlus-main：`crates/codex-plus-core/src/app_paths.rs`
- CodexPlusPlus-main：`crates/codex-plus-core/src/install`

## 输入契约

- 本地 Codex App 检测结果。
- 平台信息：Windows/macOS x64/arm64。
- feature flag：`install_assistant`。

## 输出行为

安装辅助展示：

- Codex 是否已安装。
- 未安装时的下载和安装步骤。
- 已安装但路径异常时的一键选择/修复。
- 权限、Gatekeeper、防火墙等常见问题说明。
- 安装完成后重新检测。

## 解耦要求

- 下载入口和帮助文案可由后台公告/feature payload 后续覆盖。
- 客户端只保留本地检测逻辑。
- 客户端不得硬编码套餐、价格、模型倍率、额度阈值、限流阈值或续费文案。
- 安装辅助只处理本地 Codex 可用性，不承担购买、模型权限、用量策略或续费策略来源职责。

## 禁止改动范围

- 不下载或安装第三方软件，除非用户明确点击。
- 不改静默启动器逻辑。
- 不改云服务 API。

## 测试要求

- 未安装、路径错误、已安装、权限不足。
- Windows/macOS 文案分支。
- feature flag 关闭后入口隐藏。

## 交付物

- 安装辅助视图。
- 本地检测命令接入。
- 使用说明。
