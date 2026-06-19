# 07-integration-release 集成发布阶段

## 当前状态

- Status: active
- Previous gate: `06-commerce-and-enforcement` passed。
- Current gate: 按 `INTEGRATION-VERIFICATION-CHECKLIST.md`、`PHASE1-MODULE-I-E2E-RELEASE-GATE-PLAN.md` 和 `PHASE1-MODULE-J-INTEGRATION-COORDINATOR-PLAN.md` 收集 E2E、兼容迁移、安装包、Docs product copy、business readiness 和文档同步证据。
- Exit condition: 四个并行验收 worker 返回 final report；Module I E2E/package/compatibility 证据目录完整，Docs product copy 证据目录 `test-runs/YYYYMMDD-HHMM-docs` 通过 `tools/verify-07-docs-product-copy-evidence.ps1`，并由 `tools/verify-07-release-evidence.ps1` 聚合卫生门禁消费；business readiness 证据通过 `tools/verify-07-business-readiness.ps1`；coverage/readiness summary 已生成且一致并包含 Docs product copy evidence，且 readiness summary 记录 coverage verification passed；timestamped `*-release` handoff workspace 通过 `tools/verify-07-release-handoff.ps1`；Module J final report 通过 `tools/verify-07-module-j-report.ps1 -CoverageSummaryFile ... -ReadinessSummaryFile ...` 并给出 go/no-go 与回滚说明。

## 本阶段目标

完成从购买、登录、自动配置到启动 Codex 的端到端验收，确认旧用户兼容、安装包可用、文档和产品文案同步。

最终验收必须按 [INTEGRATION-VERIFICATION-CHECKLIST.md](../INTEGRATION-VERIFICATION-CHECKLIST.md) 执行。E2E 证据按 [PHASE1-MODULE-I-E2E-RELEASE-GATE-PLAN.md](../PHASE1-MODULE-I-E2E-RELEASE-GATE-PLAN.md) 组织，最终合并和发布裁决按 [PHASE1-MODULE-J-INTEGRATION-COORDINATOR-PLAN.md](../PHASE1-MODULE-J-INTEGRATION-COORDINATOR-PLAN.md) 执行。集成会话只负责合并、验证、修小型适配问题和同步文档，不再承接新的大功能实现。

完整发布证据工作区可用 `tools/new-07-release-evidence-set.ps1` 创建；最终 release evidence set 会扩展为同一 run stamp 下的 `YYYYMMDD-HHMM-e2e`、`YYYYMMDD-HHMM-package`、`YYYYMMDD-HHMM-compatibility`、`YYYYMMDD-HHMM-docs`、`YYYYMMDD-HHMM-business` 和 `YYYYMMDD-HHMM-release`。生成物仍是待填 scaffold，不能替代真实执行证据、Docs product copy finalization、business/legal approval 或最终 go/no-go。`tools/verify-07-release-handoff.ps1 -AllowGoCandidate` 只用于真实外部证据齐全后的最终 Module J candidate handoff；scaffold 或 no-go handoff 检查必须省略该开关。

Business/legal approval remains an owner-controlled boundary: verifiers may require the phrase `business/legal approval`, but only explicit owner approval can convert that evidence lane from no-go to go or go-with-accepted-risks.

执行者接手某个 run stamp 前，可先用 `tools/report-07-release-gaps.ps1` 生成只读缺口报告，确认同一 run stamp 下 E2E、package、compatibility、Docs product copy、business readiness 和 release handoff sibling dirs、关键文件与 verify 命令是否齐全：

```text
powershell -ExecutionPolicy Bypass -File codex-plus-dev-plan/tools/report-07-release-gaps.ps1 -RunStamp YYYYMMDD-HHMM
```

## 工业级 v2 映射

- 架构层：`Platform Ops`。
- 覆盖范围：契约测试、E2E、兼容迁移、安装包检查、可观测、灰度发布、烟测、回滚和文档同步。
- 关键原则：发布验收不只验证功能可用，还要验证可定位、可回滚、可迁移。

## 并行任务列表

- `task-e2e-buy-login-launch.md`
- `task-compatibility-and-migration.md`
- `task-package-install-check.md`
- `task-docs-and-product-copy.md`

## 前置依赖

- `06-commerce-and-enforcement` 完成。
- 客户端和后台已能在测试环境联通。

## 合并顺序

1. contracts/storage/config/client API/gateway/runtime/admin/UX 各模块按 [PARALLEL-DISPATCH-PLAN.md](../PARALLEL-DISPATCH-PLAN.md) 完成并提交 final report。
2. compatibility and migration。
3. e2e buy login launch。
4. package install check。
5. docs and product copy final report plus `YYYYMMDD-HHMM-docs` evidence verified by `tools/verify-07-docs-product-copy-evidence.ps1`。
6. Module I evidence folder and release gate report。
7. Phase 9 business readiness evidence。
8. Release coverage summary and readiness summary, including Docs product copy evidence。
9. Timestamped release handoff verification。
10. Module J final integration report and rollback notes。

## 阶段验收标准

- 新用户购买后登录即可启动 Codex。
- 旧用户手动供应商配置不丢失。
- 管理员改套餐、模型、额度后客户端无需发版即可体现。
- 安装包、说明文档和 HTML 说明书同步。
- Docs product copy evidence folder exists as `test-runs/YYYYMMDD-HHMM-docs` and passes `tools/verify-07-docs-product-copy-evidence.ps1`; current static docs remain draft/pending until that release evidence is finalized.
- 契约测试、网关拒绝、用量对账、配置回滚和设备撤销都有 E2E 或烟测覆盖。
- 发布说明包含配置版本、兼容影响、回滚路径和已知风险。
- Module I 输出 E2E 证据目录和 release gate decision。
- Business readiness evidence covers production values, launch business decisions, security/compliance/legal/privacy/payment terms, observability, cost/abuse controls and paid-user support ownership.
- Coverage summary and readiness summary are generated from final evidence, readiness summary consumes the coverage summary, and both remain consistent with the final handoff workspace.
- Module J 输出最终集成报告、冲突处理记录和 go/no-go 推荐。
- 集成报告列出每个模块的合并结果、冲突处理、验证命令、E2E 结果和剩余风险。
- 已确认没有 worker 修改 forbidden 文件、引入 secrets、绕过后端权益或删除旧手动供应商。
