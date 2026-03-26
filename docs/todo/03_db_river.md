# Task 3: DB Package + River Setup

`/ready` and `/health` ping DB. Localhost dev
complete — ready to build handlers, jobs, and clients (Tasks 4–7).

> See `docs/db-schema-01.md` for the full `workflow_state` table schema,
> accumulator pattern, and index rationale.

---

## Schema

```sql
IF NOT EXISTS CREATE TABLE workflow_state (
    id             UUID           PRIMARY KEY DEFAULT gen_random_uuid(),
    job_id         BIGINT         NOT NULL UNIQUE,
    workflow_type  VARCHAR(50)    NOT NULL,
    client_id      VARCHAR(50)    NOT NULL,
    order_id       VARCHAR(50),
    current_step   INT            NOT NULL DEFAULT 0,
    status         VARCHAR(20)    NOT NULL DEFAULT 'pending',
    error          TEXT,
    data           JSONB          NOT NULL DEFAULT '{}',
    created_at     TIMESTAMPTZ    NOT NULL DEFAULT now(),
    updated_at     TIMESTAMPTZ    NOT NULL DEFAULT now()
);
```

> We may need to alter this schema when we get to the first real workflow. The
> schema is discussed in detail in `docs/db-schema-01.md`.

---

## TODO

### Dependencies

```
[✅] go get github.com/jackc/pgx/v5
[✅] go get github.com/riverqueue/river
[✅] go get github.com/riverqueue/river/riverdriver/riverpgxv5
```

### DB Package

```
[✅] internal/db/db.go
    - func NewPool(ctx, databaseURL) (*pgxpool.Pool, error)
    - Connection config: pool size, timeouts
    - Doc comment on NewPool: pool sizing rationale, timeout values
    - Inline comments on each config knob

[✅] internal/db/river.go
    - func NewRiverClient(pool, workers) (*river.Client[pgx.Tx], error)
    - Doc comment on NewRiverClient: worker wiring, queue config
```

### Migrations

```
[✅] Run River migrations programmatically at startup (river auto-migrates its own tables)

[✅] internal/db/migrations/001_workflow_state.up.sql
    - CREATE TABLE workflow_state (...)
    - SQL comments on every column: purpose, nullable rationale

[✅] internal/db/migrations/001_workflow_state.down.sql
    - DROP TABLE workflow_state;

    Note: no Makefile migrate targets needed — migrations run programmatically
    at startup via internal/db/migrate.go.
```

### Wire into main.go

```
[✅] Initialize pool in run()
[✅] Initialize River client in run()
[✅] Pass pool and river client to NewServer()
[✅] Graceful shutdown order:
      1. HTTP - stop accepting, drain in-flight requests
      2. River - stop workers
      3. Pool - close via defer
    - Inline comments on shutdown order and why it matters
```

### Update Server + Handlers

```
[✅] Update NewServer signature:
    func NewServer(logger, cfg, pool, river) http.Handler

[✅] Update addRoutes() to accept and pass pool/river

[✅] handleReady - return 200 only if DB ping succeeds
[✅] handleHealth - return version info + DB status in JSON response
```

### README.md (append during this task)

```
[✅] River migration note (runs automatically at startup)
[✅] Graceful shutdown sequence explanation
[✅] DB package notes (pool size, timeouts)
[✅] /ready and /health response shapes (with example JSON)
```

### Verify

```
[✅] make run
[✅] curl localhost:8080/ready   → 200 (DB ping succeeds)
[✅] curl localhost:8080/health  → 200 with version + DB status
[✅] make test                   → all pass
[✅] Localhost dev complete
```
