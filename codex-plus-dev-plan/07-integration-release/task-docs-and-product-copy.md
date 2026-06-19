# 文档与产品文案

## 目标

同步 README、产品说明书、HTML 展示页、用户安装教程和管理员运营手册。

## 涉及项目

- 根目录：`产品方案书.md`、`产品说明书.md`、`codex-plus-product-spec.html`
- CodexPlusPlus-main：README/README_EN 后续可选
- sub2api-main：部署和支付文档后续可选

## 输入契约

- 完整 E2E 行为。
- 后台配置中心字段。
- 客户端新手向导内容。
- `ARCHITECTURE.md` 工业级 v2 架构总纲。
- 配置版本、计量对账、网关拒绝、设备撤销和回滚验收结果。

## 输出行为

文档覆盖：

- 用户如何购买、安装、登录、启动。
- 不会 Codex 的用户如何完成第一条任务。
- 管理员如何配置价格、套餐、模型和额度。
- 常见状态：余额不足、套餐过期、设备撤销、模型不可用。
- 工业级架构：Control Plane、Data Plane、Client Runtime、Platform Ops。
- 生产运维：可观测、灰度发布、配置回滚、用量对账和风控审计。

## 解耦要求

- 文案不承诺客户端内置固定价格或固定模型。
- 所有可变销售内容指向后台配置。
- HTML 展示页必须与 README 和 `ARCHITECTURE.md` 保持一致，不能单独发明架构边界。

## 禁止改动范围

- 不修改源码。
- 不发布不合规宣传语。
- 不把内部风控规则、上游供应商凭证或真实成本结构写入公开用户文档。

## 测试要求

- 文档步骤能按测试环境走通。
- 文案与 UI 状态一致。
- HTML 展示页包含安装辅助和解耦说明。
- 文档与 HTML 的阶段数量、任务链接、架构术语一致。
- 发布说明包含已知风险、回滚路径和兼容影响。

## 交付物

- 用户指南。
- 管理员指南。
- 产品展示文案。
- 发布说明草稿。
- 工业级架构说明同步记录。

## Release evidence lane

- Final release evidence must include `test-runs/YYYYMMDD-HHMM-docs`.
- The Docs product copy evidence folder must be verified with `tools/verify-07-docs-product-copy-evidence.ps1`.
- Aggregate release evidence, release coverage, readiness, handoff and Module J final-report verification are expected to require Docs product copy evidence before any final go/go-with-risks recommendation.
- Current static docs may still contain draft or pending boundaries. They are useful staging artifacts, but real release docs evidence remains pending until final public copy, release notes, HTML sync evidence and visual/manual review evidence are finalized.
