# Codex++ Production Environment Matrix

This document records production environment inputs and operating controls for Codex++. It must never contain production secrets, tokens, full provider URLs with credentials, JWTs, or API keys.

## Status

- State: owner approval required
- Evidence posture: environment decision matrix
- Owner: production environment owner to be named by project owner
- Last updated: 2026-06-18
- Related docs:
  - [DEPLOYMENT-AUTOMATION-RUNBOOK.md](DEPLOYMENT-AUTOMATION-RUNBOOK.md)
  - [SERVER-SIZING-AND-SCALING-GUIDE.md](SERVER-SIZING-AND-SCALING-GUIDE.md)
  - [PRODUCTION-LAUNCH-PLAN.md](PRODUCTION-LAUNCH-PLAN.md)
  - [BUSINESS-CONFIG-DECISION-TABLE.md](BUSINESS-CONFIG-DECISION-TABLE.md)

## Environment Summary

| Environment | Purpose | Current state | Release boundary |
| --- | --- | --- | --- |
| Local dev | Developer validation | Available through local worker evidence where recorded | Does not prove production readiness |
| MVP test | Full local or CI E2E | Requires current E2E lane evidence | Does not authorize paid launch |
| Staging | Production-like QA | Owner/environment approval required | Required before public paid launch |
| Production | Real users and payment | Candidate inventory exists; final approval absent | Blocked until owner approves values and launch route |

## Candidate Production Server

The following values came from earlier planning evidence and remain non-secret inventory.

| Field | Value | Current state | Notes |
| --- | --- | --- | --- |
| Provider | Owner must confirm provider account | Owner approval required | Avoid relying on screenshots as authority. |
| Instance name | `CentOS-zkay` | Candidate inventory | Non-secret label from previous inventory. |
| Region | Beijing | Candidate inventory | Regional compliance and ICP/record implications need owner/legal review. |
| Public IP | `39.96.27.135` | Candidate inventory | Do not place credentials in this document. |
| Private IP | `172.24.62.185` | Candidate inventory | Internal network only. |
| CPU | 2 cores | Candidate inventory | Acceptable for staging or very small pilot. |
| Memory | 2 GiB | Capacity risk | Add swap and monitor; not ideal for public paid launch. |
| Disk | 40 GiB | Capacity risk | Strict database, log, and backup retention required. |
| Bandwidth | 200 Mbps peak | Candidate inventory | Gateway payload and streaming duration determine real pressure. |
| OS | CentOS 8.2 | Lifecycle risk | Prefer Ubuntu LTS, Rocky Linux, or AlmaLinux before serious paid launch. |
| DDoS status | normal at inventory time | Candidate inventory | Must be checked again before production exposure. |
| Verdict | Usable for staging, private beta, or tiny controlled pilot | Owner approval required | Not approved here as final paid production infrastructure. |

## Required Owner-Controlled Inputs

| Input | Required value before launch | Current state | Release effect |
| --- | --- | --- | --- |
| Root domain | Domain controlled by owner | Owner approval required | Blocks HTTPS, callback URL, CORS, client build |
| API subdomain | Dedicated subdomain for API and gateway | Owner approval required | Blocks production bootstrap |
| Admin subdomain | Dedicated subdomain for admin UI | Owner approval required | Blocks admin operations |
| Payment callback path | Provider-specific callback URL under API domain | Owner approval required | Blocks real payment |
| Environment owner | Named human who owns production values | Owner approval required | Blocks production changes |
| SSH access method | Named access process and allowed operators | Owner approval required | Blocks production deployment |
| OS strategy | Keep and harden current OS, or rebuild/replace | Owner approval required | Blocks paid launch confidence |
| Docker image registry | Registry and image tag policy | Owner approval required | Blocks repeatable deploy |
| Alert channel | Channel and recipient for P0/P1 alerts | Owner approval required | Blocks observability gate |
| Off-box backup destination | Storage target and access owner | Owner approval required | Blocks paid production data safety |
| Payment provider | Approved provider and mode | Owner approval required | Blocks real payment |
| Upstream model provider | Approved provider and secret storage path | Owner approval required | Blocks paid gateway traffic |

## Domain Plan

| Purpose | Recommended record | Current value | Required before paid launch | Current state |
| --- | --- | --- | --- | --- |
| API and gateway | `api.codex.<owner-domain>` | Owner domain approval required | Yes | Blocked |
| Admin console | `admin.codex.<owner-domain>` | Owner domain approval required | Yes | Blocked |
| Payment callback | `https://api.codex.<owner-domain>/api/payment/callback/<provider>` | Owner domain and provider approval required | Yes for real payment | Blocked |
| Download page | `download.codex.<owner-domain>` | Owner domain approval required | Optional | Can defer with owner approval |
| Status page | `status.codex.<owner-domain>` | Owner domain approval required | Optional | Can defer with owner approval |

DNS rules:

- API and admin should use isolated subdomains.
- Do not share a path under another application for the API gateway.
- Scope cookies to Codex++ subdomains and unique cookie names.
- Restrict CORS to required Codex++ Manager and admin origins.
- Issue HTTPS certificates before any real user traffic.

## Server Directory Matrix

| Path | Purpose | Owner | Backup requirement |
| --- | --- | --- | --- |
| `/opt/codex-plus/docker-compose.prod.yml` | Production Compose file | DevOps or named environment owner | Yes |
| `/opt/codex-plus/.env.production` | Runtime values and secret references | Named environment owner | Encrypted/off-box only |
| `/opt/codex-plus/.env.production.previous` | Rollback values | Named environment owner | Encrypted/off-box only |
| `/opt/codex-plus/caddy/Caddyfile` | HTTPS reverse proxy config | DevOps or named environment owner | Yes |
| `/opt/codex-plus/backups/postgres` | PostgreSQL backup output | DevOps or named environment owner | Copy off-box |
| `/opt/codex-plus/logs` | App/proxy logs if bind-mounted | Ops owner | Retention controlled |
| `/opt/codex-plus/releases` | Release metadata | Release owner | Useful |
| `/opt/codex-plus/scripts` | Deployment, rollback, backup, restore, healthcheck scripts | DevOps or named environment owner | Yes |

## Public Port Matrix

| Port | Protocol | Purpose | Public exposure | Notes |
| --- | --- | --- | --- | --- |
| 22 | TCP | SSH | Restricted | Limit by source IP where possible. |
| 80 | HTTP | ACME challenge and redirect | Yes | Reverse proxy only. |
| 443 | HTTPS | API, admin, and callbacks | Yes | Reverse proxy only. |
| Backend internal port | HTTP | Sub2API backend | No | Docker/internal network. |
| PostgreSQL | TCP | Database | No | Never expose publicly. |
| Redis | TCP | Cache/rate-limit | No | Password required. |

## Service Matrix

| Service | Default container name | Image source | Public exposure | Healthcheck | Notes |
| --- | --- | --- | --- | --- | --- |
| Reverse proxy | `codex-plus-caddy` | `caddy:2` or owner-approved Nginx image | Yes | API health through HTTPS | Terminates TLS. |
| Sub2API backend/gateway | `codex-plus-backend` | Owner-approved image registry | Via proxy | `/health` | Owns admin API, client API, gateway, payment callbacks. |
| Admin frontend | `codex-plus-admin` | Owner-approved image registry or static artifact | Via proxy | `/` or `/health` | Requires auth. |
| PostgreSQL | `codex-plus-postgres` | `postgres:16-alpine` or owner-approved equivalent | No | `pg_isready` | Use volume and backups. |
| Redis | `codex-plus-redis` | `redis:7-alpine` or owner-approved equivalent | No | `redis-cli ping` | Password required. |

## Environment Variable Matrix

Real values belong on the server, secret manager, or owner-controlled password manager. This document records names and requirements only.

| Variable | Purpose | Example format | Required | Secret |
| --- | --- | --- | --- | --- |
| `ENVIRONMENT` | Runtime mode | `production` | Yes | No |
| `API_DOMAIN` | API/gateway domain | `api.codex.owner-domain.example` | Yes | No |
| `ADMIN_DOMAIN` | Admin domain | `admin.codex.owner-domain.example` | Yes | No |
| `PUBLIC_BASE_URL` | Public API URL | `https://api.codex.owner-domain.example` | Yes | No |
| `ADMIN_PUBLIC_URL` | Public admin URL | `https://admin.codex.owner-domain.example` | Yes | No |
| `POSTGRES_DB` | Database name | `codex_plus` | Yes | No |
| `POSTGRES_USER` | Database user | `codex_plus` | Yes | No |
| `POSTGRES_PASSWORD` | Database password | Server-only secret value | Yes | Yes |
| `DATABASE_URL` | Backend DB URL | Server-only connection string | Yes | Yes |
| `REDIS_PASSWORD` | Redis password | Server-only secret value | Yes | Yes |
| `REDIS_URL` | Backend Redis URL | Server-only connection string | Yes | Yes |
| `JWT_SECRET` | Auth/session signing | Server-only secret value | Yes | Yes |
| `SESSION_SECRET` | Session encryption | Server-only secret value | Yes | Yes |
| `UPSTREAM_PROVIDER_API_KEY` | Model provider key | Server-only secret value | Yes | Yes |
| `PAYMENT_PROVIDER` | Payment provider name | Owner-approved provider name | Yes for real payment | No |
| `PAYMENT_WEBHOOK_SECRET` | Payment callback verification | Server-only secret value | Yes for real payment | Yes |
| `CORS_ALLOWED_ORIGINS` | Admin/client origins | Approved origin list | Yes | No |
| `LOG_LEVEL` | Log verbosity | `info` | Yes | No |
| `BACKUP_RETENTION_DAYS` | Local backup retention | Owner-approved number | Yes | No |
| `DAILY_COST_CAP_CENTS` | Global daily cost cap | Owner-approved number | Yes | No |
| `USER_DAILY_COST_CAP_CENTS` | User daily cost cap | Owner-approved number | Yes | No |

## Secret Storage Rules

Allowed:

- `/opt/codex-plus/.env.production` with restricted file permissions.
- Cloud secret manager controlled by the owner.
- Encrypted password manager controlled by the owner.
- Temporary shell environment during an approved deployment.

Forbidden:

- Git repository.
- Markdown docs.
- Client app bundle.
- Browser-visible frontend bundle.
- Chat messages.
- Test reports without redaction.

## Backup Matrix

| Data | Backup method | Frequency | Retention | Restore evidence |
| --- | --- | --- | --- | --- |
| PostgreSQL | `pg_dump` gzip | Daily and before deploy | At least owner-approved local retention plus off-box copy | Restore drill required before paid launch |
| `.env.production` | Encrypted/off-box copy | After changes | Latest plus previous | Manual confirmation required |
| Reverse proxy config | File copy | After changes | Latest plus previous | Review required |
| Uploaded/static files | Implementation-specific backup | Owner approval required | Owner approval required | Restore drill required if durable |
| Redis | No durable backup unless used for queues | None if cache only | None if cache only | Document behavior |

## Monitoring Matrix

| Signal | Source | Alert threshold | Current state |
| --- | --- | --- | --- |
| API health | `/health` | 2 consecutive failures | Requires alert owner |
| Admin health | Admin URL | 2 consecutive failures | Requires alert owner |
| Payment callback failures | Backend metrics/logs | Sustained failure or fulfillment failure | Requires payment owner |
| Gateway 5xx rate | Gateway metrics | Greater than 2 percent for 5 minutes | Requires alert owner |
| Upstream provider error rate | Gateway metrics | Greater than 5 percent for 5 minutes | Requires provider owner |
| PostgreSQL availability | Container health and app metrics | Unhealthy or connection failures | Requires alert owner |
| Redis availability | Container health and app metrics | Unhealthy or unreachable | Requires alert owner |
| Disk usage | Host metric | Warning at 70 percent, critical at 85 percent | Requires alert owner |
| Memory usage | Host metric | Warning at 75 percent, swap activity critical | Requires alert owner |
| Daily cost | Cost ledger | Warning at 80 percent of approved cap | Requires cost owner |

## Readiness Checklist

| Item | Current state |
| --- | --- |
| Server OS strategy approved | Blocked by owner approval |
| Swap enabled if staying on 2 GiB RAM | Requires production execution evidence |
| Docker and Compose installed | Requires production execution evidence |
| Domain records configured | Blocked by owner approval |
| HTTPS works | Requires production execution evidence |
| `.env.production` created on server only | Blocked by owner approval and production execution |
| PostgreSQL backup script tested | Requires production execution evidence |
| Restore drill tested on non-production target | Requires production execution evidence |
| Admin login smoke passed | Requires E2E or production smoke evidence |
| User bootstrap smoke passed | Requires E2E or production smoke evidence |
| Payment callback sandbox or low-value test passed | Requires explicit payment approval and evidence |
| Cost caps configured | Blocked by owner approval |
| Alert channel tested | Blocked by owner approval |
| Rollback script reviewed | Requires deployment evidence |
