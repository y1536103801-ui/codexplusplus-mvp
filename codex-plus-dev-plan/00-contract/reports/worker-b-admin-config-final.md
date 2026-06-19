# Worker B Admin Config Contract Final Report

Report status: final
Worker lane: B
Forbidden edits: none

## Changed Files

- `codex-plus-contracts/config/plan-catalog.schema.json`
- `codex-plus-contracts/config/model-catalog.schema.json`
- `codex-plus-contracts/config/usage-policy.schema.json`
- `codex-plus-contracts/config/feature-flags.schema.json`
- `codex-plus-dev-plan/00-contract/task-admin-config-contract.md`
- `codex-plus-dev-plan/00-contract/reports/worker-b-admin-config-final.md`

## Verification

- Command: `Get-Content -Raw codex-plus-contracts/config/plan-catalog.schema.json | ConvertFrom-Json; Get-Content -Raw codex-plus-contracts/config/model-catalog.schema.json | ConvertFrom-Json; Get-Content -Raw codex-plus-contracts/config/usage-policy.schema.json | ConvertFrom-Json; Get-Content -Raw codex-plus-contracts/config/feature-flags.schema.json | ConvertFrom-Json`
- Result: passed. All four config schemas parse as JSON.
- Command: `powershell -ExecutionPolicy Bypass -File codex-plus-dev-plan\tools\validate-stage-gate.ps1`
- Result: failed with 3 checks outside Worker B ownership: ledger approval-residue check still sees `| running |`, and Worker A/C final reports are not present. Worker B report checks and config schema JSON parsing passed.

## Preaudit Answers

| Item | Decision | Evidence |
| --- | --- | --- |
| B1 | fixed | All four config schemas now require the same top-level governance block: `config_version`, `draft_status`, `publish_scope`, `rollback_from`, `updated_by`, `updated_at`, and `change_reason`. |
| B2 | fixed | Device governance is first-class under `UsagePolicy.device_policy`; `FeatureFlags.strict_device_enforcement` is documented as a rollout switch only. |
| B3 | fixed | Client-visible operator copy is key-only through `copy_keys` or `message_keys`; raw localized message fields were removed from usage policy. |
| B4 | fixed | `ModelCatalog` now includes `rollout_channel`, `quality_tier`, `fallback_model_id`, `deprecation_at`, `disabled_replacement_model_id`, and `disabled_message_key`. |

## Downstream Assumptions

- Backend config center, gateway enforcement, admin UI and client API aggregation must consume these schemas as contract truth.
- Desktop/client code consumes only bootstrap/config snapshots and must not hardcode prices, plan rules, model lists, model multipliers, quota thresholds, rate limits, device policy, renewal copy or purchase copy.
- Copy keys require a downstream resolver before user-facing text is rendered.

## Completed Coverage

- Admin-controlled plan pricing, billing periods, entitlement grants, entitlement source mappings, plan-to-usage-policy links, purchase URLs and renewal URLs.
- Admin-controlled model routing, default model selection, model multipliers, rollout channels, quality tiers, fallback and disabled replacement behavior.
- Admin-controlled quota thresholds, daily and monthly quota, concurrency, RPM, TPM, burst and window limits.
- Admin-controlled device limit, replacement cooldown, revoke reason taxonomy, revoked behavior and support unlock policy.
- Admin-controlled feature switches and stable copy key references for renewal, purchase, rate limit, balance, device and feature prompts.

## Remaining Risks

- The schema contract is frozen, but backend storage choice, migration defaults and admin form behavior still need implementation in later stages.
- JSON Schema cannot enforce cross-document references such as `PlanCatalog.usage_policy_id` pointing to an existing `UsagePolicy.policy_id`; later contract tests should cover that.
