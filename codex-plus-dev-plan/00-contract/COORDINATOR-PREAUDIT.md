# 00-Contract Coordinator Preaudit

Date: 2026-06-16

Status: coordinator preaudit only. This file does not approve `00-contract` and does not replace A/B/C parallel worker final reports.

## Purpose

The previous A/B/C workers failed before returning final reports because of agent quota limits. This preaudit captures concrete gaps found by the coordinator so the next parallel restart can focus on high-value contract fixes instead of re-discovering the same issues.

## Dispatch Constraint

- Do not start `01-backend-config-center` from this file.
- Do not treat existing implementation files as contract truth.
- A/B/C workers still own their declared contract files in [PARALLEL-RESTART-PACK.md](PARALLEL-RESTART-PACK.md).
- Coordinator may update this preaudit, gate ledgers and restart prompts only.

## Worker A Preaudit: Client API Contract

### A1. Envelope Compatibility Is Not Fully Frozen

Evidence:

- Bootstrap fixtures use `status: "success"` / `status: "error"`.
- Desktop handoff fixtures use legacy `code: 0` and no top-level `status`.
- OpenAPI says consumers must accept legacy code envelopes, but the per-endpoint fixture rule is not explicit enough for contract tests.

Required worker decision:

- Either normalize all MVP fixtures to include both `status` and legacy `code/message/reason`, or explicitly document which endpoints remain legacy-compatible and how generated/manual types must unwrap them.

### A2. Desktop Handoff User Shape Is Too Loose

Evidence:

- `DesktopLoginPollResult.user` is `additionalProperties: true`.

Required worker decision:

- Freeze a minimal user object for completed desktop polling, for example `id`, `username`, `email`, `display_name`, `role`, while ensuring no token or secret is nested under `user`.

### A3. Feature Flag Snapshot Does Not Match Config Schema

Evidence:

- Config schema requires `announcements`, `force_update_prompt`, `strict_device_enforcement`.
- OpenAPI `FeatureFlagSnapshot` only requires/exposes `advanced_provider_config`, `install_assistant`, `new_user_tutorial`, `model_selector`, `diagnostic_export`.

Required worker decision:

- Either expose all user-safe feature flags in bootstrap, or document why some flags are server-only and must not appear in bootstrap.

### A4. Device Registration Required Fields Differ From Gate Text

Evidence:

- `CONTRACT-GATE.md` lists `codex_version` as a required device request field.
- OpenAPI `DeviceRegisterRequest` marks `codex_version` nullable and not required.

Required worker decision:

- Decide whether `codex_version` is required, optional, or only required when local Codex is installed. Update OpenAPI/task wording accordingly.

### A5. Renewal/Purchase Copy Is Still Text-Oriented

Evidence:

- `UsageSummary.renew_action.label` is free text.
- `PlanSummary.renew_url` is a raw URL field.

Required worker decision:

- Confirm whether bootstrap should carry `message_key` / `action_copy_key` references so admins can change renewal and purchase copy without a client release.

## Worker B Preaudit: Admin Config Contract

### B1. Governance Fields Are Inconsistent Across Schemas

Evidence:

- `plan-catalog.schema.json` includes `rollback_from`.
- `model-catalog.schema.json`, `usage-policy.schema.json` and `feature-flags.schema.json` do not include `rollback_from`.
- The task doc mentions `draft_status`, but the schemas only use `publish_scope`.

Required worker decision:

- Freeze one shared metadata block across all config documents: `config_version`, `draft_status` or `publish_scope`, `rollback_from`, `updated_by`, `updated_at`, `change_reason`.

### B2. Device Policy Is Not A First-Class Admin Config

Evidence:

- `FeatureFlags.strict_device_enforcement` exists.
- There is no schema for max devices, device replacement behavior, revoke reason taxonomy, cooldown, or support unlock policy.

Required worker decision:

- Decide whether device policy belongs in `FeatureFlags`, `UsagePolicy`, a new config schema, or is deferred to `06-commerce-and-enforcement`.

### B3. Operator-Controlled Copy Uses Raw Text Instead Of Keys

Evidence:

- `UsagePolicy.insufficient_balance_message` and `rate_limited_message` are raw strings.
- Plan purchase/renew copy has no stable message key.

Required worker decision:

- Freeze whether admin-managed copy is stored as raw localized text, message keys, or both. Client tasks require copy to be backend-driven and not hardcoded.

### B4. Model Quality Policy Fields Are Not Represented

Evidence:

- `ModelCatalog` covers route model, group, context and multiplier.
- It does not express model rollout channel, quality tier, fallback model, deprecation date, or disabled replacement.

Required worker decision:

- Decide which of these are required for industrial-grade model operations in MVP and which are deferred.

## Worker C Preaudit: Status, Errors And Events

### C1. Desktop Handoff State Model Is Split Across Errors And Events

Evidence:

- Error table includes pending auth errors and desktop session errors.
- There is no compact desktop handoff state enum such as `created`, `browser_approved`, `poll_pending`, `redeemed`, `expired`, `consumed`.

Required worker decision:

- Freeze handoff state names for audit/event payloads and client polling interpretation.

### C2. Event Metadata Allows Arbitrary Safe-Looking Keys

Evidence:

- `client-events.schema.json` allows arbitrary scalar values under `metadata`.
- The redaction policy forbids tokens, raw prompt/response and credentials, but schema cannot currently reject suspicious metadata keys.

Required worker decision:

- Add an allowlist-oriented metadata shape, or document that event writer tests must reject prohibited keys such as `access_token`, `refresh_token`, `session_token`, `poll_token`, `api_key`, `authorization`, `prompt`, `response`.

### C3. Error Rows Need Client Action Stability

Evidence:

- Status table has `User action`.
- Error table has no explicit `client_action` enum column aligned with OpenAPI `action_hint`.

Required worker decision:

- Add a stable `client_action` or `action_hint` mapping per error code so desktop UI does not invent behavior.

### C4. Message Source Needs Admin-Controlled Copy Boundary

Evidence:

- Error table `Message source` values are broad (`server`, `usage policy`, `desktop`, `gateway`).

Required worker decision:

- Clarify which messages are `message_key` driven by admin config, which are fixed local install diagnostics, and which are support-only diagnostics.

## Coordinator Follow-Up Checklist

- [ ] Restart Worker A/B/C in parallel using [PARALLEL-RESTART-PACK.md](PARALLEL-RESTART-PACK.md).
- [ ] Ask each worker to explicitly answer the preaudit items for its lane.
- [ ] After final reports, update `CONTRACT-GATE.md` checklist with actual pass/fail evidence.
- [ ] Only mark `00-contract` passed when preaudit items are either fixed or deliberately deferred with owner/stage.
