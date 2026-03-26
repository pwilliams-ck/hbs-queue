# Task 5: First Client - VCD

VCD API calls working against real server. Client
docs in README.

> VCD integration code exists in another application. Reuse and adapt types and
> patterns as needed - they will require modification.

---

## Checklist

### VCD Client

```
[ ] internal/clients/vcd/client.go
    - type Client struct { baseURL, httpClient, auth, logger }
    - func New(cfg VCDConfig) *Client
    - Lazy session auth (authenticate on first request, reuse token)
    - Doc comment on Client struct: fields and their purpose
    - Doc comment on New(): auth lifecycle, session expiry behavior

[ ] internal/clients/vcd/codec_xml.go
    - encodeXML(v any) ([]byte, error)
    - decodeXML(data []byte, v any) error
    - Doc comment: when VCD uses XML vs JSON (older endpoints use XML)

[ ] internal/clients/vcd/codec_json.go
    - encodeJSON(v any) ([]byte, error)
    - decodeJSON(data []byte, v any) error

[ ] internal/clients/vcd/types.go
    - Org, VDC, Network, OrgVDCNetwork, and other VCD resource types
    - All fields with correct xml/json struct tags
    - Doc comment on every exported type
    - Doc comment on non-obvious fields

[ ] internal/clients/vcd/errors.go
    - Classify VCD API errors as retryable vs non-retryable
    - Doc comment on retryable classification criteria
    - Examples: 429/503 = retryable; 404/400 = non-retryable
```

### Key Methods

```
[ ] CreateOrg(ctx, name string) (*Org, error)
    - Doc: VCD API endpoint called, key behavior

[ ] GetOrg(ctx, name string) (*Org, error)
[ ] DeleteOrg(ctx, name string) error

[ ] CreateVDC(ctx, orgName string, params VDCParams) (*VDC, error)
[ ] GetVDC(ctx, orgName, vdcName string) (*VDC, error)

[ ] Each method:
    - Uses structured errors from internal/errors
    - Respects context cancellation
    - Logs at debug level with request/response details
```

### Retry Helper

```
[ ] internal/retry/retry.go
    - func Do(ctx, maxAttempts int, fn func() error) error
    - Exponential backoff with jitter
    - Respects context cancellation (stops on ctx.Done())
    - Doc comment: backoff strategy, max attempts, jitter rationale
    - Used by all clients and workflow steps
```

### Config

```
[ ] Add VCD config to internal/config/config.go:
    - VCDBaseURL   string  // VCD_URL
    - VCDUser      string  // VCD_USER
    - VCDPassword  string  // VCD_PASSWORD
    - VCDOrg       string  // VCD_ORG (system org for auth)
    - Doc comment on each field

[ ] Add to .envrc.example:
    export VCD_URL=https://vcd.example.com
    export VCD_USER=admin
    export VCD_PASSWORD=
    export VCD_ORG=System

[ ] Wire VCD client in run(), pass to NewServer()
```

### Tests

```
[ ] Unit tests using httptest mock server
[ ] Test retryable vs non-retryable error classification
[ ] Test session auth lifecycle
[ ] make test → all pass
```

### README.md (append during this task)

```
[ ] VCD client config env vars (table: var, required, description)
[ ] Retry/backoff behavior (strategy, max attempts)
[ ] VCD API auth note (session token, lazy init)
```

### Verify

```
[ ] VCD client calls succeed against real dev VCD server
[ ] ✅ Milestone: VCD integration working
```
