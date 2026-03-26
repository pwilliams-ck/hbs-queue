# Task 1: Init

Service runs, `/ready` works, README started.

> See `docs/code-examples.md` for all scaffold code. Over 1,300 lines of
> patterns and complete file examples to copy from.

---

## TODO

### Repo + Scaffold Setup

```
[✅] Research Claude migration and context limitations
[✅] Learn/research code examples involving hbs-queue & river queue
[✅] Create initial hbs-queue directories -> mkdir -p hbs-queue/docs/
[✅] Start local repo with git init
[✅] Add hbs-queue.md documented tasklist after research to /docs
[✅] Add code-examples.md involving hbs-queue & river queue to /docs
[✅] Add db-schema-01.md to /docs
[✅] go mod init github.com/CloudKey-io/hbs-queue
[✅] Create .gitignore (Go, .envrc, .env, binaries)
[✅] Create .envrc.example
[✅] Create GitHub repo: github.com/CloudKey-io/hbs-queue
[✅] git add, commit, merge docs
```

### Application Files

```
[✅] cmd/hbsqueue/main.go - main() + run()
      - Doc comment on main() explaining entry point
      - Doc comment on run() explaining args, env, shutdown

[✅] internal/config/config.go - Load(getenv)
      - Doc comment on every exported field in Config struct
      - Inline comments on any non-obvious defaults

[✅] internal/httpapi/server.go - NewServer(logger, cfg)
      - Doc comment on NewServer explaining dep wiring

[✅] internal/httpapi/routes.go - addRoutes()
      - Top-of-file comment listing full route surface

[✅] internal/httpapi/handlers.go - handleReady, handleHealth, handleEcho
      - Doc comment per handler: purpose, request, response

[✅] internal/httpapi/middleware.go
      - requestID, requestLogger, panicRecovery, apiKeyAuth
      - Doc comment per middleware: what it does, order sensitivity

[✅] internal/httpapi/encode.go - encode, decode, decodeValid
      - Doc comment on Validator interface
      - Doc comment on encode/decode/decodeValid

[✅] internal/httpapi/types.go - request/response types
      - Doc comment on every exported type and field
```

### Tests + Tooling

```
[✅] Tests for run(), handlers, middleware
[✅] Makefile - comment every target with ## description (for make help)
[✅] .golangci.yml
[✅] .envrc.example
[✅] .github/workflows/ci.yml (stub - tests only, no deploy yet)
```

### README.md (append during this task)

```
[✅] Environment variable reference (mirrors config.go fields)
[✅] make target reference
```

### Verify

```
[✅] make run
[✅] curl localhost:8080/ready   → 200
[✅] curl localhost:8080/health  → 200 with version info
[✅] make test                   → all pass
```
