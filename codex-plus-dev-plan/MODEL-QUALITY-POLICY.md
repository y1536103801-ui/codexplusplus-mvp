# Model Quality Policy

本文档用于防止并行 workflow 或 subagent 因自动选择轻量模型而影响核心开发质量。它不替代 Codex 全局配置；它是本项目派工时必须写进 worker prompt 的质量约束。

## Why This Exists

Codex subagent workflow 可以让不同 agent 使用不同模型和推理强度。如果没有明确指定，Codex 可能根据任务在智能、速度和成本之间自动平衡；这适合轻量扫描，但不适合支付、鉴权、网关、契约和集成这类高风险开发。

因此，本项目采用显式模型质量分层：

- 高风险实现必须使用高能力模型和较高推理强度。
- 只读探索、日志整理、文档摘要可以使用更快、更便宜的模型。
- worker prompt 必须写清楚该模块的 model tier。

## Model Tiers

| Tier | Use for | Model guidance | Reasoning guidance |
| --- | --- | --- | --- |
| Tier 1: critical | contracts, storage decisions, auth, entitlement, payment, gateway enforcement, desktop provider write, integration | Use the strongest recommended Codex model available in the current environment, for example `gpt-5.5` when available | `high` for architecture/security/integration; `medium` minimum for implementation |
| Tier 2: standard | admin UI, ordinary backend handlers, desktop UX, focused tests | Use strong coding model; avoid mini unless the task is narrow and low-risk | `medium` |
| Tier 3: support | read-only exploration, log summarization, test-output triage, docs formatting | Faster/lower-cost model is acceptable, for example mini-class models | `low` or `medium` |

If the current environment does not expose exact model selection, the prompt must still say the intended tier and ask Codex not to delegate critical edits to a lower-capability worker.

## Module Policy

| Module | Required tier | Notes |
| --- | --- | --- |
| A Contracts | Tier 1 | Field contracts become source of truth for all consumers. |
| B Storage and migration decision | Tier 1 | Wrong storage decisions create expensive rewrites. |
| C Config Registry | Tier 1 | Config defaults and validation affect billing/entitlement behavior. |
| D Client API | Tier 1 | Auth, Key creation and bootstrap snapshots are sensitive. |
| E Gateway Enforcement | Tier 1 | Server-side policy rejection must be correct. |
| F Desktop Runtime | Tier 1 | Provider write must preserve manual configs and protect secrets. |
| G Desktop UX | Tier 2 | Can use Tier 1 if also editing command contracts or state machines. |
| H Admin Operations | Tier 1 for backend, Tier 2 for UI | Admin writes affect production behavior. |
| I E2E Release Gate | Tier 1 for final release judgment, Tier 3 for read-only checklist prep | Final readiness must be conservative. |
| J Integration | Tier 1 | Merge/conflict decisions are high risk. |

## Prompt Requirements

Every worker prompt must include:

```text
Model quality requirement:
- This module is Tier <1/2/3>.
- Do not use a lower-capability model for code edits in this module.
- Use high reasoning for contract, auth, entitlement, gateway, provider-write, migration, or integration decisions.
- If your current agent/model cannot satisfy this tier, stop and report instead of implementing.
```

## Delegation Rules

- Tier 1 workers may delegate only read-only support tasks to lighter agents.
- Tier 1 code edits must stay in the Tier 1 worker or another explicitly Tier 1 worker.
- Tier 2 workers may use Tier 3 helpers for scanning and summarizing, but must make final code decisions themselves.
- Tier 3 workers must not edit source code unless the coordinator explicitly upgrades them.

## Quality Gate

Before accepting a worker report, the coordinator checks:

- [ ] The worker stated its model tier.
- [ ] The worker did not delegate Tier 1 code edits to a lower-tier agent.
- [ ] High-risk decisions include reasoning notes and test evidence.
- [ ] Any model/config limitation was reported.
- [ ] Final verification was run or blocked with a clear reason.

## Custom Agent File Guidance

When using Codex custom agents, create separate agent files for critical and support roles:

```toml
name = "codexplus-critical-worker"
description = "High-rigor worker for Codex++ contracts, backend, gateway, desktop runtime, and integration."
model = "gpt-5.5"
model_reasoning_effort = "high"
developer_instructions = """
Use conservative engineering judgment. Do not invent contract fields. Stop on missing upstream contracts, forbidden files, auth/payment/entitlement ambiguity, or secret exposure risk.
"""
```

```toml
name = "codexplus-support-explorer"
description = "Read-only explorer for file inventory, logs, test output, and documentation summaries."
model = "gpt-5.4-mini"
model_reasoning_effort = "medium"
developer_instructions = """
Read and summarize only. Do not edit source code unless the coordinator explicitly upgrades this task.
"""
```

If these model names are unavailable in the current Codex environment, use the closest available model matching the same quality tier and record the substitution in the worker final report.
