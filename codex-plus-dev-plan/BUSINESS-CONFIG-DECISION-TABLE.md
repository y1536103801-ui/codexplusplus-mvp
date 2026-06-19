# Codex++ Business Config Decision Table

This document records the owner-controlled business configuration required before Codex++ can sell to real users. It contains no production secrets, no real payment credentials, and no legal approval.

## Status

- State: owner approval required
- Evidence posture: business configuration record
- Owner: product owner
- Last updated: 2026-06-18
- Related docs:
  - [PRODUCTION-ENVIRONMENT-MATRIX.md](PRODUCTION-ENVIRONMENT-MATRIX.md)
  - [SERVER-SIZING-AND-SCALING-GUIDE.md](SERVER-SIZING-AND-SCALING-GUIDE.md)
  - [DEPLOYMENT-AUTOMATION-RUNBOOK.md](DEPLOYMENT-AUTOMATION-RUNBOOK.md)
  - [COST-CONTROL-AND-ABUSE-RUNBOOK.md](COST-CONTROL-AND-ABUSE-RUNBOOK.md)
  - [SUPPORT-OPERATIONS-RUNBOOK.md](SUPPORT-OPERATIONS-RUNBOOK.md)

## Confirmed Configuration Principles

| Area | Confirmed baseline | Owner approval state |
| --- | --- | --- |
| Entitlement sources | Admin manual grant, redeem code, and payment-created entitlement must all be supported for the full paid product path. | Implementation expectation recorded; launch use still requires owner approval. |
| Business source of truth | Backend/admin configuration owns plans, models, quotas, feature flags, entitlement state, and payment fulfillment. | Confirmed as product architecture. |
| Client boundary | Desktop client must not hardcode pricing, quotas, upstream keys, or commercial policy. | Confirmed as release boundary. |
| Gateway enforcement | Gateway must enforce entitlement, model allow-list, device state, balance, rate limit, and cost policy before forwarding requests. | Confirmed as release boundary. |
| Manual recovery | Support/admin must be able to repair entitlement, balance, device, refund, and compensation cases with audit records. | Required before paid launch. |
| Secrets | Real API keys, JWT secrets, database passwords, payment secrets, and provider URLs with credentials stay outside Markdown and client bundles. | Required before any production setup. |

## Owner Approval Required Before Paid Launch

The following items are blocked until the product or business owner explicitly approves them in a launch record.

| Item | Required owner output | Current evidence state | Release effect |
| --- | --- | --- | --- |
| First launch region | Approved region and user availability scope | No owner approval in this worker lane | Blocks paid launch |
| Payment processor | Approved provider and payment mode | No owner approval in this worker lane | Blocks real payment |
| Public plan and price | First paid plan name, price, billing cycle, quota, and refund boundary | No owner approval in this worker lane | Blocks public sales |
| Upstream model provider | Approved provider, model set, cost assumptions, and provider terms review | No owner approval in this worker lane | Blocks paid gateway traffic |
| Cost caps | Global daily cap, user daily cap, trial cap, premium model cap, emergency stop owner | No owner approval in this worker lane | Blocks paid gateway traffic |
| Legal terms | Privacy policy, terms of service, refund policy, acceptable use policy, provider terms review | No owner/legal approval in this worker lane | Blocks public or paid launch |
| Support operations | Support owner, channel, paid-user SLA, refund authority, escalation route | No owner approval in this worker lane | Blocks paid launch |
| Production launch authorization | Named owner approval for launch, accepted risks, and rollback owner | No owner approval in this worker lane | Blocks release go |

## Launch Mode Matrix

| Stage | Entitlement method | Payment mode | Allowed use | Required before entry |
| --- | --- | --- | --- | --- |
| Private beta | Admin manual grant | No real payment | Known users only | Owner accepts private beta scope and support owner is named. |
| Controlled beta | Admin manual grant plus redeem code | No real payment or sandbox only | Small invited group | E2E, compatibility, support, and incident playbooks have current evidence. |
| Payment validation | Admin manual recovery plus sandbox or low-value test payment | Sandbox or owner-approved low-value real payment | Payment callback validation only | Payment provider terms and refund path are approved. |
| Paid launch | Payment-created entitlement plus manual recovery | Real payment | Real customers | All legal, security, observability, cost, support, package, compatibility, and E2E gates pass or are explicitly accepted by owner. |

## Plan Catalog Decision Table

No public paid plan is approved in this worker lane. The rows below define the minimum plan catalog shape that owners must fill and approve before paid launch.

| Plan ID | Launch role | Public price | Billing cycle | Model access | Quota and limits | Device limit | Current state |
| --- | --- | --- | --- | --- | --- | --- | --- |
| `manual_admin` | Support, beta, recovery, compensation | No public sale | Custom | Owner/admin assigned | Owner/admin assigned | Owner/admin assigned | Required control path |
| `redeem_code` | Private distribution and promotions | Prepaid or promotional | Code-defined | Code-defined | Code-defined | Code-defined | Required activation path |
| `starter` | First paid plan candidate | Owner approval required | Owner approval required | Conservative model group | Owner approval required | Owner approval required | Blocked for paid launch |
| `trial` | Optional acquisition path | No charge or limited use | Time limited | Low-cost model group | Very low cap | One device | Owner approval required |
| `pro` | Later paid tier | Owner approval required | Owner approval required | Broader model group | Higher cap | Owner approval required | Defer unless owner approves |

## Model Catalog Decision Table

| Field | Required value before launch | Current state | Notes |
| --- | --- | --- | --- |
| Default model | Approved model ID and display name | Owner approval required | Must be reliable and cost controlled. |
| Low-cost model group | Approved smoke-test and trial model set | Owner approval required | Used for beta and cheap validation. |
| Premium model group | Approved expensive model set and margin rules | Owner approval required | Keep disabled unless caps and alerts are ready. |
| Disabled model behavior | Reject at gateway | Confirmed | Client hiding is not sufficient. |
| Fallback behavior | Approved automatic or user-visible policy | Owner approval required | Must avoid unexpected cost jumps. |
| Provider terms | Owner/legal review of resale, proxying, and user content handling | Owner approval required | Blocks paid gateway traffic. |

## Payment Configuration Decision Table

| Item | Required value before real payment | Current state | Control requirement |
| --- | --- | --- | --- |
| Payment provider | Owner-approved provider | Owner approval required | Provider terms and refund mechanics must be reviewed. |
| Payment mode | Sandbox, low-value real test, or production | Owner approval required | Real payment requires explicit approval. |
| Callback URL | Production API domain plus provider callback path | Owner approval required | Domain and provider must be finalized. |
| Callback signature | Provider-specific signed callback verification | Required | Forged callbacks must be rejected. |
| Idempotency key | Provider event or order ID | Required | Duplicate callback must not duplicate credit. |
| Order source of truth | Backend database | Required | Admin repair path must use audit records. |
| Refund behavior | Entitlement and ledger adjustment according to policy | Owner/legal approval required | Refund policy blocks public sales. |
| Manual reconciliation | Admin repair report or export | Required | Support must fix paid-but-not-entitled cases. |

## Entitlement State Matrix

| State | User login | Bootstrap status | Gateway behavior | User-facing intent |
| --- | --- | --- | --- | --- |
| active | Allow | `available` | Allow within limits | Normal use |
| not_purchased | Allow | `not_purchased` | Reject paid model use | Purchase or redeem |
| expired | Allow | `expired` | Reject paid model use | Renew |
| low_balance | Allow | `low_balance` | Warn or reject per approved policy | Add balance or renew |
| suspended | Policy based | `suspended` | Reject | Contact support |
| device_revoked | Allow account, reject device | `device_revoked` | Reject that device | Contact support or manage device |
| model_denied | Allow | `model_denied` | Reject selected model | Choose allowed model or upgrade |

## Feature Flag Decisions

| Flag | Recommended first value | Current state | Owner |
| --- | --- | --- | --- |
| `cloud_provider_enabled` | true after E2E pass | Requires release evidence | Backend/admin |
| `manual_provider_visible` | advanced path retained | Required compatibility boundary | Backend/admin |
| `payment_enabled` | false until payment approval and evidence | Owner approval required | Product/business owner |
| `redeem_code_enabled` | true after redeem-code tests | Requires E2E evidence | Product/admin |
| `trial_enabled` | disabled unless owner approves cap | Owner approval required | Product owner |
| `premium_models_enabled` | false for first validation | Owner approval required | Product owner |
| `maintenance_mode` | false during normal service | Required emergency switch | Ops owner |

## Cost Control Decisions

| Control | Required before paid launch | Current state | Release effect |
| --- | --- | --- | --- |
| Per-user daily cost cap | Numeric cap by plan | Owner approval required | Blocks paid gateway traffic |
| Per-user monthly cost cap | Numeric cap by plan or balance | Owner approval required | Blocks paid gateway traffic |
| Global daily cost cap | Numeric emergency cap | Owner approval required | Blocks paid gateway traffic |
| Trial max cost | Numeric cap if trial exists | Owner approval required | Blocks trial |
| Premium model access | Explicit enable list and cap | Owner approval required | Premium models stay disabled |
| Abuse auto-action | Throttle, pause, suspend, or review sequence | Owner approval required | Blocks abuse automation |
| Emergency kill switch | Admin/config action plus owner/on-call | Required | Must be tested before paid launch |

## Admin Operation Requirements

The admin console and support process must support these operations before paid launch.

| Operation | Required before paid launch | Evidence owner |
| --- | --- | --- |
| Create and update plan | Yes | Admin/backend evidence |
| Enable and disable model | Yes | Admin/backend evidence |
| Grant, extend, and revoke entitlement | Yes | E2E/support evidence |
| Create and redeem code | Yes | E2E/support evidence |
| Inspect user entitlement and devices | Yes | Support evidence |
| Inspect usage and gateway rejections | Yes | Observability/support evidence |
| Inspect payment or order state | Yes for real payment | Payment/support evidence |
| Repair failed payment callback | Yes for real payment | Support/payment evidence |
| Apply compensation or balance correction | Yes for paid users | Support/admin evidence |
| Audit admin changes | Yes | Security/support evidence |

## Go/No-Go Business Checklist

| Item | Current state |
| --- | --- |
| First paid plan price approved | Blocked by owner approval |
| Default model approved | Blocked by owner approval |
| Allowed models per plan approved | Blocked by owner approval |
| Usage quota and rate limits approved | Blocked by owner approval |
| Device limits approved | Blocked by owner approval |
| Manual/admin grant evidence exists | Requires E2E/support evidence |
| Redeem code evidence exists | Requires E2E evidence |
| Payment callback evidence exists | Requires payment/E2E evidence |
| Duplicate callback protection evidence exists | Requires payment/security evidence |
| Refund and reversal policy approved | Blocked by owner/legal approval |
| Cost caps configured | Blocked by owner approval |
| Emergency kill switch tested | Requires operations evidence |
| User-facing status messages reviewed | Blocked by product owner approval |
| Support can fix entitlement/payment/device issues | Blocked by support owner approval and admin evidence |
