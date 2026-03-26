# hbsqueue

HostBill Service Queue orchestrates tenant provisioning and management workflows
across VCD, Zerto, Keycloak, HostBill, and Active Directory. Reddit CAPI is also
included for advertisement sales conversion rates.

Built on [River](https://riverqueue.com), a Postgres-backed job queue for Go.

**Progress:** Task 6 of 9 — add remaining HTTP clients, integration tests,
deployment, production preparation. See [`docs/todo/`](docs/todo/) for the full
checklist.

**[Deployment Architecture](docs/deploy-architecture.md)** includes Docker
Compose, blue/green deploys with automated rollbacks, dedicated CI/CD GitHub
Actions runner (self-hosted), and 3-2-1-1-0 database backup strategy with
restore verification.

`docs/` contains diagrams and additional information about architecture,
deployment, full API surface with OpenAPI Swagger, etc.

## Table of Contents

- [Requirements](#requirements)
- [Getting Started](#getting-started)
- [Configuration](#configuration)
- [Docker Compose](#docker-compose)
- [Development](#development)
- [Project Layout](#project-layout)
- [API](#api)
  - [Example Responses](#example-responses)
    - [GET `/ready`](#get-ready)
    - [GET `/health`](#get-health)
- [Workflow Engine](#workflow-engine)
  - [Step Interface](#step-interface)
  - [Resume After Restart](#resume-after-restart)
  - [JSONB Accumulator](#jsonb-accumulator)
  - [Job Types](#job-types)
  - [Manual Test Job](#manual-test-job)
- [Debug / Profiling](#debug--profiling)
- [Deployment](#deployment)
  - [Graceful Shutdown](#graceful-shutdown)

## Requirements

- Git
- Make
- Direnv (dev only)
- Go 1.26+
- Docker Compose

## Getting Started

```sh
git clone https://github.com/CloudKey-io/hbs-queue.git
cd hbs-queue
cp .envrc.example .envrc
direnv allow  # or: source .envrc if no direnv
make dev-up
make run
```

Verify:

```sh
curl localhost:8080/ready
curl localhost:8080/health
curl -X POST localhost:8080/api/v1/echo \
  -H "Content-Type: application/json" \
  -H "X-API-Key: dev-secret-key" \
  -d '{"message": "hello"}'
```

You can also go to [Swagger UI](http://localhost:8081) to test API
functionality.

## Configuration

All configuration is via environment variables.

| Variable                       | Default     | Description                                |
| ------------------------------ | ----------- | ------------------------------------------ |
| `PORT`                         | `8080`      | HTTP listen port                           |
| `ENV`                          | `localhost` | Environment (`localhost` / `dev` / `prod`) |
| `API_KEY`                      |             | Required for `/api/v1/*` routes            |
| `DATABASE_URL`                 |             | Postgres connection string                 |
| `HOOK_SECRET_DEBOARD_ORG`      |             | HMAC secret for deboard-org                |
| `HOOK_SECRET_ONBOARD_CONTACT`  |             | HMAC secret for onboard-contact            |
| `HOOK_SECRET_DEBOARD_CONTACT`  |             | HMAC secret for deboard-contact            |
| `HOOK_SECRET_UPDATE_PW`        |             | HMAC secret for update-pw                  |
| `HOOK_SECRET_UPDATE_BANDWIDTH` |             | HMAC secret for update-bandwidth           |
| `DEBUG_PORT`                   | `6061`      | Debug/profiling dashboard                  |
| `SWAGGER_PORT`                 | `8081`      | Swagger UI port (logged at startup)        |

## Docker Compose

Locally, Docker Compose runs Postgres and Swagger UI. In dev/prod, the compose
stack also includes Nginx (reverse proxy for blue/green deploys) and the
hbsqueue app containers. See
[deploy-architecture.md](docs/deploy-architecture.md) for details.

```sh
make dev-up      # start Postgres + Swagger UI, wait for ready
make dev-down    # stop services
make dev-reset   # stop services and wipe DB
make dev-logs    # tail container logs
```

- Postgres: `localhost:5432` (user: `hbsqueue`, db: `hbsqueue_dev`)
- Swagger UI: `localhost:8081`
- `DATABASE_URL` format:
  `postgres://hbsqueue:dev-password@localhost:5432/hbsqueue_dev?sslmode=disable`

Migrations run programmatically at startup, no CLI tool required. River migrates
its own tables, then app migrations (`internal/db/migrations/`) run in sorted
order using embedded SQL.

## Development

```sh
make run      # go run ./cmd/hbsqueue
make build    # compile binary with version info into bin/
make test     # run all tests with race detector and coverage
make lint     # run golangci-lint
make clean    # remove build artifacts
```

Run a single test:

```sh
go test -v -race -run TestHandleEcho ./internal/httpapi/
```

## Project Layout

```
cmd/hbsqueue/        main + run, debug server, integration tests
internal/
  config/            Config loaded from env via getenv func
  httpapi/           HTTP server, routes, handlers, middleware
  db/                pgxpool, River client, migrations
  jobs/              River job arg types and workers
  workflow/          Step interface, workflow state repo, runner
  clients/           VCD, Zerto, Keycloak, HostBill, AD, Reddit (in progress)
docs/
  openapi.yaml       API specification
  todo/              task checklist and progress tracking
```

## API

See [`docs/openapi.yaml`](docs/openapi.yaml) for full specification.

| Method | Path                         | Auth    | Description                |
| ------ | ---------------------------- | ------- | -------------------------- |
| GET    | `/ready`                     | none    | Readiness probe (pings DB) |
| GET    | `/health`                    | none    | Build info + DB status     |
| POST   | `/api/v1/echo`               | API key | Echo test                  |
| POST   | `/api/v1/script/onboard-org` | API key | Enqueue tenant onboarding  |
| POST   | `/hooks/deboard-org`         | Webhook | Deboard tenant             |
| POST   | `/hooks/onboard-contact`     | Webhook | Add contact                |
| POST   | `/hooks/deboard-contact`     | Webhook | Remove contact             |
| POST   | `/hooks/update-pw`           | Webhook | Password change            |
| POST   | `/hooks/update-bandwidth`    | Webhook | Update bandwidth           |

**Auth types:**

- **API key** — `X-API-Key` header, constant-time comparison.
- **Webhook** — HostBill HMAC-SHA256. Signature is computed over
  `HB-Timestamp + request body` using the per-hook secret. Headers: `HB-Hook`,
  `HB-Event`, `HB-Timestamp`, `HB-Signature`. Requests older than 60 seconds are
  rejected.

### Example Responses

#### GET `/ready`

```json
{ "status": "ok" }
```

#### GET `/health`

```json
{
  "status": "healthy",
  "version": "dev",
  "commit": "none",
  "build_time": "unknown",
  "database": "up"
}
```

## Workflow Engine

Each API endpoint enqueues a River job. Workers execute workflows as a sequence
of steps, with progress tracked in the `workflow_state` table.

### Step Interface

Every step implements `workflow.Step`:

```go
type Step interface {
    Name() string
    Run(ctx context.Context, state *WorkflowState) error
}
```

**Steps must be idempotent**: The runner may re-execute a step on retry.

### Resume After Restart

`current_step` in the DB records the last completed step index. On retry, the
runner skips completed steps and resumes from `current_step`. State updates are
transactional, so a crash mid-step causes only that step to re-run.

### JSONB Accumulator

The `data` column starts with the HostBill payload and grows as each step
completes. Steps read what they need (`state.GetString("key")`) and write what
they produce (`state.Set("key", value)`).

### Job Types

| Job Type           | Args Struct           | Enqueued By                       |
| ------------------ | --------------------- | --------------------------------- |
| `onboard_org`      | `OnboardOrgArgs`      | POST `/api/v1/script/onboard-org` |
| `deboard_org`      | `DeboardOrgArgs`      | POST `/hooks/deboard-org`         |
| `onboard_contact`  | `AddContactArgs`      | POST `/hooks/onboard-contact`     |
| `deboard_contact`  | `DeleteContactArgs`   | POST `/hooks/deboard-contact`     |
| `update_pw`        | `UpdatePwArgs`        | POST `/hooks/update-pw`           |
| `update_bandwidth` | `UpdateBandwidthArgs` | POST `/hooks/update-bandwidth`    |

### Manual Test Job

```sql
INSERT INTO river_job (args, kind, queue, state, max_attempts, priority, created_at, scheduled_at)
VALUES (
  '{"crm_id":"test001","organization_name":"TestCorp","client_username":"testuser","client_first_name":"Test","client_last_name":"User","client_email":"test@example.com","account_id":1,"country":"US","state":"Texas","postal_code":"75074","max_zerto_storage":50,"max_zerto_vms":5,"bandwidth":"100","product_id":"web-1"}',
  'onboard_org', 'default', 'available', 3, 1, now(), now()
);
```

## Debug / Profiling

A separate listener on `DEBUG_PORT` (default 6061) serves diagnostics. Available
in all environments (local, dev, prod).

| Endpoint                         | Description                   |
| -------------------------------- | ----------------------------- |
| `/debug/dashboard`               | Live metrics dashboard        |
| `/debug/pprof/`                  | pprof index                   |
| `/debug/pprof/goroutine?debug=1` | Current goroutine stacks      |
| `/debug/pprof/goroutineleak`     | Leaked goroutines (Go 1.26)   |
| `/debug/vars`                    | expvar + runtime metrics JSON |

On-demand profiling (flame graph / trace):

```sh
go tool pprof -http=:6060 http://localhost:6061/debug/pprof/profile?seconds=10
go tool pprof -http=:6060 http://localhost:6061/debug/pprof/heap
curl -o trace.out http://localhost:6061/debug/pprof/trace?seconds=5 && go tool trace trace.out
```

Build with `GOEXPERIMENT=goroutineleakprofile` to enable the goroutine leak
profile endpoint.

## Deployment

See [docs/deploy-architecture.md](docs/deploy-architecture.md) for the full
deployment architecture, CI/CD pipeline, blue/green deploys, back up strategy,
network topology, and troubleshooting.

### Graceful Shutdown

On `SIGINT`/`SIGTERM` the service shuts down in safe order:

1. **HTTP**: stop accepting new requests, drain in-flight
2. **Debug**: stop diagnostics listener
3. **River**: stop fetching jobs, let active workers finish
4. **Pool**: close database connections
