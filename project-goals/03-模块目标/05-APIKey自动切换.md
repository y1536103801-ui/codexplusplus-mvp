API Key 自动切换
================

目标
----

当后端号池中的上游 Codex 账号不可继续使用时，让后端 gateway 切换到可用 API Key 或等价可用路由。

参考项目
--------

本模块必须参考 sub2api 的 API Key 可用性和切换机制，重点参考按 Codex 账号余额、API Key 状态、账号可路由状态和风控结果选择下一枚可用 API Key 或等价可用路由。

参考 sub2api 的结果必须落到后端 gateway 调度，不得变成普通用户手动配置 API Key。

Windows 与 macOS 客户端不得接收、保存、展示或切换号池内部 API Key。API Key 或等价可用路由只在后端 gateway 内部流转。

必须做到
--------

1，切换触发条件是 Codex 账号余额不足、当前 API Key 不可用、当前 API Key 被停用或当前上游账号不可路由。
2，切换依据来自后端，不由普通用户判断。
3，后端 gateway 自动选择下一枚可用 API Key 或等价可用路由。
4，切换成功后请求继续由后端 gateway 处理。
5，切换失败时返回受控中文错误。

不得混淆
--------

1，Codex 账号余额不足才属于 API Key 自动切换原因。
2，Codex+++ token 余额不足不得被处理成 API Key 自动切换。
3，普通用户不得查看、复制或手动选择 API Key。
4，普通用户不得看到 Codex 账户凭据。
