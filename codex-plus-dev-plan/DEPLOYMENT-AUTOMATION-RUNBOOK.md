# Codex++ Deployment Automation Runbook

本文档定义 Codex++ Phase 2 生产部署自动化方案。它面向 Codex 执行，也面向人工复核。默认目标是：先用一台服务器跑通小规模生产/付费验证，再在流量、成本和稳定性证明后升级为更强规格或拆分数据库。服务器规格和扩容路线见 [SERVER-SIZING-AND-SCALING-GUIDE.md](SERVER-SIZING-AND-SCALING-GUIDE.md)。

本文件不保存真实密钥。真实密钥只允许放在服务器受控路径、云平台 Secret Manager 或人工临时注入通道。

## Current Decision

Based on the server screenshot and user decision:

- Deployment mode: single-server Docker Compose for first production launch.
- Candidate server:
  - Public IP: `39.96.27.135`
  - Private IP: `172.24.62.185`
  - Region: Beijing
  - CPU: 2 cores
  - Memory: 2 GiB
  - Disk: 40 GiB
  - Bandwidth peak: 200 Mbps
  - OS shown: CentOS 8.2
- Business launch mode: both manual/redeem entitlement and real payment entitlement must exist.
- Domain strategy: existing root domain may be reused through isolated subdomains; if this causes DNS, cookie, certificate, ICP/record or brand conflict, use a new domain.

## Server Suitability

This server can be used for:

- Private beta.
- Small paid pilot.
- Low traffic production validation.
- Staging-like production rehearsal.

Conditions:

- Add 2-4 GiB swap.
- Use Docker Compose rather than manual process management.
- Do not run heavy builds on the server if memory is tight.
- Store PostgreSQL backups outside the server or copy them off-box regularly.
- Enable disk, memory, API, gateway, payment and cost alerts.
- Keep Postgres, Redis and app logs under strict retention.

Not recommended as the final long-term production shape when:

- Traffic becomes public and unpredictable.
- Payment volume grows.
- Upstream model cost risk becomes material.
- Support expectations become strict.
- Database size grows beyond what can be safely backed up/restored on the same machine.

Upgrade trigger:

- Memory usage stays above 75% for 30 minutes.
- Swap is used during normal traffic.
- Disk usage exceeds 70%.
- PostgreSQL CPU or IO affects API latency.
- More than 20 concurrent active users are expected.

Recommended upgrade path:

1. 2 cores / 4 GiB RAM minimum for first real paid launch.
2. 4 cores / 8 GiB RAM when traffic grows.
3. Managed PostgreSQL or a separate database server when paid users depend on the service.

Detailed sizing, migration and scale-out decisions are owned by [SERVER-SIZING-AND-SCALING-GUIDE.md](SERVER-SIZING-AND-SCALING-GUIDE.md).

OS note:

- CentOS 8.2 is not an ideal long-term target because CentOS 8 is end-of-life.
- Preferred rebuild target: Ubuntu 22.04 LTS, Ubuntu 24.04 LTS, Rocky Linux 9, or AlmaLinux 9.
- If the current CentOS server must be used, Codex must record package-source and security-update risk in the deployment report.

## Domain Strategy

It is acceptable to reuse a domain from another project if the following are true:

- You control DNS records for the domain.
- The other project will not conflict with the required subdomains.
- Cookies are isolated by subdomain and cookie name.
- Reverse proxy rules do not mix old project routes with Codex++ routes.
- The domain can issue HTTPS certificates for the new subdomains.
- Operational or regulatory constraints for the old project do not block this product.

Recommended subdomains:

| Purpose | Example | Required |
| --- | --- | --- |
| API and gateway | `api.codex.example.com` | yes |
| Admin console | `admin.codex.example.com` | yes |
| Payment callback | `api.codex.example.com/api/payment/callback/<provider>` | yes |
| Client download | `download.codex.example.com` | optional |
| Status page | `status.codex.example.com` | optional |

Do not use:

- The same subdomain as the old project.
- Path-based sharing such as `example.com/codex-plus` for the API gateway.
- Shared cookies scoped to `.example.com` unless intentionally designed.

If there is any conflict, buy a new domain and use dedicated subdomains.

## Deployment Target

Default production topology:

```text
Internet
  |
  v
Caddy or Nginx
  - HTTPS
  - reverse proxy
  - security headers
  - request size and timeout limits
  |
  +--> Sub2API backend and OpenAI-compatible gateway
  +--> Admin frontend
  |
  +--> PostgreSQL
  +--> Redis
```

Default server layout:

```text
/opt/codex-plus/
  docker-compose.prod.yml
  .env.production
  .env.production.previous
  caddy/
    Caddyfile
  backups/
    postgres/
  logs/
  releases/
  scripts/
```

## Automation Artifacts

Templates live in:

- [deployment/templates/docker-compose.prod.template.yml](deployment/templates/docker-compose.prod.template.yml)
- [deployment/templates/env.production.template](deployment/templates/env.production.template)
- [deployment/templates/Caddyfile.template](deployment/templates/Caddyfile.template)

Scripts live in:

- [deployment/scripts/deploy.sh](deployment/scripts/deploy.sh)
- [deployment/scripts/rollback.sh](deployment/scripts/rollback.sh)
- [deployment/scripts/backup-postgres.sh](deployment/scripts/backup-postgres.sh)
- [deployment/scripts/restore-postgres.sh](deployment/scripts/restore-postgres.sh)
- [deployment/scripts/healthcheck.sh](deployment/scripts/healthcheck.sh)

These scripts are templates. Before real production execution, Codex must confirm:

- `DEPLOY_ROOT`
- domain names
- Docker image names
- backend health path
- admin health path
- database name/user
- backup path
- whether production payment is authorized

## First-Time Server Bootstrap

Codex may prepare a command plan, but should not execute on production without explicit authorization.

Required packages:

- Docker Engine
- Docker Compose plugin
- curl
- tar
- openssl
- cron or systemd timer for backups

Suggested initial checks:

```bash
uname -a
cat /etc/os-release
free -h
df -h
ip addr
docker --version
docker compose version
```

Swap recommendation for 2 GiB RAM:

```bash
sudo fallocate -l 4G /swapfile
sudo chmod 600 /swapfile
sudo mkswap /swapfile
sudo swapon /swapfile
echo '/swapfile none swap sw 0 0' | sudo tee -a /etc/fstab
```

Firewall minimum:

| Port | Purpose | Public |
| --- | --- | --- |
| 22 | SSH | restricted if possible |
| 80 | HTTP challenge / redirect | yes |
| 443 | HTTPS | yes |
| PostgreSQL | database | no |
| Redis | cache | no |
| backend internal port | app | no |

## Deployment Flow

1. Build or publish backend/admin images from CI or local controlled environment.
2. SSH into server.
3. Copy template files to `/opt/codex-plus`.
4. Create `.env.production` from [deployment/templates/env.production.template](deployment/templates/env.production.template).
5. Fill domains, image tags and secret names.
6. Copy [deployment/templates/Caddyfile.template](deployment/templates/Caddyfile.template) to `/opt/codex-plus/caddy/Caddyfile`.
7. Copy [deployment/templates/docker-compose.prod.template.yml](deployment/templates/docker-compose.prod.template.yml) to `/opt/codex-plus/docker-compose.prod.yml`.
8. Run backup before deploy if this is an update.
9. Run deploy script.
10. Run healthcheck script.
11. Run production smoke test from [CODEX-AUTONOMOUS-TEST-RUNBOOK.md](CODEX-AUTONOMOUS-TEST-RUNBOOK.md).

Example:

```bash
cd /opt/codex-plus
bash scripts/backup-postgres.sh
bash scripts/deploy.sh
bash scripts/healthcheck.sh
```

## Rollback Flow

Rollback must be tested before a paid launch.

Preferred rollback:

1. Keep previous image tags in `.env.production.previous`.
2. Before deploy, copy current `.env.production` to `.env.production.previous`.
3. If deploy fails, run [deployment/scripts/rollback.sh](deployment/scripts/rollback.sh).
4. Run healthcheck.
5. Run minimum production smoke test.
6. Record incident and root cause.

Database rollback:

- Prefer forward-compatible migrations.
- Avoid destructive migrations in first production launch.
- If schema rollback is needed, stop and ask for human approval.
- Restore from backup only after explicit confirmation because it may lose writes after backup time.

## Backup Flow

Minimum backup policy for single-server launch:

- PostgreSQL logical backup before every deploy.
- Daily PostgreSQL logical backup.
- Keep last 7 daily backups on server.
- Copy important backups off-box before public paid launch.
- Redis is treated as cache unless implementation uses it for durable queues.

Backup command:

```bash
cd /opt/codex-plus
bash scripts/backup-postgres.sh
```

Restore drill:

```bash
cd /opt/codex-plus
bash scripts/restore-postgres.sh backups/postgres/<backup-file>.sql.gz
```

Do not restore into production without explicit approval.

## Production Smoke Test

After deploy:

1. `https://api-domain/health` returns healthy.
2. `https://admin-domain` loads.
3. Admin test account can log in.
4. Active user can log in through Codex++ Manager.
5. Bootstrap returns `available`.
6. `Codex++ Cloud` provider syncs.
7. One low-cost model request succeeds.
8. Usage event is recorded.
9. Logs do not expose secrets.
10. Alerts and dashboards are normal.

## Stop Conditions

Codex must stop and ask before:

- Running production deploy for the first time.
- Running database migrations against production.
- Running restore against production.
- Enabling real payment callbacks.
- Triggering real payment.
- Deleting or changing real user entitlements.
- Opening production traffic to public users.
- Rotating production secrets.

## Open Decisions

These should be filled in [PRODUCTION-ENVIRONMENT-MATRIX.md](PRODUCTION-ENVIRONMENT-MATRIX.md) and [BUSINESS-CONFIG-DECISION-TABLE.md](BUSINESS-CONFIG-DECISION-TABLE.md):

- Final domain or subdomains.
- Whether to rebuild the server OS before launch.
- Payment provider.
- Production Docker image registry.
- First launch price and plan limits.
- Alert channel.
- Backup off-box destination.
