# Codex++ Observability, SLO and Alerting Plan

This plan defines the production observability, SLO, alerting, and incident response requirements for Codex++. It does not prove that dashboards or alert routes are already configured.

## Status

- State: observability owner approval required
- Evidence posture: SLO and alerting plan
- Owner: observability/on-call owner to be named by project owner
- Last updated: 2026-06-18

## Release Rule

Paid production launch is blocked until P0/P1 alerts route to a named owner, dashboards exist for the critical flows, and a cost emergency alert has a tested response path.

## Core Signals

| Signal type | Requirement |
| --- | --- |
| Metrics | Counters, rates, latency histograms, and gauges for client API, gateway, payment, entitlement, admin, database, Redis, and host resources |
| Logs | Structured, redacted, request-correlated logs |
| Request IDs | Shared IDs across client API, gateway, payment, admin, and support investigations |
| Audit events | Entitlement, device, payment, config, admin, and support actions |
| Business events | Orders, redemptions, usage, rejection, cost, and refund/compensation events |

## Required Correlation Fields

Use these fields where available:

- `request_id`
- `user_id`
- `device_id`
- `api_key_id` or redacted key summary
- `route`
- `method`
- `status_code`
- `error_code`
- `config_version`
- `snapshot_version`
- `model_id`
- `plan_id`
- `order_id`
- `duration_ms`

Never log full API keys, full JWTs, authorization headers, payment secrets, database passwords, upstream provider keys, or unredacted model request content unless an approved policy explicitly allows it.

## SLO Draft

| Area | Initial SLO target | Alert trigger |
| --- | --- | --- |
| Client bootstrap availability | 99.5 percent successful responses over 1 hour | Success rate below 98 percent for 10 minutes |
| Client bootstrap latency | p95 below 1500 ms | p95 above 3000 ms for 10 minutes |
| Gateway availability | 99.5 percent valid requests complete over 1 hour | 5xx rate above 2 percent for 10 minutes |
| Gateway latency | p95 below 5000 ms, excluding upstream long generation | p95 above 10000 ms for 10 minutes |
| Payment callback processing | 99.9 percent handler success over 1 hour | Signature failure spike, fulfillment failure, or handler 5xx |
| Entitlement freshness | Paid user sees entitlement within 60 seconds | Paid order not reflected after 5 minutes |
| Admin API availability | 99 percent over 1 hour | 5xx rate above 5 percent for 10 minutes |
| Database health | Connection pool healthy | Connection failures for 5 minutes |
| Redis health | Reachable with low error rate | Redis unavailable for 1 minute |
| Secret leak | Zero full secret exposure | Any full secret detection |

Final SLO numbers, business-hours coverage, and alert channels require owner approval.

## Dashboard Requirements

| Dashboard | Required panels |
| --- | --- |
| Executive health | Bootstrap success, gateway success, payment success, active users, total usage/cost, current P0/P1 incidents |
| Operations | API latency, route errors, gateway rejection distribution, database/Redis health, upstream provider errors, deployment and config version |
| Billing and cost | Orders by plan, upstream cost, estimated margin, top cost users, threshold breaches, payment callback failures |
| Security and abuse | Failed logins, rate-limited users, device revocations, suspicious usage spikes, admin changes, secret scan incidents |
| Support | Bootstrap failures by user, device blocks, paid-but-not-entitled cases, low balance and expired users, recent user-facing error codes |

## Alert Rules

| Severity | Alert | Required response |
| --- | --- | --- |
| P0 | Secret leak detected | Stop release or incident window, rotate secret, remove exposure, audit access |
| P0 | Bootstrap success below 90 percent for 5 minutes | Page on-call and investigate auth/config/database |
| P0 | Gateway valid request 5xx above 10 percent for 5 minutes | Page on-call and consider rollback or maintenance mode |
| P0 | Payment callback failing broadly | Disable payment exposure or route to manual recovery |
| P0 | Database unavailable | Page on-call and start restore/rollback process |
| P0 | Unpaid or expired users can access gateway | Stop gateway or policy route immediately |
| P0 | Cost exceeds emergency threshold | Apply global cap, disable expensive models, notify owner |
| P1 | Gateway 5xx above 2 percent for 10 minutes | On-call investigation |
| P1 | Payment fulfillment failure greater than zero | Support/payment owner review |
| P1 | Paid order not reflected after 5 minutes | Manual entitlement recovery path |
| P1 | Disk above 85 percent | Ops escalation |
| P1 | Backup failed | Ops escalation |
| P2 | Model-specific upstream error spike | Ticket or daily review |

## Incident Runbooks

| Incident | First checks | First actions |
| --- | --- | --- |
| Bootstrap failure | Client API, auth, config, entitlement, device, Redis, PostgreSQL, recent config publish | Roll back config, restore dependency, disable feature flag, notify users if broad |
| Payment paid but entitlement missing | Provider order, callback logs, signature result, idempotency key, order state, entitlement event | Reprocess if safe, manually open entitlement with audit note, fix root cause |
| Gateway cost spike | Top users, top models, config changes, provider pricing/routing | Apply cap, disable expensive model, rotate user-side key if compromised |
| Secret leak | Secret type, exposure path, access scope | Revoke/rotate, remove exposure, audit access, add regression scan |
| Database/Redis outage | Container health, host resources, recent deploy, backup status | Restore service, rollback if deploy-caused, notify owner |

## Observability Gate Status For Worker 2D

| Gate | Current state |
| --- | --- |
| Dashboard set exists | Requires implementation/evidence |
| P0/P1 alert rules configured | Requires implementation/evidence |
| Alert recipient/channel tested | Blocked by owner approval |
| Request IDs appear in logs | Requires E2E/observability evidence |
| Payment and gateway events queryable | Requires implementation/evidence |
| Backup failure alert configured | Requires implementation/evidence |
| TLS expiry alert configured | Requires implementation/evidence |
| Secret leak scan process exists | Requires security evidence |
