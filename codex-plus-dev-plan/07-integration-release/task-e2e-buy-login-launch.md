# E2E 购买登录启动

## 目标

验证用户从购买套餐到登录 Codex++、自动配置、启动 Codex 并完成一次请求的完整路径。

## 涉及项目

- CodexPlusPlus-main：客户端、Tauri、launcher
- sub2api-main：支付、权益、client API、gateway

## 输入契约

- 测试支付订单或手动开通套餐。
- 测试用户。
- 测试上游账号池。
- Turnstile enabled 的生产等价测试环境。
- 已实现的 browser handoff：`/api/v1/auth/desktop/start`、`/api/v1/auth/desktop/complete`、`/api/v1/auth/desktop/poll`。

## 输出行为

用户完成：

1. 购买套餐。
2. 在 Codex++ Manager 点击 browser handoff 登录，打开 `/auth/desktop/authorize`。
3. 在浏览器内完成 Web 登录、Turnstile/2FA 等安全校验，并确认 6 位授权码。
4. 桌面端轮询 `/api/v1/auth/desktop/poll` 获得 JWT。
5. 兼容路径可覆盖 `/api/v1/auth/login` 与 `/api/v1/auth/login/2fa`，但不得作为 Turnstile-enabled 生产默认路径。
6. 自动拉取 bootstrap。
7. 写入 `Codex++ Cloud`。
8. 一键启动 Codex。
9. 完成一次模型请求。

## 解耦要求

- 验证客户端没有硬编码价格、模型、额度阈值。
- 修改后台默认模型后无需客户端发版即可生效。
- 登录链路不得要求关闭 Turnstile 才能完成购买后首次使用。

## 禁止改动范围

- 不在 E2E 中绕过支付/权益主流程。
- 不手动填写 API Key。

## 测试要求

- 正常购买。
- browser handoff 登录成功后 bootstrap 可用。
- 2FA 用户可完成验证码验证，不出现缺 token、404 或 pending session 被误判为已登录。
- Turnstile enabled 时，生产登录通过 browser handoff 完成；如果只能通过桌面密码登录，本 E2E 必须判定为未达上线标准。
- 余额不足后续费恢复。
- 套餐过期。
- 下架模型。

## 交付物

- E2E 脚本或手动验收清单。
- 测试账号说明。
- 问题记录。
