# Deployment Architecture

HostBill Queue Service (hbsqueue) orchestrates customer onboarding and
de-boarding workflows across VCD, Zerto, Keycloak, HostBill, and Active
Directory. It also sends data to Reddit for calculating advertisement conversion
rates.

All 3 environments (localhost, dev, prod) all use Docker Compose. Custom shell
scripting automates blue/green deployments and DB backups for dev/prod.

This document describes how the service is built, deployed, and operated.

## Table of Contents

- [Overview Diagram](#overview-diagram)
- [Infrastructure](#infrastructure)
- [Environments](#environments)
  - [Promotion Flow](#promotion-flow)
- [Branch Protection](#branch-protection)
- [CI/CD Flow](#cicd-flow)
  - [Merge to Main (Dev Auto-Deploy)](#merge-to-main-dev-auto-deploy)
  - [Tag vX.X.X (Production Release)](#tag-vxxx-production-release)
- [Blue/Green App Deploys](#bluegreen-app-deploys)
  - [Deploy Sequence](#deploy-sequence-scriptsbg-deploysh)
  - [Check Status](#check-status)
- [Database](#database)
- [Backups (3-2-1-1-0)](#backups-3-2-1-1-0)
  - [What's Implemented Now](#whats-implemented-now)
  - [What's Planned (Future)](#whats-planned-future)
  - [Schedule](#schedule)
  - [Manual Operations](#manual-operations)
- [Why a Separate CI Server?](#why-a-separate-ci-server)
- [Network and Ports](#network-and-ports)
  - [Traffic Flow (Dev Environment)](#traffic-flow-dev-environment)
  - [Required Firewall Rules (Dev)](#required-firewall-rules-dev)
  - [Prod Network (Future)](#prod-network-future)
- [Secrets](#secrets)
  - [CI Secrets (GitHub Actions)](#ci-secrets-github-actions)
  - [Runtime Secrets (Docker Compose)](#runtime-secrets-docker-compose)
- [Connecting to Services on docker01](#connecting-to-services-on-docker01)
- [Rollback](#rollback)
- [Troubleshooting](#troubleshooting)
  - [Check What's Running](#check-whats-running)
  - [View Application Logs](#view-application-logs)
  - [Check if the App Can Reach Postgres](#check-if-the-app-can-reach-postgres)
  - [Connect to Postgres Manually](#connect-to-postgres-manually)
  - [CI Pipeline Failed](#ci-pipeline-failed)

## Overview Diagram

<p align="center">
  <img src="deploy-architecture.svg" alt="Deployment Architecture" width="600">
</p>

## Infrastructure

| Host                            | Role               | Notes                                                                                                                           |
| ------------------------------- | ------------------ | ------------------------------------------------------------------------------------------------------------------------------- |
| `ci01`                          | CI/CD server       | GitHub Actions self-hosted runner. Builds images, pushes to GHCR, triggers deploys via SSH. No application containers run here. |
| `docker01.mgmt.infra.ckdev.io`  | Application host   | Runs Postgres, Nginx, Swagger UI, and the hbs-queue app containers via Docker Compose.                                          |
| Mgmt backup server              | Backup storage     | Receives verified copy 2 of database backups. Access method TBD (mount or rsync).                                               |
| `ghcr.io/cloudkey-io/hbs-queue` | Container registry | GitHub Container Registry. All images are built on ci01 and pulled by docker01.                                                 |

## Environments

Dev and prod are on **physically separate networks in different datacenters**.
This is intentional, a misconfigured dev environment cannot reach prod data, and
destructive testing in dev carries zero risk to production.

All environments use Docker Compose for consistency — same tools, same layout,
same deploy scripts.

| Environment | Network        | Hosts                                     | Deploys via     |
| ----------- | -------------- | ----------------------------------------- | --------------- |
| Localhost   | `127.0.0.1`    | Developer workstation                     | `make run`      |
| Dev         | `10.42.0.0/16` | `docker01.mgmt.infra.ckdev.io`, `ci01`    | Merge to `main` |
| Prod        | `10.25.0.0/16` | `docker01.mgmt.infra.cloudkey.io,` `ci01` | Tag `vX.X.X`    |

### Promotion Flow

```
feature branch → PR → merge to main → auto-deploy to dev
                                          ↓
                                     test in dev
                                          ↓
                              git tag vX.X.X → push tags
                                          ↓
                              image tagged in GHCR (ready for prod)
                                          ↓
                              deploy to prod (10.25.0.0/16) — not wired yet
```

Tagged releases build a versioned image and push it to GHCR. Prod deployment is
not automated yet. When a prod host is provisioned on the `10.25.0.0/16` network,
the tag workflow will be extended to SSH and deploy there.

## Branch Protection

The `main` branch has protection rules enforced in GitHub:

- **No direct pushes** - all changes go through pull requests. This includes
  force pushes (`--force`, `--force-with-lease`), which are blocked entirely.
- **Required reviews** - PRs require at least one approval before merging.
- **Required status checks** - CI (tests + lint) must pass before a PR can be
  merged.
- **No bypass** - these rules apply to everyone, including admins.

This means code only reaches `main` through a reviewed, passing PR. The CI/CD
pipeline triggers on merge, not on push. There is no way to deploy untested or
un-reviewed code.

## CI/CD Flow

### Merge to `main` (Dev Auto-Deploy)

1. ci01 runner picks up the job
2. Runs tests and linter
3. Builds Docker image, tags with commit SHA
4. Pushes to GHCR
5. SSHs to docker01, runs `db-backup.sh` (pre-deploy safety net)
6. Pulls the new image, runs `bg-deploy.sh`
7. Health check confirms `/ready` returns 200

### Tag `vX.X.X` (Production Release)

Same as above, but:

- Image is also tagged with the version number (e.g., `v1.2.0`)
- Deploys to the prod host instead of docker01

To trigger a release:

```sh
git tag v1.0.0
git push --tags
```

## Blue/Green App Deploys

Both `hbs-queue-blue` and `hbs-queue-green` connect to the same Postgres
instance. Nginx routes all traffic to whichever slot is active.

### Deploy Sequence (`scripts/bg-deploy.sh`)

1. Identify current active slot (blue or green)
2. Pull latest image on the standby slot
3. Start standby container
4. Health-check standby (`/ready` with retries)
5. Swap Nginx upstream to the standby slot, reload Nginx
6. Verify traffic is hitting the new slot
7. Stop the old slot

**If the health check fails**, the new container is stopped and the old slot
keeps serving. No downtime, no partial deploy.

### Check Status

```sh
./scripts/bg-status.sh
```

Shows which slot is active and the health of both containers.

## Database

Single Postgres 18 instance on docker01 with a named Docker volume. Migrations
run programmatically at app startup. River migrates its own tables, then app
migrations (`internal/db/migrations/`) run in sorted order.

## Backups (3-2-1-1-0)

The backup strategy targets **3-2-1-1-0**: 3 copies, 2 media types, 1 offsite, 1
immutable, 0 unverified backups.

### What's Implemented Now

| Copy | Location                      | Method           |
| ---- | ----------------------------- | ---------------- |
| 1    | docker01 local (`./backups/`) | `pg_dump` direct |

Every backup is **restore-tested** before being copied anywhere. The script:

1. Runs `pg_dump` (compressed, timestamped)
2. Restores into a temporary database
3. Runs a sanity check (row counts, table existence)
4. Drops the temporary database
5. Only if verification passes, copies to backup server

If verification fails, the dump is flagged and **not propagated**. This is the
most valuable part of the strategy — without it, you could be backing up
corrupted dumps for weeks without knowing.

### What's Planned (Future)

| Copy | Location            | Method                                              |
| ---- | ------------------- | --------------------------------------------------- |
| 2    | Mgmt backup server  | Mount or rsync (TBD)                                |
| 3    | Immutable + offsite | S3-compatible store with object lock, or equivalent |

### Schedule

Daily at 2:00 AM via cron on docker01. Retention is configurable via
`BACKUP_RETAIN_DAYS` (local pruning).

### Manual Operations

```sh
# Run a backup now
./scripts/db-backup.sh

# Restore from a specific backup
./scripts/db-restore.sh /path/to/backup.sql.gz --yes
```

## Why a Separate CI Server?

Running GitHub Actions on the same host as production containers means a runaway
build (memory leak in tests, hung process, disk-filling artifact) can starve the
app of CPU, memory, or disk. ci01 isolates that blast radius:

- ci01 does the heavy lifting (build, test, push)
- docker01 only pulls images and runs containers
- If ci01 goes down, the running app is unaffected

## Network and Ports

### Traffic Flow (Dev Environment)

```
External (GitHub.com)
  │
  │  HTTPS (443 outbound from ci01)
  │  ├─ ci01 polls GitHub for workflow jobs
  │  └─ ci01 pushes images to ghcr.io
  │
Dev Network (10.42.0.0/16)
  │
  ├─ ci01
  │    │
  │    └── SSH (22) ──────────────▶ docker01
  │                                   │
  │                                   ├─ :8080  Nginx (reverse proxy)
  │                                   │    └─▶ app-blue or app-green (:8080 internal)
  │                                   │
  │                                   ├─ :8081  Swagger UI
  │                                   │
  │                                   ├─ :6061  Debug / pprof (internal only)
  │                                   │
  │                                   └─ :5432  Postgres (container-internal,
  │                                             not exposed to host by default)
  │
  └─ Mgmt backup server
       └── receives verified backups from docker01 (mount or rsync, TBD)
```

### Required Firewall Rules (Dev)

**Inbound to docker01:**

| Source           | Port | Protocol   | Purpose            |
| ---------------- | ---- | ---------- | ------------------ |
| ci01             | 22   | TCP (SSH)  | Deploy commands    |
| Internal clients | 8080 | TCP (HTTP) | Application access |
| Internal clients | 8081 | TCP (HTTP) | Swagger UI         |
| Internal clients | 6061 | TCP (HTTP) | Debug / pprof      |

**Outbound from docker01:**

| Destination        | Port | Protocol    | Purpose               |
| ------------------ | ---- | ----------- | --------------------- |
| `ghcr.io`          | 443  | TCP (HTTPS) | Pull container images |
| VCD endpoint       | 443  | TCP (HTTPS) | VCD API calls         |
| Keycloak endpoint  | 8443 | TCP (HTTPS) | Keycloak API calls    |
| Zerto endpoint     | 9989 | TCP (HTTPS) | Zerto API calls       |
| Active Directory   | 389  | TCP         | LDAP                  |
| Active Directory   | 636  | TCP         | LDAPS (TLS)           |
| Mgmt backup server | TBD  | TBD         | Backup copy 2         |

**Outbound from ci01:**

| Destination             | Port | Protocol    | Purpose                |
| ----------------------- | ---- | ----------- | ---------------------- |
| `github.com`, `ghcr.io` | 443  | TCP (HTTPS) | Pull jobs, push images |
| docker01                | 22   | TCP (SSH)   | Deploy commands        |

Postgres is **not exposed** on the host network by default. The app containers
connect via Docker's internal network. To connect manually for debugging, use
`docker exec` into the Postgres container or temporarily bind the port.

### Prod Network (Future)

Prod hosts will be on `10.25.0.0/16`. The same firewall rules apply — the prod
Docker host needs the same outbound access to external services and inbound SSH
from whatever runs the prod deploy.

## Secrets

Secrets live in two places, for two different purposes.

### CI Secrets (GitHub Actions)

Stored in GitHub repo settings under **Settings > Secrets > Actions**. These are
used during build and deploy, they never end up in the container image.

| Secret             | Purpose                                 |
| ------------------ | --------------------------------------- |
| `DOCKER01_HOST`    | SSH target for deploys                  |
| `DOCKER01_SSH_KEY` | Private key for deploy user on docker01 |
| `DOCKER01_USER`    | Deploy user on docker01                 |

Scoped per environment. When prod is added, a separate set will be created
(e.g., `PROD_HOST`, `PROD_SSH_KEY`). Never reuse dev secrets in prod.

### Runtime Secrets (Docker Compose)

Application secrets (`API_KEY`, `DATABASE_URL`, webhook HMAC keys, client
credentials) are stored as files on the host at `/opt/hbs-queue/secrets/`.
Docker Compose mounts them into containers at `/run/secrets/<name>` using the
`secrets` directive. They are not visible in `docker inspect` or process
listings.

```sh
# Create secret files on docker01 (chmod 600, owned by root):
echo -n "the-actual-api-key" | sudo tee /opt/hbs-queue/secrets/api_key
echo -n "postgres://user:pass@postgres:5432/hbsqueue?sslmode=disable" | sudo tee /opt/hbs-queue/secrets/database_url
sudo chmod 600 /opt/hbs-queue/secrets/*
```

In `docker-compose.prod.yml`:

```yaml
secrets:
  api_key:
    file: /opt/hbs-queue/secrets/api_key
  database_url:
    file: /opt/hbs-queue/secrets/database_url

services:
  app-blue:
    secrets:
      - api_key
      - database_url
```

| Secret         | File on Host                          | Mount Path                  |
| -------------- | ------------------------------------- | --------------------------- |
| `API_KEY`      | `/opt/hbs-queue/secrets/api_key`      | `/run/secrets/api_key`      |
| `DATABASE_URL` | `/opt/hbs-queue/secrets/database_url` | `/run/secrets/database_url` |

The app reads config from environment variables, so an entrypoint script bridges
the gap — it loads secret files into env vars before starting the binary:

```sh
#!/bin/sh
# docker-entrypoint.sh
export API_KEY=$(cat /run/secrets/api_key)
export DATABASE_URL=$(cat /run/secrets/database_url)
exec /usr/local/bin/hbsqueue "$@"
```

Non-sensitive config (`PORT`, `ENV`) stays in the compose file as regular
environment variables — no need to put everything in secrets.

## Connecting to Services on docker01

| Service         | Endpoint                                                   |
| --------------- | ---------------------------------------------------------- |
| App (via Nginx) | `http://docker01.mgmt.infra.ckdev.io:8080`                 |
| Swagger UI      | `http://docker01.mgmt.infra.ckdev.io:8081`                 |
| Readiness probe | `GET /ready`                                               |
| Health check    | `GET /health`                                              |
| Debug dashboard | `http://docker01.mgmt.infra.ckdev.io:6061/debug/dashboard` |
| Postgres        | Container-internal only. Use `docker exec` to connect.     |

## Rollback

If a deploy goes bad and the blue/green swap already happened:

```sh
# Check which slot is active
./scripts/bg-status.sh

# Re-deploy the previous image
# (bg-deploy.sh starts the stopped slot, which still has the old image)
./scripts/bg-deploy.sh
```

If the issue is a bad migration, restore from the pre-deploy backup:

```sh
./scripts/db-restore.sh /opt/hbs-queue/backups/<timestamp>.sql.gz --yes
```

## Troubleshooting

### Check What's Running

```sh
# On docker01:
docker compose -f docker-compose.prod.yml ps
./scripts/bg-status.sh
```

### View Application Logs

```sh
# Follow logs for the active app container:
docker compose -f docker-compose.prod.yml logs -f app-blue

# Or whichever slot is active:
docker compose -f docker-compose.prod.yml logs -f app-green
```

### Check if the App Can Reach Postgres

```sh
curl http://docker01.mgmt.infra.ckdev.io:8080/ready
# 200 = DB is reachable
# 503 = DB ping failed
```

### Connect to Postgres Manually

```sh
docker exec -it <postgres-container> psql -U hbsqueue -d hbsqueue_dev
```

### CI Pipeline Failed

Check the GitHub Actions run at `github.com/CloudKey-io/hbs-queue/actions`.
Common causes:

- Test failure: fix the code, push again
- GHCR auth: check that the `GITHUB_TOKEN` has `packages:write` scope
- SSH to docker01 failed: check `DOCKER01_SSH_KEY` secret and firewall rules
