# 支付权益开通流程

## 目标

把 Sub2API 现有支付系统和 Codex++ 套餐权益打通，支付成功后自动开通套餐或充值余额。

## 涉及项目

- sub2api-main：`backend/internal/service/payment_*`
- sub2api-main：`backend/internal/service/subscription_service.go`
- sub2api-main：`backend/internal/service/redeem_service.go`

## 输入契约

- PlanCatalog。
- payment order paid/completed 状态。
- 用户 ID。

## 输出行为

支付成功后：

- 根据订单套餐 ID 写入用户权益。
- 充值余额或开通订阅。
- 触发 API Key auth cache invalidation。
- usage/bootstrap 立即反映新状态。

## 解耦要求

- 支付金额与套餐权益由后台配置绑定。
- 客户端只看到续费后服务恢复，不参与权益计算。

## 禁止改动范围

- 不改客户端。
- 不改网关转发实现。
- 不改变现有支付 provider 的签名校验。

## 测试要求

- 支付成功、重复回调、订单过期后补单、退款后权益处理。
- 开通后 bootstrap 从 expired/low_balance 变为 available。

## 交付物

- 支付到权益映射服务。
- 幂等测试。
- 回调状态测试。

