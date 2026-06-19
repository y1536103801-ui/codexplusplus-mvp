# Client Usage API

## 目标

实现 `GET /api/v1/client/usage`，为客户端首页提供余额、今日用量、套餐状态、限流状态和续费入口。

## 涉及项目

- sub2api-main：`backend/internal/handler/usage_handler.go`
- sub2api-main：`backend/internal/service/usage_service.go`
- sub2api-main：`backend/internal/service/billing_cache_service.go`

## 输入契约

- 用户 JWT。
- UsagePolicy。
- 用户余额、套餐、usage log。

## 输出行为

返回 `available`、`balance`、`today_tokens`、`remaining_percent`、`rate_limit`、`renew_url`、`message`。

## 解耦要求

- `remaining_percent` 由后台按策略计算。
- `message` 来自后台状态模型或配置。
- 客户端不得根据余额自行推断是否停用。

## 禁止改动范围

- 不创建 API Key。
- 不修改支付订单。
- 不修改客户端 UI。

## 测试要求

- 正常、低余额、过期、禁用、无套餐状态。
- 今日用量统计按用户隔离。
- 续费 URL 由套餐/配置返回。

## 交付物

- usage client API。
- 状态映射测试。
- mock 响应。

