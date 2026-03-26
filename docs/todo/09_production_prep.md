# Task 9: Polish + Production Prep

Containers, CI complete, security review done,
runbook in README.

---

## Checklist

### Dockerfile

```
[ ] Multi-stage build:
    Stage 1: golang:1.22-alpine - build binary
    Stage 2: alpine:latest - copy binary only, no source

[ ] Non-root user in final image:
    RUN addgroup -S hbsqueue && adduser -S hbsqueue -G hbsqueue
    USER hbsqueue

[ ] EXPOSE port (matches cfg.Port default)
[ ] ENTRYPOINT ["/hbsqueue"]

[ ] Inline comments on each stage and non-root user setup
[ ] Verify: docker build -t hbs-queue . && docker run --env-file .env hbs-queue
```

### docker-compose.yml (prod prep)

```
[ ] Review docker-compose.yml for production:
    - Resource limits (mem_limit, cpus) on all services
    - Restart policies (restart: unless-stopped)
    - Log driver config (json-file with max-size/max-file)
    - Read-only root filesystem on app container where possible

[ ] Verify: docker compose up → /ready returns 200
```

### Health Checks

```
[ ] /ready - 200 only if DB ping succeeds (already done in Task 3)
[ ] /health - version + DB status JSON (already done in Task 3)
[ ] Confirm both are wired as HEALTHCHECK in Dockerfile
```

### Metrics (Optional)

```
[ ] If adding Prometheus:
    - middleware: count requests by method/path/status
    - GET /metrics endpoint (no auth - internal network only)
    - Doc comment on metrics middleware
[ ] Skip if not needed (YAGNI)
```

### Update CI

```
[ ] .github/workflows/ci.yml - final version:
    On push to main (runs-on: [self-hosted, dev]):
    - Lint (golangci-lint)
    - Test (docker compose provides Postgres)
    - Build binary / rebuild image
    - docker compose up -d app (rolling restart)

    On git tag (v*.*.*) (runs-on: [self-hosted, dev]):
    - Lint
    - Test
    - Pre-deploy backup (./scripts/db-backup.sh)
    - Blue/green swap (./scripts/bg-deploy.sh)
    - Deploy new app container
    - Health check
    - (Optional) Build + push Docker image to registry
```

### Backup Strategy (3-2-1-1-0)

```
[ ] 3 copies of data (production DB + 2 backups)
[ ] 2 different media types (e.g., local disk + remote object storage)
[ ] 1 offsite copy (different physical location from prod)
[ ] 1 immutable/air-gapped copy (protection against ransomware)
[ ] 0 errors — verify backups regularly, test restores on schedule

Implementation:
    - ./scripts/db-backup.sh runs pg_dump to local backup dir
    - Cron or CI pushes a copy to offsite storage
    - At least one copy is immutable (e.g., object lock, WORM, or air-gapped)
    - Restore test: ./scripts/db-restore.sh <backup-file> --yes
    - Log and alert on backup failures — a backup that silently fails is no backup
```

### Security Review

```
[ ] All secrets via environment variables only - none hardcoded
[ ] Input validation on all request types (Valid() methods)
[ ] Auth middleware on every non-health route:
    - API routes: withAPIKey
    - Webhook routes: withWebhook
[ ] Non-root user in Docker image
[ ] No debug endpoints exposed in prod
[ ] .gitignore covers .envrc, .env, binaries
[ ] Review: no credentials in logs (slog structured fields)
```

### README.md - Final Pass

```
[ ] Docker + docker-compose quickstart:
    docker compose up
    curl localhost:8080/ready

[ ] Health + metrics endpoint reference (method, path, auth, response shape)

[ ] CI/CD pipeline overview:
    - main branch → dev deploy
    - git tag → prod deploy
    - How to trigger: git tag v1.0.0 && git push --tags

[ ] Security notes:
    - Env-only secrets (never in source)
    - Auth coverage (all routes)
    - Non-root Docker user

[ ] Runbook - common operational tasks:
    Run migrations:
        The service runs migrations automatically on startup.
        To run manually: ./hbsqueue migrate-up (if CLI added)

    Roll back migration:
        psql -h <host> -U hbsqueue -d hbsqueue_prod -f migrations/001_workflow_state.down.sql

    Restart service:
        cd /opt/hbs-queue && docker compose restart app
        docker compose ps
        docker compose logs -f app

    Blue/green swap:
        ./scripts/bg-deploy.sh
        ./scripts/bg-status.sh

    Backup (manual):
        ./scripts/db-backup.sh
    Restore:
        ./scripts/db-restore.sh <backup-file> --yes

    Check job queue depth:
        psql -h <host> -U hbsqueue -d hbsqueue_prod -c \
          "SELECT state, count(*) FROM river_job GROUP BY state;"

    Manually retry a failed job:
        psql -h <host> -U hbsqueue -d hbsqueue_prod -c \
          "UPDATE river_job SET state = 'available' WHERE id = <job_id>;"

    Check workflow state for a client:
        psql -h <host> -U hbsqueue -d hbsqueue_prod -c \
          "SELECT workflow_type, status, current_step, error, updated_at
           FROM workflows WHERE client_id = '<crm_id>'
           ORDER BY created_at DESC LIMIT 10;"
```

### Verify

```
[ ] docker compose up → /ready returns 200
[ ] docker compose up → /health returns version + DB status
[ ] make test → all unit tests pass
[ ] make test-integration → all integration tests pass
[ ] Push to main → CI passes → deploys to dev VM
[ ] git tag v1.0.0 → CI passes → deploys to prod
[ ] Security checklist all green
[ ] Production ready
```
