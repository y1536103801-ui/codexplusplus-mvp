# Usage Policy Worker Final Report

Report status: final

Worker lane: Usage

## Changed files

- `frontend/src/views/admin/codexPlus/CodexPlusUsagePolicyPanel.vue`
  - Reworked the admin usage policy panel into a compact operations table.
  - Added editing for policy scope metadata, low balance threshold, daily/monthly quota, concurrency, RPM, TPM, burst/window, expired behavior, grace period, overage behavior, balance/rate-limit messages, and registry copy keys.
  - Added default values for new policies that satisfy backend registry-required usage/device policy fields.
  - Added a local read-only policy preview that explains policy selection, low balance, quota exhaustion, concurrency, RPM, TPM, and expired/grace behavior.
- `frontend/src/views/admin/codexPlus/CodexPlusFeatureFlagsPanel.vue`
  - Ensured `strict_device_enforcement` is always displayed.
  - Marked it as a server/gateway rollout switch and added operator copy stating desktop clients cannot disable enforcement locally.
- `codex-plus-dev-plan/05-admin-operations/reports/worker-usage-policy-final.md`
  - This report.

## Verification

- `pnpm typecheck` could not run: `pnpm` is not installed in this environment.
- `corepack pnpm --version` failed with Node/Corepack `ERR_VM_DYNAMIC_IMPORT_CALLBACK_MISSING`; Corepack also wrote a `packageManager` field to `frontend/package.json`, which was immediately removed to restore the pre-check file shape.
- `npm run typecheck` could not complete because `node_modules` is absent and `vue-tsc` is not available: `'vue-tsc' is not recognized as an internal or external command`.
- Static self-check completed with `rg`:
  - Confirmed `Policy preview`, `copy_keys`, and `plan usage_policy_id match` are present in `CodexPlusUsagePolicyPanel.vue`.
  - Confirmed `strict_device_enforcement`, `server/gateway`, and gateway enforcement copy are present in `CodexPlusFeatureFlagsPanel.vue`.
  - Confirmed `frontend/package.json` no longer contains `packageManager` after the Corepack cleanup.

## Coordinator follow-up

- Admin API/types were updated by another worker during this lane. The usage panel now depends on the current shared `CodexPlusUsageRule` shape including optional `applies_to`, `monthly_quota`, `burst_limit`, `rate_limit_window_seconds`, `overage_behavior`, `copy_keys`, and `device_policy`; please keep those fields in `frontend/src/api/admin/codexPlus.ts`.
- The backend client usage path currently selects a policy from `plan.usage_policy_id`; the usage panel can edit `applies_to` metadata, but plan-to-policy attachment still needs the plan/admin lane to expose and validate `usage_policy_id`.
- There is no admin policy preview API in `frontend/src/api/admin/codexPlus.ts`; this lane implemented a local read-only estimator. If the product requires authoritative hit previews, add a backend/admin preview endpoint returning selected policy, decision, and reasons.
- Registry validation requires copy-key-shaped strings. Existing seed/minimal configs still use raw `insufficient_balance_message` and `rate_limited_message` text. Coordinator should either migrate defaults to copy keys or adjust backend conversion so raw display messages are not fed into registry copy-key validation.
- Confirm whether `strict_device_enforcement` should remain admin-editable in Feature Flags or be rendered read-only with a separate rollout workflow. This UI marks it server-only and does not create a client-side bypass.

## Remaining risks

- Full typecheck/build was not possible without installing dependencies, which would write outside the lane scope.
- The local preview does not model exact gateway token windows, burst refill, paid overage, or device status; it is an operator aid only.
- Device policy advanced fields are defaulted for new policies but not fully surfaced as first-class controls beyond strict enforcement visibility.
