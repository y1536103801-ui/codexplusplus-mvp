# Worker C Status Error Event Contract Final Report

Report status: final
Worker lane: C
Forbidden edits: none

## Changed Files

- `codex-plus-contracts/status-error/client-status-errors.md`
- `codex-plus-contracts/events/client-events.schema.json`
- `codex-plus-dev-plan/00-contract/task-status-and-error-model.md`
- `codex-plus-dev-plan/00-contract/reports/worker-c-status-error-event-final.md`

## Completed Contract Coverage

- Covered purchase, not authenticated, not purchased, expired, low balance, model unavailable, device revoked, gateway unhealthy and local desktop states.
- Froze `handoff_state`: `created`, `poll_pending`, `browser_approved`, `redeemed`, `expired`, `consumed`.
- Froze per-error HTTP status, retryability, `client_action`, `user_message_source` and log field profile.
- Declared OpenAPI `action_hint` and desktop `client_action` as the same behavior enum.
- Tightened event metadata to an allowlist and prohibited token/API-key/prompt/response/body keys.
- Declared that events and logs must not record access tokens, refresh tokens, `session_token`, `poll_token`, managed API keys, user-side API keys, raw prompts, raw responses or raw request/response bodies that may contain code/prompt/output.

## Verification

- Command: `Get-Content -Raw -LiteralPath 'codex-plus-contracts/events/client-events.schema.json' | ConvertFrom-Json | Out-Null`
- Result: passed; event JSON schema parses successfully.
- Command: `powershell -ExecutionPolicy Bypass -File codex-plus-dev-plan\tools\validate-stage-gate.ps1`
- Result: failed with 2 checks outside Worker C ownership. Worker C report checks and `json-parse:client-events.schema.json` passed. Remaining failures were `worker-report-exists:codex-plus-dev-plan/00-contract/reports/worker-a-client-api-final.md` and `no-current-approval-residue:\| running \|` in coordinator/parallel-session status files.

## Preaudit Answers

| Item | Decision | Evidence |
| --- | --- | --- |
| C1 | fixed | `client-status-errors.md` defines the compact browser handoff state enum and allowed transitions; `client-events.schema.json` constrains `desktop_login_started`, `desktop_login_completed` and `desktop_login_polled` handoff states. |
| C2 | fixed | `client-events.schema.json` changes `metadata` to `additionalProperties: false` with explicit allowed fields and prohibited key names for token, API key, authorization, prompt, response and body data. |
| C3 | fixed | `client-status-errors.md` adds `client_action` for every frozen error code and states that OpenAPI `action_hint` is an alias of the same enum. |
| C4 | fixed | `client-status-errors.md` and `task-status-and-error-model.md` define `admin_config_message_key`, `contract_message_key`, `desktop_local_diagnostic` and `support_only_diagnostic` boundaries. |

## Downstream Assumptions

- Later backend, gateway and desktop workers consume these contracts and do not add status/error/event fields without a contract patch.
- Worker A may still need to align OpenAPI enum names if its `action_hint` list is narrower than the C lane `client_action` enum.
- Worker B/admin config owns the concrete message key catalog and localized copy values referenced by `admin_config_message_key`.

## Remaining Risks

- Stage gate continues to fail until Worker A final report exists and coordinator-owned `| running |` residue is resolved in the stage ledger/status trace files.
- JSON schema parsing was verified; full JSON Schema instance validation is deferred to downstream contract tests.
