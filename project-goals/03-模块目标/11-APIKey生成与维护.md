API Key 生成与维护
==================

目标
----

把号池资源转成后端可调度的 API Key。

参考项目
--------

本模块必须参考 sub2api 的 API Key 生成和维护机制，重点参考从 Codex 上游账户号池生成可调度 API Key、维护 API Key 与账号或等价路由的绑定关系、维护状态、余额关联、配额关联和停用规则。

参考 sub2api 的结果必须落到服务端受控 API Key 机制，不得让普通用户看到、复制或手动选择 API Key。

必须做到
--------

1，根据号池中的 Codex 账号生成或维护 API Key。
2，记录 API Key 对应的上游 Codex 账号或等价路由关系。
3，记录 API Key 状态。
4，记录 API Key 对应的 Codex 账号余额状态。
5，在 Codex 账号余额不足或 API Key 不可用时排除该 API Key。
6，向后端 gateway 提供可用 API Key 或等价可用路由。

不得做到
--------

1，让普通用户查看 API Key 原文。
2，让普通用户复制 API Key。
3，让普通用户手动选择 API Key。
4，用 API Key 切换绕过 Codex+++ token 余额不足。
