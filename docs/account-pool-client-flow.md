号池到客户端链路
================

适用范围
--------

本文档记录 Codex+++ 中 Codex 上游账号载入号池、后端网关调度和 Windows/macOS 客户端消费号池的产品规则。

同类项目链路
------------

已确认的同类端到端形态包括：

1，Codex Pooler：上游账号、Pool API Key、Codex backend 入口、OpenAI-compatible 入口、会话连续性和健康调度。
2，codex-lb：ChatGPT 账号池、Dashboard、API Key、Codex CLI、OpenCode 和 SDK 客户端入口。
3，codex2api：Refresh Token / Access Token 账号池、Scheduler、API Keys、后台页面和 `/v1/responses` 入口。
4，Subrouter：Codex 账号仓库、远程 server、sticky session、用量调度和本机 Codex wrapper。
5，darvell/codex-pool：多个 `auth.json`、反向代理、会话固定和 WebSocket 转发。

Codex+++ 采用后台号池、后端受控 Codex 入口、Windows/macOS 桌面客户端启动器的链路。Codex+++ 不采用根级 `/v1/*`、Anthropic、sub2api 或多供应商兼容入口作为公共产品面。

账号载入号池
------------

管理员后台支持以下账号来源：

1，ChatGPT session JSON。管理员在已登录 ChatGPT 的浏览器中打开 `https://chatgpt.com/api/auth/session`，复制完整 JSON 并粘贴到后台；后端自动提取 access token、ChatGPT account id、邮箱、套餐和最早过期时间，生成最小授权快照后加密入池。
2，邮箱密码文本或 JSON。支持 `email,password`、`email----password`、`email|password`、制表符分隔和对象字段。
3，Codex `auth.json`。
4，包含 access token、refresh token、ChatGPT account id、邮箱和套餐信息的 Token JSON。
5，Sub2API 账号备份中的 `accounts[].credentials`。
6，现有纯 Token 文本。

ChatGPT session JSON 不包含 refresh token，因此导入账号标记为短期授权。后端以真实 access token JWT 的 `exp` 与 session 过期时间中较早者作为有效期；过期后账号停止调度。管理员再次导入同一 ChatGPT account id 的新 session JSON 时，后端原位替换加密访问令牌和最小授权快照，复用已有内部 API Key，不创建重复账号。

2026-07-12 实测备注：本次用于获取 ChatGPT session 的 Google 登录账号在 Google OAuth 返回 OpenAI 登录流程后被要求进行电话号码验证。该账号没有可用电话号码，因此无法完成 ChatGPT 登录，也未取得可供后端导入的真实 session JSON。本结果只说明本次账号和登录环境受阻，不能推导为所有 Google 账号均永久无法使用。后端的 session 解析、最小 `auth.json` 生成和短期授权测试必须与真实 session 获取测试分别记录，不得用合成 token 或已有 `auth.json` 冒充真实 session 链路通过。

管理员可把完整号池导出为可重新导入的 JSON 备份。备份用于迁移与恢复，包含上游账号授权材料和待授权邮箱密码，因此必须按敏感文件保管；备份不包含 Codex+++ 内部 API Key、Key hash、公开前缀或凭据指纹。导出动作仅允许管理员执行并写入审计。

邮箱密码导入后写入待授权候选账号。密码只写入服务端加密字段。候选账号没有内部 API Key，不参与 Gateway 路由。

管理员可对候选账号发起以下 Codex app-server 登录：

1，浏览器登录：`account/login/start` 使用 `chatgpt` 类型并返回 `authUrl` 和 `loginId`。
2，device-code 登录：`account/login/start` 使用 `chatgptDeviceCode` 类型并返回 `verificationUrl`、`userCode` 和 `loginId`。

两种登录均使用独立 `CODEX_HOME`。该目录写入 `cli_auth_credentials_store = "file"`，登录完成后读取同目录 `auth.json`，导入完成后清理临时目录。

浏览器授权和 device-code 授权面板提供关闭按钮。关闭后后台取消对应授权会话、终止隔离 app-server 并清理临时目录；绑定的待授权候选账号恢复为待授权状态。添加账号面板也提供关闭按钮，关闭时不提交或保存表单内容。

浏览器登录的回调监听在后端 app-server 所在主机。管理员浏览器最终产生的 localhost 回调地址由管理员后台提交给后端。后端只接收 loopback 主机、`/auth/callback` 路径、有效端口、授权结果参数和匹配的 OAuth state，并把该请求转发到当前授权会话的 app-server 监听端口。

后台不保存 OpenAI OAuth URL、client id、token endpoint 或授权码交换实现。授权 URL、PKCE、令牌交换和刷新由 Codex app-server 管理。

app-server 登录导入要求 `auth.json` 包含 refresh token。缺少 refresh token 的结果不得进入自动刷新维护链。

授权结果绑定候选账号时校验邮箱。邮箱一致后在同一账号记录上写入授权材料、设置已授权状态、清除密码密文并创建内部 API Key。邮箱不一致时保留候选账号并记录授权失败。

号池保存字段
------------

账号来源解析后写入上游账号记录。敏感字段加密保存：

1，`access_token_cipher` 保存 access token。
2，`refresh_token_cipher` 保存 refresh token。
3，`auth_json_cipher` 保存原始 Codex `auth.json` 快照；ChatGPT session 来源只保存后端生成的最小授权快照，不保存完整 session JSON。
4，`password_cipher` 只保存待授权候选账号的临时密码；授权成功后清空。

后台可保存的非敏感元数据包括账号名称、账号分组、管理员备注、来源类型、授权状态、ChatGPT account id、email、subscription tier、expires_at、entitlement_status、账号状态、余额状态、风控状态、最近授权错误和最近检查时间。管理员备注最多 500 个字符，可在号池账号行的操作中编辑，并随完整号池备份导出和恢复；审计只记录备注长度，不记录备注正文。

每条成功 Gateway 用量记录同时保存普通用户网关凭据 ID、内部上游账号 ID 和内部路由 API Key ID，只用于后台归属和统计，不返回普通用户。号池账号行优先显示存在进行中 Gateway 请求，或在成功 Gateway 归属基础上由运行中客户端续租的普通用户；没有当前使用用户时只显示绿色“空闲”，悬浮后显示最近使用用户及其最近 Codex 会话的开始、结束时间，旧记录没有会话标识时起止时间相同。客户端不能提交或选择上游账号 ID。行内同时显示该账号当日经 Codex+++ 路由的 token、上游检查返回的剩余百分比或 credits、管理员备注以及账号状态。

已授权账号导入或更新后，后台维护一条绑定该上游账号的内部 API Key。待授权候选账号没有内部 API Key。该 API Key 是后端网关调度材料，不是上游账号授权材料。

客户端消费号池
--------------

Windows 与 macOS 客户端只作为 Codex 本机启动器和配置桥。

桌面客户端不显示号池账号列表，不导入号池账号，不接收号池邮箱密码，不发起上游 OAuth，不处理 app-server 回调，不读取后端上游授权材料。

客户端登录 Codex+++ 后检查本机 Codex 状态，并在启动前调用 `/api/client/launch/prepare`。客户端只提交本机识别到的账号标识和认证模式，不上传 ChatGPT token 或 `auth.json`。

本机没有可识别认证时，后端直接准备当前普通用户的受控网关访问 Key。本机已有 ChatGPT 登录态时，后端必须先用 ChatGPT account id 或邮箱匹配本后端号池；匹配成功才允许继续准备。ChatGPT 账号不匹配、无法识别，或检测到用户自己的 API Key 等其他已有认证时，统一返回 `personal_codex_login_detected`；客户端只提示用户先退出自己的登录，不调用本地配置写入命令。普通用户网关访问 Key 与号池内部路由 API Key 分表保存，不绑定上游账号；当前客户端取得的 Key 绑定普通用户和登录设备，并要求该设备持续提供短期运行租约。普通用户启动不预选上游账号。

允许准备时，该接口只返回可用于 Codex+++ provider 的受控 token，不返回上游账号授权材料、上游账号 id、路由 id、网关内部字段或号池账号展示字段。客户端先保存本机启动前的 `config.toml` 和 `auth.json` 快照，再临时切换到 API-key 模式；关闭窗口时客户端进入托盘继续管理，明确停止、退出或注销时停止 Codex 并恢复原快照。异常退出后设备运行租约在两分钟内失效，受控 token 随即停止被后端接受。

客户端把返回的 token 写入本机 Codex `auth.json` 的 API-key 模式，并写入 `~/.codex/config.toml` 的 Codex+++ provider：

1，`model_provider = "codexppp"`。
2，`[model_providers.codexppp]`。
3，`wire_api = "responses"`。
4，`base_url = "<backend API base>/codex/v1"`。
5，`requires_openai_auth = true`。

客户端不得向 Codex 进程注入 Codex+++ 管理的 `CODEX_HOME`。客户端不得使用 provider `auth.command`、`env_key` 或 `experimental_bearer_token` 作为 Codex+++ 登录机制。

Codex 请求进入 `/api/codex/v1/models` 或 `/api/codex/v1/responses` 后，后端先校验普通用户、设备绑定和运行租约，再执行 Token 余额检查、内部路由 API Key 状态检查、上游账号状态检查、路由选择、上游调用、用量验证、扣费、审计和失败切换。用户网关凭据不包含上游授权材料，也不允许客户端选择上游账号。

`/api/codex/v1/responses` 把清理后的 Responses 请求发送到 Codex 官方 Responses 端点。后端使用所选号池账号的 access token 和 ChatGPT account id 完成上游鉴权，原样保存并返回 SSE 响应体、Content-Type 和允许透传的 Codex 响应头。`x-codex-turn-state` 保持连续。

Responses 中的 shell、文件和其他工具调用返回本机 Codex。本机 Codex 在用户机器上执行工具并提交工具结果。后端不得为普通用户任务创建 app-server thread 或 turn，不得在服务器目录中执行本机项目工具。

刷新所有权
----------

导入到 Codex+++ 号池的授权链由 Codex+++ 后端维护。

同一份 refresh token 链不得在多个后端实例、本机 Codex、其他账号池工具或自动化流程中并发使用。

后台刷新通过隔离 `CODEX_HOME` 内的 `codex app-server --listen stdio://` 执行。后台写入该上游账号的 `auth.json`，调用 `account/read` 且设置 `refreshToken = true`，读取刷新后的 `auth.json`，再把新的 access token、refresh token 和 `auth.json` 快照写回同一上游账号记录。

后台检测到 refresh token 失效、复用、过期或被撤销时，该上游账号进入不可调度状态，并由管理员重新授权。

后端按北京时间每天 09:00 对全部已授权号池账号执行与后台手动“检查”相同的授权、额度和风险检查并写入系统审计。若服务在 09:00 时离线，当天 09:00 后恢复时仅补检从该时间点起尚未检查过的已授权账号，避免同日重启反复检查。

会话与调度
----------

同一 Codex 会话的后续请求持久保留上游账号粘性。同一普通用户的新会话优先继续使用当前健康上游；每个上游账号默认最多同时绑定两个普通用户，限制可通过部署环境调整。只有当前上游余额、认证、限流、路由或服务状态不可继续使用时才允许受控切换；客户端停止或运行租约到期后释放用户占用。

Codex+++ token 余额不足时，后端停止向客户端提供后续可用路由。

Codex 上游账号额度不足、授权失败、限流、无效路由或服务不可用时，后端可标记该上游账号不可调度并尝试下一条可用路由。
