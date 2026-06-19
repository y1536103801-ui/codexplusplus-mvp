# Codex++ Client Status and Error Contract

This file is the shared status/error source for backend handlers, gateway
enforcement, desktop runtime, desktop UI, audit events and log redaction.

Contract version: `00-contract.worker-c.2026-06-16`

## Contract Scope

- Backend `action_hint` and desktop `client_action` are the same stable enum.
- Backend, gateway and desktop-local errors must map to one row in this file.
- The desktop client must not invent purchase, renewal, quota, model, rate limit
  or support copy. It displays message keys and action keys supplied by backend
  config or the fixed local diagnostic keys defined here.
- Log/event writers may only emit the log fields listed in this file and the
  metadata allowlist in `codex-plus-contracts/events/client-events.schema.json`.

## Service Status

| Status | Source | Default `client_action` | Retryable same request | `user_message_source` | Notes |
| --- | --- | --- | --- | --- | --- |
| `available` | bootstrap/usage | `none` | yes | `contract_message_key` | Normal state. |
| `not_authenticated` | auth middleware / desktop handoff | `sign_in` | yes | `contract_message_key` | JWT missing, expired or invalid; desktop handoff is used instead of password login. |
| `not_purchased` | entitlement | `purchase` | no | `admin_config_message_key` | User has no usable Codex++ entitlement; purchase copy is backend/admin controlled. |
| `expired` | entitlement | `renew` | no | `admin_config_message_key` | Entitlement exists but is expired; renewal copy is backend/admin controlled. |
| `low_balance` | usage policy | `recharge` | yes | `admin_config_message_key` | Server decides low balance; client never calculates thresholds. |
| `disabled` | admin/policy | `contact_support` | no | `admin_config_message_key` | User, plan or feature disabled. |
| `device_revoked` | device policy | `contact_support` | no | `admin_config_message_key` | Device is revoked or blocked. |
| `model_unavailable` | model policy | `choose_available_model` | no | `admin_config_message_key` | Model disabled or no longer allowed by entitlement. |
| `rate_limited` | gateway policy | `retry_later` | yes | `admin_config_message_key` | RPM/TPM/concurrency/daily quota limit hit. |
| `gateway_unhealthy` | health/router/gateway | `retry_later` | yes | `contract_message_key` | Gateway config, router or upstream route unhealthy. |
| `local_codex_missing` | desktop local check | `install_codex` | no | `desktop_local_diagnostic` | Local desktop-only status. |
| `local_config_failed` | desktop local write | `repair_local_config` | yes | `desktop_local_diagnostic` | Provider write failed locally. |

## Desktop Browser Handoff State Enum

The audit/event payload field is `handoff_state`. API responses may continue to
use endpoint-specific `data.status` values such as `pending` and `completed`,
but event payloads and client polling interpretation must use this enum.

| `handoff_state` | Meaning | Event usage | Terminal | Secret logging rule |
| --- | --- | --- | --- | --- |
| `created` | `/auth/desktop/start` created a pending browser handoff. | `desktop_login_started` | no | Do not log `session_token`, `poll_token` or verification code; log only `handoff_session_hash` and `handoff_expires_at`. |
| `poll_pending` | Desktop polled before browser approval or before a terminal decision. | sampled `desktop_login_polled` | no | Do not log `poll_token`, request body or authorization headers. |
| `browser_approved` | Authenticated browser approved the handoff for a user. | `desktop_login_completed` | no | Do not return or log desktop access/refresh tokens. |
| `redeemed` | Desktop poll exchanged the approved handoff for token pair once. | `desktop_login_polled` | yes | Do not log access token or refresh token. |
| `expired` | Pending or approved handoff exceeded TTL before redemption. | `desktop_login_polled` or error event | yes | Do not log original token values. |
| `consumed` | A repeated poll or reuse attempted after redemption/consumption. | `desktop_login_polled` or error event | yes | Do not log original token values. |

Allowed transitions:

- `created` -> `poll_pending`
- `created` -> `browser_approved`
- `created` / `poll_pending` / `browser_approved` -> `expired`
- `browser_approved` -> `redeemed`
- `redeemed` -> `consumed` for repeat or replay attempts

## Client Action Enum

| `client_action` | Required client behavior |
| --- | --- |
| `none` | No visible action. |
| `sign_in` | Start normal sign-in or browser handoff. |
| `open_browser_handoff` | Open the browser authorization URL from desktop login start. |
| `wait_for_browser_approval` | Keep polling at server-provided interval. |
| `restart_desktop_handoff` | Discard pending handoff and request a new start session. |
| `purchase` | Show backend/admin-provided purchase action. |
| `renew` | Show backend/admin-provided renewal action. |
| `recharge` | Show backend/admin-provided balance recharge or renewal action. |
| `redeem_code` | Keep user in redeem flow and allow code correction. |
| `choose_available_model` | Prompt user to select a model returned by bootstrap. |
| `retry_request` | Retry the same operation after a short client-controlled delay. |
| `retry_later` | Stop tight retry loops; obey `Retry-After` or backoff. |
| `contact_support` | Show support action from backend/admin config. |
| `install_codex` | Start local Codex install assistant. |
| `repair_local_config` | Retry or repair the local provider configuration write. |

## User Message Source Enum

| `user_message_source` | Boundary |
| --- | --- |
| `admin_config_message_key` | User-facing copy is selected by backend/admin config, usage policy, plan catalog or model catalog. Client consumes a `message_key` or `action_copy_key` and must not hardcode operational copy. |
| `contract_message_key` | Stable generic copy is owned by this contract/backend contract, for example auth, handoff and temporary gateway health messages. Client may ship fixed fallback text for these keys only. |
| `desktop_local_diagnostic` | Fixed local install/config diagnostics owned by the desktop app. These must describe only local environment issues and never plan, price, quota or entitlement rules. |
| `support_only_diagnostic` | Diagnostic detail is for logs/support tools only and must not be displayed as user-facing copy. |

Message payloads may include a short fallback message for legacy clients, but
new clients must prefer `message_key` / `action_copy_key` from config snapshots
or the contract-defined diagnostic key.

## Log Field Profiles

Each error row below names one log field profile. A profile is an allowlist:
extra fields require a contract patch.

| Profile | Exact allowed fields |
| --- | --- |
| `backend_common` | `request_id`, `user_id`, `device_id`, `config_version`, `snapshot_version`, `error_code`, `service_status`, `http_status`, `client_action`, `user_message_source`, `redaction_applied` |
| `desktop_handoff` | `request_id`, `user_id`, `device_id`, `handoff_state`, `handoff_session_hash`, `handoff_expires_at`, `error_code`, `http_status`, `client_action`, `user_message_source`, `redaction_applied` |
| `entitlement_policy` | `request_id`, `user_id`, `device_id`, `plan_id`, `entitlement_state`, `config_version`, `snapshot_version`, `error_code`, `service_status`, `http_status`, `client_action`, `user_message_source`, `redaction_applied` |
| `device_policy` | `request_id`, `user_id`, `device_id`, `device_status`, `revoke_reason_key`, `config_version`, `error_code`, `service_status`, `http_status`, `client_action`, `user_message_source`, `redaction_applied` |
| `gateway_policy` | `request_id`, `user_id`, `device_id`, `provider_key_id`, `model_id`, `model_group`, `config_version`, `policy_decision_id`, `error_code`, `service_status`, `http_status`, `client_action`, `user_message_source`, `redaction_applied` |
| `desktop_local` | `request_id`, `device_id`, `local_operation`, `client_version`, `platform`, `error_code`, `service_status`, `client_action`, `user_message_source`, `redaction_applied` |

## Error Codes

`HTTP` is `n/a` only for desktop-local errors that do not cross an HTTP
boundary. Retryability means the same operation can be retried without changing
input; user-guided recovery is expressed through `client_action`.

| Code | HTTP | Status | Retryable | `client_action` | `user_message_source` | Log fields | Notes |
| --- | ---: | --- | --- | --- | --- | --- | --- |
| `CLIENT_AUTH_NOT_AUTHENTICATED` | 401 | `not_authenticated` | yes | `sign_in` | `contract_message_key` | `backend_common` | Missing or invalid access token. |
| `CLIENT_AUTH_TOKEN_EXPIRED` | 401 | `not_authenticated` | yes | `sign_in` | `contract_message_key` | `backend_common` | Expired JWT. |
| `CLIENT_AUTH_DESKTOP_SESSION_CREATE_FAILED` | 500 | `not_authenticated` | yes | `retry_request` | `contract_message_key` | `desktop_handoff` | Desktop handoff start failed before a usable session was created. |
| `CLIENT_AUTH_DESKTOP_SESSION_INVALID` | 400 | `not_authenticated` | no | `restart_desktop_handoff` | `contract_message_key` | `desktop_handoff` | Malformed, unknown or invalid pending handoff credentials. |
| `CLIENT_AUTH_DESKTOP_TARGET_MISMATCH` | 409 | `not_authenticated` | no | `restart_desktop_handoff` | `contract_message_key` | `desktop_handoff` | Pending session was approved by a different browser user. |
| `CLIENT_AUTH_DESKTOP_SERVICE_NOT_READY` | 503 | `gateway_unhealthy` | yes | `retry_later` | `contract_message_key` | `desktop_handoff` | Desktop handoff storage or approval service unavailable. |
| `CLIENT_AUTH_DESKTOP_SESSION_UPDATE_FAILED` | 500 | `not_authenticated` | yes | `retry_request` | `contract_message_key` | `desktop_handoff` | Browser approval update failed transactionally. |
| `PENDING_AUTH_SESSION_NOT_FOUND` | 404 | `not_authenticated` | no | `restart_desktop_handoff` | `contract_message_key` | `desktop_handoff` | Pending handoff session does not exist. |
| `PENDING_AUTH_SESSION_EXPIRED` | 401 | `not_authenticated` | no | `restart_desktop_handoff` | `contract_message_key` | `desktop_handoff` | Pending handoff session expired. |
| `PENDING_AUTH_SESSION_CONSUMED` | 401 | `not_authenticated` | no | `restart_desktop_handoff` | `contract_message_key` | `desktop_handoff` | Handoff was already redeemed or consumed. |
| `PENDING_AUTH_BROWSER_MISMATCH` | 401 | `not_authenticated` | no | `restart_desktop_handoff` | `contract_message_key` | `desktop_handoff` | Browser approval context does not match pending session. |
| `CLIENT_ENTITLEMENT_NOT_PURCHASED` | 403 | `not_purchased` | no | `purchase` | `admin_config_message_key` | `entitlement_policy` | User has no active Codex++ entitlement. |
| `CLIENT_ENTITLEMENT_EXPIRED` | 403 | `expired` | no | `renew` | `admin_config_message_key` | `entitlement_policy` | Entitlement exists but is expired. |
| `CLIENT_ENTITLEMENT_DISABLED` | 403 | `disabled` | no | `contact_support` | `admin_config_message_key` | `entitlement_policy` | Entitlement, user or plan was disabled by policy. |
| `CLIENT_ENTITLEMENT_LOW_BALANCE` | 200 | `low_balance` | yes | `recharge` | `admin_config_message_key` | `entitlement_policy` | Bootstrap/usage warning; gateway hard failure uses `GATEWAY_POLICY_BALANCE_INSUFFICIENT`. |
| `CLIENT_ENTITLEMENT_REDEEM_CODE_INVALID` | 400 | `not_purchased` | no | `redeem_code` | `admin_config_message_key` | `entitlement_policy` | Redeem code is invalid and no entitlement changes. |
| `CLIENT_ENTITLEMENT_REDEEM_CODE_EXPIRED` | 410 | `not_purchased` | no | `purchase` | `admin_config_message_key` | `entitlement_policy` | Redeem code expired or was already used. |
| `CLIENT_DEVICE_REVOKED` | 403 | `device_revoked` | no | `contact_support` | `admin_config_message_key` | `device_policy` | Device was revoked by user/admin policy. |
| `CLIENT_DEVICE_BLOCKED` | 403 | `device_revoked` | no | `contact_support` | `admin_config_message_key` | `device_policy` | Device is blocked by risk or replacement policy. |
| `CLIENT_PROVIDER_KEY_CREATE_FAILED` | 500 | `gateway_unhealthy` | yes | `retry_later` | `contract_message_key` | `backend_common` | Managed provider key creation failed; never log key material. |
| `CLIENT_PROVIDER_KEY_REVEAL_DENIED` | 403 | `disabled` | no | `contact_support` | `contract_message_key` | `backend_common` | Backend refused to reveal or reuse managed provider credentials. |
| `CLIENT_LOCAL_CODEX_MISSING` | n/a | `local_codex_missing` | no | `install_codex` | `desktop_local_diagnostic` | `desktop_local` | Local Codex executable or runtime is missing. |
| `CLIENT_LOCAL_CONFIG_WRITE_FAILED` | n/a | `local_config_failed` | yes | `repair_local_config` | `desktop_local_diagnostic` | `desktop_local` | Desktop failed to write local provider config. |
| `GATEWAY_POLICY_MODEL_NOT_ALLOWED` | 403 | `model_unavailable` | no | `choose_available_model` | `admin_config_message_key` | `gateway_policy` | Requested model is disabled, hidden or outside entitlement. |
| `GATEWAY_POLICY_BALANCE_INSUFFICIENT` | 402 | `low_balance` | no | `recharge` | `admin_config_message_key` | `gateway_policy` | Balance is insufficient for this request. |
| `GATEWAY_POLICY_ENTITLEMENT_EXPIRED` | 403 | `expired` | no | `renew` | `admin_config_message_key` | `gateway_policy` | Gateway rejected an expired entitlement. |
| `GATEWAY_POLICY_DEVICE_REVOKED` | 403 | `device_revoked` | no | `contact_support` | `admin_config_message_key` | `gateway_policy` | Gateway rejected a revoked or blocked device. |
| `GATEWAY_POLICY_RATE_LIMITED` | 429 | `rate_limited` | yes | `retry_later` | `admin_config_message_key` | `gateway_policy` | RPM, TPM, concurrency or daily quota limit hit. |
| `GATEWAY_POLICY_CONFIG_UNAVAILABLE` | 503 | `gateway_unhealthy` | yes | `retry_later` | `contract_message_key` | `gateway_policy` | Gateway cannot load a valid config snapshot. |
| `GATEWAY_POLICY_UPSTREAM_UNHEALTHY` | 503 | `gateway_unhealthy` | yes | `retry_later` | `contract_message_key` | `gateway_policy` | Upstream route, router or provider returned a transient health failure. |

## Display Rules

- `client_action` is the only UI action selector. Legacy OpenAPI
  `action_hint` fields must be treated as aliases of `client_action`.
- Purchase, renewal, recharge, low-balance, rate-limit, disabled-plan,
  model-unavailable and support copy must come from `admin_config_message_key`.
- Auth and desktop handoff copy may use fixed `contract_message_key` fallback
  strings because these are security flows, not operator-controlled marketing
  copy.
- `support_only_diagnostic` content must never be rendered in the normal client
  UI; it is for support tooling and sanitized logs only.
- Desktop local diagnostics must only describe install/config repair steps and
  must not mention price, package, quota or entitlement policy details.

## Required Event/Log Redaction

Always redact or reject these names and values in logs, event metadata,
diagnostic exports and support bundles:

- `access_token`
- `refresh_token`
- `session_token`
- `poll_token`
- managed provider API keys and user-side API keys, including `sk-*`
- JWTs
- `Authorization` headers
- upstream provider credentials
- Base URL query tokens
- local config file secrets
- raw prompt text
- raw response text
- raw request/response bodies that may contain project code, prompt or model
  output

Allowed identifiers after redaction:

- masked user-side key suffix, for example `sk-user-...abcd`
- `request_id`
- app-generated random `device_id`
- `handoff_session_hash`
- `provider_key_id` without the key value
- `config_version`
- `snapshot_version`
- `policy_decision_id`
