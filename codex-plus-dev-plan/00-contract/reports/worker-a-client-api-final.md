# Worker A Client API Contract Final Report

Report status: final
Worker lane: A
Forbidden edits: none

## Changed Files

- `codex-plus-contracts/api/client-openapi.yaml`
- `codex-plus-contracts/test-fixtures/client/bootstrap.available.json`
- `codex-plus-contracts/test-fixtures/client/bootstrap.device_revoked.json`
- `codex-plus-contracts/test-fixtures/client/bootstrap.expired.json`
- `codex-plus-contracts/test-fixtures/client/bootstrap.gateway_unhealthy.json`
- `codex-plus-contracts/test-fixtures/client/bootstrap.low_balance.json`
- `codex-plus-contracts/test-fixtures/client/bootstrap.model_unavailable.json`
- `codex-plus-contracts/test-fixtures/client/bootstrap.not_authenticated.json`
- `codex-plus-contracts/test-fixtures/client/bootstrap.not_purchased.json`
- `codex-plus-contracts/test-fixtures/client/desktop-handoff.complete.json`
- `codex-plus-contracts/test-fixtures/client/desktop-handoff.poll.completed.json`
- `codex-plus-contracts/test-fixtures/client/desktop-handoff.poll.pending.json`
- `codex-plus-contracts/test-fixtures/client/desktop-handoff.start.json`
- `codex-plus-contracts/test-fixtures/client/devices.registered.json`
- `codex-plus-contracts/test-fixtures/client/redeem.applied.json`
- `codex-plus-contracts/test-fixtures/client/usage.available.json`
- `codex-plus-dev-plan/00-contract/task-client-api-contract.md`
- `codex-plus-dev-plan/00-contract/reports/worker-a-client-api-final.md`

## Verification

- Command: `Get-ChildItem -LiteralPath 'codex-plus-contracts/test-fixtures/client' -Filter '*.json' | ForEach-Object { Get-Content -Raw -LiteralPath $_.FullName | ConvertFrom-Json | Out-Null; $_.Name }`
- Result: passed; all client JSON fixtures parse successfully.
- Command: `Get-ChildItem -LiteralPath 'codex-plus-contracts/test-fixtures/client' -Filter '*.json' | ForEach-Object { ... envelope field check ... }`
- Result: passed; every client fixture includes `code`, `status`, `message`, `reason`, `error_code` and `data`.
- Command: `Get-ChildItem -LiteralPath 'codex-plus-contracts/test-fixtures/client' -Filter 'bootstrap*.json' | ForEach-Object { ... feature flag check ... }`
- Result: passed; every bootstrap success fixture includes the full user-safe feature flag snapshot.
- Command: `powershell -ExecutionPolicy Bypass -File codex-plus-dev-plan\tools\validate-stage-gate.ps1`
- Result: failed with 1 failing check: `no-current-approval-residue:\| running \|` in coordinator-owned files (`STAGE-GATE-LEDGER.md`, `MULTI-SESSION-EXECUTION-TRACE.md`, `IMPLEMENTATION-STATUS.md`). A/B/C report existence and content checks passed; all client JSON fixture parse checks passed. The failure is not due to missing worker reports.

## Preaudit Answers

| Item | Decision | Evidence |
| --- | --- | --- |
| A1 | fixed | OpenAPI `EnvelopeBase` now requires dual-compatible envelope fields; all MVP client fixtures include `code/status/message/reason/error_code/data`. |
| A2 | fixed | `DesktopLoginPollResult.user` now references `DesktopLoginUser`, requires `id`, `username`, `email`, `display_name`, `role`, and disallows additional properties. |
| A3 | fixed | `FeatureFlagSnapshot` now requires `announcements`, `force_update_prompt`, and `strict_device_enforcement` in addition to the earlier user-visible flags; bootstrap fixtures include them. |
| A4 | fixed | `DeviceRegisterRequest.codex_version` is a required request key; null is allowed only when local Codex is missing or version probing failed. |
| A5 | fixed | `plan.commerce_action` and `usage.renew_action` now carry `message_key` and `action_copy_key`; clients display server-resolved labels from the snapshot. |

## Downstream Assumptions

- Desktop and generated API clients must unwrap the dual-compatible envelope first, then endpoint `data`; older code/message/reason-only responses are migration compatibility only.
- Desktop clients must not hardcode prices, plan choices, model multipliers, quota thresholds, rate-limit policy, or renewal/purchase copy.
- Backend bootstrap aggregation owns all user-facing service/action copy keys and resolved labels for purchase, renew, recharge, support and plan-management actions.
- Completed desktop polling may return tokens only at top level; the nested `user` object remains non-secret and minimal.

## Remaining Risks

- Final stage approval still needs coordinator-owned cleanup for active session rows before `validate-stage-gate.ps1` can pass.
- OpenAPI YAML received structural self-check and path/schema trace checks in this lane; full OpenAPI parser validation should run in coordinator integration if a YAML validator is available.
