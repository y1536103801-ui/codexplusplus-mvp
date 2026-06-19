# 管理员用量策略管理

## 目标

实现管理员配置 Codex++ 用户用量策略，包括余额提醒、每日额度、并发、RPM、TPM 和过期行为。

## 涉及项目

- sub2api-main backend：UsagePolicy 配置服务
- sub2api-main frontend：admin settings views
- sub2api-main gateway enforcement 后续消费

## 输入契约

- UsagePolicy。
- 用户组/套餐可选覆盖规则。

## 输出行为

管理员可：

- 设置全局策略。
- 对套餐或用户组覆盖策略。
- 配置低余额提示和续费入口。
- 配置严格设备执行开关：从兼容模式切到要求设备上下文的网关 enforcement。
- 预览策略命中结果。

## 解耦要求

- 客户端只展示后端聚合后的状态。
- 网关强制执行策略，客户端隐藏按钮不算 enforcement。
- 严格设备执行由后台配置决定，客户端不得通过本地状态关闭。

## 禁止改动范围

- 不改支付订单。
- 不改客户端本地逻辑。
- 不改模型目录。

## 测试要求

- 策略覆盖优先级。
- 无效限流值校验。
- 策略变化后 usage/bootstrap 更新。
- `strict_device_enforcement` 开启后，缺失设备 ID 的托管网关请求被拒绝。

## 交付物

- 管理配置页面。
- 后端校验。
- 策略预览接口。
