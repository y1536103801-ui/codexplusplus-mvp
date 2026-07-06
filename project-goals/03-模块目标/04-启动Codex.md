启动 Codex
==========

目标
----

让普通用户点击一次即可打开 Codex 并正常使用。

参考项目
--------

本模块必须参考 Codex++ 中适合桌面端复用的本机能力，包括 Tauri 桌面壳、本机 Codex 检测、受控配置清理、启动流程、更新和脱敏诊断。

参考 Codex++ 的结果必须落到真实可用的一键启动流程，不得只做一个没有本机能力支撑的外壳。

首版只要求 Windows 客户端完成本机 Codex 准备和一键启动。

Windows 客户端允许在启动 Codex 前备份并修改本机 Codex 配置。已有本机 ChatGPT 登录态时，客户端保留该登录态并清理 Codex+++ 历史 API-key 状态；没有本机 Codex 登录态时，客户端向后端获取服务端生成的 Codex API key，写入 Codex `auth.json`，并写入 Codex+++ provider 配置。

自动登录必须参考 Codex++ 的 provider 配置边界和 Codex 官方 API-key 登录机制：`config.toml` 写入 Codex+++ 管理的 provider，provider 设置 `requires_openai_auth = true`；`auth.json` 写入 `auth_mode = "apikey"` 和 `OPENAI_API_KEY`。provider 不使用 `auth.command`、`env_key` 或 `experimental_bearer_token`。客户端不得注入 `CODEX_HOME`。客户端显示的 Codex 账号优先来自本机 `~/.codex/auth.json` 中的本机 Codex 账号信息；API-key 登录态显示脱敏 API key；后端号池账号不得作为客户端 Codex 账号显示来源。

必须做到
--------

1，主动作在可用时显示为“启动 Codex”。
2，启动前检查登录状态、Codex+++ token 余额、设备状态和服务状态。
3，启动前由客户端自动清理 Codex+++ 历史遗留本机配置。
4，启动过程不要求普通用户理解配置文件、proxy、endpoint、base_url 或 API Key。
5，启动失败时显示中文提示和下一步动作。
6，Codex+++ token 余额不足时，客户端不额外显示“余额不足”提示；后端不再提供后续 Codex 账号、API Key 或等价可用路由。
7，清理本机配置前创建时间戳备份。
8，使用结构化配置读写，只清理 Codex+++ 管理的键和值。
9，保留用户已有的插件、MCP、features、desktop、memories、hooks 和 projects 等非 Codex+++ 配置。
10，清理失败或配置解析失败时不得继续启动。
11，客户端显示启动状态，至少包括未启动、正在启动、已启动和启动失败。
12，客户端显示的 Codex 账号只能来自本机 Codex 登录态。
13，Codex 未登录时，客户端先向后端获取服务端生成的 Codex API key，再写入 Codex `auth.json` 和 Codex+++ provider 配置。
14，启动失败是普通用户本机行为，客户端只在本机显示中文提示和下一步动作。
15，Windows Store 版 Codex 不得按普通命令行程序处理。
16，`cmd /C start` 或等价 shell 二次解析方式启动 WindowsApps 里的 `Codex.exe` 属于已复现失败路径，不得作为启动方案、重试方案或 fallback。
17，直接 `CreateProcess` 执行 WindowsApps 里的 `Codex.exe` 属于已复现失败路径，不得作为启动方案、重试方案或 fallback。
18，Windows Store 版 Codex 启动方案发布前必须在部署机验证 Codex 安装检测、主界面打开、运行状态刷新、停止状态刷新、Codex+++ provider token 可用和本机 Codex 账号识别。
19，启动失败诊断必须记录可定位的本机失败阶段，不得只保留 `codex_launch_failed` 这类无法区分失败路径的单一结果。

不得做到
--------

1，暴露旧 Codex++ 高级工作台入口。
2，要求普通用户手动选择 provider。
3，要求普通用户手动填写 API Key。
4，要求普通用户手动配置 proxy、endpoint 或 base_url。
5，整文件覆盖本机 Codex 配置。
6，保留 Codex++、sub2api 或历史实现留下的旧配置项作为 fallback。
7，启动 Codex 前读取后端号池账号作为本机 Codex 账号。
8，token 余额不足时自动发起充值、自动联系管理员、自动切换 API Key 或继续提供后续账号。
9，把管理员后台做成监控每个普通用户本机启动过程的面板。
10，把 Windows Store Codex 的 WindowsApps `Codex.exe` 当作普通 exe 反复尝试启动。
11，让普通用户手动处理 `model_provider = "codexppp"` 或 `[model_providers.codexppp]`。
12，使用 provider `auth.command`、`env_key` 或 `experimental_bearer_token` 作为 Codex 登录机制。
13，向 Codex 进程注入 Codex+++ 管理的 `CODEX_HOME`。
