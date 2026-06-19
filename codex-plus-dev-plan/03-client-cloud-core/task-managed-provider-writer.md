# 托管供应商写入

## 目标

把 bootstrap 中的 provider 快照写成 Codex 本地可使用的 `Codex++ Cloud` 托管供应商。

## 涉及项目

- CodexPlusPlus-main：`crates/codex-plus-core/src/relay_config.rs`
- CodexPlusPlus-main：`crates/codex-plus-core/src/relay_switch.rs`
- CodexPlusPlus-main：`crates/codex-plus-core/src/settings.rs`

## 输入契约

- `provider.name`
- `provider.gateway_base_url`
- `provider.api_key`
- `provider.wire_api`
- `models.default_model`

## 输出行为

生成或更新本地托管供应商：

- name：`Codex++ Cloud`
- provider：`custom` 或现有兼容 provider 名称。
- wire API：以后端返回为准。
- api key：写入 auth 或配置文件，遵循现有纯 API/托管路径。
- Codex 主进程的 `base_url` 指向本地协议 helper，由 helper 转发到后台 gateway 并注入设备 header。
- 真实后台 gateway base URL 保存在托管 profile 的 upstream/base URL 字段中，不能被本地 helper URL 覆盖。
- 托管 key 必须可从 profile 的 `auth_contents.OPENAI_API_KEY` 恢复，不能依赖只存在于内存或被序列化隐藏的字段。

## 解耦要求

- 不写入价格、倍率、套餐和限流字段。
- 不根据客户端内置模型表覆盖后端默认模型。
- 后端下架模型后，下次 bootstrap 必须覆盖本地默认模型。
- 客户端不得绕过本地 helper 直接让 Codex 主请求打到后台 gateway，否则设备 enforcement 无法闭环。

## 禁止改动范围

- 不删除用户已有手动供应商配置。
- 不重构完整供应商系统。
- 不修改 Codex 原始安装文件。

## 测试要求

- 首次写入。
- 重复写入幂等。
- 后端默认模型变化后本地更新。
- 保留用户手动供应商。
- 应用重启后，托管 key 仍能从 `auth_contents` 读取并用于本地 helper 转发。
- 本地 helper 转发 Responses、Chat Completions 和 Models 请求时携带 Codex++ 设备 header。

## 交付物

- 托管供应商写入函数。
- 配置备份和回滚路径。
- relay_config 测试。
