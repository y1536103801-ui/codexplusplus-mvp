# Codex++ Phase 1 Module H Admin Operations Plan

本文档把 Module H 的后台运营工作落到可实现的 Sub2API 后端/前端文件、配置发布、价格/模型/额度/功能开关管理、用户权益查看和审计边界。它解决的核心问题是：管理员可以在后台调控 Codex++ 产品，而客户端只消费 bootstrap 快照，不硬编码业务策略。

## Status

- State: ready after Module C config service names are published; UI can prototype against schemas
- Owner: Module H / admin operations worker
- Primary repo: `sub2api-main`
- Dependency:
  - `codex-plus-contracts/config/*.schema.json`
  - `codex-plus-contracts/storage-decision.md`
  - `codex-plus-dev-plan/PHASE1-MODULE-C-BACKEND-FOUNDATION-PLAN.md`
  - existing admin payment/group/subscription/user APIs

## Source Evidence

Current Sub2API admin already provides these reusable foundations:

| Capability | Existing path | Module H decision |
| --- | --- | --- |
| Admin route root | `backend/internal/server/routes/admin.go` | Add a small Codex++ admin route group; do not disturb existing admin routes. |
| Settings storage | `backend/internal/service/setting*`, `frontend/src/api/admin/settings.ts` | Use Module C `codexplus_config_v1` service; do not write raw settings JSON directly from handlers. |
| Payment plans and prices | `/admin/payment/plans`, `frontend/src/api/admin/payment.ts` | Real sale price/order behavior remains owned by existing payment plan/order system. |
| Groups, models, rate multipliers, RPM overrides | `/admin/groups`, `frontend/src/api/admin/groups.ts`, `GroupsView.vue` | Reuse groups as model/policy source; Codex++ UI may link/compose but not duplicate enforcement. |
| User subscriptions | `/admin/subscriptions`, `frontend/src/api/admin/subscriptions.ts`, `SubscriptionsView.vue` | Reuse for entitlement assign/extend/revoke/reset. |
| Redeem codes | `/admin/redeem-codes`, `RedeemView.vue` | Reuse for activation codes; Codex++ UI may provide focused shortcuts. |
| Usage and user views | `/admin/usage`, `/admin/users/*` | Reuse for support and entitlement drilldown. |
| Router and sidebar | `frontend/src/router/index.ts`, layout/sidebar components | Add `/admin/codex-plus` as an admin route and nav item if adopted. |

Important current constraint:

- There are already mature admin surfaces for payment, groups, subscriptions and usage. Module H should build a Codex++ operations console that composes them, not a separate billing/admin product.

## Goal

Implement a Codex++ admin operations surface that lets administrators:

1. View and publish the current `codexplus_config_v1` config version.
2. Edit PlanCatalog, ModelCatalog, UsagePolicy and FeatureFlags through validated forms.
3. Map saleable Codex++ plans to existing payment plans, groups and subscription behavior.
4. Pick default model and allowed models from backend/group/account-derived candidates.
5. Adjust usage policy, rate policy and device policy without client releases.
6. Inspect a user's Codex++ entitlement, managed provider key mapping, devices and recent events.
7. Roll back to a previous validated config version as a new version.
8. Show audit metadata for admin writes and gateway/client-related events.

## Non Goals

- Do not implement gateway enforcement.
- Do not implement `/api/v1/client/*`.
- Do not implement desktop runtime or desktop UI.
- Do not create a parallel payment/order system.
- Do not create a parallel entitlement system.
- Do not store upstream provider secrets in Codex++ config.
- Do not edit contracts directly after Phase 0; request a contract patch if a field is missing.

## Target Backend Structure

Recommended additive backend files:

```text
sub2api-main/backend/internal/
  handler/admin/
    codexplus_handler.go
    codexplus_handler_test.go

  service/
    codexplus_admin_service.go
    codexplus_admin_service_test.go

  server/routes/
    codexplus_admin.go
```

Route registration:

```text
/api/v1/admin/codex-plus/config
/api/v1/admin/codex-plus/config/validate
/api/v1/admin/codex-plus/config/publish
/api/v1/admin/codex-plus/config/versions
/api/v1/admin/codex-plus/config/rollback
/api/v1/admin/codex-plus/options
/api/v1/admin/codex-plus/users/:id/entitlement
/api/v1/admin/codex-plus/users/:id/devices
/api/v1/admin/codex-plus/users/:id/events
```

MVP may collapse validate/publish into `PUT /config` if the existing admin pattern prefers it, but it must still:

- validate before write
- increment `config_version`
- record `updated_by`
- record `rollback_from` when rolling back
- return the written version

## Backend Service Responsibilities

### Config Facade

The admin service should consume Module C config primitives:

```text
GetCurrentCodexPlusConfig(ctx)
ValidateCodexPlusConfig(ctx, draft)
PublishCodexPlusConfig(ctx, draft, actor)
ListCodexPlusConfigVersions(ctx)
RollbackCodexPlusConfig(ctx, version, actor)
```

Rules:

- The handler must not parse and write `settings.key = "codexplus_config_v1"` directly.
- Validation must use the frozen schema concepts from `codex-plus-contracts/config/*.schema.json`.
- Unknown fields are rejected unless Module C explicitly supports forward-compatible metadata.
- Existing old config fields must receive safe defaults from Module C.

### Options Facade

`GET /options` should provide dropdown candidates assembled from existing sources:

- active subscription groups
- existing payment plans
- group/platform model candidates
- available feature flag names from schema/defaults
- policy presets if Module C exposes them

Rules:

- Options are for admin convenience only.
- Gateway enforcement still reads published config/policy and existing entitlement data.
- The admin UI must not hard-code model lists or plan choices.

### User Entitlement Facade

`GET /users/:id/entitlement` should aggregate:

- user identity and status
- active subscriptions
- group membership / allowed groups
- balance and usage summary
- managed provider key mapping existence, masked only
- device count and revoked devices
- recent Codex++ events

Rules:

- Never return full user-side gateway key.
- Never return upstream credentials.
- Include enough context for support/admin repair without exposing secrets.

## Target Frontend Structure

Recommended additive frontend files:

```text
sub2api-main/frontend/src/
  api/admin/codexPlus.ts

  views/admin/codexPlus/
    CodexPlusView.vue
    CodexPlusConfigPanel.vue
    CodexPlusPlanCatalogPanel.vue
    CodexPlusModelCatalogPanel.vue
    CodexPlusUsagePolicyPanel.vue
    CodexPlusFeatureFlagsPanel.vue
    CodexPlusUserEntitlementPanel.vue
    CodexPlusConfigVersionsPanel.vue

  views/admin/codexPlus/__tests__/
    CodexPlusView.spec.ts
    codexPlusConfigValidation.spec.ts
```

Router:

```text
path: "/admin/codex-plus"
name: "AdminCodexPlus"
requiresAdmin: true
```

Sidebar/nav:

- Add a Codex++ operations item near payment/groups/subscriptions.
- Keep direct links to payment plans, groups, subscriptions and usage.

## Admin UI Layout

Recommended tabs:

1. Overview
2. Plans
3. Models
4. Usage Policy
5. Feature Flags
6. Users and Devices
7. Versions and Audit

The page should be dense and operational:

- no marketing hero
- no decorative animation
- no nested card stacks
- compact tables and forms
- clear publish/rollback buttons
- change summary before publish

## Plan Catalog Rules

PlanCatalog controls what the Codex++ client sees and how entitlement is interpreted. It must map to existing billing/entitlement primitives:

| Codex++ plan field | Source/target |
| --- | --- |
| plan id/name/description | `codexplus_config_v1` |
| payment plan mapping | existing `/admin/payment/plans` |
| entitlement group mapping | existing `/admin/groups` and subscriptions |
| displayed price/currency | existing payment plan when available, or admin config only if contract allows |
| renewal/purchase action | backend/admin configured URL/copy, not client hard-code |

Rules:

- Actual charging remains existing payment order/payment plan behavior.
- Actual entitlement remains existing subscription/group behavior.
- Do not let Codex++ PlanCatalog silently diverge from selected payment plan price. UI should warn or block when mapped price/display config conflicts.
- Disabled plans must not appear as available purchase options in bootstrap unless backend intentionally exposes them as historical/current-user states.

## Model Catalog Rules

ModelCatalog controls default model, allowed models and UI labels for Codex++ Cloud.

Rules:

- Candidate models come from existing group/account/channel model sources or Module C options facade.
- Admin can add labels/order/visibility if the schema supports it.
- The default model must be enabled and allowed for at least one mapped active plan.
- Disabling a model used by an active plan requires explicit warning.
- Client receives model availability only through bootstrap, never through a hard-coded desktop list.

## Usage Policy Rules

UsagePolicy controls:

- period windows
- low-balance/limited thresholds
- per-plan usage caps if supported
- device limit and device revoke behavior
- rate-limit surface for Codex++ Cloud
- gateway rejection wording/status mapping

Rules:

- The server must enforce policy in Module E.
- Admin UI must validate negative or contradictory limits.
- Device limit changes must be reflected in bootstrap/device status.
- Rate/RPM settings must be compatible with existing group/account/RPM controls.

## Feature Flags Rules

FeatureFlags can control:

- show advanced provider settings in desktop
- enable install tutorial
- enable redeem in desktop
- enable device management
- enable diagnostics panel
- enable maintenance assistant
- enable staged rollout behavior if added later

Rules:

- Flags live in `codexplus_config_v1`.
- Desktop consumes flags from bootstrap.
- Admin UI cannot assume disabled features are secure enforcement; enforcement remains backend/gateway.

## User Entitlement View

The user view should support support/admin workflows:

- search user
- see active/expired/revoked subscriptions
- see mapped Codex++ plans
- see usage summary
- see device list and device statuses
- revoke/unrevoke device if backend supports it
- see managed provider key presence with masked identifier
- jump to existing subscriptions/user/API keys/usage pages

Rules:

- Full API key is never shown.
- Admin can trigger repair actions only through backend-approved endpoints.
- Manual entitlement changes must call existing subscription assignment/extend/revoke APIs.

## Audit And Versioning

Every publish/rollback/device/user-support action should produce or reference an event:

- `codexplus_config_published`
- `codexplus_config_rollback`
- `codexplus_user_entitlement_viewed` if audit policy requires
- `codexplus_device_revoked`
- `codexplus_device_unrevoked`

MVP event storage follows Module C `codexplus_events`.

Config versions:

- `config_version` increments on every validated publish.
- rollback writes an old document as a new version with `rollback_from`.
- UI shows diff summary before publish and rollback.
- UI can show the last successful publish actor/time.

## Parallel Boundaries

Module H may edit:

- `sub2api-main/backend/internal/handler/admin/codexplus_*`
- `sub2api-main/backend/internal/service/codexplus_admin_*`
- `sub2api-main/backend/internal/server/routes/codexplus_admin.go`
- minimal registration in `backend/internal/server/routes/admin.go`
- `sub2api-main/frontend/src/api/admin/codexPlus.ts`
- `sub2api-main/frontend/src/views/admin/codexPlus/**`
- minimal route/nav/i18n additions
- focused backend/frontend tests for admin Codex++ operations

Module H may consume but should not own:

- Module C config repository/service
- existing payment plans APIs
- existing group APIs
- existing subscription APIs
- existing usage APIs

Module H must not edit:

- gateway handlers or gateway routes owned by Module E
- `/api/v1/client/*` handlers owned by Module D
- desktop runtime/UI owned by Modules F/G
- DB migrations/schemas owned by Module C unless a small additive migration is approved by integration owner
- `codex-plus-contracts/**` directly

## Tests

Backend tests:

- config validate rejects invalid plan/model/usage/feature flag drafts
- publish increments version and records actor metadata
- rollback writes a new version with `rollback_from`
- options endpoint returns payment plans/groups/models without secrets
- user entitlement view masks managed provider key
- device/event views enforce admin auth

Frontend tests:

- Codex++ route renders with mocked config/options
- publish button disabled for invalid drafts
- change summary appears before publish
- rollback requires confirmation
- plan mapping warns on payment-plan mismatch
- model default cannot be disabled without validation error
- user entitlement view never renders full API key

Suggested commands when toolchain is available:

```powershell
cd sub2api-main/backend
go test ./internal/handler/admin/... ./internal/service/...

cd sub2api-main/frontend
corepack pnpm install
corepack pnpm run type-check
corepack pnpm run test -- codexPlus
```

Current workspace audit showed Go missing and global pnpm missing, so workers must report exact unavailable commands.

## Exit Gate

Module H is complete only when:

- Admin can view and publish `codexplus_config_v1`.
- Admin can adjust plan/model/usage/feature flag config without client changes.
- UI validates and warns before risky publish.
- Config publish increments version and supports rollback.
- User entitlement view aggregates subscription/group/usage/device/key status without secrets.
- Existing payment/group/subscription/usage systems remain the source of actual charging and entitlement.
- Gateway enforcement and client bootstrap still consume backend-published config, not admin UI state.
- Backend and frontend tests are run, or toolchain blockers are explicitly reported.
