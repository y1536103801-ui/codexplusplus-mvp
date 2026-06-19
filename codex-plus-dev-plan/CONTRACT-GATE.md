# Contract Gate

本文档定义 `00-contract` 阶段的硬门禁。任何实现会话在开始改后端、客户端或管理后台之前，必须拿到本文件列出的契约产物；没有契约的字段不得由 worker 自行发明。

## Current Gate Status

Status: `passed for 00-contract`

Reason: 用户指出前一轮没有真正按多会话并行和计划阶段推进。本门禁于 2026-06-16 重新打开，随后 A/B/C 三个 `00-contract` worker 重新并行完成 final report，coordinator 复核后运行 `tools/validate-stage-gate.ps1` 通过。

Impact:

- `01-backend-config-center` 已允许启动。
- `02-backend-client-api` 到 `07-integration-release` 继续暂停新增实现派发，直到前置阶段通过。
- 后续实现必须以本阶段契约文件、mock、状态错误和配置 schema 为事实来源。

## Gate Principle

- Contract producers run before contract consumers.
- API shape, config shape, status/error shape, event shape and storage decision must be explicit.
- Frontend and desktop workers may start early only when given stable mock fixtures.
- Contract changes after implementation starts require coordinator review and downstream prompt updates.

## Required Contract Artifacts

| Artifact | Suggested path | Owner | Required before |
| --- | --- | --- | --- |
| Client OpenAPI | `codex-plus-contracts/api/client-openapi.yaml` | Contract/API worker | Backend auth handoff, backend client API, desktop API client, E2E |
| Config schema | `codex-plus-contracts/config/*.schema.json` | Contract/config worker | Config center, admin UI, bootstrap aggregation |
| Status and errors | `codex-plus-contracts/status-error/client-status-errors.md` | Contract/status worker | Backend handlers, client UI, log redaction |
| Event schema | `codex-plus-contracts/events/*.schema.json` | Contract/events worker | Audit, usage events, gateway rejection |
| Mock fixtures | `codex-plus-contracts/test-fixtures/client/*.json` | Contract/API worker | Desktop runtime, UI, contract tests |
| Type generation plan | `codex-plus-contracts/compatibility-matrix.md` | Coordinator | TypeScript/Rust/Go consumers |
| Change policy | `codex-plus-contracts/change-review-policy.md` | Coordinator | Any field change after Phase 0 |

If the repo chooses a different folder, the coordinator must update `PARALLEL-DISPATCH-PLAN.md`, `FILE-OWNERSHIP-MATRIX.md` and worker prompts before dispatch.

## MVP API Contracts

### `POST /api/v1/auth/desktop/start`

Purpose: create a short-lived browser-approved desktop login session without exposing a password-login bypass in the desktop app.

Required request fields:

- optional `device_id`
- optional `device_name`

Required response fields:

- `session_token`: browser-visible pending session identifier.
- `poll_token`: desktop-private polling secret.
- `authorize_url`: Web URL containing `session_token` and `verification_code` only.
- `verification_code`: 6 digit code displayed in both desktop and Web authorization page.
- `expires_at`
- `poll_interval_seconds`

Required behavior:

- Does not require user JWT.
- Must rate-limit by IP and avoid creating long-lived pending sessions.
- Must never put `poll_token`, JWT, API Key or upstream credentials in `authorize_url`.

### `POST /api/v1/auth/desktop/complete`

Purpose: approve a pending desktop login from a normal authenticated browser session, so Turnstile/Web-login policy stays centralized.

Required request fields:

- `session_token`

Required response fields:

- `status`: `completed`

Required behavior:

- Requires browser Web JWT.
- Returns no access token and no refresh token.
- Rejects sessions already approved by another user.

### `POST /api/v1/auth/desktop/poll`

Purpose: let the desktop redeem a browser-approved login using the private polling secret.

Required request fields:

- `session_token`
- `poll_token`

Required response fields:

- `status`: `pending` or `completed`
- when completed only: `access_token`, `refresh_token`, `expires_in`, `token_type`, `user`

Required behavior:

- Does not require user JWT, but requires a valid `poll_token`.
- A completed poll must consume the pending session before issuing a token pair.
- Repeated completed polls must fail closed as consumed/expired.

### `GET /api/v1/client/bootstrap`

Purpose: return the minimum runtime snapshot the desktop client needs.

Required response sections:

- `service`: availability, status, user-facing message, support action, error code.
- `provider`: provider ID, display name, gateway base URL, user-side API key, key summary, default model.
- `plan`: current plan ID/name/status/expiry/renew URL, without exposing pricing logic.
- `models`: available model list with display ID, route model, label, default marker, disabled reason.
- `usage`: balance/credit summary, low-balance flag, current period usage summary, renewal/action hint.
- `feature_flags`: user-visible toggles only.
- `announcements`: currently active notices.
- `version_policy`: `config_version`, `snapshot_version`, refresh TTL, force-refresh flag, minimum client version.
- `device`: current device status if a device ID was supplied or registered.

Required behavior:

- Requires user JWT.
- Does not return upstream real provider credentials.
- Reuses an existing Codex++ user-side Key when possible.
- Creates a Codex++ user-side Key only through an idempotent path.
- Logs request ID, user ID, device ID and config version; redacts secrets.

Required MVP mocks:

- `bootstrap.available.json`
- `bootstrap.not_authenticated.json`
- `bootstrap.not_purchased.json`
- `bootstrap.expired.json`
- `bootstrap.low_balance.json`
- `bootstrap.device_revoked.json`
- `bootstrap.model_unavailable.json`
- `bootstrap.gateway_unhealthy.json`

### `GET /api/v1/client/usage`

Purpose: return a refreshable usage summary without exposing backend pricing internals.

Required sections:

- `service_status`
- `balance_summary`
- `period_usage`
- `rate_limit_state`
- `renew_action`
- `last_updated_at`
- `snapshot_version`

### `POST /api/v1/client/devices`

Purpose: idempotently register or refresh a desktop device.

Required request fields:

- `device_id`
- `platform`
- `app_version`
- `codex_version`
- `last_seen_at`

Required response fields:

- `device_id`
- `status`: `active`, `revoked`, `blocked`
- `message`
- `snapshot_version`

Required behavior:

- Same user + same `device_id` is an upsert.
- One user cannot see or mutate another user's device.
- A revoked device makes bootstrap return `device_revoked`.

### `POST /api/v1/client/redeem`

Purpose: allow a client-side redeem entry without coupling client UI to entitlement rules.

Required request fields:

- `code`
- optional `device_id`

Required response fields:

- `redeem_status`
- `entitlement_delta_summary`
- `service_status_after`
- `snapshot_version`
- `message`

MVP may implement this as a thin wrapper over existing redeem service, but must preserve idempotency and error mapping.

## MVP Config Contracts

### `PlanCatalog`

Required fields:

- `plan_id`
- `name`
- `description`
- `billing_period`
- `currency`
- `display_price`
- `entitlement_grant`
- `entitlement_sources`
- `model_groups`
- `renew_url`
- `is_listed`
- `config_version`
- `publish_scope`
- `updated_by`
- `updated_at`
- `change_reason`

MVP simplification:

- Full draft/gray/rollback workflows may be represented by metadata fields first.
- Pricing calculations stay backend-side; client receives only display-safe summary.
- `entitlement_sources` is the only bridge from existing subscription/API-key groups into Codex++ plan entitlement; missing mapping must fail closed rather than grant a default plan.

### `ModelCatalog`

Required fields:

- `model_id`
- `display_name`
- `route_model`
- `model_group`
- `context_window`
- `billing_multiplier`
- `is_default`
- `is_enabled`
- `is_hidden`
- `disabled_reason`
- `config_version`

Required validations:

- Exactly one default model per publish scope.
- A disabled default model is a configuration error.
- Client bootstrap only returns models allowed by the user's entitlement.

### `UsagePolicy`

Required fields:

- `low_balance_threshold`
- `daily_quota`
- `concurrency_limit`
- `rpm_limit`
- `tpm_limit`
- `expired_behavior`
- `insufficient_balance_message`
- `config_version`

Required behavior:

- Client never calculates low balance, expiry policy, concurrency, RPM or TPM.
- Gateway enforcement and usage API read the same policy decision result.

### `FeatureFlags`

Required fields:

- `advanced_provider_config`
- `install_assistant`
- `new_user_tutorial`
- `model_selector`
- `diagnostic_export`
- `announcements`
- `force_update_prompt`
- `strict_device_enforcement`
- `config_version`

Default-safe behavior:

- Advanced provider config hidden.
- Strict device enforcement defaults to false until the desktop helper and gateway log verification are proven in integration.
- Install assistant enabled.
- New-user tutorial enabled.
- Diagnostic export enabled only if log redaction is active.

## Status and Error Model

Required service statuses:

- `available`
- `not_authenticated`
- `not_purchased`
- `expired`
- `low_balance`
- `disabled`
- `device_revoked`
- `model_unavailable`
- `rate_limited`
- `gateway_unhealthy`
- `local_codex_missing`
- `local_config_failed`

Required error code prefixes:

- `CLIENT_AUTH_*`
- `CLIENT_ENTITLEMENT_*`
- `CLIENT_DEVICE_*`
- `CLIENT_PROVIDER_*`
- `CLIENT_LOCAL_*`
- `GATEWAY_POLICY_*`

Every error code must define:

- HTTP status when used by backend.
- User-facing message source.
- Retryability.
- Client action hint.
- Log fields and redaction rules.

## Event Contracts

MVP must define these events before implementation:

- `bootstrap_requested`
- `device_registered`
- `provider_key_created`
- `provider_key_reused`
- `usage_viewed`
- `redeem_attempted`
- `desktop_login_started`
- `desktop_login_completed`
- `desktop_login_polled`
- `gateway_policy_rejected`
- `usage_recorded`
- `local_provider_write_failed`

Required event fields:

- `event_id`
- `event_type`
- `user_id`, nullable only for pre-auth desktop handoff start events
- optional `device_id`
- optional `request_id`
- optional `config_version`
- optional `snapshot_version`
- optional `model_id`
- optional `error_code`
- `created_at`
- `redaction_applied`

## Storage Decision Gate

Before Phase 1 starts, the coordinator must decide and record:

- Whether Codex++ config uses existing setting JSON, new typed tables, or hybrid storage.
- Whether device records use a new table or existing user attribute/metadata mechanism.
- Whether entitlement uses existing subscription/balance structures or a Codex++ overlay.
- Which unique constraints or idempotency keys protect Key creation, device upsert, redeem, and payment fulfillment.
- How old users and old configs receive default values.
- Rollback path for every migration.

## Contract Review Checklist

Before declaring `00-contract` complete in the current correction round:

- [ ] Worker A final report confirms OpenAPI covers desktop browser handoff, bootstrap, usage, devices and redeem.
- [ ] Worker B final report confirms config schema covers PlanCatalog, ModelCatalog, UsagePolicy and FeatureFlags.
- [ ] Worker C final report confirms status/error table covers backend, gateway and desktop-local errors.
- [ ] Worker A final report confirms mock fixtures cover success and major failure states.
- [ ] Worker C final report confirms event schema covers bootstrap, devices, usage, redeem, gateway rejection and local write failure.
- [ ] Coordinator confirms storage decision is recorded with migration and rollback notes.
- [ ] Coordinator confirms type generation or manual type ownership is assigned for Go, TypeScript and Rust.
- [ ] Worker C final report confirms redaction rules cover `sk-*`, JWT, Authorization header, API Key, Base URL query token and local config files.
- [ ] Coordinator confirms downstream worker prompts reference exact contract files and version.
- [ ] Coordinator confirms every item in `00-contract/COORDINATOR-PREAUDIT.md` is fixed, deliberately deferred to a named later stage, or rejected with rationale.
- [ ] `powershell -ExecutionPolicy Bypass -File codex-plus-dev-plan/tools/validate-stage-gate.ps1` exits successfully.

Coordinator review result:

- Phase 0 contract approval passed on 2026-06-16 after the A/B/C parallel restart and stage gate validation.
- The authoritative implementation decisions are recorded in `codex-plus-contracts/storage-decision.md` and the updated `codex-plus-contracts/**` contract files.
- Any change after re-approval must follow `codex-plus-contracts/change-review-policy.md`.

## Change Control

After Phase 0:

- Any response field addition/removal/rename requires a contract patch.
- Any new status/error code requires status table update and mock fixture.
- Any new config field requires validation, default value and rollback note.
- Any worker discovering a contract mismatch must stop and report to the coordinator instead of patching both sides ad hoc.
