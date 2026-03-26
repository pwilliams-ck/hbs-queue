# hbs-queue Database

## Overview

River manages its own tables (`river_job`, `river_leader`, etc.) via
auto-migrate on startup. The `workflow_state` table is ours - it tracks step
progress and intermediate data for idempotent workflows.

Deduplication is handled by River (`UniqueOpts{ByArgs: true}`) at job insertion
time. This table only records what happened, keyed by River's `job_id`.

## workflow_state Table Shchema

This table looks bad in terminals but renders fine. I do not think we will need
`order_id` but maybe. We can tell what is a hook by the workflow type.

| Column          | Type                   | Default             | Description                                                                                                                                      |
| --------------- | ---------------------- | ------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------ |
| `id`            | UUID                   | `gen_random_uuid()` | Primary key                                                                                                                                      |
| `job_id`        | BIGINT NOT NULL UNIQUE | -                   | River job ID, always unique                                                                                                                      |
| `workflow_type` | VARCHAR(50) NOT NULL   | -                   | `onboard_customer`, `provision_vdc`, `deboard_customer`, `add_contact`, `delete_contact`, `update_bandwidth`                                     |
| `client_id`     | VARCHAR(50) NOT NULL   | -                   | HostBill CRM ID - primary business key. Used by every external service (VCD org name, Keycloak client, Zerto CRM identifier, LDAP zorg username) |
| `order_id`      | VARCHAR(50)            | NULL                | HostBill order ID (NULL for hook-triggered jobs)                                                                                                 |
| `current_step`  | INT NOT NULL           | `0`                 | Last completed step index. On retry, worker resumes here                                                                                         |
| `status`        | VARCHAR(20) NOT NULL   | `'pending'`         | `pending`, `running`, `completed`, `failed`                                                                                                      |
| `error`         | TEXT                   | NULL                | Last error message if failed                                                                                                                     |
| `data`          | JSONB NOT NULL         | `'{}'`              | Starts as HostBill payload, accumulates intermediate results as each step completes                                                              |
| `created_at`    | TIMESTAMPTZ NOT NULL   | `now()`             | -                                                                                                                                                |
| `updated_at`    | TIMESTAMPTZ NOT NULL   | `now()`             | -                                                                                                                                                |

## Indexes

| Name                 | Columns                    | Purpose                   |
| -------------------- | -------------------------- | ------------------------- |
| `idx_ws_client_id`   | `client_id`                | Lookup by CRM ID          |
| `idx_ws_status`      | `status`                   | Filter by workflow status |
| `idx_ws_type_client` | `workflow_type, client_id` | Filter by type + client   |

No uniqueness enforcement in indexes - River handles dedup.

```sql
CREATE TABLE workflows (
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

## data JSONB - Accumulator Pattern

The `data` column starts with the HostBill payload and accumulates intermediate
results as each step completes. Each step reads what it needs and writes what it
produces.

### Example: onboard_customer workflow

**Initial state** (written by handler at job insertion):

```json
{
  "organization_name": "AcmeCorp",
  "client_username": "client123456",
  "client_first_name": "John",
  "client_last_name": "Doe",
  "client_email": "jdoe@acme.com",
  "crm_id": "123456",
  "account_id": 789,
  "country": "US",
  "state": "Texas",
  "postal_code": "75074",
  "max_zerto_storage": 50,
  "max_zerto_vms": 5,
  "bandwidth": "100",
  "product_id": "web-server-1"
}
```

**Step-by-step accumulation:**

| Step | Name           | Adds to data                                                        |
| ---- | -------------- | ------------------------------------------------------------------- |
| 0    | Check tenant   | `tenant_type`                                                       |
| 1    | Network config | `vcd_virtual_dc_name`, `default_network_id`, `default_network_uuid` |
| 2    | SAML config    | `vcd_org_id`, `vcd_uuid`, `keycloak_entity_id`                      |
| 3    | Zerto setup    | `zerto_org_id`, `zerto_domain`, `vdc_id`, `vdc_uuid`                |
| 4    | LDAP update    | `ldap_dn`, `zorg_username`                                          |
| 5    | Keycloak sync  | (no new data, triggers sync)                                        |
| 6    | vApp template  | `vapp_name`                                                         |

**Final state** after all steps complete:

```json
{
  "organization_name": "AcmeCorp",
  "client_username": "client123456",
  "client_first_name": "John",
  "client_last_name": "Doe",
  "client_email": "jdoe@acme.com",
  "crm_id": "123456",
  "account_id": 789,
  "country": "US",
  "state": "Texas",
  "postal_code": "75074",
  "max_zerto_storage": 50,
  "max_zerto_vms": 5,
  "bandwidth": "100",
  "product_id": "web-server-1",
  "tenant_type": "new",
  "vcd_virtual_dc_name": "789-site1",
  "default_network_id": "urn:vcloud:network:net-001",
  "default_network_uuid": "net-001",
  "vcd_org_id": "urn:vcloud:org:abc123",
  "vcd_uuid": "abc123",
  "keycloak_entity_id": "https://keycloak.cloudkey.io/...",
  "zerto_org_id": "zorg-456",
  "zerto_domain": "123456.cloudkey.io",
  "vdc_id": "urn:vcloud:vdc:xyz789",
  "vdc_uuid": "xyz789",
  "ldap_dn": "cn=jdoe,ou=users,dc=cloudkey,dc=io",
  "zorg_username": "jdoe@123456",
  "vapp_name": "web-server-template"
}
```
