# 09 Usage Events Audit

Run folder: 20260619-1940-e2e
Status: executed
Result: pass

## Scope

Local deterministic mode: false. Admin audit/event rows were read only if -AllowAdminAuditReads was supplied.

This runner reads admin-visible Codex++ events for the active success path and policy rejection paths. It does not mutate entitlements, devices, plans, model policy, balances, package state, compatibility snapshots or provider settings.

## Usage And Admin Audit Correlation

Usage rows and admin audit events for success and rejection paths: pass
Gateway request_id correlation: pass
Usage admin audit success and rejection evidence includes gateway_policy_rejected, GATEWAY_POLICY_* errors, request_id, Gateway request_id correlation: pass, config_version, and redaction_applied.

| Scenario | HTTP | Events | Event types | Expected signal | Error code | Service status | Request ID | Gateway request_id | Request ID correlation | Config version | Device match | Redaction | Result | Note |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| user_active | 200 | 48 | bootstrap_requested,desktop_login_completed,desktop_login_polled,device_registered,usage_recorded,usage_requested | usage_recorded |  | available | present | f9d15b06-5dfe-42bc-9d5b-2197b95fd4e6,mock_8365ee9f-cf25-490b-ab51-738aefb9e856 | matched | present | yes | redaction_applied | pass | Raw admin event payload omitted. |
| user_not_purchased | 200 | 10 | bootstrap_requested,gateway_policy_rejected | gateway_policy_rejected | GATEWAY_POLICY_ENTITLEMENT_NOT_PURCHASED | not_purchased | present | 0b7c2eda-7403-4d44-895a-d4f937b4a154 | matched | present | yes | redaction_applied | pass | Raw admin event payload omitted. |
| user_expired | 200 | 10 | bootstrap_requested,gateway_policy_rejected | gateway_policy_rejected | GATEWAY_POLICY_ENTITLEMENT_EXPIRED | expired | present | abb92be5-c4ca-4595-93d6-b44e0a603a5a | matched | present | yes | redaction_applied | pass | Raw admin event payload omitted. |
| user_low_balance | 200 | 10 | bootstrap_requested,gateway_policy_rejected | gateway_policy_rejected | GATEWAY_POLICY_QUOTA_EXCEEDED | rate_limited | present | 91aade08-aeb3-43d1-81e8-d38b44719133 | matched | present | yes | redaction_applied | pass | Raw admin event payload omitted. |
| user_device_revoked | 200 | 10 | bootstrap_requested,gateway_policy_rejected | gateway_policy_rejected | GATEWAY_POLICY_DEVICE_REVOKED | device_revoked | present | 80b4875f-dc85-4395-9810-33703704af82 | matched | present | yes | redaction_applied | pass | Raw admin event payload omitted. |
| user_model_denied | 200 | 10 | bootstrap_requested,gateway_policy_rejected | gateway_policy_rejected | GATEWAY_POLICY_MODEL_NOT_ALLOWED | model_unavailable | present | c499a875-09f1-4996-aa1b-6be7300488f5 | matched | present | yes | redaction_applied | pass | Raw admin event payload omitted. |

## Structured Signals Required

- Success path: usage_recorded, matching gateway request_id from 05-gateway-policy-e2e.md, config_version, matching test device ID when supplied, and redaction_applied.
- Rejection paths: gateway_policy_rejected, GATEWAY_POLICY_* error code, service status, matching gateway request_id from 05-gateway-policy-e2e.md, config_version, matching test device ID when supplied, and redaction_applied.

## Redaction

Redacted evidence: true; token values, gateway API Keys, Authorization headers, upstream provider Keys and raw event payloads are intentionally not printed.
