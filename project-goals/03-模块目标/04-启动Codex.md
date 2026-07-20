启动 Codex
==========

目标
----

让普通用户点击一次即可打开 Codex 并正常使用。

参考项目
--------

本模块必须参考 Codex++ 中适合桌面端复用的本机能力，包括 Tauri 桌面壳、本机 Codex 检测、受控配置清理、启动流程、更新和脱敏诊断。

参考 Codex++ 的结果必须落到真实可用的一键启动流程，不得只做一个没有本机能力支撑的外壳。

Windows 与 macOS 客户端都必须完成本机 Codex 准备和一键启动，并共用同一后端准备、运行租约、心跳和恢复契约。Windows 平台使用 Microsoft Store/AppX 边界；macOS 平台使用 OpenAI 官方签名的 `ChatGPT.app` 边界，两者不得互相套用安装、版本或进程检测规则。

Windows 与 macOS 客户端允许在启动 Codex 前备份并修改本机 Codex 配置。已有本机 ChatGPT 登录态时，客户端只把本机识别出的账号标识提交给后端做号池归属匹配，不提交 token 或 `auth.json`；只有匹配本后端号池时才备份并临时切换到 Codex+++ provider。ChatGPT 账号不匹配、无法识别，或检测到用户自己的 API Key 等其他已有认证时，保持本机配置原样并提示用户先退出自己的登录。没有可识别本机认证时，客户端直接向后端获取当前普通用户和当前设备专属的 Codex+++ 网关访问 key。停止 Codex 后恢复启动前快照；关闭客户端窗口时进入托盘继续管理，不得让运行中的 Codex 脱离心跳管理。

自动登录必须参考 Codex++ 的 provider 配置边界和 Codex 官方 API-key 登录机制：`config.toml` 写入 Codex+++ 管理的 provider，provider 设置 `requires_openai_auth = true`；`auth.json` 写入 `auth_mode = "apikey"` 和 `OPENAI_API_KEY`。provider 不使用 `auth.command`、`env_key` 或 `experimental_bearer_token`。客户端不得注入 `CODEX_HOME`。后端启动准备接口只返回当前普通用户和当前设备专属的 Codex+++ 网关访问 key，不返回号池内部路由 key 或账号显示字段。该 key 必须同时校验启用设备和短期运行租约，但不绑定或暴露上游账号；用户网关访问 key 与号池内部路由 key 分表保存，路由仍由后端动态选择。

普通用户客户端服务状态区域中的 `Codex 账号` 字段只来自本机 Codex 认证状态检测结果。已有 ChatGPT 账号信息时显示本机识别出的账号信息；只有 API-key 登录态时显示本机 `auth.json` 中 `OPENAI_API_KEY` 的脱敏值；本机无法识别出认证信息时显示“未识别”。该字段不得读取后端号池账号、后端会话 token、启动器登录账号或后端返回的展示账号字段。

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
12，服务状态区域中的 `Codex 账号` 字段只能来自本机 Codex 认证状态检测结果；API-key 登录态显示本机 `OPENAI_API_KEY` 的脱敏值。
13，Codex 未登录，或本机 ChatGPT 登录账号匹配本后端号池时，客户端先向后端获取当前普通用户和当前设备专属的 Codex+++ 网关访问 key，再写入 Codex `auth.json` 和 Codex+++ provider 配置；不匹配的 ChatGPT 登录、无法识别的登录和用户自己的 API Key 等其他已有认证均不得修改。
14，启动失败是普通用户本机行为，客户端只在本机显示中文提示和下一步动作。
15，Windows Store 版 Codex 不得按普通命令行程序处理。
16，`cmd /C start` 或等价 shell 二次解析方式启动 WindowsApps 里的 `Codex.exe` 属于已复现失败路径，不得作为启动方案、重试方案或 fallback。
17，直接 `CreateProcess` 执行 WindowsApps 里的 `Codex.exe` 属于已复现失败路径，不得作为启动方案、重试方案或 fallback。
18，Windows 客户端只允许把 OpenAI 官方 ChatGPT 桌面应用的 `OpenAI.Codex` Store/AppX 包识别为 Codex 桌面端；Windows 应用名称显示为 ChatGPT，Codex 是其中的原生编程代理工作流。命令行版不得作为安装成功、启动目标、重试或 fallback。
19，“安装 Codex”必须是客户端内的一键流程：优先调用 winget 安装 Store 产品；winget 缺失或失败时由客户端内部获取官方 Store Web Installer、校验 Microsoft 签名并运行；不得要求普通用户单独下载安装器。
20，安装流程只有在检测到官方桌面应用 AppID 后才能显示成功；CLI 文件存在、CLI 版本可执行或安装器进程启动均不得视为成功。
21，Windows Store 版 Codex 启动方案发布前必须在部署机验证 Codex 安装检测、主界面打开、运行状态刷新、停止状态刷新、号池 API key 可用和本机 Codex 账号识别。
22，客户端必须识别官方 Store 包的实际安装版本，但不得在代码或后端配置中硬编码某个 Codex 版本、发布线或尾部构建号作为“最新”或“兼容”。每次客户端会话必须先刷新官方 `msstore` 源，再查询 Microsoft Store 官方产品 ID 的完整安装行；只有查询到的安装版本与实际 AppX 版本一致且没有 `Available` 版本时才能通过，缺行、不一致或查询失败一律不得默认放行。有更新时主按钮改为更新动作，由客户端内部完成 Store 更新后才允许启动。WinGet 不可用时必须显示版本待验证，并在用户点击后由微软签名的 Store Web Installer 静默检查并按需更新。桌面内部资源或插件构建号不得当作 AppX 包版本比较；只有首次激活后才会释放到本机的官方运行时能力，在桌面进程启动后另行验证。
22，启动失败诊断必须记录可定位的本机失败阶段，不得只保留 `codex_launch_failed` 这类无法区分失败路径的单一结果。
23，客户端写入本机 Codex 的 API-key 登录态只代表 Codex+++ provider token，不代表上游 Codex 账号授权材料。
24，Codex 请求进入 `/api/codex/v1` 后，路由选择、上游账号选择、扣费、审计和失败切换全部归属后端 gateway。
25，Responses 工具调用由本机 Codex 执行。后端只负责受控转发、路由、用量验证和扣费，不得在服务器目录执行本机项目工具。
26，运行态检测不得以 WindowsApps 安装目录可读作为前置条件，也不得只依赖单一进程枚举来源。客户端优先使用 Windows `tasklist /APPS` 返回的 Store 包全名，并要求其精确属于 `OpenAI.Codex...__2p2nqsd0c76g0`；常规进程路径和实时进程包族身份作为次级校验。只有官方包中的 `ChatGPT.exe`/`Codex.exe` 才能显示为运行中，同名独立程序不得误判。
27，客户端必须通过 Windows `IApplicationActivationManager` 激活精确的 `OpenAI.Codex` AppID，不得再以 `explorer.exe` 进程创建成功替代应用启动结果。只有 Windows 返回非零且仍存活的应用进程 ID 后才能显示“已启动”并发送启动心跳；该进程 ID 是本轮运行态与停止操作的首要依据，其他受保护进程检查仅作异步增强确认。COM 初始化、激活服务创建、激活请求、无进程和进程立即退出必须保留为不同的脱敏失败阶段。
28，Store 包内受保护的 `app\resources\codex.exe` 不得作为插件命令执行 fallback。首次启动时客户端先完成配置写入并激活官方 AppID，再等待桌面应用释放 `%LOCALAPPDATA%\OpenAI\Codex\bin\<runtime-id>` 下的官方运行时并自动配置 Browser；Browser 配置失败不得把已经运行的桌面应用误报为启动失败。
29，客户端窗口关闭时必须隐藏到系统托盘并继续发送运行心跳；托盘必须提供“打开客户端”和“停止 Codex 并退出”。明确退出或注销时停止本轮受管 Codex 并恢复启动前快照。
30，客户端异常退出、被结束进程或设备断网时，设备运行租约最多保留两分钟；租约到期后设备绑定的 provider key 不得继续调用 Codex gateway。下一次启动发现 Codex 已停止时必须恢复未完成的本机快照。
31，正常启动 Codex 成功后，Codex+++ 主窗口必须立即隐藏到系统托盘，并继续在后台维护运行心跳；用户可通过托盘重新打开客户端。
32，安装或更新 Codex 只负责安装、版本验证和保持 Codex 关闭。微软安装器若自动激活 Codex，客户端必须将其关闭；不得把安装器激活视为用户启动，也不得在尚未完成后端账号准备时留下未登录的 Codex 窗口。只有用户随后明确点击“启动 Codex”时，才执行账号准备、配置写入和正式启动。
33，macOS 客户端只允许识别 OpenAI 官方签名且通过 Gatekeeper 校验的新版 `ChatGPT.app`。ChatGPT Classic、独立 CLI 或同名第三方应用不得作为 Codex 桌面端。
34，macOS 的“安装 Codex”必须由客户端从 OpenAI 官方 macOS 下载地址获取 DMG，挂载后复查应用布局、OpenAI bundle identifier、开发者团队签名和 Gatekeeper，再安装到当前用户的 `~/Applications`；安装完成后保持应用关闭。
35，macOS 不硬编码 Codex 版本号。客户端每次会话读取已安装应用的 bundle version，并通过 OpenAI 官方 DMG 的实时 ETag（无 ETag 时使用 Last-Modified 与 Content-Length）与客户端上次安装记录比较；官方制品变化、安装版本变化、标记缺失或签名失败时均要求客户端内更新后才允许启动。
36，macOS 启动必须使用系统 `open` 直接打开已验证的应用包；运行态和停止动作只匹配 `ChatGPT.app/Contents/MacOS/ChatGPT` 主进程，不得把 ChatGPT Classic、CLI 或同名进程误判为 Codex 正在运行。
37，macOS 的用户 Codex 配置仍位于 `~/.codex`；Codex+++ 快照、安装验证标记和客户端运行数据位于 `~/Library/Application Support/Codex+++`。停止、退出和异常恢复规则与 Windows 保持一致。

不得做到
--------

1，暴露旧 Codex++ 高级工作台入口。
2，要求普通用户手动选择 provider。
3，要求普通用户手动填写 API Key。
4，要求普通用户手动配置 proxy、endpoint 或 base_url。
5，整文件覆盖本机 Codex 配置。
6，保留 Codex++、sub2api 或历史实现留下的旧配置项作为 fallback。
7，在服务状态区域把后端号池账号、后端会话 token、启动器登录账号或后端返回的展示账号字段显示为 `Codex 账号`。
8，在客户端生成、保存或返回号池内部路由 key、上游 access token 或 refresh token。
9，让不带有效设备运行租约的 Codex+++ provider key 继续调用后端。
10，token 余额不足时自动发起充值、自动联系管理员、自动切换 API Key 或继续提供后续账号。
11，把管理员后台做成监控每个普通用户本机启动过程的面板。
12，把 Windows Store Codex 的 WindowsApps `Codex.exe` 当作普通 exe 反复尝试启动。
13，让普通用户手动处理 `model_provider = "codexppp"` 或 `[model_providers.codexppp]`。
14，使用 provider `auth.command`、`env_key` 或 `experimental_bearer_token` 作为 Codex 登录机制。
15，向 Codex 进程注入 Codex+++ 管理的 `CODEX_HOME`。
16，向普通用户客户端返回上游账号授权材料。
17，让普通用户客户端选择号池账号。
