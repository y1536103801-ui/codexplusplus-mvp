# 兼容性与迁移

## 目标

确保新托管模式不会破坏现有 Codex++ 用户的手动供应商配置、增强功能、日志、更新和安装入口。

## 涉及项目

- CodexPlusPlus-main：settings、relay config、manager UI
- CodexPlusPlus-main：launcher/update/install

## 输入契约

- 旧版 relayProfiles/settings。
- 新版 `Codex++ Cloud` 托管供应商。

## 输出行为

升级后：

- 旧手动供应商保留。
- 普通用户默认显示托管首页。
- 高级用户可进入供应商配置。
- 清除云登录不删除手动供应商。

## 解耦要求

- 迁移不写入套餐、价格、倍率和用量规则。
- 托管供应商只保存运行必要配置。

## 禁止改动范围

- 不重构完整供应商系统。
- 不删除历史配置。
- 不改 Codex 官方文件。

## 测试要求

- 旧设置升级。
- 云登录退出。
- 手动供应商切换。
- provider sync 不被破坏。

## 交付物

- 迁移说明。
- 兼容测试。
- 回滚说明。
- 可复跑的兼容证据生成、provider 快照检查和校验脚本：`tools/new-07-compatibility-evidence.ps1`、`tools/inspect-07-compatibility-snapshots.ps1`、`tools/verify-07-compatibility-evidence.ps1`。
