CREATE TABLE IF NOT EXISTS workflow_state (
    -- Primary key.
    id             UUID           PRIMARY KEY DEFAULT gen_random_uuid(),

    -- River job ID. Always unique — one workflow row per job.
    job_id         BIGINT         NOT NULL UNIQUE,

    -- Workflow kind: onboard_org, deboard_org, onboard_contact, etc.
    workflow_type  VARCHAR(50)    NOT NULL,

    -- HostBill CRM ID — the primary business key used across all external
    -- services (VCD org name, Keycloak client, Zerto CRM identifier).
    client_id      VARCHAR(50)    NOT NULL,

    -- HostBill order ID. NULL for hook-triggered jobs that have no order.
    order_id       VARCHAR(50),

    -- Last completed step index. On retry the worker resumes from here.
    current_step   INT            NOT NULL DEFAULT 0,

    -- pending → running → completed | failed.
    status         VARCHAR(20)    NOT NULL DEFAULT 'pending',

    -- Last error message when status is failed.
    error          TEXT,

    -- Starts as the HostBill payload, accumulates intermediate results
    -- as each workflow step completes.
    data           JSONB          NOT NULL DEFAULT '{}',

    created_at     TIMESTAMPTZ    NOT NULL DEFAULT now(),
    updated_at     TIMESTAMPTZ    NOT NULL DEFAULT now()
);

-- Lookup by CRM ID (all workflows for a tenant).
CREATE INDEX IF NOT EXISTS idx_ws_client_id ON workflow_state (client_id);

-- Filter by status (find running/failed workflows).
CREATE INDEX IF NOT EXISTS idx_ws_status ON workflow_state (status);

-- Filter by type + client (e.g. all onboard jobs for a tenant).
CREATE INDEX IF NOT EXISTS idx_ws_type_client ON workflow_state (workflow_type, client_id);
