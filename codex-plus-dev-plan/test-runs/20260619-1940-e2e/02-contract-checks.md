# 02 Contract Checks

Run folder: 20260619-1940-e2e
Status: executed
Result: pass

## Scope

Local deterministic mode: false. Calls were made against the configured test backend.

Client API contract checks covered bootstrap, usage, devices, and redeem path presence from codex-plus-contracts/api/client-openapi.yaml.

## Paths Checked

- /api/v1/client/bootstrap
- /api/v1/client/usage
- /api/v1/client/devices
- /api/v1/client/redeem

## Result Summary

| Scenario | Method | Path | HTTP | Service status | Envelope | Reason | Error code | Snapshot | Result | Note |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| user_active | GET | /api/v1/client/bootstrap | 200 | available | success |  |  |  | pass | Token value intentionally not printed. |
| user_not_purchased | GET | /api/v1/client/bootstrap | 200 | not_purchased | success |  |  |  | pass | Token value intentionally not printed. |
| user_expired | GET | /api/v1/client/bootstrap | 200 | expired | success |  |  |  | pass | Token value intentionally not printed. |
| user_low_balance | GET | /api/v1/client/bootstrap | 200 | low_balance | success |  |  |  | pass | Token value intentionally not printed. |
| user_device_revoked | GET | /api/v1/client/bootstrap | 200 | device_revoked | success |  |  |  | pass | Token value intentionally not printed. |
| user_model_denied | GET | /api/v1/client/bootstrap | 200 | available | success |  |  |  | pass | Token value intentionally not printed. |
| user_active_usage | GET | /api/v1/client/usage | 200 | available | success |  |  | snap_20260619_114303_available | pass | Usage response only; admin audit/event rows are separate evidence. |
| user_active_device | POST | /api/v1/client/devices | 200 |  | success |  |  | snap_20260619_114304_active | pass | Idempotent test-device refresh. |


## Browser Handoff Contract Checks

Result: pass

Local deterministic mode: false. Real desktop handoff requests are gated by -AllowSessionStart and -AllowBrowserComplete.

Paths checked:
- /api/v1/auth/desktop/start
- /api/v1/auth/desktop/complete
- /api/v1/auth/desktop/poll

Safety checks:
- poll_token not in authorize_url
- verification_code is 6 digit code
- complete response never returns a desktop access token
- completed desktop poll token values are redacted from evidence
