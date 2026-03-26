# Task 2: Local Postgres in Docker

Postgres running in Docker Compose, DATABASE_URL
wired, golang-migrate dependency added.

> All work in this task is local (macOS/Darwin). Deployment infrastructure
> moves to Task 8.

---

## TODO

### Docker Compose - Dev Stack

```
[✅] docker-compose.yml with services:
    - postgres: postgres:18-alpine, port 5432, named volume
    - (app service added later - for now just DB)

[✅] .env wired with:
    DATABASE_URL=postgres://hbsqueue:dev-password@localhost:5432/hbsqueue_dev?sslmode=disable

[✅] docker compose up -d → postgres running
[✅] psql -h localhost -p 5432 -U hbsqueue -d hbsqueue_dev → connects
```

### Wire .envrc

```
[✅] Add DATABASE_URL to .envrc:
    export DATABASE_URL=postgres://hbsqueue:dev-password@localhost:5432/hbsqueue_dev?sslmode=disable

[✅] Add DATABASE_URL to .envrc.example (with placeholder password)
[✅] Uncomment DatabaseURL field in internal/config/config.go
[✅] Uncomment DatabaseURL in config.Load()
```

### Add Migration Library

```
[✅] Add golang-migrate as a Go library dependency (no CLI - migrations run via API):
    go get -tags 'postgres' github.com/golang-migrate/migrate/v4
```

### README.md (append during this task)

```
[✅] Docker Compose dev setup (docker compose up → ready)
[✅] DATABASE_URL format + sslmode note
[✅] Note: migrations run programmatically, no CLI tool required
```

### Verify

```
[✅] docker compose up -d → postgres running
[✅] psql -h localhost -p 5432 -U hbsqueue -d hbsqueue_dev → connects
[✅] make run (service starts with DATABASE_URL set)
```
