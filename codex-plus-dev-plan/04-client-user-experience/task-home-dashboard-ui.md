# 首页仪表盘 UI

## 目标

实现普通用户首页，展示服务状态、套餐、余额、今日用量、当前模型、一键启动和诊断修复。

## 涉及项目

- CodexPlusPlus-main：`apps/codex-plus-manager/src/App.tsx`
- CodexPlusPlus-main：`apps/codex-plus-manager/src/styles.css`

## 输入契约

- `cloud_bootstrap_status`
- `cloud_refresh_bootstrap`
- `GET /api/v1/client/usage`
- feature flags。

## 输出行为

首页展示：

- 登录状态。
- 服务可用性。
- 套餐名称和到期时间。
- 余额/今日用量。
- 后台返回的默认模型。
- 一键启动 Codex。
- 续费/修复/导出日志入口。

## 解耦要求

- 不硬编码套餐名称、价格、额度阈值、模型名和续费文案。
- 所有运营内容来自 bootstrap/usage。
- 客户端不得硬编码套餐、价格、模型倍率、额度阈值、限流阈值或续费文案。
- feature flags、默认模型、续费入口、用量提示和低余额提示必须由 bootstrap/usage 或后台配置快照驱动。

## 禁止改动范围

- 不重写 relay detail editor。
- 不改 Tauri 后端业务逻辑。
- 不删除高级配置入口。

## 测试要求

- 登录前、可用、低余额、过期、禁用、网络失败。
- feature flag 关闭高级配置时入口隐藏。
- 文本在移动和桌面视口不溢出。

## 交付物

- 首页组件。
- 状态视图。
- UI 测试或截图说明。
