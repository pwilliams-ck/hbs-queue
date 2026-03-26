# CLAUDE.md

**hbs-queue** — Go service orchestrating customer onboarding/offboarding across
VCD, Zerto, Keycloak, HostBill, and Active Directory via
[River](https://riverqueue.com) (Postgres-backed job queue). Through Task 5;
remaining: other clients, integration tests, k3s deploy, prod hardening.

## Commands

```bash
make test         # go test -v -race -cover ./...
make lint         # golangci-lint run
make check        # lint + test
make dev-up       # docker compose up (Postgres + Swagger UI)
make dev-reset    # docker compose down -v (wipe DB)
```

Single test: `go test -v -race -run TestName ./internal/pkg/`

## Conventions

- **Go 1.26**, module `github.com/CloudKey-io/hbs-queue`
- **Commits**: `pkg/subpkg: short description` — do not commit or push without
  being asked
- **Doc comments on every exported symbol**
- **Minimize deps** — stdlib preferred
- **Linting**: `goimports` local prefix `github.com/CloudKey-io/hbs-queue`
- **New features**: follow patterns in `docs/todo/code-examples.md`

## Where to Look

- **Config & env vars** — `internal/config/config.go`
- **API routes** — `internal/httpapi/routes.go`
- **DB schema** — `internal/db/migrations/` + `docs/todo/db-schema-01.md`
- **Tasks & roadmap** — `docs/todo/01_init.md` … `09_production_prep.md`
- **Architecture & patterns** — `docs/todo/README.md`,
  `docs/todo/code-examples.md`
