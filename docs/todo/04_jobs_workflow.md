# Task 4: Handler Structure + Jobs + Workflow Engine

All routes wired with stub handlers, first job runs
end-to-end through the workflow engine. API surface and workflow docs in README.

> See `docs/db-schema-01.md` for the `workflows` table schema and the JSONB
> accumulator pattern - the workflow engine reads and writes this table as
> needed, at every step.

---

## Checklist

### Handler Layout

```
[✅] Flat layout in internal/httpapi/ (one handle_*.go file per handler).
     Sub-packages (handlers/script_provision/, handlers/hooks/) were
     considered but the flat layout is simpler and idiomatic Go.

     internal/httpapi/
     ├── handlers.go              # handleReady, handleHealth, handleEcho
     ├── handle_onboard_org.go
     ├── handle_deboard_org.go
     ├── handle_onboard_contact.go
     ├── handle_deboard_contact.go
     ├── handle_update_pw.go
     └── handle_update_bandwidth.go
```

### Stub Handlers

```
[✅] Each stub returns 501 Not Implemented
[✅] Each stub has a doc comment describing:
      - Intended behavior
      - Expected request payload
      - Expected response

[✅] handle_onboard_org.go
[✅] handle_deboard_org.go
[✅] handle_onboard_contact.go
[✅] handle_deboard_contact.go
[✅] handle_update_pw.go
[✅] handle_update_bandwidth.go
```

### Types

```
[✅] Add request/response types for each handler (all in types.go)
[✅] JobAcceptedResponse shared by all handlers that enqueue jobs
[✅] OnboardOrgRequest with validation matching HostBill payload (crm_id tag)
[✅] DeboardOrgRequest, OnboardContactRequest, DeboardContactRequest,
     UpdatePwRequest, UpdateBandwidthRequest — all with Valid() methods
[✅] Doc comment on every exported type
```

### Wire Routes in routes.go

```
[✅] POST /api/v1/script/onboard-org     → withAPIKey(handleOnboardOrg)
[✅] POST /hooks/deboard-org             → webhookAuth(handleDeboardOrg)
[✅] POST /hooks/onboard-contact         → webhookAuth(handleOnboardContact)
[✅] POST /hooks/deboard-contact         → webhookAuth(handleDeboardContact)
[✅] POST /hooks/update-pw               → webhookAuth(handlePWChange)
[✅] POST /hooks/update-bandwidth        → webhookAuth(handleBandwidthUpdate)

[✅] Top-of-file comment in routes.go listing full route surface
```

### Handler Tests

```
[✅] Update tests to hit all new routes
[✅] All new routes return 501 - tests assert 501 for now
[✅] make test passes
```

### Job Args

```
[✅] internal/jobs/args.go
    - OnboardOrgArgs        - implements river.JobArgs (crm_id JSON tag, no order_id)
    - DeboardOrgArgs        - implements river.JobArgs
    - AddContactArgs        - implements river.JobArgs
    - DeleteContactArgs     - implements river.JobArgs
    - UpdatePwArgs          - implements river.JobArgs
    - UpdateBandwidthArgs   - implements river.JobArgs
    - Doc comment per args struct
```

### Worker Registry

```
[✅] internal/jobs/workers.go
    - func Register(pool, repo, logger) *river.Workers
    - Register all workers
    - Doc comment on Register(): lists all registered worker types
```

### Workflow State Repository

```
[✅] internal/workflow/state.go
    - WorkflowState struct (mirrors workflows table columns)
    - Repository interface: Create, Get, UpdateStep
    - PgxRepository concrete implementation
    - Doc comment on struct fields: purpose, nullable rationale
    - Doc comment on each repo method: what it reads/writes, tx behavior
```

### Step Interface

```
[✅] internal/workflow/step.go
    - type Step interface { Name() string; Run(ctx, state) error }
    - PermanentError struct + IsPermanent() helper
    - Doc comment on interface:
        - Idempotency contract
        - Error handling expectations (retryable vs permanent)
        - How state.data accumulator is used
```

### Workflow Runner

```
[✅] internal/workflow/runner.go
    - Runner: Load state → find current step → run → save
    - Handles resume after crash/restart (picks up at current_step)
    - Doc comment on Runner: resume semantics, failure behavior
    - Inline comments on step dispatch logic
    - 5 tests: all-complete, resume, failure, permanent error, accumulates data
```

### First Worker - OnboardOrg

```
[✅] internal/jobs/onboard_org.go
    - OnboardOrgWorker implements river.Worker[OnboardOrgArgs]
    - Uses workflow.Runner with ordered steps
    - Each step is independently idempotent — checks external system state,
      no global new/existing branching flag needed
    - Doc comment listing full step sequence:
        Step 0: check_org       — look up org in VCD, store metadata
        Step 1: network_config  — configure VDC default network (always runs)
        Step 2: saml_config     — check Keycloak SAML client; skip if exists
        Step 3: zerto_setup     — check Zerto org; update if exists, create if not
        Step 4: ldap_update     — check LDAP attrs; skip if set
        Step 5: keycloak_sync   — trigger user sync (always safe)
        Step 6: vapp_template   — provision if product_id matches template
    - Steps are stubs at this task - implemented in Tasks 5-7
```

### Wire Workers in run()

```
[✅] workers := jobs.Register(pool, repo, logger)
[✅] Pass workers to NewRiverClient()
[✅] River client starts workers on startup
```

### README.md

```
[✅] Full API surface table (provision-vdc removed — see design note below)
[✅] Auth key: withAPIKey (X-API-Key) vs webhookAuth (HMAC-SHA256)
[✅] Workflow engine overview:
    - Step interface contract
    - Resume-after-restart behavior (current_step in DB)
    - JSONB accumulator pattern (data grows as steps complete)
[✅] Job args reference table
[✅] How to manually insert a test job (psql snippet)
```

### Debug / Profiling Listener

```
[✅] Start separate HTTP listener on DEBUG_PORT (default :6061)
    - net/http/pprof registered on debug mux (5 explicit handlers)
    - /debug/pprof/goroutineleak served via pprof.Index path matching
      (no explicit registration needed — works when built with
       GOEXPERIMENT=goroutineleakprofile)
    - No auth — internal diagnostics only

[✅] Register runtime/metrics snapshot on expvar
    - Read metrics.All(), publish to /debug/vars JSON
    - GC stats, memory breakdown, goroutine counts, scheduler metrics

[✅] /debug/dashboard — live HTML dashboard (auto-refreshes every 10s)
    - Configurable refresh interval via query param (?interval=30)
    - Single HTML template with JS fetch loop — no external dependencies

[✅] Add to internal/config/config.go:
    - DebugPort string // DEBUG_PORT, default "6061"
[✅] Add to .envrc.example:
    export DEBUG_PORT=6061

[✅] Wire in run():
    - Started in goroutine, shutdown alongside main server
```

### Debug Quick Reference

```
Endpoints (always available while app is running):
  http://localhost:6061/debug/dashboard           Live metrics dashboard
  http://localhost:6061/debug/pprof/              Profile index (HTML)
  http://localhost:6061/debug/pprof/goroutine?debug=1   Current goroutine stacks
  http://localhost:6061/debug/pprof/goroutineleak       Leaked goroutines (1.26)
  http://localhost:6061/debug/vars                      expvar + runtime metrics JSON

On-demand profiling (flame graph / trace — run from terminal):
  go tool pprof -http=:6060 http://localhost:6061/debug/pprof/profile?seconds=10
  go tool pprof -http=:6060 http://localhost:6061/debug/pprof/heap
  curl -o trace.out http://localhost:6061/debug/pprof/trace?seconds=5 && go tool trace trace.out
```

### Verify

```
[✅] make test → all pass (stubs return 501 as expected)
[ ] Insert job manually via psql
[ ] Verify River worker picks it up
[ ] Verify workflow_state row is created and updated at each step
[ ] make test → all pass
[ ] ✅ Milestone: full route surface wired, first job runs through workflow engine
```

---

## Design Notes

### provision-vdc removed

HostBill's Script Provisioner always sends the same payload regardless of
whether the customer is new or existing. The onboard-org workflow with
idempotent steps handles both cases naturally — each step checks external
system state and either performs the operation or skips. A separate
provision-vdc route/job had no distinct HostBill trigger and was removed.

### crm_id JSON tag

HostBill sends the client ID field as `crm_id`. OnboardOrgArgs and request
types use this tag to match the actual payload.

### No order_id on onboard-org

The old HostBill handler did not receive an order_id. The field was removed
from OnboardOrgArgs and from the runner.Run() signature. The workflow_state
table still has the nullable order_id column for other workflow types that
may need it (e.g. update_bandwidth).
