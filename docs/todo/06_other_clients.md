# Task 6: Remaining Clients

All five integrations stubbed and unit tested. Full
env var reference table in README.

> Integration code exists in another application. Reuse and adapt - types and
> patterns will require modification. Bring in only what's needed (YAGNI).

---

## Checklist

### Keycloak Client

```
[ ] internal/clients/keycloak/client.go
    - type Client struct { baseURL, realm, httpClient, token, logger }
    - func New(cfg KeycloakConfig) *Client
    - Admin REST API, bearer token auth
    - Doc comment: which Keycloak APIs are used, token refresh behavior

[ ] internal/clients/keycloak/types.go
    - Realm, User, Client, Protocol, ProtocolMapper types
    - Doc comment on every exported type

[ ] Key methods:
    - GetSAMLEntityID(ctx, clientID string) (string, error)
    - TriggerUserSync(ctx, realm string) error
    - CreateClient(ctx, realm string, client Client) error
    - Doc comment per method: endpoint called, key behavior
```

### Zerto Client

```
[ ] internal/clients/zerto/client.go
    - type Client struct { baseURL, httpClient, session, logger }
    - func New(cfg ZertoConfig) *Client
    - Session auth (login endpoint → token → reuse)
    - Doc comment: DR API overview, session auth lifecycle

[ ] internal/clients/zerto/types.go
    - ZOrg, VPG, VRA types with correct tags
    - Doc comment on non-obvious fields

[ ] Key methods:
    - CreateZOrg(ctx, params ZOrgParams) (*ZOrg, error)
    - GetZOrg(ctx, crmID string) (*ZOrg, error)
    - SetZOrgLimits(ctx, zOrgID string, limits ZOrgLimits) error
    - Doc comment per method
```

### HostBill Client

```
[ ] internal/clients/hostbill/client.go
    - type Client struct { baseURL, apiID, apiKey, httpClient, logger }
    - func New(cfg HostBillConfig) *Client
    - API key auth (apiid + apikey query params or headers)
    - Doc comment: which HostBill APIs are used, auth method

[ ] internal/clients/hostbill/types.go
    - Account, Order, CustomField types
    - Doc comment on non-obvious fields

[ ] Key methods:
    - GetAccount(ctx, clientID string) (*Account, error)
    - UpdateAccountField(ctx, clientID, field, value string) error
    - Doc comment per method
```

### Active Directory / LDAP Client

```
[ ] internal/clients/adsvc/client.go
    - type Client struct { addr, bindDN, bindPW, baseDN, conn, logger }
    - func New(cfg ADConfig) *Client
    - Lazy dial: connect on first use, reconnect on error
    - Doc comment: lazy dial rationale, bind behavior, connection lifecycle

[ ] internal/clients/adsvc/errors.go
    - Classify LDAP errors as retryable vs non-retryable
    - Doc comment on classification criteria

[ ] Key methods:
    - AddUserToGroup(ctx, userDN, groupDN string) error
    - UpdateUserAttribute(ctx, userDN, attr, value string) error
    - Doc comment per method
```

### Structured Errors

```
[ ] internal/errors/errors.go
    - Error kinds: NotFound, Conflict, Retryable, Permanent, Internal
    - type Error struct { Kind, Op, Fields, Err }
    - Wrap(err, kind, op) *Error
    - IsRetryable(err) bool
    - Doc comment on error kinds and when to use each
    - Doc comment on usage pattern (wrap at the client boundary)
```

### Config

```
[ ] Add all client configs to internal/config/config.go:
    Keycloak:
    - KeycloakBaseURL  // KEYCLOAK_URL
    - KeycloakRealm    // KEYCLOAK_REALM
    - KeycloakUser     // KEYCLOAK_USER
    - KeycloakPassword // KEYCLOAK_PASSWORD

    Zerto:
    - ZertoBaseURL     // ZERTO_URL
    - ZertoUser        // ZERTO_USER
    - ZertoPassword    // ZERTO_PASSWORD

    HostBill:
    - HostBillBaseURL  // HOSTBILL_URL
    - HostBillAPIID    // HOSTBILL_API_ID
    - HostBillAPIKey   // HOSTBILL_API_KEY

    Active Directory:
    - ADAddr           // AD_ADDR
    - ADBindDN         // AD_BIND_DN
    - ADBindPassword   // AD_BIND_PASSWORD
    - ADBaseDN         // AD_BASE_DN

[ ] Doc comment on every new config field
[ ] Add all vars to .envrc.example (with placeholder values)
[ ] Wire all clients in run(), pass to NewServer()
```

### Tests

```
[ ] Unit tests for each client using httptest mock server (or mock LDAP)
[ ] Test retryable error classification for each client
[ ] make test → all pass
```

### README.md (append during this task)

```
[ ] Full env var reference table (all five clients):
    | Variable            | Required | Description                    |
    | ------------------- | -------- | ------------------------------ |
    | VCD_URL             | yes      | VCD base URL                   |
    | VCD_USER            | yes      | VCD admin username             |
    | ...                 | ...      | ...                            |

[ ] One-line client description (purpose + auth method):
    - VCD: Virtual Cloud Director API - session token auth
    - Zerto: DR platform API - session token auth
    - Keycloak: OIDC/SAML identity provider - bearer token auth
    - HostBill: Billing/CRM API - API key auth
    - Active Directory: LDAP directory - bind DN/password auth
```

### Verify

```
[ ] All clients connect to real dev servers successfully
[ ] Unit tests pass for all clients
[ ] make test → all pass
[ ] All integrations stubbed and tested
```
