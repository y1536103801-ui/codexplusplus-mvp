# Codex++ Server Sizing and Scaling Guide

本文档用来回答两个问题：

- Codex++ 第一版上线应该买什么规格的服务器。
- 用户量增长后，是不是要整个项目迁移，还是可以平滑扩容。

结论先行：

- 当前 2C2G / 40G 服务器可以用于学习部署、staging、小范围内测和很小规模付费验证。
- 第一次正式付费上线，推荐至少 2C4G / 80G，更稳妥是 4C8G / 100G。
- 后续扩容不应该重写项目；正确路线是先纵向升级，再迁移到更强 ECS 或独立数据库，最后才多应用服务器 + 负载均衡。

## What This Server Actually Does

Codex++ 服务器不是模型训练机，也不是模型推理机。真正消耗大算力的是上游模型供应商。

Codex++ 生产服务器主要承担：

- 用户登录、会话、设备绑定。
- 套餐、权益、余额、模型权限判断。
- `bootstrap` 配置下发。
- OpenAI-compatible gateway 请求转发。
- 流式响应转发。
- 用量、计费、审计和日志记录。
- 支付回调和幂等处理。
- Admin 后台。
- PostgreSQL、Redis、Caddy/Nginx。
- 备份、监控、告警和健康检查。

因此第一阶段最容易成为瓶颈的通常不是 CPU，而是：

- Memory
- PostgreSQL connection and IO
- Redis availability
- Streaming request concurrency
- Disk usage from logs/backups/database
- Upstream provider timeout/retry behavior
- Gateway cost and abuse controls

## Current Server Verdict

Current candidate:

| Item | Value | Verdict |
| --- | --- | --- |
| CPU | 2 cores | acceptable for pilot |
| Memory | 2 GiB | tight |
| Disk | 40 GiB | tight |
| Bandwidth | 200 Mbps peak | enough for early API traffic |
| OS | CentOS 8.2 | not ideal long-term |
| Role | staging / pilot | yes |
| Role | long-term paid production | not recommended |

It can be used for:

- Learning production deployment.
- Docker Compose rehearsal.
- Domain/HTTPS test.
- Staging QA.
- Private beta.
- Very small paid pilot.

It should not be treated as:

- Final production capacity.
- Reliable long-term paid-user infrastructure.
- High-concurrency gateway infrastructure.
- Safe database host without off-box backup.

Required hardening if used:

- Add 2-4 GiB swap.
- Enforce log retention.
- Keep PostgreSQL backups.
- Copy important backups off-box.
- Enable memory/disk/API/payment/cost alerts.
- Avoid building heavy Docker images on the server.
- Consider OS rebuild before serious paid launch.

## Recommended Server Specs

| Stage | Recommended spec | Use case | Notes |
| --- | --- | --- | --- |
| Learning deployment | 2C2G / 40G | current server | Good enough for practice and staging. |
| Private beta | 2C2G or 2C4G / 60-80G | known users | Add swap and strict monitoring. |
| First paid validation | 2C4G / 80G minimum | small paid group | Lowest comfortable production line. |
| Safer first paid launch | 4C8G / 100G | recommended | Better for operators without production experience. |
| Growing paid product | 4C8G app + managed/separate database | stable paid users | Reduce data-loss and backup risk. |
| Scale-out stage | multiple app servers + load balancer + separate DB/Redis | larger traffic | Requires stateless app design. |

Recommended decision:

- Use the current 2C2G server to learn and test deployment.
- For real first paid launch, buy or upgrade to 4C8G / 100G if budget allows.
- If budget is tight, 2C4G / 80G is the minimum acceptable paid-launch target.

## Approximate Capacity Thinking

The exact capacity depends on implementation, provider latency, streaming duration, database indexes and logging volume. Do not promise a fixed number before load testing.

Use this conservative planning model:

| Active concurrent users | Suggested shape | Notes |
| --- | --- | --- |
| 1-5 | 2C2G with swap | Good for testing and private beta. |
| 5-20 | 2C4G | Keep logs and DB under control. |
| 20-50 | 4C8G | Recommended for first real paid launch confidence. |
| 50-150 | 4C8G app + separate/managed DB | Reduce database pressure on app machine. |
| 150+ | multiple app servers + load balancer | Requires production-grade observability and autoscaling plan. |

Important:

- "Registered users" is not the same as "concurrent active users".
- Long streaming responses hold connections for longer.
- One abusive user can create more pressure than many normal users.
- Cost caps and rate limits are as important as server specs.

## Scaling Principles

Design from day one so that later scaling is migration, not rewrite.

Rules:

- Keep backend application containers stateless.
- Store durable state in PostgreSQL.
- Store cache/rate-limit/session acceleration in Redis.
- Do not store critical user data only inside container filesystem.
- Keep all production config in `.env.production` or secret manager.
- Keep domain routing in Caddy/Nginx config.
- Keep backups restorable on another machine.
- Make gateway enforcement backend-owned, not client-owned.
- Use request IDs and user/device IDs in logs for diagnosis.

If these rules are followed, adding servers later is mostly infrastructure work.

## Scaling Path

### Stage 0: Current Server for Learning and Staging

Shape:

```text
2C2G / 40G
Caddy
Sub2API backend/admin
PostgreSQL
Redis
```

Use for:

- Learn SSH, Docker, Caddy, HTTPS.
- Test deployment scripts.
- Run staging QA.
- Verify backup and restore.

Exit trigger:

- You are ready to accept real paid users.
- Memory pressure appears.
- You want a cleaner OS baseline.

### Stage 1: First Paid Single Server

Recommended:

```text
4C8G / 100G
Caddy
Sub2API backend/admin
PostgreSQL
Redis
```

Minimum:

```text
2C4G / 80G
```

Use for:

- First paid launch.
- Small controlled public launch.
- Real payment callback.
- Real support workflow.

Required:

- Off-box PostgreSQL backup.
- Rollback script.
- Payment idempotency tests.
- Cost caps.
- Production smoke test.
- 24-72 hour observation window.

### Stage 2: Separate Data Layer

Shape:

```text
Caddy/Nginx
  -> app server
       -> managed/separate PostgreSQL
       -> managed/separate Redis
```

Use when:

- Database IO affects API latency.
- Disk or backup size becomes uncomfortable.
- Paid users depend on service stability.
- You want safer database restore and upgrade paths.

This is not a product rewrite. It is mostly:

- Create new database.
- Restore PostgreSQL backup.
- Point `DATABASE_URL` to new database.
- Point `REDIS_URL` to new Redis.
- Restart services.
- Run smoke tests.

### Stage 3: Multiple App Servers

Shape:

```text
Load balancer
  -> app server 1
  -> app server 2
  -> app server N
       -> shared PostgreSQL
       -> shared Redis
```

Use when:

- One app server CPU/memory is not enough.
- You need zero-downtime deploys.
- You need higher availability.
- Traffic has predictable spikes.

Requirements before this stage:

- Backend is stateless.
- Sessions/JWT do not depend on local disk.
- Uploads or generated files are externalized if used.
- PostgreSQL and Redis are shared.
- Gateway rate limits work across app instances.
- Logs are centralized or at least collected consistently.

### Stage 4: Autoscaling

Use only after Stage 3 is stable.

Autoscaling needs:

- Load balancer health checks.
- Immutable deploy artifacts.
- Shared database/Redis.
- Centralized secrets.
- Centralized logs/metrics.
- Safe deploy/rollback.

Do not start here. It adds complexity before the product needs it.

## Upgrade vs Migration vs Horizontal Scaling

| Option | What changes | Downtime | Complexity | When to use |
| --- | --- | --- | --- | --- |
| Vertical upgrade | Same server, bigger CPU/RAM/disk | short restart | low | first capacity issue |
| New bigger server migration | New server, same Compose shape | DNS cutover window | medium | OS cleanup or light server limit |
| Separate DB/Redis | Data layer moves out | planned window | medium | paid users and DB risk grow |
| Multiple app servers | app becomes horizontal | can be low | high | traffic/high availability need |
| Autoscaling | app count changes automatically | low if mature | high | larger scale only |

## Migration Does Not Mean Rewriting the Product

If the project follows the deployment docs, migration should look like:

1. Freeze deployment window.
2. Backup PostgreSQL.
3. Provision new server or database.
4. Copy deployment templates.
5. Create `.env.production` on new target.
6. Restore database.
7. Start services.
8. Run healthcheck.
9. Run production smoke test.
10. Switch DNS or load balancer target.
11. Observe metrics.
12. Keep old server for rollback until stable.

The code should not need to change if:

- Domains are configurable.
- Database URL is configurable.
- Redis URL is configurable.
- Provider keys are configurable.
- Client reads API base URL from production config/build setting.

## Metrics That Decide Upgrade

Collect these before spending money:

| Metric | Warning | Action |
| --- | --- | --- |
| Memory usage | >75% for 30 minutes | upgrade or reduce services |
| Swap usage | used during normal traffic | upgrade RAM |
| Disk usage | >70% | clean logs/backups or expand disk |
| API p95 latency | rising under normal traffic | inspect DB/gateway/upstream |
| Gateway 5xx | >2% for 5 minutes | incident review |
| PostgreSQL connections | near max | tune pool or upgrade DB |
| Redis unhealthy | any sustained issue | fix before paid launch |
| Payment callback failures | any sustained issue | stop paid launch |
| Daily upstream cost | >80% cap | throttle or disable expensive models |
| Backup duration | too long for deploy window | separate DB or optimize backup |

## Practical Recommendation for This Project

For Codex++:

1. Keep current 2C2G server for deployment learning and staging.
2. Add swap and practice Docker Compose deployment.
3. Do not put large public paid traffic on this server.
4. Before paid launch, choose:
   - minimum: 2C4G / 80G
   - recommended: 4C8G / 100G
5. Keep the first production launch single-server unless there is a clear reason to split early.
6. When paid users appear, prioritize off-box backup before fancy autoscaling.
7. Split PostgreSQL/Redis before adding many app servers.
8. Add load balancer only after backend is proven stateless.

## Provider Notes

For Alibaba Cloud style deployment:

- Lightweight Application Server can be upgraded through the console, but upgrade may involve restart and should be preceded by snapshot/backup.
- Lightweight server can be migrated to ECS through custom image / migration paths.
- ECS plus load balancer / auto scaling is the later-stage path, not the first step.

Official references:

- [Alibaba Cloud Simple Application Server upgrade guide](https://help.aliyun.com/zh/simple-application-server/user-guide/upgrade-a-simple-application-server)
- [Migrate data from Lightweight Application Server to ECS](https://help.aliyun.com/zh/simple-application-server/use-cases/migrate-data-from-lightweight-application-servers-to-ecs-instances-through)
- [Alibaba Cloud Auto Scaling overview](https://www.alibabacloud.com/zh/product/auto-scaling)

## Open Decisions

Before paid launch, decide:

- Whether to keep current server or buy a new one.
- Whether to rebuild OS before launch.
- First paid launch target spec.
- Off-box backup location.
- Alert channel.
- Maximum acceptable monthly server budget.
- Maximum daily upstream model cost risk.
