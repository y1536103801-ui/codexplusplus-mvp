# 04 Client API E2E

Run folder: 20260619-1940-e2e
Status: executed
Result: pass

## Scope

Local deterministic mode: false. Calls were made against the configured test backend.

Scope result for this client API coverage: pass

This runner covers bootstrap, usage, and idempotent test-device refresh. Browser handoff start, complete, desktop poll completion, desktop Manager login, Codex launch, gateway model request and package install are outside this runner and must be supplied by the broader Module I evidence flow.

## Client API Observations

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


## Browser Handoff E2E Subset

Browser handoff subset result: pass

Local deterministic mode: false. Real desktop handoff requests are gated by -AllowSessionStart and -AllowBrowserComplete.

This runner covers desktop start, pre-complete desktop poll, authenticated browser complete, and completed desktop poll. It does not validate Manager UI rendering, Turnstile UI completion, provider write, Codex launch, package install, compatibility migration or payment.

Paths exercised:
- /api/v1/auth/desktop/start
- /api/v1/auth/desktop/complete
- /api/v1/auth/desktop/poll

Safety checks:
- poll_token not in authorize_url
- verification_code is 6 digit code
- complete response never returns a desktop access token
- completed desktop poll token values are redacted from evidence

## Browser Handoff Observations

| Step | HTTP | Status | Result | Note |
| --- | --- | --- | --- | --- |
| desktop-start | 200 | session-created | pass | authorize_url query secrets redacted; poll_token not in authorize_url=True. |
| desktop-poll-before-complete | 200 | pre-complete | pass | Pre-complete poll returned no desktop tokens. |
| browser-complete | 200 | completed | pass | Complete response must not return desktop tokens. |
| desktop-poll-after-complete | 200 | completed | pass | Desktop tokens were present only in completed poll and values were redacted. |

## Redaction

Session token, poll token, browser JWT, desktop access token, refresh token and Authorization headers are intentionally not printed. The authorize URL is recorded only after query-secret redaction.
