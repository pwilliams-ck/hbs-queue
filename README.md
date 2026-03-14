# hbsqueue

HostBill Service Queue — orchestrates tenant provisioning and management
workflows across VCD, Zerto, Keycloak, HostBill, and Active Directory.

Built on [River](https://riverqueue.com), a Postgres-backed job queue for Go.

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

| Variable       | Default | Description                    |
|----------------|---------|--------------------------------|
| `PORT`         | `8080`  | HTTP listen port               |
| `ENV`          | `dev`   | Environment (`dev` / `prod`)   |
| `API_KEY`      | —       | Required for `/api/v1/*` routes|
| `DATABASE_URL` | —       | Postgres connection string     |

## Docker Compose

Postgres and Swagger UI run in Docker Compose:

```sh
docker compose up -d        # start Postgres + Swagger UI
docker compose ps           # check status
docker compose down         # stop services
```

- Postgres: `localhost:5432` (user: `hbsqueue`, db: `hbsqueue_dev`)
- Swagger UI: `localhost:8081`
- `DATABASE_URL` format: `postgres://hbsqueue:dev-password@localhost:5432/hbsqueue_dev?sslmode=disable`

Migrations run programmatically via River — no CLI tool required.

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
  db/                (future) pgxpool + River client
  clients/           (future) VCD, Zerto, Keycloak, HostBill, AD
  jobs/              (future) River job workers
  workflow/          (future) step runner with JSONB accumulator
docs/
  openapi.yaml       API specification
  todo/              task tracking and reference docs
```

## API

See [`docs/openapi.yaml`](docs/openapi.yaml) for the full specification.

| Method | Path              | Auth    | Description       |
|--------|-------------------|---------|-------------------|
| GET    | `/ready`          | none    | Readiness probe   |
| GET    | `/health`         | none    | Build/version info|
| POST   | `/api/v1/echo`    | API key | Echo test         |
