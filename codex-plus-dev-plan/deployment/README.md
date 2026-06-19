# Codex++ Deployment Templates

This folder contains production deployment templates for the first single-server Docker Compose launch.

Use with:

- [../DEPLOYMENT-AUTOMATION-RUNBOOK.md](../DEPLOYMENT-AUTOMATION-RUNBOOK.md)
- [../PRODUCTION-ENVIRONMENT-MATRIX.md](../PRODUCTION-ENVIRONMENT-MATRIX.md)

Files:

- `templates/docker-compose.prod.template.yml`: Compose template.
- `templates/env.production.template`: environment variable template without real secrets.
- `templates/Caddyfile.template`: HTTPS reverse proxy template.
- `scripts/deploy.sh`: pull and start services.
- `scripts/rollback.sh`: restore previous env/image tags and restart.
- `scripts/backup-postgres.sh`: create PostgreSQL logical backup.
- `scripts/restore-postgres.sh`: restore backup into a target database.
- `scripts/healthcheck.sh`: verify API/admin health.

Before production use:

1. Copy templates to `/opt/codex-plus`.
2. Fill `.env.production` on the server only.
3. Confirm Docker image names.
4. Confirm domains.
5. Run backup before deploy.
6. Get explicit human authorization before real production deploy.
