# 05 Gateway Policy E2E

Run folder: 20260619-1940-e2e
Status: executed
Result: pass

## Scope

Local deterministic mode: false. Gateway requests were made only if -AllowGatewayRequests was supplied.

This runner covers one low-cost active-user gateway request and rejection probes for no entitlement, expired entitlement, insufficient balance, revoked device and unauthorized model. It does not validate desktop launch, package install, compatibility migration, payment, admin screenshots or full audit-event visibility.

## Gateway Policy Observations

| Scenario | Model | HTTP | Expected | Observed | RequestId | ErrorCode | ServiceStatus | Reason | BodyParse | AuditCorrelation | Result | Note |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| user_active | codex-standard | 200 | 2xx success | 2xx success | f9d15b06-5dfe-42bc-9d5b-2197b95fd4e6,mock_8365ee9f-cf25-490b-ab51-738aefb9e856 | completed |  |  | json | ready-for-admin-audit | pass | Response body redacted; safe fields retained. |
| user_not_purchased | codex-standard | 403 | rejection | structured rejection | 0b7c2eda-7403-4d44-895a-d4f937b4a154 | GATEWAY_POLICY_ENTITLEMENT_NOT_PURCHASED | not_purchased | Codex++ entitlement is missing | json | ready-for-admin-audit | pass | Response body redacted; safe fields retained. |
| user_expired | codex-standard | 403 | rejection | structured rejection | abb92be5-c4ca-4595-93d6-b44e0a603a5a | GATEWAY_POLICY_ENTITLEMENT_EXPIRED | expired | subscription is not active | json | ready-for-admin-audit | pass | Response body redacted; safe fields retained. |
| user_low_balance | codex-standard | 429 | rejection | structured rejection | 91aade08-aeb3-43d1-81e8-d38b44719133 | GATEWAY_POLICY_QUOTA_EXCEEDED | low_balance | Codex++ usage quota is exhausted | json | ready-for-admin-audit | pass | Response body redacted; safe fields retained. |
| user_device_revoked | codex-standard | 403 | rejection | structured rejection | 80b4875f-dc85-4395-9810-33703704af82 | GATEWAY_POLICY_DEVICE_REVOKED | device_revoked | Codex++ device is revoked | json | ready-for-admin-audit | pass | Response body redacted; safe fields retained. |
| user_model_denied | codex-denied-local | 403 | rejection | structured rejection | c499a875-09f1-4996-aa1b-6be7300488f5 | GATEWAY_POLICY_MODEL_NOT_ALLOWED | model_unavailable | requested model is not in Codex++ model catalog | json | ready-for-admin-audit | pass | Response body redacted; safe fields retained. |

## Redaction

User-side gateway API Keys, upstream provider Keys, Authorization headers, JWTs and response bodies are redacted or summarized. Token values are intentionally not printed.
