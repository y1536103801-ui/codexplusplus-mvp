# Client Bootstrap API

## 目标

实现 `GET /api/v1/client/bootstrap`，为 Codex++ 客户端返回当前用户的托管供应商快照。

## 涉及项目

- sub2api-main：`backend/internal/server/routes/user.go` 或新增 client routes
- sub2api-main：`backend/internal/handler`
- sub2api-main：`backend/internal/service/api_key_service.go`
- sub2api-main：配置中心只读服务

## 输入契约

- 用户 JWT。
- PlanCatalog、ModelCatalog、UsagePolicy、FeatureFlags。
- 用户余额、套餐、状态。
- Control Plane 输出的配置版本和策略决策结果。
- Data Plane 侧设备、Key、风险和限流状态。

## 输出行为

返回 `service`、`provider`、`plan`、`models`、`usage`、`feature_flags`、`announcements`、`version_policy`。如果用户无可用 API Key，可为该用户创建或复用一个 `Codex++ Cloud` 专用 Key。

响应必须包含：

- `config_version`：本快照使用的控制面配置版本。
- `snapshot_version`：本次聚合快照版本。
- `refresh_policy`：客户端何时刷新、何时强制重新登录。
- `degraded_mode`：后端部分能力不可用时客户端如何展示。

## 解耦要求

- 返回聚合后的模型列表和当前套餐，不返回完整后台配置。
- 不把价格规则、倍率规则、限流规则下发给客户端。
- 网关地址由后台配置，不由客户端拼接。

## 禁止改动范围

- 不改支付流程。
- 不改客户端代码。
- 不改网关转发逻辑。

## 测试要求

- 未登录返回 401。
- 无套餐、过期、余额不足、被禁用分别返回明确状态。
- 同一用户重复调用不重复创建 Key。
- 响应日志不打印完整 `api_key`。
- 配置版本回滚后，bootstrap 能返回新的快照版本。
- bootstrap 调用产生结构化访问事件，包含用户、设备、请求 ID 和配置版本。

## 交付物

- handler、service、route。
- bootstrap 单元测试/接口测试。
- mock 响应更新。
- 事件埋点和可观测字段说明。
