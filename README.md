# hbsqueue

HostBill Service Queue orchestrates tenant provisioning and management workflows
across VCD, Zerto, Keycloak, HostBill, and Active Directory.

Built on [River](https://riverqueue.com), a Postgres-backed job queue for Go.

`docs/` contains additional information about architecture, deployment, full API
surface with OpenAPI Swagger, etc.

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
- [Deployment](#deployment)
  - [Graceful Shutdown](#graceful-shutdown)

## Requirements

- Go 1.26+
- PostgreSQL 18+
- Docker Compose

## Getting Started

```sh
git clone https://github.com/CloudKey-io/hbs-queue.git
cd hbs-queue
cp .envrc.example .envrc
direnv allow  # or: source .envrc
docker compose up -d
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

## Configuration

All configuration is via environment variables.

| Variable       | Default | Description                     |
| -------------- | ------- | ------------------------------- |
| `PORT`         | `8080`  | HTTP listen port                |
| `ENV`          | `dev`   | Environment (`dev` / `prod`)    |
| `API_KEY`      |         | Required for `/api/v1/*` routes |
| `DATABASE_URL` |         | Postgres connection string      |

## Docker Compose

Nginx, Postgres, Swagger UI, and hbsqueue run in Docker Compose:

```sh
docker compose up -d        # start Postgres + Swagger UI
docker compose ps           # check status
docker compose down         # stop services
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
cmd/hbsqueue/        main + run, integration tests
internal/
  config/            Config loaded from env via getenv func
  httpapi/           HTTP server, routes, handlers, middleware
  db/                pgxpool, River client, migrations
  clients/           (in progress) VCD, Zerto, Keycloak, HostBill, AD
  jobs/              (in progress) River job workers
  workflow/          (in progress) step runner with JSONB accumulator
docs/
  openapi.yaml       API specification
  todo/              task tracking and reference docs
```

## API

See [`docs/openapi.yaml`](docs/openapi.yaml) for full specification.

| Method | Path           | Auth    | Description                |
| ------ | -------------- | ------- | -------------------------- |
| GET    | `/ready`       | none    | Readiness probe (pings db) |
| GET    | `/health`      | none    | Build info + db status     |
| POST   | `/api/v1/echo` | API key | Echo test                  |

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

## Deployment

See [docs/deploy-architecture.md](docs/deploy-architecture.md) for the full
deployment architecture, CI/CD pipeline, blue/green deploys, back up strategy,
network topology, and troubleshooting.

### Graceful Shutdown

On `SIGINT`/`SIGTERM` the service shuts down in safe order:

1. **HTTP**: stop accepting new requests, drain in-flight
2. **River**: stop fetching jobs, let active workers finish
3. **Pool**: close database connections
