# Parallel Dispatch Plan

本文档用 `parallel-module-development` 口径把 `00` 到 `07` 阶段改造成可派发的多会话开发计划。默认原则是依赖感知并行，不追求所有任务同时开跑。

## Dispatch Rules

- `00-contract` 是硬门禁；除只读调研外，所有实现 worker 都必须等待对应契约冻结。
- 同一阶段内只有写入范围互不重叠的模块可以并行。
- 中央路由、schema/migration、setting service、gateway hooks、lockfile、全局 UI 入口必须有单一 owner。
- 高风险实现 worker 必须遵守 [MODEL-QUALITY-POLICY.md](MODEL-QUALITY-POLICY.md)，不得把 Tier 1 代码编辑交给低能力/轻量模型。
- 后续 worker 可以基于 mock/stub 提前开发，但不得修改契约生产文件。
- 集成会话不做新功能，只合并、验证、修小型适配问题和更新下游提示词。

## Module DAG

```text
A Contracts
  -> C Config Registry
  -> D Client API
  -> F Desktop Runtime
  -> G Desktop UX
  -> J Integration

A Contracts
  -> E Gateway Policy Enforcement
  -> J Integration

B Storage and Migration Decision
  -> C Config Registry
  -> D Client API
  -> E Gateway Policy Enforcement
  -> H Admin Operations
  -> J Integration

C Config Registry
  -> D Client API
  -> E Gateway Policy Enforcement
  -> H Admin Operations

D Client API
  -> F Desktop Runtime
  -> G Desktop UX
  -> I E2E Release Gate

E Gateway Policy Enforcement
  -> I E2E Release Gate

F Desktop Runtime
  -> G Desktop UX
  -> I E2E Release Gate

H Admin Operations
  -> I E2E Release Gate

I E2E Release Gate
  -> J Integration
```

## Modules

| Module | Name | Primary output | Depends on | Safe parallel notes |
| --- | --- | --- | --- | --- |
| A | Contracts | OpenAPI, config schema, status/errors, mocks, events | none | Can run with B if storage decisions are coordinated daily. |
| B | Storage and migration decision | table/JSON storage plan, migration notes, unique constraints | none | Must finish before DB-affecting implementation. |
| C | Config Registry | PlanCatalog, ModelCatalog, UsagePolicy, FeatureFlags backend services | A, B | Can run with D only if D consumes stable interfaces and avoids config storage files. |
| D | Client API | `/api/v1/client/*` handlers/services/routes | A, B, C partial | Should own client routes; avoid gateway hooks. |
| E | Gateway Enforcement | model/entitlement/usage policy rejection in gateway path | A, B, C | Owns gateway policy hooks; avoid client API route files. |
| F | Desktop Runtime | bootstrap consumer, session/cache, managed provider writer, redaction, Tauri command contract | A, D mocks | Follow `PHASE1-MODULE-F-DESKTOP-RUNTIME-PLAN.md`; can start with mocks after A, final integration waits for D. |
| G | Desktop UX | cloud home route, login binding, install assistant, tutorial, user-safe status copy | A, D mocks, F | Follow `PHASE1-MODULE-G-DESKTOP-UX-PLAN.md`; can prototype against fixtures, final integration waits for F command payloads. |
| H | Admin Operations | admin config publish/rollback, plan/model/usage/flag management, user entitlement/device view | A, B, C | Follow `PHASE1-MODULE-H-ADMIN-OPERATIONS-PLAN.md`; compose existing payment/group/subscription APIs. |
| I | E2E Release Gate | buy/open entitlement -> login -> launch checklist/script, evidence folder, go/no-go input | D, E, F, G, H | Follow `PHASE1-MODULE-I-E2E-RELEASE-GATE-PLAN.md`; read-only prep can start early, final tests wait for implementation. |
| J | Integration Coordinator | merges, conflict resolution, final verification, docs/HTML sync, release readiness report | all | Follow `PHASE1-MODULE-J-INTEGRATION-COORDINATOR-PLAN.md`; never runs broad feature work in parallel with modules. |

## Phase Plan

### MVP Phase 0: Discovery and Contract Gate

Run:

- Module A: Contracts.
- Module B: Storage and migration decision.
- Coordinator: update `FILE-OWNERSHIP-MATRIX.md` with any new hotspot found.

Do not run:

- Backend route implementation.
- Desktop provider writer implementation.
- Admin UI implementation.

Exit gate:

- `CONTRACT-GATE.md` checklist complete.
- Storage decision recorded.
- Worker prompts updated with exact contract paths.

### MVP Phase 1: Backend Foundations

Run after Phase 0:

- Module C1: Config Registry service and validation for `codexplus_config_v1`.
- Module C2: Entitlement/device/Key foundation for `codexplus_devices`, `codexplus_managed_provider_keys` and `codexplus_events`.

Can run in parallel only if:

- C1 owns config storage/service files.
- C2 owns device/Key/entitlement files.
- Both avoid central route registries until Module D.

Exit gate:

- Config defaults and validation tests pass.
- Device upsert, managed provider Key reuse/create and event-write tests pass.
- No route or frontend worker depends on provisional storage names.

### MVP Phase 2: Backend API and Gateway

Run after Phase 1:

- Module D: Client API handlers/routes/services, following `PHASE1-MODULE-D-CLIENT-API-PLAN.md`.
- Module E: Gateway policy enforcement and usage/rejection events, following `PHASE1-MODULE-E-GATEWAY-ENFORCEMENT-PLAN.md`.

Can run in parallel only if:

- Module D owns `/api/v1/client/*` routes and handlers.
- Module E owns gateway hooks and policy resolver.
- Shared policy decision interface is frozen before both start.

Exit gate:

- Bootstrap/usage/device/redeem contract tests pass.
- Gateway rejects unauthorized model, expired, low balance, revoked device and rate-limited states.
- Logs and events are redacted.

### MVP Phase 3: Desktop Runtime and Admin Operations

Run after Phase 2 contracts are available; desktop can start earlier with mocks:

- Module F: Desktop runtime, following `PHASE1-MODULE-F-DESKTOP-RUNTIME-PLAN.md`.
- Module H: Admin operations, following `PHASE1-MODULE-H-ADMIN-OPERATIONS-PLAN.md`.

Can run in parallel because:

- Desktop writes `CodexPlusPlus-main`.
- Admin writes `sub2api-main`.
- Both consume backend contracts and avoid modifying them.
- Module F owns cloud runtime command shapes consumed later by Module G.
- Module H owns admin Codex++ facade/page and must not duplicate payment or entitlement systems.

Exit gate:

- Desktop can write `Codex++ Cloud` without deleting manual providers.
- Admin can change default model/plan/flags and inspect user entitlement.
- Typecheck and targeted tests pass.

### MVP Phase 4: Desktop UX and E2E Prep

Run:

- Module G: Desktop UX shell, following `PHASE1-MODULE-G-DESKTOP-UX-PLAN.md`.
- Module I: E2E release gate prep, following `PHASE1-MODULE-I-E2E-RELEASE-GATE-PLAN.md`.

Can run in parallel if:

- Module I only prepares checklist/scripts and does not edit runtime/UI code.
- Module G consumes frozen statuses and mock fixtures.
- Module G consumes Module F Tauri command payloads through a typed adapter.
- Module I names required owner/module for every step and records blockers rather than weakening the gate.

Exit gate:

- UI covers success, not purchased, expired, low balance, revoked device, gateway unhealthy and local config failed.
- E2E checklist is executable against test environment.

### MVP Phase 5: Integration and Release

Run:

- Module J only.
- Module J follows `PHASE1-MODULE-J-INTEGRATION-COORDINATOR-PLAN.md`.

Merge order:

1. A contracts and B storage plan.
2. C backend foundations.
3. D client API.
4. E gateway enforcement.
5. F desktop runtime.
6. H admin operations.
7. G desktop UX.
8. I E2E release artifacts.
9. Docs and HTML sync.
10. Final report and go/no-go recommendation.

Exit gate:

- Full test plan from `MVP-IMPLEMENTATION-PLAN.md` is run or explicitly marked unavailable.
- Release notes include config version, compatibility impact, rollback path and known risks.
- Module I evidence folder and Module J final report are both present.

## Worktree Strategy

This plan directory is not a source repo. In the current desktop workspace the source roots are:

- `C:\Users\23293\Desktop\codex+++\sub2api-main`
- `C:\Users\23293\Desktop\codex+++\CodexPlusPlus-main`
- `C:\Users\23293\Desktop\codex+++\codex-plus-dev-plan`

If these extracted folders are not git repositories, initialize a clean baseline or re-clone the official repositories before broad multi-session development. Create parallel worktrees or working copies beside the workspace, not inside the source repos.

Suggested roots:

```powershell
# Backend/admin repo
cd C:\Users\23293\Desktop\codex+++\sub2api-main
git status --short
git branch --show-current

# Desktop repo
cd C:\Users\23293\Desktop\codex+++\CodexPlusPlus-main
git status --short
git branch --show-current
```

Suggested branch naming:

- `codex/codexplus-mvp-contracts`
- `codex/codexplus-mvp-storage`
- `codex/codexplus-mvp-config`
- `codex/codexplus-mvp-client-api`
- `codex/codexplus-mvp-gateway`
- `codex/codexplus-mvp-desktop-runtime`
- `codex/codexplus-mvp-desktop-ux`
- `codex/codexplus-mvp-admin`
- `codex/codexplus-mvp-e2e`
- `codex/codexplus-mvp-integration`

Do not create worktrees inside an existing source repo. Use a sibling folder such as `C:\Users\23293\Desktop\codex+++-parallel\sub2api-main-*` and `C:\Users\23293\Desktop\codex+++-parallel\CodexPlusPlus-main-*`.

## Dispatch Gate Checklist

Before starting any worker:

- [ ] Worker has one module from this plan.
- [ ] Worker has a branch/worktree path.
- [ ] Worker prompt states its model quality tier from `MODEL-QUALITY-POLICY.md`.
- [ ] Worker prompt names owned files and forbidden files.
- [ ] Worker prompt includes contract inputs and outputs.
- [ ] Worker prompt includes verification commands.
- [ ] Worker prompt says to stop on missing upstream contract or forbidden file.
- [ ] Coordinator knows merge order and conflict owner.

## Integration Gate Checklist

Before merging each module:

- [ ] Declared dependencies were satisfied or assumptions are documented.
- [ ] Forbidden files were not edited.
- [ ] Expected contract outputs were produced.
- [ ] No real secrets or unredacted keys were added.
- [ ] Package manifests/lockfiles changed only if module owned dependency updates.
- [ ] Downstream prompts were updated if contract fields changed.
- [ ] Smallest meaningful tests were run.

## When to Reduce Concurrency

Reduce to one active implementation worker when:

- API response fields are still changing.
- DB/schema/storage decision is unresolved.
- Two modules need `setting_service.go`, `routes/*.go`, `gateway.go`, `App.tsx`, global CSS, or lockfiles.
- Work touches payment callbacks, refunds, credits, production credentials or irreversible ledger writes.
- A failing backend test invalidates frontend assumptions.

## Coordinator Responsibilities

- Own `CONTRACT-GATE.md`, `FILE-OWNERSHIP-MATRIX.md`, `WORKER-PROMPTS.md` and this file.
- Approve any field, route, table, env var or event name change after Phase 0.
- Review worker final reports before starting dependent modules.
- Merge in planned order and run verification after risky merges.
- Keep MVP scope aligned with `MVP-IMPLEMENTATION-PLAN.md`.
