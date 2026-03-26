# Task 7: Complete Handlers + Integration

End-to-end flows working. Flow diagram and job
status endpoint in README.

> This is where stubs become real. Each handler enqueues a River job. Each
> worker runs the full workflow step sequence against real external services.

---

## Checklist

### Implement script_provision Handlers

```
[ ] handlers/script_provision/onboard_org.go
    - Validate request → insert River job → return 202 + job ID
    - Doc comment: full request/response contract, job type enqueued

[ ] handlers/script_provision/provision_vdc.go
    - Validate request → insert River job → return 202 + job ID
    - Doc comment: full request/response contract, job type enqueued
```

### Implement hooks Handlers

```
[ ] Each hook handler:
    - Validate hook auth (already handled by withWebhook middleware)
    - Decode and validate request payload
    - Insert River job → return 202

[ ] hooks/deboard_org.go
[ ] hooks/onboard_contact.go
[ ] hooks/deboard_contact.go
[ ] hooks/update_pw.go
[ ] hooks/update_bandwidth.go

[ ] Doc comment per handler: expected hook payload, job type enqueued
```

### Implement Job Workers (Full Step Sequences)

```
[ ] internal/jobs/onboard_org.go - OnboardOrgWorker
    Steps (implement each as a named Step):
    Step 0: Check tenant (existing vs new)
    Step 1: Network config (VCD - get default network)
    Step 2: SAML config (VCD org + Keycloak entity ID)
    Step 3: Zerto setup (create Zorg, set limits, get VDC IDs)
    Step 4: LDAP update (AD - add user, set attrs)
    Step 5: Keycloak sync (trigger user federation sync)
    Step 6: vApp template (VCD - catalog item lookup)
    - Doc comment listing full step sequence with one-line purpose each

[ ] internal/jobs/provision_vdc.go - ProvisionVDCWorker
    - Steps TBD based on VCD provisioning flow
    - Doc comment listing full step sequence

[ ] internal/jobs/deboard_org.go - DeboardOrgWorker
    - Steps: reverse of onboarding (cleanup Zerto, Keycloak, VCD, AD)
    - Doc comment listing full step sequence

[ ] internal/jobs/add_contact.go - AddContactWorker
[ ] internal/jobs/delete_contact.go - DeleteContactWorker
[ ] internal/jobs/update_bandwidth.go - UpdateBandwidthWorker
```

### Job Status Endpoint

```
[ ] GET /api/v1/jobs/:id
    - Auth: withAPIKey
    - Query workflows table by job_id
    - Return: job_id, workflow_type, client_id, current_step, status, error, updated_at
    - 404 if not found
    - Doc comment: response shape, auth required

[ ] Wire in routes.go
[ ] Add types to types.go
[ ] Tests for status endpoint (found, not found, unauthorized)
```

### Integration Tests

```
[ ] Use testcontainers-go for Postgres
[ ] Full flow test:
    HTTP request → River job inserted → worker runs steps → workflow_state updated → status endpoint reflects completion
[ ] At minimum: onboard_org happy path
[ ] make test-integration target in Makefile
```

### README.md (append during this task)

```
[ ] End-to-end flow diagram:
    HTTP POST → handler validates → River job inserted
         ↓
    River worker picks up job
         ↓
    workflow.Runner loads state from DB
         ↓
    Steps execute in order (each writes to data JSONB)
         ↓
    Status: completed

[ ] Job status endpoint usage:
    curl -H "X-API-Key: $API_KEY" http://localhost:8080/api/v1/jobs/12345

[ ] Integration test instructions:
    make test-integration
```

### Verify

```
[ ] POST /api/v1/script/onboard-org → 202, job ID returned
[ ] River worker picks up job, runs all steps
[ ] GET /api/v1/jobs/:id → reflects current status
[ ] make test → unit tests pass
[ ] make test-integration → integration tests pass
[ ] End-to-end flows working
```
