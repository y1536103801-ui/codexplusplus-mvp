Gateway 调度
============

目标
----

承接 Codex 请求，并选择可用 API Key 完成上游调用。

参考项目
--------

本模块必须参考 sub2api 的 gateway 调度机制，重点参考鉴权、策略检查、API Key 状态检查、Codex 账号余额检查、可用账号或可用 API Key 选择、上游调用、失败切换和受控响应。

参考 sub2api 的结果必须落到 Codex+++ 自有 gateway，不得直接暴露 sub2api 路由、页面、权限模型或上游错误原文。

首版 gateway 不做任何旧接口兼容，不兼容 OpenAI、Anthropic、sub2api 或历史实现中的多接口形态。

Codex+++ gateway 只能暴露 Codex+++ 明确定义的唯一干净契约。不得保留根级 `/v1/chat/completions`、`/v1/responses`、`/v1/messages`、Images 或其他历史兼容路由作为 fallback。

允许保留唯一的 Codex 形态鉴权入口 `/api/codex/v1/models` 和 `/api/codex/v1/responses`。该入口属于后端受控适配能力，必须复用后端内部 gateway executor 的鉴权、余额检查、扣费、幂等、路由和审计逻辑，不得扩展成旧 OpenAI 兼容层。Windows 客户端在无本机 Codex 登录态时可以把该入口写入 Codex provider 配置，使 Codex 请求进入后端受控适配入口。

必须做到
--------

1，验证普通用户身份和设备状态。
2，检查普通用户可用状态和 Codex+++ token 余额。
3，检查 API Key 状态。
4，检查 API Key 对应的 Codex 账号余额。
5，根据策略选择可用 API Key。
6，调用上游 Codex 账号。
7，返回受控响应。
8，发现旧路由、旧字段、旧配置或旧兼容层时必须清理。

不得混淆
--------

1，Codex+++ token 余额不足时不得继续调度 API Key。
2，Codex 账号余额不足时可以切换 API Key。
3，上游错误不得原文透传给普通用户。
4，不得把 gateway 写成 OpenAI、Anthropic、sub2api 或旧实现兼容层。
