# hbsqueue Project Documentation

## Table of Contents

- [Project Summary](#project-summary)
- [Policies](#policies)
  - [Documentation](#documentation-policy)
  - [Environment Strategy](#environment-strategy)
  - [Git Commits](#git-commits)
  - [Code Examples](#code-examples)
- [Quick Reference - Days at a Glance](#quick-reference--days-at-a-glance)
- [Target Directory Tree](#target-directory-tree)
- [TODO Files](#todo-files)

---

## Project Summary

**hbsqueue** HostBill Service Queue that orchestrates multi-tenant resource
provisioning workflows across several external platforms: VCD, Zerto, Keycloak,
HostBill, Reddit, and Active Directory.

**Target state:** Full River-backed job queue service with
VCD/Zerto/Keycloak/HostBill/AD integrations. Stateful, idempotent writes to DB
at every possible step in workflow jobs using a custom `workflows` table.

The VCD/Zerto/Keycloak/HostBill/AD integration code was written for another
application and will be reused and adapted as needed. Types and patterns will
require modification.

---

## Policies

### Documentation Policy

There is no dedicated documentation task. All docs are written inline as each
task is completed. Every file, function, type, and config gets its doc comment
when it is written - not later. README sections are updated at the end of each
task. If it isn't documented when the task closes, it is not done.

### Environment Strategy

All tasks through Task 7 run entirely on localhost with Postgres in Docker
Compose. Handlers can be tested locally by temporarily pointing HostBill's
script provisioning / webhook URLs at the developer's workstation IP. Once the
application code is proven locally, Task 8 moves to Docker Compose on dev/prod
hosts with CI/CD pipeline.

- `main` branch auto-deploys to dev (Docker Compose on Ubuntu 24.04)
- Git tags trigger prod deployments
- Blue/green app deploys with automated rollback
- DB backups follow the 3-2-1-1-0 rule with restore verification

### Git Commits

All git operations are done by a human. Help writing commit messages follows
this format:

```
net/http: handle foo when bar

[longer description here in the body]

Fixes #nnnn
```

### Code Examples

`docs/todo/code-examples.md` contains River and hbs-queue patterns to follow for
readable, scalable, testable Go in a standard application structure. Follow
these patterns. Keep external package dependencies to a minimum.

---

## Quick Reference

| Task                        | Status | Description                                                   |
| --------------------------- | ------ | ------------------------------------------------------------- |
| Task 1: Init                | ✅     | Repo created, service runs, /ready works, README started      |
| Task 2: Local Docker        | ✅     | Postgres in Docker Compose, DATABASE_URL wired                |
| Task 3: DB + River          | ✅     | /ready and /health ping DB, localhost dev complete             |
| Task 4: Handlers + Workflow | ✅     | Routes wired, first job runs through workflow engine           |
| Task 5: VCD Client          | ✅     | VCD calls working, client docs in README                      |
| Task 6: Other Clients       |        | All integrations stubbed, env var table complete               |
| Task 7: Full Handlers       |        | End-to-end flows working, flow diagram in README               |
| Task 8: Deploy              |        | Docker Compose on dev/prod, CI runner, auto-deploy, backups    |
| Task 9: Polish              |        | Containers, CI, security review, runbook in README             |

---

## Target Directory Tree

```
hbsqueue/
├── cmd/hbsqueue/
│   ├── main.go
│   ├── main_test.go
│   └── debug.go
│
├── internal/
│   ├── config/
│   │   ├── config.go
│   │   └── config_test.go
│   │
│   ├── httpapi/
│   │   ├── server.go
│   │   ├── routes.go
│   │   ├── middleware.go
│   │   ├── encode.go
│   │   ├── types.go
│   │   ├── handlers.go                  # ready, health
│   │   ├── handle_onboard_org.go
│   │   ├── handle_deboard_org.go
│   │   ├── handle_onboard_contact.go
│   │   ├── handle_deboard_contact.go
│   │   ├── handle_update_pw.go
│   │   └── handle_update_bandwidth.go
│   │
│   ├── db/
│   │   ├── db.go                        # NewPool
│   │   ├── migrate.go                   # Embedded SQL migrations
│   │   ├── river.go                     # NewRiverClient
│   │   └── migrations/
│   │       ├── 001_workflow_state.up.sql
│   │       └── 001_workflow_state.down.sql
│   │
│   ├── clients/
│   │   ├── vcd/
│   │   │   ├── client.go               # Client struct, New(), auth, shared helpers
│   │   │   ├── codec.go
│   │   │   ├── errors.go
│   │   │   ├── org.go                   # CreateOrg, GetOrg, DeleteOrg
│   │   │   ├── vdc.go                   # CreateVDC, GetVDC, configureVDC
│   │   │   └── types.go
│   │   ├── zerto/                       # (future)
│   │   ├── keycloak/                    # (future)
│   │   ├── hostbill/                    # (future)
│   │   └── adsvc/                       # (future)
│   │
│   ├── jobs/
│   │   ├── args.go                      # River job arg structs
│   │   ├── workers.go                   # Register()
│   │   └── onboard_org.go
│   │
│   ├── workflow/
│   │   ├── state.go                     # WorkflowState repository
│   │   ├── runner.go                    # Step runner
│   │   └── step.go                      # Step interface
│   │
│   └── retry/
│       └── retry.go                     # Backoff helper
│
├── .github/workflows/ci.yml
├── .golangci.yml
├── .envrc.example
├── .gitignore
├── Dockerfile
├── docker-compose.yml
├── Makefile
├── README.md
└── go.mod
```

---

## TODO Files

| File                                                       | Task | Status | Focus                             |
| ---------------------------------------------------------- | ---- | ------ | --------------------------------- |
| [01_init.md](./01_init.md)                                 | 1    | ✅     | Repo, scaffold, HTTP service      |
| [02_localhost.md](./02_localhost.md)                        | 2    | ✅     | Local Postgres in Docker          |
| [03_db_river.md](./03_db_river.md)                         | 3    | ✅     | DB package, River client          |
| [04_jobs_workflow.md](./04_jobs_workflow.md)                | 4    | ✅     | Handlers + Jobs + Workflow engine |
| [05_vcd_client.md](./05_vcd_client.md)                     | 5    | ✅     | VCD client                        |
| [06_other_clients.md](./06_other_clients.md)               | 6    |        | Keycloak, Zerto, HB, AD           |
| [07_integration.md](./07_integration.md)                   | 7    |        | Full handlers + integration       |
| [08_deploy.md](./08_deploy.md)                             | 8    |        | Docker Compose + CI/CD + ops      |
| [09_production_prep.md](./09_production_prep.md)           | 9    |        | Containers, CI, prod prep         |

**Reference docs:**

- [db-schema-01.md](./db-schema-01.md) - `workflows` table schema and JSONB
  accumulator pattern
- [code-examples.md](./code-examples.md) - Go patterns, River patterns, full
  scaffold code
