# 管理员模型管理

## 目标

实现管理员调整 Codex++ 可售模型、展示名、默认模型、模型组和开放状态。

## 涉及项目

- sub2api-main backend：ModelCatalog 配置服务
- sub2api-main frontend：admin settings/channel views

## 输入契约

- ModelCatalog。
- PlanCatalog 中的模型组引用。

## 输出行为

管理员可：

- 新增模型展示项。
- 绑定真实路由模型。
- 调整 badge、上下文窗口、倍率。
- 设置默认模型。
- 下架模型。

## 解耦要求

- 客户端模型列表完全来自 bootstrap。
- 网关 enforcement 必须读取同一模型权限。

## 禁止改动范围

- 不改客户端内置模型文件。
- 不改支付逻辑。
- 不改上游账号导入。

## 测试要求

- 默认模型只能有一个有效项。
- 下架模型后 bootstrap 不返回，网关拒绝。

## 交付物

- 管理接口/页面。
- 模型校验。
- 端到端配置变更测试。

