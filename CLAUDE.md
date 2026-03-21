# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

**hbs-queue** is a HostBill Queue Service (Go) that orchestrates customer onboarding/offboarding workflows across five external platforms: VCD, Zerto, Keycloak, HostBill, and Active Directory. It uses [River](https://riverqueue.com) as a Postgres-backed job queue.

The project is early-stage - currently at Task 1 (init/scaffold). Code examples and target architecture are documented in `docs/todo/code-examples.md` and `docs/todo/README.md`. Follow those patterns when building new features.

## Build & Dev Commands

```bash
make run          # go run ./cmd/hbsqueue
make build        # go build with ldflags (version, commit, build time)
make test         # go test -v -race -cover ./...
make lint         # golangci-lint run
make clean        # rm -rf bin/
```

Run a single test:
```bash
go test -v -race -run TestHandleEcho ./internal/httpapi/
```

Environment setup: `cp .envrc.example .envrc && direnv allow` (or `source .envrc`).

## Architecture

Entry point is `cmd/hbsqueue/main.go` → `run()` which accepts `os.Args`, `os.Getenv`, stdout, stderr as explicit dependencies for testability.

### Package Layout (target state in `docs/todo/README.md`)

- **`cmd/hbsqueue/`** — main + run, integration tests via real HTTP server on free port
- **`internal/config/`** — `Config` struct loaded via `Load(getenv)`, version info set via ldflags
- **`internal/httpapi/`** — HTTP layer: `NewServer()` returns `http.Handler`, routes in `addRoutes()`, generic `encode`/`decode`/`decodeValid` JSON helpers with `Validator` interface
- **`internal/db/`** — (future) pgxpool + River client setup
- **`internal/clients/`** — (future) VCD, Zerto, Keycloak, HostBill, AD HTTP clients
- **`internal/jobs/`** — (future) River job arg structs + workers
- **`internal/workflow/`** — (future) step runner with `workflow_state` DB table using JSONB accumulator pattern

### Key Patterns

- **Handler pattern**: handlers return `http.Handler` (not `http.HandlerFunc`). One-time setup outside the closure, request logic inside.
- **Middleware**: `requestID` → `requestLogger` → `panicRecovery` (outermost wraps first). Route-level auth via `apiKeyAuth` / `hookAuth` middleware factories.
- **Request validation**: request types implement `Validator` interface (`Valid(ctx) map[string]string`), decoded via generic `decodeValid[T]()`.
- **Workflow state**: each River job gets a `workflow_state` row with a JSONB `data` column that accumulates results step-by-step (see `docs/todo/db-schema-01.md`).

## Conventions

- **Go 1.26**, module `github.com/CloudKey-io/hbs-queue`
- **Commit message format**: Go-style `pkg/subpkg: short description` (see `docs/todo/README.md` → Git Commits)
- **All git operations are done by a human** — do not commit or push without being asked
- **Doc comments on every exported symbol** — documentation is written inline as code is written, not after
- **Minimize external dependencies** — stdlib preferred, only River and pgx are expected dependencies
- **Linting**: golangci-lint with `goimports` using local prefix `github.com/CloudKey-io/hbs-queue`

## Environment Variables

| Variable       | Default | Description                    |
|---------------|---------|--------------------------------|
| `PORT`        | `8080`  | HTTP listen port               |
| `ENV`         | `dev`   | Environment (`dev` / `prod`)   |
| `API_KEY`     | —       | Required for `/api/v1/*` routes |
| `DATABASE_URL`| —       | (future) Postgres connection   |

## Task Tracking

Project tasks are in `docs/todo/task-*.md`. Reference docs: `docs/todo/db-schema-01.md` (DB schema), `docs/todo/code-examples.md` (scaffold code and patterns to follow).
