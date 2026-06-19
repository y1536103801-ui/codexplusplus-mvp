# 登录绑定 UI

## 目标

实现 Codex++ 登录绑定界面，让用户登录购买账号并绑定本机设备。

## 涉及项目

- CodexPlusPlus-main：`apps/codex-plus-manager/src/App.tsx`
- CodexPlusPlus-main：Tauri cloud auth commands

## 输入契约

- 后台登录方式沿用 Sub2API 现有用户认证。
- 生产默认登录方式为 browser handoff：桌面调用 `/api/v1/auth/desktop/start` 获得授权 URL，浏览器登录后调用 `/api/v1/auth/desktop/complete`，桌面再调用 `/api/v1/auth/desktop/poll` 换取正式 JWT。
- 桌面运行时使用 `/api/v1/auth/login` 获取用户 JWT，再调用 `/api/v1/client/bootstrap`。
- 启用 2FA 的账号由 `/api/v1/auth/login` 返回 `requires_2fa` 和 `temp_token`，桌面运行时再调用 `/api/v1/auth/login/2fa` 换取正式 JWT。
- `/api/v1/auth/login` 只作为兼容登录路径；生产环境若启用 Turnstile，普通用户默认不得依赖桌面密码登录，不得要求管理员关闭 Turnstile。
- 本地 session store。
- device API。

## 输出行为

用户可以：

- 默认点击“用浏览器登录”，在系统浏览器完成 Web 登录、安全校验和授权确认。
- 在 Manager 与浏览器授权页显示同一个 6 位确认码，帮助用户确认授权的是当前桌面端。
- 桌面端只保存 pending handoff 所需的 `poll_token`，不把它展示给 UI 或写入诊断。
- 输入账号密码完成兼容登录；兼容登录不得成为 Turnstile-enabled 生产默认路径。
- 账号密码登录使用后端 `AuthResponse.access_token`，不得调用不存在的 client 专用登录端点。
- 账号启用 2FA 时，必须展示动态验证码输入，不把 pending session 当作已登录状态。
- 2FA 验证成功后必须进入同一 bootstrap/device/provider 写入链路。
- 查看当前登录账号。
- 退出登录。
- 登录后自动刷新 bootstrap。
- 设备被撤销时看到重新绑定或联系支持提示。

## 解耦要求

- 登录 UI 不展示 API Key。
- 不包含套餐价格和购买规则。
- 续费入口以后端返回 URL 为准。
- 客户端不得硬编码套餐、价格、模型倍率、额度阈值、限流阈值或续费文案。
- 登录后的服务状态、权益状态、设备状态和行动提示必须来自 bootstrap、usage 或后台配置快照。

## 禁止改动范围

- 不实现后台认证。
- 不改 Codex 官方登录。
- 不改支付页面。
- 不绕过 Turnstile 或降低现有 Web 登录风控。

## 测试要求

- 登录成功、失败、token 过期、退出登录。
- `/api/v1/auth/login` 成功响应、2FA required 响应和失败响应。
- `/api/v1/auth/login/2fa` 成功响应、验证码错误响应和 pending session 丢失响应。
- Turnstile enabled 场景必须验证桌面登录不会依赖“关闭 Turnstile”才能上线。
- 登录后自动登记设备并刷新服务状态。

## 交付物

- 登录绑定视图。
- 状态提示。
- 手动测试说明。
