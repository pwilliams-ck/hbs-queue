package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/CloudKey-io/hbs-queue/internal/httpapi"
)

func TestRun(t *testing.T) {
	t.Parallel()

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}

	port := freePort(t)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	debugPort := freePort(t)

	getenv := mockGetenv(map[string]string{
		"PORT":         port,
		"ENV":          "test",
		"API_KEY":      "test-key",
		"DATABASE_URL": databaseURL,
		"DEBUG_PORT":   debugPort,
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

	t.Run("health returns version and db status", func(t *testing.T) {
		resp, err := http.Get(baseURL + "/health")
		if err != nil {
			t.Fatalf("GET /health: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("got status %d, want 200", resp.StatusCode)
		}

		var body httpapi.HealthResponse
		json.NewDecoder(resp.Body).Decode(&body)
		if body.Database != "up" {
			t.Errorf("got database %q, want %q", body.Database, "up")
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

	// Stub handlers return 501 Not Implemented.
	t.Run("script onboard-org requires API key", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodPost, baseURL+"/api/v1/script/onboard-org", nil)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("request: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("got status %d, want 401", resp.StatusCode)
		}
	})

	t.Run("script onboard-org returns 503 without vcd client", func(t *testing.T) {
		body := `{"crm_id":"167","client_first_name":"Test","client_last_name":"User","client_email":"test@example.com","account_id":1,"bandwidth":"100"}`
		req, _ := http.NewRequest(http.MethodPost, baseURL+"/api/v1/script/onboard-org",
			bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-API-Key", "test-key")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("request: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusServiceUnavailable {
			t.Errorf("got status %d, want 503", resp.StatusCode)
		}
	})

	t.Run("hooks require webhook auth", func(t *testing.T) {
		paths := []string{
			"/hooks/deboard-org",
			"/hooks/onboard-contact",
			"/hooks/deboard-contact",
			"/hooks/update-pw",
			"/hooks/update-bandwidth",
		}
		for _, path := range paths {
			req, _ := http.NewRequest(http.MethodPost, baseURL+path, nil)
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatalf("POST %s: %v", path, err)
			}
			resp.Body.Close()

			// No webhook secrets configured → 401.
			if resp.StatusCode != http.StatusUnauthorized {
				t.Errorf("POST %s: got status %d, want 401", path, resp.StatusCode)
			}
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

func mockGetenv(m map[string]string) func(string) string {
	return func(key string) string {
		return m[key]
	}
}

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
