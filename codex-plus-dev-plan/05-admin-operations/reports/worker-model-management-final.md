# Admin Model Management Worker Report

Report status: final
Worker lane: Model

## Changed files

- `sub2api-main/frontend/src/views/admin/codexPlus/CodexPlusModelCatalogPanel.vue`
- `codex-plus-dev-plan/05-admin-operations/reports/worker-model-management-final.md`

## Implementation

- Expanded the model catalog panel into a compact admin operations table for model id, display name, route model, fallback model, model group, context window, billing multiplier, sort order, rollout channel, quality tier, operator badge/tags, default model, status, disabled reason, replacement model, disabled message key, and deprecation timestamp.
- Added a status selector that maps to the existing catalog booleans:
  - `active`: `is_enabled=true`, `is_hidden=false`
  - `hidden`: `is_enabled=true`, `is_hidden=true`
  - `disabled`: `is_enabled=false`, `is_hidden=false`
  - `delisted`: `is_enabled=false`, `is_hidden=true`
- Added default-model guard UI. Choosing a default automatically enables and unhides that row. Disabling, hiding, or delisting the current default clears it and promotes the next enabled visible model when one exists. If no effective default is available, the panel shows a warning and leaves backend validation as the final authority.
- Added datalist helpers for upstream candidate models, configured model ids, and configured model groups.
- Kept all edits out of client built-in model files, payment logic, backend files, and shared API files.

## Verification

- `pnpm --version` failed: `pnpm` is not available in the current shell.
- `npm run typecheck` failed before project type checking because `vue-tsc` is not installed/available. `frontend/node_modules` is absent.
- `npm run build` failed at the same dependency boundary: `vue-tsc -b` is not recognized.
- Static spot-check with `rg` confirmed the edited panel contains the default guard, status mapping, disabled reason, operator tags/badge field, rollout channel, and quality tier UI.

## Coordinator follow-up

- I did not edit `frontend/src/api/admin/codexPlus.ts`. The current working copy already exposes optional model fields used by this panel: `rollout_channel`, `quality_tier`, `fallback_model_id`, `deprecation_at`, `disabled_replacement_model_id`, `disabled_message_key`, `sort_order`, and `operator_tags`. If the coordinator merge target does not include those fields, add them to `CodexPlusModel` with the same optional/nullability shape so this panel compiles against backend payloads.
- There is no distinct public `badge` field in the current backend/frontend model catalog contract. This panel uses `operator_tags` for the requested badge/tag editing. If product expects customer-facing model badges in bootstrap, coordinate a shared field such as `badge?: string | null` or `display_badges?: string[]` across backend registry, admin API types, and client bootstrap DTOs.
- Parent draft validation in `CodexPlusView.vue` checks exactly one default and default enabled; server-side registry validation also rejects hidden defaults. The model panel now warns and auto-handles hidden/disabled defaults, but a parent-level hidden-default draft validation message would make the global validation banner more complete.

## Remaining risks

- Full TypeScript/build verification is blocked until frontend dependencies and `pnpm`/`vue-tsc` are available.
- Because tests could not run, template type checking for the expanded optional fields still needs confirmation in a dependency-ready environment.
- Delisted state is represented by the existing `is_enabled=false` plus `is_hidden=true` contract; if backend later adds a dedicated lifecycle enum, the UI mapping should be revisited.
