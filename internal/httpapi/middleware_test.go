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

	t.Run("no key configured rejects all", func(t *testing.T) {
		t.Parallel()

		noKey := apiKeyAuth("")
		h := noKey(inner)

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("X-API-Key", "anything")
		rec := httptest.NewRecorder()

		h.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Errorf("got %d, want 401", rec.Code)
		}
	})
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
