# 04-client-user-experience 客户端用户体验阶段

## 当前状态

- Status: passed
- Previous gate: `03-client-cloud-core` passed。
- Current gate: Cloud home、登录绑定、安装辅助和新手教程已按本阶段任务文件实现并通过 coordinator 验证。
- Next gate: `05-admin-operations` can become active。
- Blocked stages: `06-commerce-and-enforcement` 到 `07-integration-release` remain blocked until their prior stages pass。

## 本阶段目标

把 Codex++ Manager 改造成普通用户可理解的买完即用入口，同时保留高级供应商配置。重点解决不会安装 Codex、不会使用 Codex、不会配置 API 的问题。

## 工业级 v2 映射

- 架构层：`Client Runtime / UX Shell`。
- 覆盖范围：首页、登录绑定、安装辅助、新手教学、状态提示、高级配置入口和降级展示。
- 关键原则：UI 只展示服务端返回的用户态状态和文案，不内置销售规则。

## 并行任务列表

- `task-home-dashboard-ui.md`
- `task-login-binding-ui.md`
- `task-install-assistant-ui.md`
- `task-new-user-tutorial-ui.md`

## 前置依赖

- `03-client-cloud-core` 提供 Tauri 命令或 mock。
- `02-backend-client-api` 提供 bootstrap/usage。

## 合并顺序

1. login binding UI。
2. home dashboard UI。
3. install assistant UI。
4. new user tutorial UI。

## 阶段验收标准

- 普通用户无需进入供应商配置即可启动 Codex。
- 未安装 Codex 时能看到安装辅助。
- 首次使用有任务示例和结果确认教学。
- 高级配置默认隐藏但可进入。
- 余额不足、套餐过期、设备撤销、模型不可用等状态与后端错误模型一致。
- 续费入口、模型展示名、额度提示和公告文案来自 bootstrap 或 usage API。
