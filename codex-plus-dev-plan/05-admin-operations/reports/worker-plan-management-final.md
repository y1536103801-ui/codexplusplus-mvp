Report status: final
Worker lane: Plan

## Summary

Implemented the Codex++ admin plan catalog panel as a compact operations table for creating, editing, and downlisting plans without relying on client-side pricing rules or positional/default-plan authorization.

The panel now covers:

- Plan identity, description, sort order, and usage policy ID.
- Billing period, currency, backend-provided display price, and minor-unit amount as editable config fields.
- Purchase and renew URLs plus external product/SKU refs.
- Entitlement grants including balance credit, duration, daily quota, and period quota.
- Explicit entitlement source mapping for subscription group IDs, API key group IDs, and group names.
- Model group binding and copy-key editing.
- Status/listing controls with a Downlist action that preserves the plan row and mapping identity while clearing purchase availability.
- User-side summary preview based only on configured display fields.

## Changed files

- `sub2api-main/frontend/src/views/admin/codexPlus/CodexPlusPlanCatalogPanel.vue`
- `codex-plus-dev-plan/05-admin-operations/reports/worker-plan-management-final.md`

## Verification

- Passed: `& C:\Users\1\Desktop\codex+++\codex-plus-dev-plan\tools\verify-05-static.ps1 -Root C:\Users\1\Desktop\codex+++`
- Attempted: `npm run typecheck` in `sub2api-main/frontend`
- Blocked: `npm run typecheck` failed because local frontend dependencies are not installed; `vue-tsc` is not recognized and `node_modules` is absent.
- Checked: targeted source scan confirmed the panel writes explicit `entitlement_sources`, purchase/renew URLs, and downlisting state. It does not use first-plan/default-plan authorization logic or calculate display price from configured price fields.

## Coordinator follow-up

- No required shared API type change from this lane. The current `frontend/src/api/admin/codexPlus.ts` already includes `price_amount_minor`, `entitlement_sources`, `subscription_group_ids`, `api_key_group_ids`, `copy_keys`, `usage_policy_id`, `purchase_url`, `sort_order`, and `external_billing_refs`.
- Optional follow-up: pass `options.groups` into `CodexPlusPlanCatalogPanel.vue` in a later coordinated parent-view edit so the source mapping inputs can show all group labels/statuses, not only payment-plan group IDs plus free-form entry.

## Remaining risks

- Full Vue typecheck/build remains unverified until frontend dependencies are installed.
- Source group ID validation is still server-owned; this panel keeps IDs/names editable but cannot show complete group metadata without a coordinated parent prop change.
