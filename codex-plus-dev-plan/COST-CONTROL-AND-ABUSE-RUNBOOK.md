# Codex++ Cost Control and Abuse Runbook

This runbook defines cost controls, abuse signals, enforcement actions, incident handling, and admin requirements for Codex++. It does not approve numeric cost caps.

## Status

- State: cost and abuse owner approval required
- Evidence posture: cost control and abuse response runbook
- Owner: cost/abuse owner to be named by project owner
- Last updated: 2026-06-18

## Release Rule

Paid gateway traffic is blocked until the owner approves numeric cost caps, emergency stop authority, and abuse response owners.

## Cost Control Goals

- Keep per-user and global upstream model cost bounded.
- Keep plan margin measurable.
- Prevent model/provider changes from silently breaking margin.
- Reject not purchased, expired, low-balance, revoked-device, model-denied, rate-limited, and suspended users at the gateway.
- Detect and respond to unusual device, account, payment, request, and cost patterns.

## Owner Inputs Required

| Input | Required owner output |
| --- | --- |
| Plan prices | Public or private price by plan |
| Model groups | Models allowed per plan |
| Model cost assumptions | Input/output cost and multiplier by model |
| Per-user daily cap | Numeric cap by plan or account type |
| Per-user monthly cap | Numeric cap by plan or account type |
| Global daily cap | Numeric emergency cap for all traffic |
| Trial cap | Numeric cap if trial exists |
| Low-balance behavior | Hard block, warning, or limited overrun |
| Account sharing policy | Allowed, restricted, or forbidden |
| Device count | Number of devices per plan |
| Abuse action sequence | Throttle, pause, suspend, manual review |
| Emergency stop owner | Human allowed to disable traffic or models |

## Cost Model Requirements

For every model:

- `display_model_id`
- `route_model`
- `provider`
- `input_cost_unit`
- `output_cost_unit`
- `estimated_multiplier`
- `billing_multiplier`
- `allowed_plan_groups`
- `enabled`
- `emergency_disable`

For every gateway request:

- `user_id`
- `device_id`
- `model_id`
- `request_id`
- `estimated_cost`
- `actual_cost`
- `balance_before`
- `balance_after`
- `policy_decision`
- `rejection_reason`

## Required Limits

| Level | Required controls |
| --- | --- |
| User | Per-minute requests, per-day requests, per-day token/cost, concurrency, expensive model cap, low-balance threshold |
| Plan | Allowed model groups, daily quota, monthly quota or balance, RPM, TPM, concurrent requests |
| Global | All-site daily cost cap, provider cap, model cap, emergency kill switch, emergency admin-only or maintenance mode |

## Abuse Signals

Monitor:

- Sudden cost spike by user.
- Sudden token spike by user.
- Many devices on one account.
- Repeated rate-limit hits.
- Repeated model-denied attempts.
- Repeated failed logins.
- Multiple accounts sharing payment or device fingerprints if available.
- Payment chargeback or refund abuse.
- Long-running high-cost model usage immediately after purchase.
- Gateway request bursts after bootstrap refresh or local provider repair.

## Enforcement Actions

| Action | Use when | Owner |
| --- | --- | --- |
| Soft warning | User approaches limit | Automated/client API |
| Temporary rate limit | Suspicious burst | Gateway |
| Model downgrade or disable | Expensive model abuse or provider issue | Admin/config owner |
| Device revoke | Suspicious or policy-breaking device | Admin/support |
| User-side key rotation | Key leak suspected | Support/security |
| Account pause | Serious abuse or payment risk | Admin/security |
| Refund or chargeback review | Payment abuse | Support/finance |
| Global kill switch | System-wide cost spike or enforcement risk | Owner/on-call |

## Incident Runbooks

### User Cost Spike

1. Identify user, device, API key summary, model, and request IDs.
2. Check entitlement and plan.
3. Check recent requests and cost.
4. Check if user-side key leak is likely.
5. Temporarily rate limit or pause if suspicious.
6. Rotate user-side key if required.
7. Contact user if legitimate but excessive.
8. Record incident and owner decision.

### Global Cost Spike

1. Check provider and model breakdown.
2. Check top users.
3. Check recent config changes.
4. Enable global emergency cap if required.
5. Disable expensive model if required.
6. Notify owner.
7. Prepare user-facing incident note if service is degraded.

### Payment Abuse

1. Review payment and order history.
2. Check chargebacks and refunds.
3. Check entitlement changes.
4. Pause account if abuse is likely.
5. Preserve audit evidence.
6. Apply refund or termination policy only after owner-approved rules.

### Gateway Enforcement Failure

1. Stop or degrade gateway if unpaid or expired users can use it.
2. Identify affected requests.
3. Estimate cost exposure.
4. Patch policy enforcement.
5. Run regression tests.
6. Restore service.
7. Add alert and test coverage.

## Admin Controls Required

| Control | Required before paid launch |
| --- | --- |
| Disable model globally | Yes |
| Disable model for plan group | Yes |
| Change default model | Yes |
| Set per-plan quotas | Yes |
| Set global emergency cap | Yes |
| Pause user | Yes |
| Revoke device | Yes |
| Rotate user-side key | Yes |
| View top cost users | Yes |
| View policy rejections | Yes |
| Manually adjust balance with audit note | Yes |

## QA Tests

| Test | Release effect if absent |
| --- | --- |
| Unpaid user cannot use gateway | Blocks launch |
| Expired user cannot use gateway | Blocks launch |
| Insufficient balance blocks request | Blocks launch |
| Unauthorized model blocks request | Blocks launch |
| Revoked device blocks request | Blocks launch |
| RPM/concurrency limit blocks burst | Blocks launch |
| Expensive model can be emergency disabled | Blocks expensive model exposure |
| Duplicate retries do not double charge incorrectly | Blocks paid launch |
| Usage event records estimated and actual cost | Blocks cost dashboard |
| Top user cost dashboard shows test data | Blocks cost operations |

## Cost Gate Status For Worker 2D

| Gate | Current state |
| --- | --- |
| Per-user and global limits approved | Blocked by owner approval |
| Gateway enforcement tests pass | Requires E2E evidence |
| Cost dashboard exists | Requires observability evidence |
| Cost P0/P1 alerts configured | Requires observability evidence and owner approval |
| Emergency model disable works | Requires admin/e2e evidence |
| User pause/device revoke/key rotation works | Requires admin/e2e evidence |
| Manual balance correction has audit log | Requires support/admin evidence |
