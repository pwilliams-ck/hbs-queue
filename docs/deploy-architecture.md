# Deployment Architecture

HostBill Queue Service (hbs-queue) orchestrates customer onboarding and
offboarding workflows across VCD, Zerto, Keycloak, HostBill, and Active
Directory. It is a Go service backed by Postgres and
[River](https://riverqueue.com) (a Postgres-backed job queue).

This document describes how the service is built, deployed, and operated.

## Table of Contents

- [Infrastructure](#infrastructure)
- [Environments](#environments)
  - [Promotion Flow](#promotion-flow)
- [CI/CD Flow](#cicd-flow)
  - [Push to Main (Dev Auto-Deploy)](#push-to-main-dev-auto-deploy)
  - [Tag vX.X.X (Production Release)](#tag-vxxx-production-release)
- [Blue/Green App Deploys](#bluegreen-app-deploys)
  - [Deploy Sequence](#deploy-sequence-scriptsbg-deploysh)
  - [Check Status](#check-status)
- [Database](#database)
  - [Why Not Blue/Green on the Database?](#why-not-bluegreen-on-the-database)
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
- [Scaling to Docker Swarm](#scaling-to-docker-swarm)
  - [Additional Firewall Rules for Multi-Node Swarm](#additional-firewall-rules-for-multi-node-swarm)
- [Secrets](#secrets)
- [Connecting to Services on docker01](#connecting-to-services-on-docker01)
- [Rollback](#rollback)
- [Troubleshooting](#troubleshooting)
  - [Check What's Running](#check-whats-running)
  - [View Application Logs](#view-application-logs)
  - [Check if the App Can Reach Postgres](#check-if-the-app-can-reach-postgres)
  - [Connect to Postgres Manually](#connect-to-postgres-manually)
  - [CI Pipeline Failed](#ci-pipeline-failed)

<picture>
  <source media="(prefers-color-scheme: dark)" srcset="deploy-architecture-dark.svg">
  <img src="deploy-architecture-light.svg" alt="Deployment Architecture">
</picture>

## Infrastructure

| Host                            | Role               | Notes                                                                                                                             |
| ------------------------------- | ------------------ | --------------------------------------------------------------------------------------------------------------------------------- |
| `ci01`                          | CI/CD server       | GitHub Actions self-hosted runner. Builds images, pushes to GHCR, triggers deploys via SSH. No application containers run here.   |
| `docker01.mgmt.infra.ckdev.io`  | Application host   | Runs Postgres, Nginx, and the hbs-queue app containers. Single-node Docker Swarm (ready for multi-node expansion to docker02/03). |
| Mgmt backup server              | Backup storage     | Receives copy 2 of database backups. Access method TBD (mount or rsync).                                                          |
| `ghcr.io/cloudkey-io/hbs-queue` | Container registry | GitHub Container Registry. All images are built on ci01 and pulled by docker01.                                                   |

## Environments

Dev and prod are on **physically separate networks in different datacenters**.
This is intentional — a misconfigured dev environment cannot reach prod data,
and destructive testing in dev carries zero risk to production.

| Environment | Network        | Hosts                                  | Deploys via     |
| ----------- | -------------- | -------------------------------------- | --------------- |
| Dev         | `10.42.0.0/16` | `docker01.mgmt.infra.ckdev.io`, `ci01` | Merge to `main` |
| Prod        | `10.25.0.0/16` | TBD                                    | Tag `vX.X.X`    |

### Promotion Flow

```
feature branch → PR → merge to main → auto-deploy to dev (10.42)
                                          ↓
                                     test in dev
                                          ↓
                              git tag vX.X.X → push tags
                                          ↓
                              image tagged in GHCR (ready for prod)
                                          ↓
                              deploy to prod (10.25) — not wired yet
```

Tagged releases build a versioned image and push it to GHCR. Prod deployment is
not automated yet — when a prod host is provisioned on the `10.25` network, the
tag workflow will be extended to SSH and deploy there.

## CI/CD Flow

### Push to `main` (Dev Auto-Deploy)

1. ci01 runner picks up the job
2. Runs tests and linter
3. Builds Docker image, tags with commit SHA
4. Pushes to GHCR
5. SSHs to docker01, pulls the new image, runs `bg-deploy.sh`
6. Health check confirms `/ready` returns 200

### Tag `vX.X.X` (Production Release)

Same as above, but before deploying:

- Runs `db-backup.sh` on docker01 (pre-deploy safety net)
- Image is tagged with the version number (e.g., `v1.2.0`)

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
run programmatically at app startup — River migrates its own tables, then app
migrations (`internal/db/migrations/`) run in sorted order.

### Why Not Blue/Green on the Database?

The app is stateless; the database is not. Running two Postgres instances
introduces data sync complexity (replication or write-freezes) that isn't
justified at this scale. Instead:

- Write backward-compatible migrations (add columns, don't rename/drop)
- Run `pg_dump` before every tagged release
- Restore from backup if a migration goes wrong

If zero-downtime schema changes on large datasets become necessary later, tools
like `pgroll` or logical replication are the right path — not dual databases.

## Backups (3-2-1-1-0)

The backup strategy targets **3-2-1-1-0**: 3 copies, 2 media types, 1 offsite, 1
immutable, 0 unverified backups.

### What's Implemented Now

| Copy | Location                      | Method           |
| ---- | ----------------------------- | ---------------- |
| 1    | docker01 local (`./backups/`) | `pg_dump` direct |

Every backup is **restore-tested** before being copied anywhere. The script
restores into a temporary database, runs a sanity check, then drops it. If
verification fails, the dump is flagged and not propagated. This is the most
valuable part of the strategy — without it, you could be backing up corrupted
dumps for weeks without knowing.

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
  │                                   ├─ :5432  Postgres (container-internal,
  │                                   │         not exposed to host by default)
  │                                   │
  │                                   └─ :2377  Docker Swarm management
  │                                      (only needed when adding docker02/03)
  │
  └─ Mgmt backup server
       └── receives backups from docker01 (mount or rsync, TBD)
```

### Required Firewall Rules (dev)

| Source           | Destination             | Port | Protocol    | Purpose                                            |
| ---------------- | ----------------------- | ---- | ----------- | -------------------------------------------------- |
| ci01             | `github.com`, `ghcr.io` | 443  | TCP (HTTPS) | Pull jobs, push images                             |
| ci01             | docker01                | 22   | TCP (SSH)   | Deploy commands                                    |
| docker01         | `ghcr.io`               | 443  | TCP (HTTPS) | Pull container images                              |
| docker01         | Mgmt backup server      | TBD  | TBD         | Backup copy 2                                      |
| Internal clients | docker01                | 8080 | TCP (HTTP)  | Application access                                 |
| docker01         | docker01                | 5432 | TCP         | App → Postgres (container-internal, stays on host) |

Postgres is **not exposed** on the host network by default. The app containers
connect via Docker's internal network. To connect manually for debugging, use
`docker exec` into the Postgres container or temporarily bind the port.

### Prod Network (Future)

Prod hosts will be on `10.25.0.0/16`. The same firewall rules apply — the prod
Docker host needs outbound HTTPS to `ghcr.io` and inbound SSH from whatever runs
the prod deploy (could be ci01 if it has cross-network access, or a separate
runner on the prod network).

## Scaling to Docker Swarm

docker01 is initialized as a single-node swarm. When docker02 and docker03 are
ready:

```sh
# On docker02/03:
docker swarm join --token <worker-or-manager-token> docker01:2377
```

The compose file gains `deploy:` directives (replicas, placement constraints,
rolling update config), but the core structure stays the same. Nginx can be
replaced by Swarm's built-in routing mesh if desired.

### Additional Firewall Rules for Multi-Node Swarm

| Source      | Destination | Port | Protocol | Purpose                 |
| ----------- | ----------- | ---- | -------- | ----------------------- |
| Swarm nodes | Swarm nodes | 2377 | TCP      | Cluster management      |
| Swarm nodes | Swarm nodes | 7946 | TCP/UDP  | Node discovery          |
| Swarm nodes | Swarm nodes | 4789 | UDP      | Overlay network (VXLAN) |

## Secrets

Stored in GitHub repo settings under **Settings > Secrets > Actions**:

| Secret             | Purpose                                 |
| ------------------ | --------------------------------------- |
| `DOCKER01_HOST`    | SSH target for deploys                  |
| `DOCKER01_SSH_KEY` | Private key for deploy user on docker01 |
| `DOCKER01_USER`    | Deploy user on docker01                 |
| `API_KEY`          | App API key                             |
| `DATABASE_URL`     | Postgres connection string on docker01  |

Secrets are scoped per environment. When prod is added, a separate set of
secrets will be created (e.g., `PROD_HOST`, `PROD_SSH_KEY`, `PROD_API_KEY`).
Never reuse dev secrets in prod.

## Connecting to Services on docker01

| Service         | Endpoint                                               |
| --------------- | ------------------------------------------------------ |
| App (via Nginx) | `http://docker01.mgmt.infra.ckdev.io:8080`             |
| Readiness probe | `GET /ready`                                           |
| Health check    | `GET /health`                                          |
| Postgres        | Container-internal only. Use `docker exec` to connect. |

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

- Test failure — fix the code, push again
- GHCR auth — check that the `GITHUB_TOKEN` has `packages:write` scope
- SSH to docker01 failed — check `DOCKER01_SSH_KEY` secret and firewall rules
