# Codex++ Compliance, Privacy and Legal Checklist

This checklist records the compliance, privacy, legal, refund, payment, and provider-terms work required before production launch. It is not legal advice and it is not an approval.

## Status

- State: owner and legal approval required
- Evidence posture: legal/privacy readiness checklist
- Owner: business/legal owner to be named by project owner
- Last updated: 2026-06-18

## Release Rule

Public or paid launch is blocked until the project owner or legal owner approves the required public documents and provider/payment terms. This worker did not receive that approval.

## Required Public Documents

| Document | Required before paid launch | Current state |
| --- | --- | --- |
| Privacy Policy | Yes | Owner/legal approval required |
| Terms of Service | Yes | Owner/legal approval required |
| Refund and Cancellation Policy | Yes | Owner/legal approval required |
| Acceptable Use Policy | Yes | Owner/legal approval required |
| Service Status or Support Policy | Yes | Owner approval required |
| Data Processing and Retention Policy | Yes | Owner/legal approval required |
| Data Processing Agreement | Required for some enterprise or regulated use | Owner/legal scope required |
| Security overview | Recommended for enterprise customers | Security owner approval required |
| Invoice and tax policy | Required if selling where tax rules apply | Business/legal approval required |

## Jurisdiction and Business Entity Inputs

| Input | Required owner output |
| --- | --- |
| Launch countries or regions | Approved availability scope |
| Merchant or contracting entity | Legal entity or individual receiving payment |
| Currency and tax handling | Supported currencies and invoice/tax process |
| Minor users | Whether the service allows minors |
| Enterprise users | Whether companies may buy or use the service |
| Sensitive data | Whether users may submit personal data, proprietary code, or confidential material |
| Data rights requests | Export, deletion, correction, and support process |

## Privacy Policy Checklist

The privacy policy must disclose:

- Account data such as email, username, and login provider identifiers.
- Device data such as device ID, OS, app version, Codex version, and last seen time.
- Billing data such as order ID, plan, payment status, and payment provider reference.
- Usage metadata such as request timestamp, model ID, token/cost summary, and rejection reason.
- Local data stored by Codex++ Manager.
- Data not stored in the desktop client, especially upstream real provider secrets.
- Use purposes such as entitlement, billing, abuse prevention, support, and debugging.
- Third-party processors such as hosting, payment, model, email, analytics, and monitoring providers.
- Retention periods for account, order, usage, audit, device, and support data.
- User deletion and export request process.
- Security contact.

## Terms Checklist

Terms must cover:

- What Codex++ provides.
- User responsibility for local Codex usage.
- Account sharing restrictions.
- Device limits and revocation.
- Payment, renewal, expiration, refunds, and cancellation.
- Prohibited use.
- Service availability limitations.
- Upstream model provider dependency.
- Suspension and termination rules.
- Limitation of liability.
- Changes to plans, models, pricing, and availability.
- Support channel and response expectations.

## Refund and Cancellation Checklist

Refund policy must define:

- Eligible refund window.
- Whether used quota reduces refund.
- How duplicate payments are handled.
- How payment callback failures are corrected.
- How chargebacks are handled.
- How refunds affect entitlement and balance.
- Manual compensation policy and approval authority.

## Acceptable Use Checklist

The acceptable use policy must prohibit:

- Illegal activity.
- Credential theft or abuse.
- Spam, malware, phishing, or fraud.
- Attempts to bypass rate limits, entitlement, or billing.
- Reselling accounts or shared access if disallowed.
- Excessive automated usage outside plan limits.
- Abuse of upstream model providers.

Technical enforcement should map to gateway limits, device revocation, account pause, user-side API key rotation, and audit records.

## Data Retention Table

| Data category | Examples | Required owner output |
| --- | --- | --- |
| Account | Email, user ID, login provider ID | Retention number and deletion process |
| Orders/payment records | Order ID, provider reference, status | Retention number and legal/tax basis |
| Usage summaries | Model, tokens, cost, timestamp | Retention number and access limits |
| Gateway logs | Request ID, status, rejection reason | Retention number and redaction rule |
| Audit logs | Admin changes, entitlement changes | Retention number and access owner |
| Device records | Device ID, platform, state | Retention number and user request process |
| Diagnostic exports | User-provided logs | Deletion time after support issue closes |

## Third-Party Provider Review

| Provider category | Required review |
| --- | --- |
| Hosting provider | Data region, access control, backup handling |
| Database/Redis provider | Data storage, access control, backup/restore |
| Payment provider | Payment terms, refund/chargeback flow, tax implications |
| Model provider | Commercial usage, proxying/resale, content handling, rate policy |
| Email provider | User data shared and unsubscribe/notification rules |
| Monitoring/error tracking provider | Log content, redaction, data region, retention |

## Compliance Gate Status For Worker 2D

| Gate | Current state |
| --- | --- |
| Privacy policy approved | Blocked by owner/legal approval |
| Terms approved | Blocked by owner/legal approval |
| Refund policy approved | Blocked by owner/legal approval |
| Acceptable use policy approved | Blocked by owner/legal approval |
| Data retention numbers approved | Blocked by owner/legal approval |
| Delete/export process approved | Blocked by owner/legal approval |
| Payment/tax process approved | Blocked by owner/legal approval |
| Model provider terms reviewed | Blocked by owner/legal approval |
| Support process for refund/security requests approved | Blocked by owner approval |
