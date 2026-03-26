# Task 8: Deploy Dev Environment - Docker Compose + Self-Hosted Runner

Dev host live with Docker Compose, CI runner on a
separate VM, push to `main` builds and deploys automatically. Tagged releases
(`vX.X.X`) trigger production deploys.

> This task moves off localhost. The service has already been proven locally
> through Task 7. Everything here is infrastructure, ops, and CI/CD wiring.
> Docker Compose is used across all environments (localhost/dev/prod) for
> consistency.

## Architecture

```
ci01 (CI/CD server)          docker01.mgmt.infra.ckdev.io       backup01
┌─────────────────┐          ┌────────────────────────────┐     ┌──────────┐
│ GitHub Actions   │──SSH──▶ │ Nginx (reverse proxy :8080)│     │ verified │
│ Self-Hosted      │         │   ├─▶ hbs-queue-blue       │────▶│ pg_dump  │
│ Runner           │         │   └─▶ hbs-queue-green      │     │ files    │
│                  │         │                             │     │ (copy 2) │
│ Builds image,    │         │ Postgres 18 (:5432)         │     └──────────┘
│ pushes to GHCR,  │         │   └─ named volume           │
│ triggers deploy  │         │                             │
│                  │         │ Swagger UI (:8081)           │
└─────────────────┘          └────────────────────────────┘
```

**Key decisions:**
- **Blue/green on the app, not the database.** The app is stateless — swap
  containers for zero-downtime deploys. Postgres is a single instance with
  backups. Destructive DB migrations are handled by backward-compatible schema
  changes + pre-deploy `pg_dump`, not by running two databases.
- **Separate CI server.** Running CI on the app host risks starving production
  containers during builds. ci01 builds and pushes to GHCR; docker01 only pulls
  and runs.
- **GHCR for images.** `ghcr.io/cloudkey-io/hbs-queue` decouples build
  artifacts from any single VM. Any Docker host can pull the same image.
- **Docker Compose secrets.** Secret files on host at `/opt/hbs-queue/secrets/`
  (chmod 600, root-owned), mounted via Compose `secrets` directive. No plaintext
  `.env` files.

---

## Checklist

### Prepare docker01 (Ubuntu 24.04)

```
[ ] SSH access confirmed to docker01.mgmt.infra.ckdev.io
[ ] Uninstall conflicting packages if needed:
    sudo apt remove $(dpkg --get-selections docker.io docker-compose \
    docker-compose-v2 docker-doc podman-docker containerd runc | cut -f1)
[ ] Install Docker Engine (not Docker Desktop):
    sudo apt update
    sudo apt install ca-certificates curl
    sudo install -m 0755 -d /etc/apt/keyrings
    sudo curl -fsSL https://download.docker.com/linux/ubuntu/gpg \
      -o /etc/apt/keyrings/docker.asc
    sudo chmod a+r /etc/apt/keyrings/docker.asc
    echo "deb [arch=$(dpkg --print-architecture) \
      signed-by=/etc/apt/keyrings/docker.asc] \
      https://download.docker.com/linux/ubuntu \
      $(. /etc/os-release && echo "$VERSION_CODENAME") stable" | \
      sudo tee /etc/apt/sources.list.d/docker.list > /dev/null
    sudo apt update
    sudo apt install docker-ce docker-ce-cli containerd.io \
      docker-buildx-plugin docker-compose-plugin

[ ] Add deploy user to docker group:
    sudo usermod -aG docker $USER

[ ] Verify: docker compose version
```

### Prepare ci01 (CI/CD Server)

```
[ ] SSH access confirmed to ci01
[ ] Install Docker Engine (same steps as docker01 - needed for docker build)
[ ] Go to: github.com/CloudKey-io/hbs-queue → Settings → Actions → Runners
[ ] Install the runner on ci01:
    - Download actions-runner for linux-x64
    - ./config.sh --url https://github.com/CloudKey-io/hbs-queue \
        --token <TOKEN> --labels self-hosted,ci
    - sudo ./svc.sh install
    - sudo ./svc.sh start

[ ] Verify runner shows as online in GitHub UI with label: ci
```

### CI Secrets (GitHub Actions)

```
[ ] Add secrets in GitHub repo settings → Secrets → Actions:
    - DOCKER01_HOST     (e.g., docker01.mgmt.infra.ckdev.io)
    - DOCKER01_SSH_KEY  (private key for deploy user on docker01)
    - DOCKER01_USER     (deploy user on docker01)

    Note: API_KEY and DATABASE_URL are NOT stored here.
    They are Docker Swarm secrets on docker01 (see below).
```

### Runtime Secrets (Docker Compose)

```
[ ] Create secret directory on docker01:
    sudo mkdir -p /opt/hbs-queue/secrets
    sudo chmod 700 /opt/hbs-queue/secrets

[ ] Create secret files (chmod 600, root-owned):
    echo -n "<api-key>" | sudo tee /opt/hbs-queue/secrets/api_key
    echo -n "<database-url>" | sudo tee /opt/hbs-queue/secrets/database_url
    sudo chmod 600 /opt/hbs-queue/secrets/*

[ ] docker-compose.prod.yml uses secrets directive:
    secrets:
      api_key:
        file: /opt/hbs-queue/secrets/api_key
      database_url:
        file: /opt/hbs-queue/secrets/database_url

[ ] docker-entrypoint.sh - bridge secret files to env vars:
    - Reads /run/secrets/api_key → exports API_KEY
    - Reads /run/secrets/database_url → exports DATABASE_URL
    - exec's the binary

    Secret files are not visible in docker inspect.
    No plaintext .env files on disk.
```

### Dockerfile

```
[ ] Dockerfile - multi-stage build:
    Stage 1 (build): golang:1.26-alpine, build with ldflags
    Stage 2 (run):   alpine:latest, copy binary, entrypoint script, non-root user
```

### Docker Compose - Production Stack on docker01

```
[ ] docker-compose.prod.yml with services:
    - postgres: postgres:18-alpine, port 5432, named volume, healthcheck
    - app-blue: hbs-queue from GHCR, connects to postgres, uses secrets
    - app-green: hbs-queue from GHCR, connects to postgres (scaled to 0 initially)
    - nginx: nginx:alpine, reverse proxy routing to active app slot
    - swagger-ui: swaggerapi/swagger-ui, port 8081, serves openapi.yaml

[ ] Non-sensitive config as environment variables in compose file:
    ENV=dev, PORT=8080, etc.

[ ] docker compose -f docker-compose.prod.yml up -d
[ ] Verify: curl http://docker01.mgmt.infra.ckdev.io:8080/ready → 200
[ ] Verify: Swagger UI accessible at http://docker01.mgmt.infra.ckdev.io:8081
```

### Nginx Reverse Proxy Config

```
[ ] nginx/nginx.conf - HTTP reverse proxy:
    - upstream app_active { server app-blue:8080; }
    - server { listen 8080; proxy_pass http://app_active; }

[ ] Health check pass-through so nginx returns 502 if app is down
```

### Blue/Green Deploy Script

```
[ ] scripts/bg-deploy.sh - automated blue/green app deploy:
    1. Identify current active slot (blue or green)
    2. Pull latest image on inactive slot
    3. Start inactive slot
    4. Health-check inactive slot (curl /ready with retries)
    5. Swap nginx upstream to inactive slot (rewrite conf + nginx reload)
    6. Verify traffic is hitting new slot
    7. Stop old slot

    - All steps logged with timestamps
    - Exit on any failure with clear error message
    - Rollback: if health check fails, stop the new container, keep old running
    - Usage: ./scripts/bg-deploy.sh [--dry-run]

[ ] scripts/bg-status.sh - show current active slot and health of both
```

### Backup Scripts (3-2-1-1-0 Strategy)

```
Target: 3 copies, 2 media types, 1 offsite, 1 immutable, 0 unverified backups.

Today (Task 4):
  - Copy 1: local on docker01
  - Copy 2: mgmt backup server (access method TBD - mount or rsync)
  - Verify: automated restore test after every dump (the "0")

Future:
  - Copy 3 + immutable: S3-compatible store with object lock, or
    VMware-level immutable snapshots (the "1 offsite" + "1 immutable")

[ ] scripts/db-backup.sh - automated pg_dump + restore-verify:
    1. pg_dump from postgres container (compressed, timestamped)
    2. Restore into temp DB, run sanity check (table existence, row counts)
    3. Drop temp DB
       - If verify fails, log error + skip copy steps (don't propagate bad backups)
       - This is the "0" in 3-2-1-1-0: zero unverified backups
    4. Store locally in ./backups/ (copy 1 - local disk)
    5. Copy to backup server (copy 2 - configurable path, mount or rsync)
    6. (future) Push to immutable store (copy 3 - offsite + immutable)
    7. Prune local backups older than N days (configurable)

    - Logs every step with timestamps
    - Usage: ./scripts/db-backup.sh
    - Config via env vars: BACKUP_LOCAL_DIR, BACKUP_REMOTE_HOST,
      BACKUP_REMOTE_DIR, BACKUP_RETAIN_DAYS

[ ] scripts/db-restore.sh - restore from a backup file:
    1. Accept backup file path as argument
    2. Confirm target database (prompt or --yes flag)
    3. Drop and recreate target database
    4. pg_restore / psql < dump
    5. Verify restore with a row count or health check

    - Usage: ./scripts/db-restore.sh <backup-file> [--yes]

[ ] Test backup + restore cycle on VM:
    - Insert test data → backup → wipe DB → restore → verify data intact
```

### CI/CD Workflow

```
[ ] Update .github/workflows/ci.yml:

    On push to main:
      runs-on: [self-hosted, ci]
      steps:
        - Checkout code
        - Run tests + lint
        - Build Docker image with version tags
        - Push to ghcr.io/cloudkey-io/hbs-queue
        - SSH to docker01: pull image + run bg-deploy.sh
        - Health check: curl docker01:8080/ready

    On git tag (v*.*.*):
      runs-on: [self-hosted, ci]
      steps:
        - Checkout code
        - Run tests + lint
        - Build Docker image tagged with version
        - Push to GHCR
        - SSH to docker01: run db-backup.sh (pre-deploy backup)
        - SSH to docker01: pull image + run bg-deploy.sh
        - Health check
        - (optional) Notify

[ ] Makefile targets for VM ops:
    - make deploy - pull latest, run bg-deploy.sh
    - make vm-status - docker compose ps + bg-status.sh
```

### Backup Cron on docker01

```
[ ] Set up cron job for automated backups:
    # Daily at 2 AM
    0 2 * * * /opt/hbs-queue/scripts/db-backup.sh >> \
    /var/log/hbs-queue-backup.log 2>&1

[ ] Configure BACKUP_REMOTE_HOST for copy 2 (backup01)
[ ] Verify: manually run db-backup.sh → check copies 1 and 2 exist
```

### README.md (append during this task)

```
[ ] Deployment architecture overview (link to docs/deploy-architecture diagram)
[ ] CI/CD server setup (ci01 with self-hosted runner)
[ ] docker01 setup (Docker Engine + single-node swarm)
[ ] Deployment model: push to main = auto-deploy, tags = prod deploy
[ ] How to trigger a deploy: git tag v1.0.0 && git push --tags
[ ] Blue/green app deploy flow
[ ] Backup/restore script usage and 3-2-1 strategy overview
[ ] Backup cron schedule
```

### Verify

```
[ ] git push origin main → ci01 runner picks up job
[ ] Image built and pushed to GHCR
[ ] ci01 SSHs to docker01, pulls image, runs bg-deploy.sh
[ ] curl http://docker01.mgmt.infra.ckdev.io:8080/ready  → 200
[ ] curl http://docker01.mgmt.infra.ckdev.io:8080/health → 200
[ ] ./scripts/bg-deploy.sh → swaps to standby slot, zero downtime
[ ] ./scripts/bg-status.sh → shows correct active slot
[ ] ./scripts/db-backup.sh → creates timestamped dump + copies to backup01
[ ] ./scripts/db-restore.sh <dump> --yes → restores successfully
[ ] Backup cron runs, copies 1 + 2 verified
[ ] ✅ Milestone: dev host live, Docker Compose running, blue/green app deploys,
    restore-verified backups, CI/CD pipeline working end-to-end
```
