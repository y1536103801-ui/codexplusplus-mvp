# Codex++ Security Review Plan

This document defines the security review work required before Codex++ can expose production traffic or real payment. It is a review plan and checklist, not a completed security sign-off.

## Status

- State: security owner approval required
- Evidence posture: review plan
- Owner: security owner to be named by project owner
- Last updated: 2026-06-18

## Release Rule

Production launch is blocked if any of these are true:

- A P0 or P1 security issue is open.
- Secret scan evidence is absent or shows a real secret leak.
- Unpaid, expired, revoked-device, or model-denied users can use the gateway.
- Admin APIs are reachable by non-admin users.
- Payment callbacks can be forged or replayed for duplicate entitlement.
- Production database or Redis is publicly exposed.

## Scope

| Area | Review requirement |
| --- | --- |
| Backend/admin/client APIs | Auth, authorization, user isolation, error hygiene |
| OpenAI-compatible gateway | Entitlement, model, device, balance, and rate enforcement |
| Desktop manager | Managed provider isolation, logout behavior, diagnostic redaction |
| Payment callbacks | Signature verification, idempotency, replay defense, refund/reversal behavior |
| Entitlement and balance | Transactional updates, audit trail, manual repair control |
| Secrets and logs | No full secrets in code, docs, logs, evidence, or bundles |
| Infrastructure | HTTPS, CORS, firewall, non-public database/Redis, backup access |
| Admin operations | RBAC, audit records, recovery controls |

## Severity Model

| Level | Example | Release decision |
| --- | --- | --- |
| P0 | Secret leak, unpaid use, payment spoofing, admin bypass, data loss | Stop release |
| P1 | Missing gateway enforcement, duplicate payment credit, rollback impossible | Fix or approved emergency mitigation |
| P2 | Incomplete audit field, low-risk dependency issue | May defer only with owner acceptance |
| P3 | Copy or cosmetic security issue | May defer |

## Required Review Evidence

Security evidence should be stored in the active release evidence set or the security evidence folder chosen by the coordinator.

| Evidence file | Required content | Release effect if absent |
| --- | --- | --- |
| `threat-model-review.md` | Assets, threats, controls, P0/P1 summary | Blocks security gate |
| `authz-test-results.md` | User/admin auth and user isolation cases | Blocks security gate |
| `payment-security-results.md` | Signature, replay, idempotency, refund/reversal cases | Blocks real payment |
| `gateway-enforcement-results.md` | Not purchased, expired, low balance, revoked device, model denied, rate limit | Blocks paid gateway |
| `secret-scan-results.md` | Code, logs, docs, evidence, artifacts scan summary | Blocks release if absent |
| `dependency-audit-results.md` | Backend/frontend/desktop dependency review | Blocks release if P0/P1 present |
| `infrastructure-security-check.md` | HTTPS, CORS, firewall, database/Redis exposure | Blocks production exposure |
| `desktop-security-check.md` | Local storage, managed/manual provider separation, diagnostic redaction | Blocks desktop release |

## Review Checklist

| Check | Required outcome |
| --- | --- |
| Client receives no upstream provider secret | Full provider secrets stay server-side only. |
| User-side API keys are redacted | Support and logs show only summaries. |
| JWT/session values are not logged | Authorization headers and full JWTs are absent from logs and evidence. |
| Admin APIs require admin auth | Ordinary users receive denial. |
| User A cannot read User B data | Devices, usage, keys, and entitlement are scoped. |
| Payment callback signature is enforced | Invalid signatures are rejected. |
| Payment idempotency is enforced | Duplicate events do not duplicate entitlement or balance. |
| Gateway policy is server-side | UI hiding alone is not treated as enforcement. |
| Secret leak response exists | Revoke, rotate, remove, audit, and regression-scan path is defined. |
| Production database/Redis are private | Public network exposure is forbidden. |

## Security Owner Actions Before Launch

| Action | Current state |
| --- | --- |
| Name security owner | Blocked by owner approval |
| Execute or accept authz/security test results | Requires evidence |
| Review secret scan output | Requires evidence |
| Review payment callback security | Requires evidence and payment provider approval |
| Approve P2/P3 deferrals, if any | Requires owner acceptance |
| Confirm no P0/P1 security items remain | Requires owner/security sign-off |

## Security Gate Status For Worker 2D

Worker 2D did not execute security tests and did not receive owner approval. Therefore this plan supports the required review structure, but it does not close the security gate for production launch.
