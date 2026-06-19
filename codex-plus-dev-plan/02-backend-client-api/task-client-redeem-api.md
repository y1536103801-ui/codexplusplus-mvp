# Client Redeem API

## 目标

实现 `POST /api/v1/client/redeem`，允许 Codex++ 客户端输入兑换码完成补偿、活动发放或分销激活。

## 涉及项目

- sub2api-main：`backend/internal/service/redeem_service.go`
- sub2api-main：`backend/internal/handler/redeem_handler.go`
- sub2api-main：client routes

## 输入契约

- 用户 JWT。
- `code`。
- 现有 redeem code 类型和状态。

## 输出行为

兑换成功后返回更新后的 entitlement/usage 摘要，客户端可立即刷新 bootstrap。

## 解耦要求

- 兑换码面额、套餐、有效期由后台 redeem 配置决定。
- 客户端不解析兑换码内容。

## 禁止改动范围

- 不改管理员兑换码生成逻辑，除非契约缺字段。
- 不改支付订单。
- 不改客户端 UI。

## 测试要求

- 无效码、已使用、过期、类型不支持、成功兑换。
- 重复提交保持正确错误。
- 兑换后 bootstrap 状态更新。

## 交付物

- client redeem API。
- redeem 状态映射。
- 接口测试。

