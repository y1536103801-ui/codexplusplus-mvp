# File Ownership Matrix

本文档定义多会话开发时的文件归属和冲突规则。任何 worker 的提示词都必须引用本文件；如果任务需要修改 forbidden 文件，worker 必须停止并向 coordinator 报告。

## Ownership Labels

- primary: 该模块是唯一预期写入者。
- secondary: 只能按提示词中明确的小范围修改。
- read-only: 可阅读，不可修改。
- forbidden: 不可修改；需要修改时停止报告。
- merge authority: 仅集成会话在合并后处理冲突或小型适配。

## Module Owners

| Module | Owner scope |
| --- | --- |
| A Contracts | `codex-plus-contracts/`, contract docs, mocks, error/event schema |
| B Storage | migration/storage design docs, schema decision notes |
| C Config Registry | Codex++ config backend service, validation, defaults |
| D Client API | `/api/v1/client/*` routes, handlers, client API service |
| E Gateway Enforcement | gateway policy resolver, billing/rate-limit hooks, rejection events |
| F Desktop Runtime | Rust runtime, bootstrap consumer, session/cache, provider writer |
| G Desktop UX | React manager UI for Codex++ Cloud user experience |
| H Admin Operations | admin backend handlers and Vue admin pages for Codex++ config/entitlement |
| I E2E Release Gate | E2E scripts/checklists, release docs, package check docs |
| J Integration | merge conflict resolution, final verification, docs sync |

## Contract and Docs Ownership

| Path | Primary | Secondary | Forbidden |
| --- | --- | --- | --- |
| `codex-plus-contracts/api/**` | A | J merge authority | C/D/E/F/G/H without contract approval |
| `codex-plus-contracts/config/**` | A | C may propose patch notes | D/E/F/G/H direct edits |
| `codex-plus-contracts/events/**` | A | E/I may propose patch notes | Direct edits after Phase 0 |
| `codex-plus-contracts/status-error/**` | A | F/G may propose UI wording gaps | Direct edits after Phase 0 |
| `codex-plus-contracts/test-fixtures/**` | A | F/G may add consumer-specific fixtures with approval | Unreviewed fixture drift |
| `codex-plus-dev-plan/**` | J | A/B may update owned sections in Phase 0 | Feature workers rewriting scope mid-phase |

## Sub2API Backend Ownership

| Path or area | Primary | Secondary | Notes |
| --- | --- | --- | --- |
| `backend/internal/server/router.go` | J merge authority | D secondary for route mount only | Treat as hotspot. Prefer additive route files. |
| `backend/internal/server/routes/user.go` | D | J merge authority | Client API routes only. |
| `backend/internal/server/routes/gateway.go` | E | J merge authority | Gateway enforcement only. |
| `backend/internal/server/routes/admin.go` | H | J merge authority | Admin Codex++ routes only. |
| `backend/internal/server/middleware/**` | D or E by explicit prompt | J merge authority | Avoid unless auth/request ID changes are required. |
| `backend/internal/handler/client/**` or equivalent new folder | D | none | Prefer new folder over editing broad handlers. |
| `backend/internal/handler/admin/**codexplus*` | H | none | Prefer additive admin files. |
| `backend/internal/handler/admin/setting_handler.go` | C/H split by prompt | J merge authority | Hotspot; do not let C and H edit same regions concurrently. |
| `backend/internal/service/setting_service.go` | C | H read-only, J merge authority | Hotspot; add Codex++ helpers in new files if possible. |
| `backend/internal/service/setting.go` | C | J merge authority | Config schema/defaults only. |
| `backend/internal/service/api_key_service.go` | D | J merge authority | Key reuse/create behavior; no unrelated auth changes. |
| `backend/internal/service/redeem_service.go` | D | H read-only | Client redeem wrapper only unless prompt owns admin redeem. |
| `backend/internal/service/payment_*` | H or later commerce module | J merge authority | MVP read-only unless explicit payment phase. |
| `backend/internal/service/billing_*` | E | J merge authority | Gateway/usage enforcement only. |
| `backend/internal/service/ratelimit_service.go` | E | J merge authority | Policy enforcement only. |
| `backend/internal/repository/**` | C/D/E by approved storage decision | J merge authority | New repositories should be additive; Codex++ storage names are frozen in `storage-decision.md`. |
| `backend/ent/schema/*codexplus*` and related migrations | C2 | J merge authority | Owns `codexplus_devices`, `codexplus_managed_provider_keys` and `codexplus_events`; no other module edits migrations/schema. |
| `backend/ent/**` generated files | C2 or J after codegen | J merge authority | Generated changes must match approved schema ownership. |
| `backend/internal/codexplus/**` | C/D/E by subfolder | J merge authority | Preferred new module root if adopted. |

## Sub2API Frontend Ownership

| Path or area | Primary | Secondary | Notes |
| --- | --- | --- | --- |
| `frontend/src/views/admin/SettingsView.vue` | H | C read-only | Hotspot; avoid broad rewrite. |
| `frontend/src/views/admin/orders/AdminPaymentPlansView.vue` | H | read-only for others | Plan/admin UI only. |
| `frontend/src/views/admin/RedeemView.vue` | H | D read-only | Redeem admin behavior only. |
| `frontend/src/views/admin/UsageView.vue` | H | E read-only | Usage display only after API fields freeze. |
| `frontend/src/views/admin/UsersView.vue` | H | D read-only | User entitlement view. |
| `frontend/src/views/admin/**/__tests__/**` | H | J merge authority | Tests must track owned UI changes. |
| `frontend/src/router/**` or route registry | H | J merge authority | Hotspot; additive route only. |
| `frontend/package.json`, `pnpm-lock.yaml` | J or explicit dependency module | none | Do not change for ordinary UI work. |
| `frontend/src/styles/**`, global CSS | H or J by prompt | none | Hotspot; prefer scoped styles. |

## CodexPlusPlus Desktop Ownership

| Path or area | Primary | Secondary | Notes |
| --- | --- | --- | --- |
| `crates/codex-plus-core/src/settings.rs` | F | J merge authority | Local session/provider settings; no unrelated settings rewrite. |
| `crates/codex-plus-core/src/relay_config.rs` | F | J merge authority | Managed provider write; preserve manual providers. |
| `crates/codex-plus-core/src/relay_switch.rs` | F | J merge authority | Only if managed provider selection requires it. |
| `crates/codex-plus-core/src/http_client.rs` | F | read-only for G | Bootstrap HTTP client if needed. |
| `crates/codex-plus-core/src/diagnostic_log.rs` | F | G read-only | Redaction and export only. |
| `crates/codex-plus-core/src/launcher.rs` | F | I read-only | Launch integration only. |
| `crates/codex-plus-core/src/install/**` | G/I by prompt | F read-only | Install assistant/package check only. |
| `apps/codex-plus-manager/src/App.tsx` | G | F secondary for command wiring, J merge authority | Hotspot; prefer extracting components. |
| `apps/codex-plus-manager/src/features/codexplus-cloud/**` | G | F read-only | Preferred UI module root. |
| `apps/codex-plus-manager/src-tauri/src/commands.rs` | F | G secondary for command exposure, J merge authority | Hotspot; command names must match UI. |
| `apps/codex-plus-manager/src-tauri/src/lib.rs` | F | J merge authority | Tauri command registration only. |
| `apps/codex-plus-manager/src/styles.css` | G | J merge authority | Hotspot; keep scoped and minimal. |
| `apps/codex-plus-manager/package.json`, `package-lock.json` | J or explicit dependency module | none | Do not change unless dependency is approved. |
| `Cargo.toml`, `Cargo.lock` | J or explicit dependency module | none | Do not change unless dependency is approved. |

## Test and Tooling Ownership

| Path or command | Primary | Notes |
| --- | --- | --- |
| `sub2api-main/backend/Makefile` | J | Do not edit for feature work. |
| `sub2api-main/Makefile` | J | Do not edit for feature work. |
| `sub2api-main/tools/**` | I/J | E2E or secret-scan helpers only. |
| Backend tests beside changed files | Owning backend module | Each module adds focused tests. |
| Frontend tests beside changed files | H or G | Each UI module owns its tests. |
| Desktop Rust tests | F | Add targeted tests for provider write/redaction. |
| E2E scripts/checklists | I | Final execution coordinated by J. |

## Conflict Matrix

| Pair | Conflict risk | Rule |
| --- | --- | --- |
| A x D/F/G/H | API/UI field drift | D/F/G/H consume mocks only; no contract edits without coordinator. |
| B x C/D/E | Storage/schema mismatch | C/D/E wait for B storage decision or use explicit stub. |
| C x H | `setting_service.go` and admin settings UI | C owns backend config service; H owns admin UI/handler only after C interface freezes. |
| D x E | shared policy and route registration | D owns client routes; E owns gateway hooks; shared policy interface must be frozen. |
| D x F/G | bootstrap field shape | F/G consume OpenAPI/mocks; D cannot rename fields without contract patch. |
| E x H | usage/policy display vs enforcement | E owns execution; H displays configured values and summaries only. |
| F x G | `App.tsx`, Tauri commands, UI state | F owns core commands/runtime; G owns UI. Command registration coordinated by J if both touch. |
| F x I | launch/package behavior | I reads and tests; F implements runtime. I does not patch launcher during feature work. |
| Any x lockfiles | dependency churn | Only explicit dependency module or J may edit lockfiles. |
| Any x docs | scope drift | Feature workers update final report; J updates plan docs. |

## Stop Conditions

A worker must stop and report when:

- It needs to edit a forbidden file.
- It needs a contract field that does not exist.
- It finds two sources of truth for pricing, model permission, balance, device status or feature flags.
- It would expose or log a full API Key, JWT, Authorization header or upstream credential.
- It would change payment/refund/credit behavior outside its explicit module.
- It would remove or overwrite existing manual provider settings.
- It would require a production API call, real payment callback or destructive git command.

## Merge Authority Rules

Module J may resolve conflicts only by following these priorities:

1. Frozen contract files win over consumer assumptions.
2. Storage/migration decision wins over ad hoc service state.
3. Backend truth wins over frontend-only checks.
4. Gateway enforcement wins over client hidden UI.
5. Existing user manual provider data must be preserved.
6. Secrets and credentials must be redacted even if it reduces debugging detail.

## Required Final Report From Each Worker

Each worker must report:

- Changed files.
- Contract inputs consumed.
- Contract outputs produced.
- Verification commands and results.
- Any forbidden file pressure or conflict risk.
- Any assumptions that downstream modules must know.
