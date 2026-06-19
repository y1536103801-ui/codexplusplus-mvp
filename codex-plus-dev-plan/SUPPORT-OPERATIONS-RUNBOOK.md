# Codex++ Support Operations Runbook

This runbook defines the support, refund, entitlement recovery, device, account, client, and incident workflows needed for paid users. It does not name an approved support owner or channel.

## Status

- State: support owner approval required
- Evidence posture: support operations runbook
- Owner: support owner to be named by project owner
- Last updated: 2026-06-18

## Release Rule

Paid launch is blocked until the owner approves the support channel, support owner, refund authority, escalation path, and paid-user response expectations.

## Support Channel Requirements

| Item | Required owner output |
| --- | --- |
| Primary support channel | Email, ticket system, chat, or other official route |
| Support hours | Business hours or 24/7 commitment |
| Emergency channel | Route for paid-user outage, payment, or security incident |
| Language scope | Chinese, English, bilingual, or other |
| Target response time | SLA by severity |
| Refund authority | Person or role allowed to approve refunds |
| Entitlement correction authority | Person or role allowed to apply manual repair |
| Incident owner | Person or role accountable for production incidents |

## Ticket Severity

| Severity | Example | Draft target response |
| --- | --- | --- |
| S0 | Many paid users cannot use, payment entitlement broken, security leak | Immediate owner escalation |
| S1 | Single paid user cannot use, paid but no entitlement, account incorrectly blocked | Same day target after owner approval |
| S2 | Feature issue, model unavailable, local install problem | 1-2 business days after owner approval |
| S3 | How-to, copy, minor UI issue | Best effort after owner approval |

Final response targets require owner approval.

## Required Admin Support Tools

| Tool | Required before paid launch |
| --- | --- |
| Search user by email, user ID, or order ID | Yes |
| View current plan, expiry, balance, model group | Yes |
| View recent bootstrap status | Yes |
| View recent gateway rejections | Yes |
| View devices and revoke or restore device | Yes |
| View API key summary and rotate user-side key | Yes |
| View payment/order status | Yes for real payment |
| Manually open or extend entitlement with reason | Yes |
| Manually adjust balance with reason | Yes |
| Add support note | Yes |
| See audit log of admin changes | Yes |

Support must never ask users to paste full API keys, full JWTs, payment secrets, passwords, or private provider keys into tickets or chat.

## Common Support Playbooks

### User Paid But Cannot Use

Ask for account email, order/payment reference, approximate payment time, and screenshot of client status. Do not ask for secrets.

Checks:

1. Find order.
2. Confirm payment provider status.
3. Check payment callback logs.
4. Check entitlement record.
5. Check bootstrap result.
6. Check device status.
7. Check gateway rejection reason if a request was attempted.

Actions:

- If payment succeeded but entitlement is missing, manually open entitlement with audit note and fix callback root cause.
- If callback is delayed, tell user expected wait time or manually verify.
- If payment failed, direct user to retry or contact payment provider.

Close when bootstrap status is `available` and the user or support test confirms a request works.

### User Cannot Log In

Checks:

1. Account exists.
2. Auth provider status.
3. Email verification if used.
4. Recent failed login attempts.
5. Account disabled status.
6. Browser/client version.

Actions:

- Send reset or login instructions.
- Clear invalid session if supported.
- Escalate auth provider outage if broad.

### Expired Or Not Purchased But User Believes Active

Checks:

1. Plan status.
2. Expiry time and timezone.
3. Last payment or order.
4. Entitlement change audit log.
5. Bootstrap snapshot version.

Actions:

- Correct entitlement if admin/payment bug.
- Explain expiry if correct.
- Offer renewal or refund policy path if applicable.

### Balance Or Usage Is Wrong

Checks:

1. Usage events.
2. Gateway request IDs.
3. Model IDs and cost calculation.
4. Failed or retried requests.
5. Manual balance adjustments.

Actions:

- If billing error is confirmed, adjust balance with audit note.
- If usage is correct, explain usage summary.
- If unclear, escalate to engineering with request IDs.

### Device Revoked Or Device Limit Reached

Checks:

1. Device list.
2. Revocation audit log.
3. Suspicious activity.
4. Account sharing policy.

Actions:

- Restore device if accidental.
- Revoke suspicious device.
- Explain device policy.
- Rotate user-side key if account sharing or leak is suspected.

### Client Cannot Write Provider Or Launch Codex

Ask for OS version, Codex++ Manager version, Codex version, screenshot/error code, and diagnostic export. Do not ask for API keys.

Checks:

1. Local Codex installed.
2. Config path permissions.
3. Existing manual providers.
4. Diagnostic log redaction.

Actions:

- Provide install or permission instructions.
- Escalate if provider writer regression is likely.
- Preserve manual providers during any fix.

### Refund Request

Checks:

1. Refund policy.
2. Payment status.
3. Usage amount.
4. Previous refunds or chargebacks.
5. Entitlement state.

Actions:

- Approve or deny only under owner-approved policy.
- If approved, process via payment provider.
- Update entitlement and balance consistently.
- Record support note and audit event.

### Suspected Key Leak Or Abuse

Checks:

1. Recent usage spike.
2. Device list.
3. IP/geography if available.
4. User-side key usage.
5. Gateway rejections.

Actions:

- Rotate user-side key.
- Revoke suspicious device.
- Temporarily pause account if severe.
- Notify user.
- Escalate to security if broad.

### Service Outage

Checks:

1. Dashboard health.
2. Bootstrap success rate.
3. Gateway 5xx.
4. Payment callback failures.
5. Database/Redis.
6. Upstream provider.
7. Recent deploy or config change.

Actions:

- Declare incident if multiple users are affected.
- Roll back recent config or deploy if likely cause.
- Publish status update if an approved channel exists.
- Track timeline.
- Complete post-incident review.

## Support Escalation Matrix

| Issue | First owner | Escalate to |
| --- | --- | --- |
| Payment paid but no entitlement | Support/admin | Backend/payment owner |
| Login failure | Support | Auth/backend owner |
| Gateway rejection unclear | Support | Backend/gateway owner |
| Cost spike | Support/admin | Security/cost owner |
| Secret leak | Anyone | Security owner immediately |
| Client local failure | Support | Desktop owner |
| Production outage | On-call | Owner/engineering |
| Refund dispute | Support | Business owner |

## Admin Manual Action Rules

Every manual action must record:

- Admin ID.
- User ID.
- Action type.
- Before value.
- After value.
- Reason.
- Ticket ID.
- Timestamp.

Manual actions requiring owner approval:

- Large balance adjustment.
- Refund outside normal policy.
- Account suspension.
- Permanent ban.
- Production config rollback.
- Global model disable if many users are affected.

## Support Gate Status For Worker 2D

| Gate | Current state |
| --- | --- |
| Support channel approved | Blocked by owner approval |
| Paid-user response target approved | Blocked by owner approval |
| Admin can find users/orders/devices/usage | Requires admin evidence |
| Manual entitlement and balance adjustment have audit logs | Requires admin/support evidence |
| Refund policy exists and is approved | Blocked by owner/legal approval |
| Common issue playbooks exist | Documented here |
| Incident escalation owner is known | Blocked by owner approval |
| User-facing status/update channel exists or is explicitly deferred | Blocked by owner approval |
