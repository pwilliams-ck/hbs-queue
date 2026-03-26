## Code Examples to Get Started

## go.mod

```go
module github.com/CloudKey-io/hbs-queue

go 1.26
```

---

## Makefile

```makefile
VERSION    ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT     ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "none")
BUILD_TIME ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)

LDFLAGS := -ldflags "\
 -X github.com/CloudKey-io/hbs-queue/internal/config.Version=$(VERSION) \
 -X github.com/CloudKey-io/hbs-queue/internal/config.Commit=$(COMMIT) \
 -X github.com/CloudKey-io/hbs-queue/internal/config.BuildTime=$(BUILD_TIME)"

.PHONY: run build test lint clean

run:
 go run ./cmd/hbsqueue

build:
 go build $(LDFLAGS) -o bin/hbsqueue ./cmd/hbsqueue

test:
 go test -v -race -cover ./...

lint:
 golangci-lint run

clean:
 rm -rf bin/
```

---

## cmd/hbsqueue/main.go

```go
package main

import (
 "context"
 "fmt"
 "io"
 "log/slog"
 "net/http"
 "os"
 "os/signal"
 "sync"
 "syscall"
 "time"

 "github.com/CloudKey-io/hbs-queue/internal/config"
 "github.com/CloudKey-io/hbs-queue/internal/httpapi"
)

func main() {
 ctx := context.Background()
 if err := run(ctx, os.Args, os.Getenv, os.Stdout, os.Stderr); err != nil {
  fmt.Fprintf(os.Stderr, "error: %v\n", err)
  os.Exit(1)
 }
}

func run(
 ctx context.Context,
 args []string,
 getenv func(string) string,
 stdout, stderr io.Writer,
) error {
 ctx, cancel := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
 defer cancel()

 // Config
 cfg := config.Load(getenv)

 // Logger
 logLevel := slog.LevelInfo
 if cfg.Env == "dev" {
  logLevel = slog.LevelDebug
 }
 logger := slog.New(slog.NewJSONHandler(stdout, &slog.HandlerOptions{Level: logLevel}))

 // Future: DB pool
 // pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
 // if err != nil {
 //     return fmt.Errorf("db connect: %w", err)
 // }
 // defer pool.Close()

 // Future: River client
 // riverClient, err := river.NewClient(...)
 // if err != nil {
 //     return fmt.Errorf("river client: %w", err)
 // }

 // Server
 srv := httpapi.NewServer(
  logger,
  cfg,
  // pool,
  // riverClient,
 )

 httpServer := &http.Server{
  Addr:              ":" + cfg.Port,
  Handler:           srv,
  ReadHeaderTimeout: 10 * time.Second,
  ReadTimeout:       30 * time.Second,
  WriteTimeout:      30 * time.Second,
  IdleTimeout:       60 * time.Second,
 }

 // Start HTTP server
 var wg sync.WaitGroup

 wg.Add(1)
 go func() {
  defer wg.Done()
  logger.Info("server starting", "addr", httpServer.Addr, "env", cfg.Env)
  if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
   logger.Error("http server error", "err", err)
  }
 }()

 // Wait for shutdown signal
 <-ctx.Done()
 logger.Info("shutdown signal received")

 // Graceful shutdown
 shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
 defer shutdownCancel()

 // 1. HTTP: stop accepting, drain in-flight
 if err := httpServer.Shutdown(shutdownCtx); err != nil {
  logger.Error("http shutdown error", "err", err)
 }

 // 2. Future: River stop
 // if err := riverClient.Stop(shutdownCtx); err != nil {
 //     logger.Error("river shutdown error", "err", err)
 // }

 // 3. Future: DB pool closes via defer above

 wg.Wait()
 logger.Info("shutdown complete")
 return nil
}
```

---

## cmd/hbsqueue/main_test.go

```go
package main

import (
 "bytes"
 "context"
 "encoding/json"
 "fmt"
 "io"
 "net"
 "net/http"
 "testing"
 "time"

 "github.com/CloudKey-io/hbs-queue/internal/httpapi"
)

func TestRun(t *testing.T) {
 t.Parallel()

 port := freePort(t)
 ctx, cancel := context.WithCancel(context.Background())
 t.Cleanup(cancel)

 getenv := mockGetenv(map[string]string{
  "PORT":    port,
  "ENV":     "test",
  "API_KEY": "test-key",
 })

 stdout := &bytes.Buffer{}
 stderr := &bytes.Buffer{}

 runErr := make(chan error, 1)
 go func() {
  runErr <- run(ctx, []string{"hbsqueue"}, getenv, stdout, stderr)
 }()

 baseURL := fmt.Sprintf("http://localhost:%s", port)
 if err := waitForReady(ctx, 5*time.Second, baseURL+"/ready"); err != nil {
  t.Fatalf("server not ready: %v", err)
 }

 t.Run("ready returns 200", func(t *testing.T) {
  resp, err := http.Get(baseURL + "/ready")
  if err != nil {
   t.Fatalf("GET /ready: %v", err)
  }
  defer resp.Body.Close()

  if resp.StatusCode != http.StatusOK {
   t.Errorf("got status %d, want 200", resp.StatusCode)
  }

  var body httpapi.ReadyResponse
  json.NewDecoder(resp.Body).Decode(&body)
  if body.Status != "ok" {
   t.Errorf("got status %q, want %q", body.Status, "ok")
  }
 })

 t.Run("health returns version info", func(t *testing.T) {
  resp, err := http.Get(baseURL + "/health")
  if err != nil {
   t.Fatalf("GET /health: %v", err)
  }
  defer resp.Body.Close()

  if resp.StatusCode != http.StatusOK {
   t.Errorf("got status %d, want 200", resp.StatusCode)
  }
 })

 t.Run("echo requires API key", func(t *testing.T) {
  req, _ := http.NewRequest(http.MethodPost, baseURL+"/api/v1/echo",
   bytes.NewBufferString(`{"message":"hello"}`))
  req.Header.Set("Content-Type", "application/json")

  resp, err := http.DefaultClient.Do(req)
  if err != nil {
   t.Fatalf("POST /api/v1/echo: %v", err)
  }
  defer resp.Body.Close()

  if resp.StatusCode != http.StatusUnauthorized {
   t.Errorf("got status %d, want 401", resp.StatusCode)
  }
 })

 t.Run("echo with valid API key", func(t *testing.T) {
  req, _ := http.NewRequest(http.MethodPost, baseURL+"/api/v1/echo",
   bytes.NewBufferString(`{"message":"hello"}`))
  req.Header.Set("Content-Type", "application/json")
  req.Header.Set("X-API-Key", "test-key")

  resp, err := http.DefaultClient.Do(req)
  if err != nil {
   t.Fatalf("POST /api/v1/echo: %v", err)
  }
  defer resp.Body.Close()

  if resp.StatusCode != http.StatusOK {
   t.Errorf("got status %d, want 200", resp.StatusCode)
  }

  var body httpapi.EchoResponse
  json.NewDecoder(resp.Body).Decode(&body)
  if body.Echo != "hello" {
   t.Errorf("got echo %q, want %q", body.Echo, "hello")
  }
  if body.RequestID == "" {
   t.Error("expected request_id in response")
  }
 })

 t.Run("echo validates input", func(t *testing.T) {
  req, _ := http.NewRequest(http.MethodPost, baseURL+"/api/v1/echo",
   bytes.NewBufferString(`{"message":""}`))
  req.Header.Set("Content-Type", "application/json")
  req.Header.Set("X-API-Key", "test-key")

  resp, err := http.DefaultClient.Do(req)
  if err != nil {
   t.Fatalf("POST /api/v1/echo: %v", err)
  }
  defer resp.Body.Close()

  if resp.StatusCode != http.StatusUnprocessableEntity {
   t.Errorf("got status %d, want 422", resp.StatusCode)
  }
 })

 t.Run("request ID flows through", func(t *testing.T) {
  req, _ := http.NewRequest(http.MethodPost, baseURL+"/api/v1/echo",
   bytes.NewBufferString(`{"message":"test"}`))
  req.Header.Set("Content-Type", "application/json")
  req.Header.Set("X-API-Key", "test-key")
  req.Header.Set("X-Request-ID", "my-trace-id")

  resp, err := http.DefaultClient.Do(req)
  if err != nil {
   t.Fatalf("request: %v", err)
  }
  defer resp.Body.Close()

  if got := resp.Header.Get("X-Request-ID"); got != "my-trace-id" {
   t.Errorf("got request ID %q, want %q", got, "my-trace-id")
  }

  var body httpapi.EchoResponse
  json.NewDecoder(resp.Body).Decode(&body)
  if body.RequestID != "my-trace-id" {
   t.Errorf("got body request ID %q, want %q", body.RequestID, "my-trace-id")
  }
 })

 // Shutdown
 cancel()

 select {
 case err := <-runErr:
  if err != nil {
   t.Errorf("run error: %v", err)
  }
 case <-time.After(5 * time.Second):
  t.Error("shutdown timed out")
 }
}

func TestRunParallel(t *testing.T) {
 t.Parallel()

 tests := []struct {
  name   string
  env    string
  apiKey string
 }{
  {"dev", "dev", "dev-key"},
  {"prod", "prod", "prod-key"},
 }

 for _, tt := range tests {
  t.Run(tt.name, func(t *testing.T) {
   t.Parallel()

   port := freePort(t)
   ctx, cancel := context.WithCancel(context.Background())
   t.Cleanup(cancel)

   getenv := mockGetenv(map[string]string{
    "PORT":    port,
    "ENV":     tt.env,
    "API_KEY": tt.apiKey,
   })

   go run(ctx, []string{"hbsqueue"}, getenv, io.Discard, io.Discard)

   baseURL := fmt.Sprintf("http://localhost:%s", port)
   if err := waitForReady(ctx, 5*time.Second, baseURL+"/ready"); err != nil {
    t.Fatalf("server not ready: %v", err)
   }

   req, _ := http.NewRequest(http.MethodPost, baseURL+"/api/v1/echo",
    bytes.NewBufferString(`{"message":"test"}`))
   req.Header.Set("Content-Type", "application/json")
   req.Header.Set("X-API-Key", tt.apiKey)

   resp, _ := http.DefaultClient.Do(req)
   defer resp.Body.Close()

   if resp.StatusCode != http.StatusOK {
    t.Errorf("got status %d, want 200", resp.StatusCode)
   }
  })
 }
}

// mockGetenv returns a getenv func from a map
func mockGetenv(m map[string]string) func(string) string {
 return func(key string) string {
  return m[key]
 }
}

// freePort returns an available port
func freePort(t *testing.T) string {
 t.Helper()
 l, err := net.Listen("tcp", "localhost:0")
 if err != nil {
  t.Fatalf("get free port: %v", err)
 }
 defer l.Close()
 _, port, _ := net.SplitHostPort(l.Addr().String())
 return port
}

// waitForReady polls until endpoint returns 200
func waitForReady(ctx context.Context, timeout time.Duration, endpoint string) error {
 client := &http.Client{Timeout: 1 * time.Second}
 deadline := time.Now().Add(timeout)

 for time.Now().Before(deadline) {
  req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
  if err != nil {
   return err
  }

  resp, err := client.Do(req)
  if err == nil {
   resp.Body.Close()
   if resp.StatusCode == http.StatusOK {
    return nil
   }
  }

  select {
  case <-ctx.Done():
   return ctx.Err()
  case <-time.After(50 * time.Millisecond):
  }
 }
 return fmt.Errorf("timeout waiting for %s", endpoint)
}
```

---

## internal/config/config.go

```go
package config

// Set via ldflags
var (
 Version   = "dev"
 Commit    = "none"
 BuildTime = "unknown"
)

type Config struct {
 // App
 Port string
 Env  string // dev, prod

 // Auth
 APIKey string

 // Build info
 Version   string
 Commit    string
 BuildTime string

 // Future
 // DatabaseURL string
}

func Load(getenv func(string) string) *Config {
 return &Config{
  Port:      envOr(getenv, "PORT", "8080"),
  Env:       envOr(getenv, "ENV", "dev"),
  APIKey:    getenv("API_KEY"),
  Version:   Version,
  Commit:    Commit,
  BuildTime: BuildTime,
  // DatabaseURL: getenv("DATABASE_URL"),
 }
}

func envOr(getenv func(string) string, key, fallback string) string {
 if v := getenv(key); v != "" {
  return v
 }
 return fallback
}
```

---

## internal/httpapi/server.go

```go
package httpapi

import (
 "log/slog"
 "net/http"

 "github.com/CloudKey-io/hbs-queue/internal/config"
)

func NewServer(
 logger *slog.Logger,
 cfg *config.Config,
 // pool *pgxpool.Pool,      // Future
 // river *river.Client[pgx.Tx], // Future
) http.Handler {
 mux := http.NewServeMux()

 addRoutes(
  mux,
  logger,
  cfg,
  // pool,
  // river,
 )

 // Global middleware - outermost wraps first, executes first
 var handler http.Handler = mux
 handler = panicRecovery(handler, logger)
 handler = requestLogger(handler, logger)
 handler = requestID(handler)

 return handler
}
```

---

## internal/httpapi/routes.go

```go
package httpapi

import (
 "log/slog"
 "net/http"

 "github.com/CloudKey-io/hbs-queue/internal/config"
)

func addRoutes(
 mux *http.ServeMux,
 logger *slog.Logger,
 cfg *config.Config,
 // pool *pgxpool.Pool,
 // river *river.Client[pgx.Tx],
) {
 // Middleware factories
 withAPIKey := apiKeyAuth(cfg.APIKey)

 // Health - no auth
 mux.Handle("GET /ready", handleReady())
 mux.Handle("GET /health", handleHealth(cfg))

 // API - API key auth
 mux.Handle("POST /api/v1/echo", withAPIKey(handleEcho(logger)))

 // Future: hook routes with per-hook auth
 // withWebhook := hookAuth(hookSecrets)
 // mux.Handle("POST /hooks/billing", withWebhook(handleBillingWebhook(logger, river)))

 // Future: org onboarding
 // mux.Handle("POST /api/v1/orgs", withAPIKey(handleCreateOrg(logger, river)))

 // Fallback
 mux.Handle("/", http.NotFoundHandler())
}
```

---

## internal/httpapi/handlers.go

```go
package httpapi

import (
 "log/slog"
 "net/http"

 "github.com/CloudKey-io/hbs-queue/internal/config"
)

// handleReady returns 200 if service can handle requests.
// Used by load balancers and orchestrators.
func handleReady() http.Handler {
 return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
  // Future: add pool.Ping(r.Context()) check
  encode(w, http.StatusOK, ReadyResponse{Status: "ok"})
 })
}

// handleHealth returns diagnostic info.
func handleHealth(cfg *config.Config) http.Handler {
 // Build response once at startup
 resp := HealthResponse{
  Status:    "healthy",
  Version:   cfg.Version,
  Commit:    cfg.Commit,
  BuildTime: cfg.BuildTime,
 }

 return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
  encode(w, http.StatusOK, resp)
 })
}

// handleEcho demonstrates the full request lifecycle:
// decode → validate → process → encode
func handleEcho(logger *slog.Logger) http.Handler {
 // One-time init outside closure
 return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
  ctx := r.Context()
  reqID := RequestID(ctx)

  req, problems, err := decodeValid[EchoRequest](r)
  if err != nil {
   if len(problems) > 0 {
    logger.Debug("validation failed",
     "request_id", reqID,
     "problems", problems,
    )
    encode(w, http.StatusUnprocessableEntity, ErrorResponse{
     Error:    "validation failed",
     Problems: problems,
    })
    return
   }
   logger.Debug("decode failed", "request_id", reqID, "err", err)
   encode(w, http.StatusBadRequest, ErrorResponse{Error: "invalid json"})
   return
  }

  logger.Debug("echo request",
   "request_id", reqID,
   "message", req.Message,
  )

  encode(w, http.StatusOK, EchoResponse{
   Echo:      req.Message,
   RequestID: reqID,
  })
 })
}

// Future handler example with River job insertion:
//
// func handleCreateOrg(logger *slog.Logger, river *river.Client[pgx.Tx]) http.Handler {
//     return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
//         req, problems, err := decodeValid[CreateOrgRequest](r)
//         if err != nil {
//             // handle error...
//         }
//
//         // Insert River job
//         job, err := river.Insert(r.Context(), jobs.OnboardOrgArgs{
//             OrgName: req.OrgName,
//             // ...
//         }, nil)
//         if err != nil {
//             encode(w, http.StatusInternalServerError, ErrorResponse{Error: "failed to queue job"})
//             return
//         }
//
//         encode(w, http.StatusAccepted, CreateOrgResponse{
//             JobID:  job.ID,
//             Status: "queued",
//         })
//     })
// }
```

---

## internal/httpapi/middleware.go

```go
package httpapi

import (
 "context"
 "crypto/rand"
 "encoding/hex"
 "log/slog"
 "net/http"
 "time"
)

type ctxKey string

const requestIDKey ctxKey = "request_id"

// requestID adds a unique ID to context and response header.
func requestID(next http.Handler) http.Handler {
 return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
  id := r.Header.Get("X-Request-ID")
  if id == "" {
   id = generateID()
  }

  ctx := context.WithValue(r.Context(), requestIDKey, id)
  w.Header().Set("X-Request-ID", id)
  next.ServeHTTP(w, r.WithContext(ctx))
 })
}

// RequestID retrieves the request ID from context.
func RequestID(ctx context.Context) string {
 if id, ok := ctx.Value(requestIDKey).(string); ok {
  return id
 }
 return ""
}

func generateID() string {
 b := make([]byte, 8)
 rand.Read(b)
 return hex.EncodeToString(b)
}

// requestLogger logs each request with duration.
func requestLogger(next http.Handler, logger *slog.Logger) http.Handler {
 return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
  start := time.Now()

  wrapped := &statusWriter{ResponseWriter: w, status: http.StatusOK}
  next.ServeHTTP(wrapped, r)

  logger.Info("request",
   "method", r.Method,
   "path", r.URL.Path,
   "status", wrapped.status,
   "duration_ms", time.Since(start).Milliseconds(),
   "request_id", RequestID(r.Context()),
  )
 })
}

type statusWriter struct {
 http.ResponseWriter
 status int
}

func (w *statusWriter) WriteHeader(status int) {
 w.status = status
 w.ResponseWriter.WriteHeader(status)
}

// panicRecovery catches panics and returns 500.
func panicRecovery(next http.Handler, logger *slog.Logger) http.Handler {
 return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
  defer func() {
   if err := recover(); err != nil {
    logger.Error("panic recovered",
     "err", err,
     "request_id", RequestID(r.Context()),
     "path", r.URL.Path,
    )
    http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
   }
  }()
  next.ServeHTTP(w, r)
 })
}

// apiKeyAuth returns middleware that validates API key header.
func apiKeyAuth(expectedKey string) func(http.Handler) http.Handler {
 return func(next http.Handler) http.Handler {
  return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
   if expectedKey == "" {
    // No key configured - reject all
    http.Error(w, "unauthorized", http.StatusUnauthorized)
    return
   }

   key := r.Header.Get("X-API-Key")
   if key == "" || key != expectedKey {
    http.Error(w, "unauthorized", http.StatusUnauthorized)
    return
   }
   next.ServeHTTP(w, r)
  })
 }
}

// hookAuth returns middleware for per-hook secret validation.
func hookAuth(secrets map[string]string) func(http.Handler) http.Handler {
 return func(next http.Handler) http.Handler {
  return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
   hookID := r.Header.Get("X-Webhook-ID")
   secret := r.Header.Get("X-Webhook-Secret")

   expected, ok := secrets[hookID]
   if !ok || secret != expected {
    http.Error(w, "unauthorized", http.StatusUnauthorized)
    return
   }
   next.ServeHTTP(w, r)
  })
 }
}
```

---

## internal/httpapi/encode.go

```go
package httpapi

import (
 "context"
 "encoding/json"
 "fmt"
 "net/http"
)

func encode[T any](w http.ResponseWriter, status int, v T) error {
 w.Header().Set("Content-Type", "application/json")
 w.WriteHeader(status)
 if err := json.NewEncoder(w).Encode(v); err != nil {
  return fmt.Errorf("encode json: %w", err)
 }
 return nil
}

func decode[T any](r *http.Request) (T, error) {
 var v T
 if err := json.NewDecoder(r.Body).Decode(&v); err != nil {
  return v, fmt.Errorf("decode json: %w", err)
 }
 return v, nil
}

// Validator is implemented by request types that can self-validate.
type Validator interface {
 Valid(ctx context.Context) map[string]string
}

// decodeValid decodes and validates in one call.
func decodeValid[T Validator](r *http.Request) (T, map[string]string, error) {
 var v T
 if err := json.NewDecoder(r.Body).Decode(&v); err != nil {
  return v, nil, fmt.Errorf("decode json: %w", err)
 }
 if problems := v.Valid(r.Context()); len(problems) > 0 {
  return v, problems, fmt.Errorf("invalid %T", v)
 }
 return v, nil, nil
}
```

---

## internal/httpapi/types.go

```go
package httpapi

import "context"

// --- Health ---

type ReadyResponse struct {
 Status string `json:"status"`
}

type HealthResponse struct {
 Status    string `json:"status"`
 Version   string `json:"version"`
 Commit    string `json:"commit"`
 BuildTime string `json:"build_time"`
}

// --- Echo ---

type EchoRequest struct {
 Message string `json:"message"`
}

func (r EchoRequest) Valid(ctx context.Context) map[string]string {
 problems := make(map[string]string)
 if r.Message == "" {
  problems["message"] = "required"
 }
 if len(r.Message) > 1000 {
  problems["message"] = "max 1000 characters"
 }
 return problems
}

type EchoResponse struct {
 Echo      string `json:"echo"`
 RequestID string `json:"request_id"`
}

// --- Errors ---

type ErrorResponse struct {
 Error    string            `json:"error"`
 Problems map[string]string `json:"problems,omitempty"`
}

// --- Future: Org ---
//
// type CreateOrgRequest struct {
//     OrgName string `json:"org_name"`
//     // ...
// }
//
// func (r CreateOrgRequest) Valid(ctx context.Context) map[string]string {
//     problems := make(map[string]string)
//     if r.OrgName == "" {
//         problems["org_name"] = "required"
//     }
//     return problems
// }
//
// type CreateOrgResponse struct {
//     JobID  int64  `json:"job_id"`
//     Status string `json:"status"`
// }
```

---

## internal/httpapi/middleware_test.go

```go
package httpapi

import (
 "bytes"
 "io"
 "log/slog"
 "net/http"
 "net/http/httptest"
 "testing"
)

func TestRequestID(t *testing.T) {
 t.Parallel()

 t.Run("generates ID when missing", func(t *testing.T) {
  t.Parallel()

  var capturedID string
  inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
   capturedID = RequestID(r.Context())
  })

  handler := requestID(inner)
  req := httptest.NewRequest(http.MethodGet, "/", nil)
  rec := httptest.NewRecorder()

  handler.ServeHTTP(rec, req)

  if capturedID == "" {
   t.Error("expected request ID in context")
  }
  if rec.Header().Get("X-Request-ID") == "" {
   t.Error("expected X-Request-ID header")
  }
  if rec.Header().Get("X-Request-ID") != capturedID {
   t.Error("header and context ID mismatch")
  }
 })

 t.Run("preserves provided ID", func(t *testing.T) {
  t.Parallel()

  var capturedID string
  inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
   capturedID = RequestID(r.Context())
  })

  handler := requestID(inner)
  req := httptest.NewRequest(http.MethodGet, "/", nil)
  req.Header.Set("X-Request-ID", "trace-123")
  rec := httptest.NewRecorder()

  handler.ServeHTTP(rec, req)

  if capturedID != "trace-123" {
   t.Errorf("got %q, want %q", capturedID, "trace-123")
  }
 })
}

func TestPanicRecovery(t *testing.T) {
 t.Parallel()

 inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
  panic("boom")
 })

 logger := slog.New(slog.NewTextHandler(io.Discard, nil))
 handler := panicRecovery(inner, logger)

 req := httptest.NewRequest(http.MethodGet, "/", nil)
 rec := httptest.NewRecorder()

 handler.ServeHTTP(rec, req)

 if rec.Code != http.StatusInternalServerError {
  t.Errorf("got %d, want 500", rec.Code)
 }
}

func TestAPIKeyAuth(t *testing.T) {
 t.Parallel()

 inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
  w.WriteHeader(http.StatusOK)
 })

 middleware := apiKeyAuth("secret")
 handler := middleware(inner)

 tests := []struct {
  name       string
  key        string
  wantStatus int
 }{
  {"valid", "secret", http.StatusOK},
  {"invalid", "wrong", http.StatusUnauthorized},
  {"missing", "", http.StatusUnauthorized},
 }

 for _, tt := range tests {
  t.Run(tt.name, func(t *testing.T) {
   t.Parallel()

   req := httptest.NewRequest(http.MethodGet, "/", nil)
   if tt.key != "" {
    req.Header.Set("X-API-Key", tt.key)
   }
   rec := httptest.NewRecorder()

   handler.ServeHTTP(rec, req)

   if rec.Code != tt.wantStatus {
    t.Errorf("got %d, want %d", rec.Code, tt.wantStatus)
   }
  })
 }
}

func TestRequestLogger(t *testing.T) {
 t.Parallel()

 var buf bytes.Buffer
 logger := slog.New(slog.NewTextHandler(&buf, nil))

 inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
  w.WriteHeader(http.StatusCreated)
 })

 handler := requestLogger(inner, logger)
 req := httptest.NewRequest(http.MethodPost, "/test", nil)
 rec := httptest.NewRecorder()

 handler.ServeHTTP(rec, req)

 if !bytes.Contains(buf.Bytes(), []byte("POST")) {
  t.Error("expected method in log")
 }
 if !bytes.Contains(buf.Bytes(), []byte("/test")) {
  t.Error("expected path in log")
 }
}
```

---

## internal/httpapi/handlers_test.go

```go
package httpapi

import (
 "bytes"
 "encoding/json"
 "io"
 "log/slog"
 "net/http"
 "net/http/httptest"
 "testing"

 "github.com/CloudKey-io/hbs-queue/internal/config"
)

func TestHandleReady(t *testing.T) {
 t.Parallel()

 handler := handleReady()

 req := httptest.NewRequest(http.MethodGet, "/ready", nil)
 rec := httptest.NewRecorder()

 handler.ServeHTTP(rec, req)

 if rec.Code != http.StatusOK {
  t.Errorf("got %d, want 200", rec.Code)
 }

 var resp ReadyResponse
 json.NewDecoder(rec.Body).Decode(&resp)
 if resp.Status != "ok" {
  t.Errorf("got %q, want %q", resp.Status, "ok")
 }
}

func TestHandleHealth(t *testing.T) {
 t.Parallel()

 cfg := &config.Config{
  Version:   "1.0.0",
  Commit:    "abc123",
  BuildTime: "2024-01-01",
 }

 handler := handleHealth(cfg)

 req := httptest.NewRequest(http.MethodGet, "/health", nil)
 rec := httptest.NewRecorder()

 handler.ServeHTTP(rec, req)

 if rec.Code != http.StatusOK {
  t.Errorf("got %d, want 200", rec.Code)
 }

 var resp HealthResponse
 json.NewDecoder(rec.Body).Decode(&resp)

 if resp.Version != "1.0.0" {
  t.Errorf("got version %q, want %q", resp.Version, "1.0.0")
 }
}

func TestHandleEcho(t *testing.T) {
 t.Parallel()

 logger := slog.New(slog.NewTextHandler(io.Discard, nil))

 tests := []struct {
  name       string
  body       string
  wantStatus int
  wantEcho   string
 }{
  {
   name:       "valid",
   body:       `{"message":"hello"}`,
   wantStatus: http.StatusOK,
   wantEcho:   "hello",
  },
  {
   name:       "empty message",
   body:       `{"message":""}`,
   wantStatus: http.StatusUnprocessableEntity,
  },
  {
   name:       "invalid json",
   body:       `{bad`,
   wantStatus: http.StatusBadRequest,
  },
 }

 for _, tt := range tests {
  t.Run(tt.name, func(t *testing.T) {
   t.Parallel()

   handler := handleEcho(logger)

   req := httptest.NewRequest(http.MethodPost, "/api/v1/echo",
    bytes.NewBufferString(tt.body))
   req.Header.Set("Content-Type", "application/json")
   rec := httptest.NewRecorder()

   handler.ServeHTTP(rec, req)

   if rec.Code != tt.wantStatus {
    t.Errorf("got %d, want %d", rec.Code, tt.wantStatus)
   }

   if tt.wantEcho != "" {
    var resp EchoResponse
    json.NewDecoder(rec.Body).Decode(&resp)
    if resp.Echo != tt.wantEcho {
     t.Errorf("got %q, want %q", resp.Echo, tt.wantEcho)
    }
   }
  })
 }
}
```

---

## .envrc.example

```bash
# Copy to .envrc, then: direnv allow
export PORT=8080
export ENV=dev
export API_KEY=dev-secret-key

# Future
# export DATABASE_URL=postgres://localhost:5432/hbsqueue_dev?sslmode=disable
```

---

## .golangci.yml

```yaml
run:
  timeout: 5m

linters:
  enable:
    - errcheck
    - govet
    - ineffassign
    - staticcheck
    - unused
    - gosimple
    - gofmt
    - goimports
    - misspell
    - unconvert

linters-settings:
  goimports:
    local-prefixes: github.com/CloudKey-io/hbs-queue

issues:
  exclude-use-default: false
```

---

## .github/workflows/ci.yml

```yaml
name: CI

on:
  push:
    branches: [main]
  pull_request:
    branches: [main]

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - uses: actions/setup-go@v5
        with:
          go-version: "1.22"

      - name: Lint
        uses: golangci/golangci-lint-action@v4
        with:
          version: latest

      - name: Test
        run: go test -v -race -coverprofile=coverage.out ./...

      - name: Upload coverage
        uses: codecov/codecov-action@v4
        with:
          files: coverage.out
        continue-on-error: true
```

---

## .gitignore

```gitignore
# Binaries
bin/
*.exe

# Environment
.envrc
.env
*.local

# IDE
.idea/
.vscode/
*.swp

# Test
coverage.out

# OS
.DS_Store
Thumbs.db
```

---

## README.md

````markdown
# hbs-queue

HostBill Queue Service - orchestrates org onboarding/deboarding via River jobs.

## Quick Start

```bash
# Setup env
cp .envrc.example .envrc
direnv allow  # or: source .envrc

# Run
make run

# Test
curl localhost:8080/ready
curl localhost:8080/health

# Echo (requires API key)
curl -X POST localhost:8080/api/v1/echo \
  -H "Content-Type: application/json" \
  -H "X-API-Key: dev-secret-key" \
  -d '{"message": "hello"}'
```
````

## Development

```bash
make test   # Run tests
make lint   # Run linter
make build  # Build binary with version info
```

## Architecture

- `cmd/hbsqueue/main.go` - `main()` calls `run()` with OS primitives
- `internal/config/` - Config loaded from `getenv` function
- `internal/httpapi/` - HTTP server, routes, handlers, middleware
- `internal/jobs/` - (future) River job workers
- `internal/clients/` - (future) VCD, Zerto, etc.

````

---

## Test It

```bash
# Terminal 1
cp .envrc.example .envrc
source .envrc  # or: direnv allow
make run

# Terminal 2
curl -i localhost:8080/ready
curl -i localhost:8080/health
curl -i -X POST localhost:8080/api/v1/echo \
  -H "Content-Type: application/json" \
  -H "X-API-Key: dev-secret-key" \
  -d '{"message": "hello"}'

# Run tests
make test
````

---
