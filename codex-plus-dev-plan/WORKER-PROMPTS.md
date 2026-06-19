# Worker Prompts

本文件提供可复制给多个 Codex 会话的模块提示词。使用前把 `Workspace` 调整为实际 worktree 绝对路径，并确认该模块的依赖 gate 已满足。

## Current Correction Notice

当前 `00-contract` 已重新打开。不要直接使用下方旧的 Module A 总包提示词启动一个大 worker；请使用 [00-contract/PARALLEL-RESTART-PACK.md](00-contract/PARALLEL-RESTART-PACK.md) 中的 A/B/C 三个并行 worker 提示词。

在 `STAGE-GATE-LEDGER.md` 将 `00-contract` 标记为 passed 之前，Module C 到 Module J 的实现提示词只能作为后续参考，不得派发执行。

通用要求：

- 读取 `MVP-IMPLEMENTATION-PLAN.md`、`CONTRACT-GATE.md`、`PARALLEL-DISPATCH-PLAN.md`、`FILE-OWNERSHIP-MATRIX.md` 后再开始。
- 读取 `MODEL-QUALITY-POLICY.md`，并在开工前确认本模块的 model tier。
- 不要修改 forbidden 文件。
- 不要发明契约字段。
- 不要提交真实 API Key、JWT、Authorization header、上游凭证或 `.env` secrets。
- 不要执行 destructive git commands。
- 最终报告必须列出 changed files、verification、contract assumptions、integration risks。

## Module A: Contracts

```text
You are responsible for Module A: Contracts.

Workspace:
C:\Users\23293\Desktop\codex+++-parallel\codexplus-mvp-contracts

Branch:
codex/codexplus-mvp-contracts

Phase:
MVP Phase 0: Discovery and Contract Gate (re-opened; use 00-contract/PARALLEL-RESTART-PACK.md for current dispatch)

Objective:
Freeze the MVP contracts for Codex++ client APIs, admin config, statuses/errors, events, and mock fixtures so backend, desktop, and admin workers do not invent fields independently.

Model quality requirement:
- This module is Tier 1.
- Do not use a lower-capability model for contract decisions or contract edits.
- Use high reasoning for API shape, config schema, event schema, status/error, compatibility and downstream impact decisions.
- If your current agent/model cannot satisfy this tier, stop and report instead of implementing.

Dependencies:
- No hard dependency.
- Read-only dependency on the current dev plan docs.

Gate conditions:
- You may edit only contract/doc artifacts.
- Stop and report if a field requires a storage decision that Module B has not made.

Contract inputs:
- MVP scope from MVP-IMPLEMENTATION-PLAN.md.
- Required gate checklist from CONTRACT-GATE.md.

Contract outputs:
- Client OpenAPI for bootstrap, usage, devices, redeem.
- JSON schema for PlanCatalog, ModelCatalog, UsagePolicy, FeatureFlags.
- Status/error table.
- Event schemas.
- Mock fixtures for success and failure states.
- Compatibility/change policy notes.

Owned scope:
- codex-plus-contracts/api/**
- codex-plus-contracts/config/**
- codex-plus-contracts/events/**
- codex-plus-contracts/status-error/**
- codex-plus-contracts/test-fixtures/**
- codex-plus-contracts/compatibility-matrix.md
- codex-plus-contracts/change-review-policy.md

Avoid:
- sub2api-main/**
- CodexPlusPlus-main/**
- Payment, gateway, desktop runtime implementation.

Requirements:
- Include config_version and snapshot_version where required.
- Include redaction rules for API Key, JWT, Authorization header, and Base URL query tokens.
- Provide mocks that desktop and UI workers can use before backend implementation.

Safety constraints:
- Do not include real secrets or real user data.
- Do not call external services.
- Do not change implementation files.

Verification:
- Validate YAML/JSON syntax where practical.
- Confirm every required item in CONTRACT-GATE.md is represented.

Final response:
- Summarize contract artifacts created or updated.
- List exact contract outputs downstream modules must consume.
- List unresolved storage assumptions for Module B/coordinator.
```

## Module B: Storage and Migration Decision

```text
You are responsible for Module B: Storage and Migration Decision.

Workspace:
C:\Users\23293\Desktop\codex+++-parallel\sub2api-main-storage

Branch:
codex/codexplus-mvp-storage

Phase:
MVP Phase 0: Discovery and Contract Gate

Objective:
Inspect Sub2API storage patterns and decide where Codex++ config, devices, entitlement overlays, usage events, and idempotency keys should live. The current candidate decision is recorded in `codex-plus-contracts/storage-decision.md`, but downstream implementation still waits for the active `00-contract` correction gate.

Model quality requirement:
- This module is Tier 1.
- Do not use a lower-capability model for storage, migration, entitlement or idempotency decisions.
- Use high reasoning for old-data compatibility, rollback and unique-constraint decisions.
- If your current agent/model cannot satisfy this tier, stop and report instead of implementing.

Dependencies:
- Soft dependency on Module A draft contracts.
- Read-only access to sub2api-main backend patterns.

Gate conditions:
- Do not implement migrations unless explicitly asked after the decision is approved.
- Stop and report if existing data model makes the MVP contract unsafe or contradictory.

Contract inputs:
- PlanCatalog, ModelCatalog, UsagePolicy, FeatureFlags shape.
- Device and entitlement requirements.
- Idempotency requirements for Key creation, device upsert, redeem, and future payment fulfillment.

Contract outputs:
- Storage decision doc.
- Migration/rollback notes.
- Unique constraints/idempotency strategy.
- Old-data defaulting strategy.
- Repository/service ownership notes for Modules C/D/E/H.
- Frozen MVP decisions: `codexplus_config_v1`, `codexplus_devices`, `codexplus_managed_provider_keys`, and `codexplus_events`.

Owned scope:
- Storage decision markdown under codex-plus-contracts or platform docs.
- Read-only inspection of backend/internal/repository/**
- Read-only inspection of backend/ent/** or generated schema area.

Avoid:
- backend/internal/server/routes/**
- backend/internal/service/payment_* implementation changes.
- Desktop client files.

Requirements:
- Decide setting JSON vs typed tables vs hybrid storage.
- Define how config versions are stored.
- Define how device ownership is enforced.
- Define how revoked devices affect bootstrap.
- Define rollback path for any schema change.

Safety constraints:
- Do not run destructive DB commands.
- Do not edit production config or secrets.

Verification:
- No implementation verification required if plan-only.
- If you create schema stubs, run relevant Go generation/tests only if safe.

Final response:
- Summarize storage decision.
- List required migrations or no-migration path.
- List risks and modules blocked by unresolved decisions.
```

## Module C: Config Registry

```text
You are responsible for Module C: Config Registry.

Workspace:
C:\Users\23293\Desktop\codex+++-parallel\sub2api-main-config

Branch:
codex/codexplus-mvp-config

Phase:
MVP Phase 1: Backend Foundations

Objective:
Implement MVP backend configuration services for PlanCatalog, ModelCatalog, UsagePolicy, and FeatureFlags using the frozen contracts and storage decision.

Model quality requirement:
- This module is Tier 1.
- Do not use a lower-capability model for backend config service edits.
- Use high reasoning for validation, defaults, config versioning and entitlement-impact decisions.
- If your current agent/model cannot satisfy this tier, stop and report instead of implementing.

Dependencies:
- Hard dependency on Module A config schemas.
- Hard dependency on Module B storage decision in `codex-plus-contracts/storage-decision.md`.
- Hard dependency on `codex-plus-dev-plan/PHASE1-MODULE-C-BACKEND-FOUNDATION-PLAN.md`.

Gate conditions:
- Do not start implementation until config schemas and storage path are frozen.
- Stop if you need to rename contract fields.

Contract inputs:
- PlanCatalog schema.
- ModelCatalog schema.
- UsagePolicy schema.
- FeatureFlags schema.
- Config version/change metadata rules.
- MVP storage key: `settings.key = "codexplus_config_v1"`.
- Backend foundation file/schema/test blueprint from `PHASE1-MODULE-C-BACKEND-FOUNDATION-PLAN.md`.

Contract outputs:
- Backend read/write or read-only service interface for config.
- Validation behavior for invalid plans, default model, usage limits, and feature flags.
- Defaults for missing old config fields.

Owned scope:
- backend/internal/service/setting.go
- backend/internal/service/setting_service.go only in Codex++ config sections
- backend/internal/codexplus/controlplane/config_registry/** if adopted
- backend/internal/service/*codexplus*config*.go if additive files are used
- backend tests beside changed service files

Avoid:
- backend/internal/server/routes/user.go
- backend/internal/server/routes/gateway.go
- Payment fulfillment/refund code.
- Desktop client files.
- Admin Vue UI unless explicitly split into Module H.

Requirements:
- Store the current Codex++ config as one validated JSON document under `codexplus_config_v1`.
- Increment `config_version` on every validated admin write.
- Treat rollback as a new validated write with `rollback_from`.
- Validate duplicate plan IDs, negative prices, empty model groups, disabled default model, invalid rate limits.
- Return safe defaults for missing old fields.
- Expose a stable read interface for Module D and Module E.

Safety constraints:
- Do not expose real upstream credentials.
- Do not change unrelated settings behavior.

Verification:
- cd sub2api-main/backend && go test ./internal/service/...
- Run narrower tests if full package is too slow; report what ran.

Final response:
- Summarize config services and validations.
- List changed files.
- List verification commands/results.
- List contract outputs consumed by D/E/H.
```

## Module D: Client API

```text
You are responsible for Module D: Client API.

Workspace:
C:\Users\23293\Desktop\codex+++-parallel\sub2api-main-client-api

Branch:
codex/codexplus-mvp-client-api

Phase:
MVP Phase 2: Backend API and Gateway

Objective:
Implement /api/v1/client/bootstrap, usage, devices, and redeem endpoints according to the frozen contract.

Model quality requirement:
- This module is Tier 1.
- Do not use a lower-capability model for auth, Key creation, device upsert, bootstrap aggregation or redeem behavior edits.
- Use high reasoning for auth, user isolation, idempotency, redaction and contract compatibility decisions.
- If your current agent/model cannot satisfy this tier, stop and report instead of implementing.

Dependencies:
- Hard dependency on Module A client OpenAPI, status/errors, events, and mocks.
- Hard dependency on Module B storage decision.
- Hard or merge-gate dependency on Module C config read interface.
- Hard dependency on `codex-plus-dev-plan/PHASE1-MODULE-D-CLIENT-API-PLAN.md`.

Gate conditions:
- Do not implement fields missing from the OpenAPI/mocks.
- Stop if Key creation or device upsert cannot be made idempotent.

Contract inputs:
- Client OpenAPI.
- Status/error table.
- Event schema.
- Config read interface.
- Storage/idempotency decision.
- Client API route/handler/service blueprint from `PHASE1-MODULE-D-CLIENT-API-PLAN.md`.

Contract outputs:
- Backend handlers/services/routes for /api/v1/client/*.
- Bootstrap aggregation behavior.
- Device upsert behavior.
- Usage/redeem response mapping.

Owned scope:
- backend/internal/server/routes/user.go or additive client route file.
- backend/internal/handler/client/** or additive client handlers.
- backend/internal/service/api_key_service.go only for Codex++ Key reuse/create.
- backend/internal/service/redeem_service.go only for client redeem mapping.
- Backend tests for client API.

Avoid:
- backend/internal/server/routes/gateway.go
- backend/internal/service/billing_* gateway enforcement.
- Admin UI.
- Desktop client.

Requirements:
- Require user JWT.
- Do not leak upstream real credentials.
- Reuse existing `api_keys` for user-side gateway key material.
- Use `codexplus_managed_provider_keys` as the source of truth for `user_id + managed_provider_id` idempotency.
- Return `provider.api_key` as the full user-side gateway key only in successful authenticated bootstrap responses; redact it everywhere else.
- Use `codexplus_devices` for device upsert/status and `(user_id, device_id)` ownership.
- Redact full Key in logs.
- Produce structured events for bootstrap, device registration, usage, redeem.

Safety constraints:
- Do not change unrelated auth or admin routes.
- Do not bypass existing permission checks.

Verification:
- cd sub2api-main/backend && go test ./internal/server/... ./internal/handler/... ./internal/service/...
- Add focused tests for auth, no套餐/expired/low_balance/device_revoked, repeated bootstrap Key reuse, redaction.

Final response:
- Summarize endpoints implemented.
- List changed files.
- List verification commands/results.
- List any contract mismatches or downstream notes for F/G/I.
```

## Module E: Gateway Enforcement

```text
You are responsible for Module E: Gateway Policy Enforcement.

Workspace:
C:\Users\23293\Desktop\codex+++-parallel\sub2api-main-gateway

Branch:
codex/codexplus-mvp-gateway

Phase:
MVP Phase 2: Backend API and Gateway

Objective:
Enforce Codex++ model permission, balance/entitlement, device revocation, and rate/usage policy in the Sub2API gateway path.

Model quality requirement:
- This module is Tier 1.
- Do not use a lower-capability model for gateway enforcement edits.
- Use high reasoning for policy execution, billing/usage side effects, rejection behavior and valid-request preservation.
- If your current agent/model cannot satisfy this tier, stop and report instead of implementing.

Dependencies:
- Hard dependency on Module A status/errors and event schema.
- Hard dependency on Module B storage decision.
- Hard dependency on Module C policy/config read interface.
- Hard dependency on `codex-plus-dev-plan/PHASE1-MODULE-E-GATEWAY-ENFORCEMENT-PLAN.md`.

Gate conditions:
- Do not start until policy decision input shape is frozen.
- Stop if enforcement would rely on client UI state.

Contract inputs:
- ModelCatalog and UsagePolicy schema.
- Entitlement/device storage decision.
- Gateway rejection error codes.
- Usage/rejection event schema.
- Gateway enforcement blueprint from `PHASE1-MODULE-E-GATEWAY-ENFORCEMENT-PLAN.md`.

Contract outputs:
- Policy resolver interface.
- Gateway enforcement hooks.
- Rejection event and usage event behavior.
- Tests for allowed and denied requests.

Owned scope:
- backend/internal/server/routes/gateway.go
- backend/internal/service/ratelimit_service.go only for Codex++ policy integration.
- backend/internal/service/billing_* only for MVP enforcement/usage integration.
- backend/internal/codexplus/dataplane/policy_enforcement/** if adopted.
- Gateway tests.

Avoid:
- /api/v1/client route files.
- Admin UI.
- Desktop runtime.
- Payment/refund behavior unless explicitly approved.

Requirements:
- Reject unauthorized model.
- Reject expired or disabled entitlement.
- Reject insufficient balance/low balance according to MVP policy.
- Reject revoked/blocked device when device context is available.
- Keep valid requests forwarding and billing normally.
- Record structured rejection reasons.

Safety constraints:
- Do not alter unrelated gateway behavior for non-Codex++ paths.
- Do not log full API keys or upstream credentials.

Verification:
- cd sub2api-main/backend && go test ./internal/server/routes/... ./internal/service/...
- Include tests for denied and allowed paths.

Final response:
- Summarize enforcement points.
- List changed files.
- List verification commands/results.
- List any policy assumptions for admin and E2E workers.
```

## Module F: Desktop Runtime

```text
You are responsible for Module F: Desktop Runtime.

Workspace:
C:\Users\23293\Desktop\codex+++-parallel\CodexPlusPlus-main-runtime

Branch:
codex/codexplus-mvp-desktop-runtime

Phase:
MVP Phase 3: Desktop Runtime and Admin Operations

Objective:
Implement the Codex++ desktop runtime that logs in, fetches bootstrap, caches minimal session state, writes the managed Codex++ Cloud provider, and redacts sensitive logs.

Model quality requirement:
- This module is Tier 1.
- Do not use a lower-capability model for provider write, local secret handling, session/cache or redaction edits.
- Use high reasoning for preserving manual providers, config safety and local error mapping.
- If your current agent/model cannot satisfy this tier, stop and report instead of implementing.

Dependencies:
- Hard dependency on Module A client API mocks and status/error table.
- Merge-gate dependency on Module D real API.
- Hard dependency on `codex-plus-dev-plan/PHASE1-MODULE-F-DESKTOP-RUNTIME-PLAN.md`.

Gate conditions:
- You may develop against mocks before Module D is merged.
- Stop if provider write would delete or overwrite manual providers.

Contract inputs:
- bootstrap response mock.
- device API contract.
- status/error table.
- local redaction rules.
- Desktop runtime/provider writer blueprint from `PHASE1-MODULE-F-DESKTOP-RUNTIME-PLAN.md`.

Contract outputs:
- Tauri/Rust commands consumed by Module G.
- Provider write behavior.
- Session/cache data shape.
- Local error mapping.

Owned scope:
- crates/codex-plus-core/src/codexplus_cloud/**
- crates/codex-plus-core/src/lib.rs only to export the new module
- apps/codex-plus-manager/src-tauri/src/cloud_commands.rs
- apps/codex-plus-manager/src-tauri/src/lib.rs only for command registration
- focused Rust tests beside new cloud modules

Minimal touch only if unavoidable:
- crates/codex-plus-core/src/settings.rs for managed-provider compatibility only
- crates/codex-plus-core/src/relay_config.rs or relay_switch.rs only to expose existing pure helpers
- crates/codex-plus-core/src/http_client.rs only for shared client helper reuse

Avoid:
- apps/codex-plus-manager/src/App.tsx except minimal command wiring agreed with Module G.
- UI styling.
- Backend repos.
- codex-plus-contracts/**
- Cargo.toml/Cargo.lock unless dependency change is approved by the integration owner.

Requirements:
- Preserve manual providers.
- Write/update only the managed Codex++ Cloud provider.
- Redact keys/JWT/Authorization in local logs and diagnostic export.
- Cache only minimum required login/device/bootstrap data.
- Map backend and local errors to user-safe statuses.
- Expose the Tauri commands listed in `PHASE1-MODULE-F-DESKTOP-RUNTIME-PLAN.md` or publish an approved replacement list before Module G starts final UI integration.

Safety constraints:
- Do not store upstream real credentials.
- Do not modify Codex official files beyond existing provider/config mechanism.

Verification:
- cd CodexPlusPlus-main && cargo test --workspace
- cd CodexPlusPlus-main/apps/codex-plus-manager && npm run check
- Run narrower checks if full build is unavailable.

Final response:
- Summarize runtime behavior.
- List changed files.
- List verification commands/results.
- List command names and response shapes for Module G.
```

## Module G: Desktop UX

```text
You are responsible for Module G: Desktop UX.

Workspace:
C:\Users\23293\Desktop\codex+++-parallel\CodexPlusPlus-main-ux

Branch:
codex/codexplus-mvp-desktop-ux

Phase:
MVP Phase 4: Desktop UX and E2E Prep

Objective:
Build the MVP Codex++ Cloud user experience: cloud home route, login binding, service dashboard, backend/runtime-provided status summaries, install assistant shell, new-user tutorial entry, and one-click launch.

Model quality requirement:
- This module is Tier 2.
- Do not use a lower-capability model for source edits; Tier 3 helpers may only do read-only exploration or fixture summaries.
- Use medium reasoning minimum; use high reasoning if changing command contracts or state machines.
- If your current agent/model cannot satisfy this tier, stop and report instead of implementing.

Dependencies:
- Hard dependency on Module A mocks/status table.
- Merge-gate dependency on Module F Tauri commands.
- Merge-gate dependency on Module D real backend.
- Hard dependency on `codex-plus-dev-plan/PHASE1-MODULE-G-DESKTOP-UX-PLAN.md`.

Gate conditions:
- Use mocks or command stubs until Module F/D outputs are merged.
- Stop if you need fields not present in contract mocks.

Contract inputs:
- bootstrap mock fixtures.
- status/error table.
- feature flag contract.
- Tauri command names from Module F.
- Desktop UX blueprint from `PHASE1-MODULE-G-DESKTOP-UX-PLAN.md`.

Contract outputs:
- UI states and components.
- Any UI-specific fixture gaps as patch notes to Module A/coordinator.

Owned scope:
- apps/codex-plus-manager/src/cloud/**
- apps/codex-plus-manager/src/App.tsx only for route/action/type wiring
- apps/codex-plus-manager/src/components/** only for additive reusable UI
- apps/codex-plus-manager/src/styles.css or src/cloud/cloud.css with scoped cloud UI changes
- UI tests if test setup exists

Avoid:
- Rust provider write logic.
- Backend repos.
- Package manifests/lockfiles.
- Hardcoded prices, model lists, quota thresholds, renewal copy.

Requirements:
- Cover available, not purchased, expired, low balance, device revoked, gateway unhealthy, local Codex missing, local config failed.
- Show backend-provided user-facing messages or local install diagnostics.
- Hide advanced provider config by default unless feature flag enables it.
- Keep manual provider advanced path accessible for advanced users.
- Do not hard-code prices, model availability, quotas, rate limits, model multipliers or renewal copy.
- Use animation only for real command progress, state transitions or repair progress; no decorative background motion.

Safety constraints:
- Do not display full API Key by default.
- Do not invent entitlement logic in frontend.
- Do not maintain a frontend model catalog for Codex++ Cloud.

Verification:
- cd CodexPlusPlus-main/apps/codex-plus-manager && npm run check
- Run UI build if practical: npm run vite:build

Final response:
- Summarize UI changes.
- List changed files.
- List verification commands/results.
- List command/contract assumptions for integration.
```

## Module H: Admin Operations

```text
You are responsible for Module H: Admin Operations.

Workspace:
C:\Users\23293\Desktop\codex+++-parallel\sub2api-main-admin

Branch:
codex/codexplus-mvp-admin

Phase:
MVP Phase 3: Desktop Runtime and Admin Operations

Objective:
Provide MVP admin controls for Codex++ plans, models, usage policy, feature flags, and user entitlement/device view.

Model quality requirement:
- This module is Tier 1 for backend/admin writes and Tier 2 for purely visual UI edits.
- Do not use a lower-capability model for backend validation, entitlement, permission or admin mutation behavior.
- Use high reasoning for backend changes and medium reasoning minimum for UI.
- If your current agent/model cannot satisfy this tier, stop and report instead of implementing.

Dependencies:
- Hard dependency on Module A config schemas.
- Hard dependency on Module B storage decision.
- Hard or merge-gate dependency on Module C config service.
- Hard dependency on `codex-plus-dev-plan/PHASE1-MODULE-H-ADMIN-OPERATIONS-PLAN.md`.

Gate conditions:
- Do not start backend writes until config service interface is frozen.
- Stop if admin UI would need to make pricing/entitlement decisions that backend does not own.

Contract inputs:
- PlanCatalog, ModelCatalog, UsagePolicy, FeatureFlags.
- User entitlement/device summary API shape.
- Admin permission patterns.
- Admin operations blueprint from `PHASE1-MODULE-H-ADMIN-OPERATIONS-PLAN.md`.

Contract outputs:
- Admin handlers/pages for MVP configuration and user support.
- Admin validation behavior and UI error states.

Owned scope:
- backend/internal/handler/admin/codexplus_*
- backend/internal/service/codexplus_admin_*
- backend/internal/server/routes/codexplus_admin.go
- backend/internal/server/routes/admin.go only for additive route registration.
- frontend/src/api/admin/codexPlus.ts
- frontend/src/views/admin/codexPlus/**
- frontend/src/router/index.ts only for `/admin/codex-plus` route registration
- sidebar/i18n additions only for Codex++ admin nav labels
- Frontend/admin tests beside changed files.

Avoid:
- backend/internal/server/routes/user.go
- backend/internal/server/routes/gateway.go
- Desktop runtime.
- Payment/refund implementation and payment order semantics.
- Existing payment/group/subscription APIs except read/composition calls through admin facade.
- package.json/pnpm-lock.yaml.

Requirements:
- Admin can configure plan/model/default model/usage policy/feature flags.
- Admin can publish and roll back `codexplus_config_v1` through validated server-side writes.
- Admin can inspect a test user's Codex++ entitlement by aggregating existing subscriptions/groups/usage/devices.
- Admin can see device status and Key summary without full secrets.
- Validation errors are clear and server-side enforced.
- Existing payment plans remain the source for actual charging.
- Existing subscriptions/groups remain the source for actual entitlement.

Safety constraints:
- Do not expose upstream credentials.
- Do not rely on frontend-only validation.

Verification:
- cd sub2api-main/backend && go test ./internal/handler/admin/... ./internal/service/...
- cd sub2api-main/frontend && pnpm run lint:check && pnpm run typecheck && pnpm run test:run
- Run narrower checks if full suite is unavailable.

Final response:
- Summarize admin capabilities.
- List changed files.
- List verification commands/results.
- List any operator workflow assumptions.
```

## Module I: E2E Release Gate

```text
You are responsible for Module I: E2E Release Gate.

Workspace:
C:\Users\23293\Desktop\codex+++-parallel\codexplus-mvp-e2e

Branch:
codex/codexplus-mvp-e2e

Phase:
MVP Phase 4: Desktop UX and E2E Prep

Objective:
Prepare and later execute the MVP end-to-end verification path from entitlement opening to Codex launch and successful model request.

Model quality requirement:
- This module is Tier 1 for final release judgment and Tier 3 only for read-only checklist prep.
- Do not use a lower-capability model to decide readiness, waive tests, or assess high-risk failures.
- Use high reasoning for final E2E interpretation and rollback decisions.
- If your current agent/model cannot satisfy this tier, stop and report instead of implementing.

Dependencies:
- Read-only dependency on all modules during prep.
- Hard dependency on D/E/F/G/H before final execution.
- Hard dependency on `codex-plus-dev-plan/PHASE1-MODULE-I-E2E-RELEASE-GATE-PLAN.md`.

Gate conditions:
- During parallel feature work, only write docs/scripts/checklists.
- Do not patch implementation files unless coordinator explicitly assigns a small integration fix.

Contract inputs:
- Client API contract.
- Admin entitlement workflow.
- Gateway enforcement behavior.
- Desktop runtime command behavior.
- E2E release gate blueprint from `PHASE1-MODULE-I-E2E-RELEASE-GATE-PLAN.md`.

Contract outputs:
- E2E manual checklist or script.
- Test account/setup requirements.
- Package install check.
- Compatibility and rollback notes.
- Release-readiness report template.

Owned scope:
- platform-ops/e2e/** if created.
- codex-plus-dev-plan/07-integration-release/** updates.
- Release checklist docs.
- Test scripts under an approved tools/e2e path.

Avoid:
- Backend service implementation.
- Desktop runtime implementation.
- Admin UI feature implementation.
- Payment/refund live calls.

Requirements:
- Cover open/manual entitlement, login, bootstrap, provider write, launch, request, usage/log check.
- Cover expired, low balance, model removed, device revoked.
- Include rollback and known-risk sections.

Safety constraints:
- Use test accounts only.
- Do not call real payment providers without explicit authorization.
- Do not include real credentials in docs.

Verification:
- Execute checklist only after implementation modules are merged.
- If not executable yet, validate that each step names required owner/module.

Final response:
- Summarize E2E artifacts.
- List changed files.
- List verification status.
- List blockers for final E2E run.
```

## Module J: Integration Coordinator

```text
You are responsible for Module J: Integration Coordinator.

Workspace:
C:\Users\23293\Desktop\codex+++-parallel\codexplus-mvp-integration

Branch:
codex/codexplus-mvp-integration

Phase:
MVP Phase 5: Integration and Release

Objective:
Merge modules in dependency order, resolve conflicts according to file ownership, run verification, update docs, and produce the final release readiness report.

Model quality requirement:
- This module is Tier 1.
- Do not use a lower-capability model for merge/conflict, release readiness, rollback or contract-drift decisions.
- Use high reasoning for all integration decisions.
- If your current agent/model cannot satisfy this tier, stop and report instead of implementing.

Dependencies:
- Hard dependency on all implementation module final reports.
- Hard dependency on `codex-plus-dev-plan/PHASE1-MODULE-J-INTEGRATION-COORDINATOR-PLAN.md`.

Gate conditions:
- Do not start broad integration while feature modules are still making contract changes.
- Do not implement new feature work except small compatibility fixes discovered during merge.

Contract inputs:
- Final contract artifacts from A.
- Storage/migration decision from B.
- Final reports from C/D/E/F/G/H/I.
- Integration coordinator blueprint from `PHASE1-MODULE-J-INTEGRATION-COORDINATOR-PLAN.md`.

Contract outputs:
- Integrated branch.
- Updated docs and downstream assumptions.
- Verification results.
- Release notes and rollback notes.

Owned scope:
- Merge conflict resolution across repos.
- codex-plus-dev-plan/**
- package manifests/lockfiles only if required by approved dependency changes.
- Final verification checklist.

Avoid:
- Rewriting feature modules without reviewing owner intent.
- Reverting unrelated user changes.
- Destructive git commands.

Requirements:
- Merge in planned order from PARALLEL-DISPATCH-PLAN.md.
- Inspect git status and diff stats before each merge.
- Confirm forbidden files were not edited by worker branches.
- Run smallest meaningful verification after risky merges and full gate at end.
- Update docs if implemented scope differs from planned scope.

Safety constraints:
- No secrets in repo.
- No production API/payment calls unless explicitly authorized.

Verification:
- Backend: cd sub2api-main && make test
- Frontend: cd sub2api-main/frontend && pnpm run lint:check && pnpm run typecheck && pnpm run test:run
- Desktop: cd CodexPlusPlus-main && cargo test --workspace
- Manager: cd CodexPlusPlus-main/apps/codex-plus-manager && npm run check && npm run build
- E2E: execute Module I checklist in test environment.

Final response:
- List modules merged.
- List conflicts resolved and ownership rule used.
- List verification commands/results.
- List contract changes from original plan.
- List remaining risks, manual deployment steps, and rollback notes.
```
