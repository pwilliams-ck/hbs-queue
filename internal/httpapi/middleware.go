package httpapi

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"time"
)

type ctxKey string

const requestIDKey ctxKey = "request_id"

// requestID injects a unique identifier into the request context and the
// X-Request-ID response header. If the caller supplies X-Request-ID it is
// preserved; otherwise a random hex string is generated.
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
	b := make([]byte, 16)
	_, _ = rand.Read(b) // crypto/rand.Read never errors in Go 1.20+
	return hex.EncodeToString(b)
}

// requestLogger logs each completed request with method, path, status,
// and duration.
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

// panicRecovery catches panics in downstream handlers and returns 500.
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

// devCORS allows cross-origin requests in dev mode so Swagger UI
// (localhost:8081) can reach the API (localhost:8080). Only applied
// when ENV=dev; production traffic is same-origin and doesn't need this.
func devCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-API-Key, X-Request-ID")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// apiKeyAuth returns middleware that rejects requests without a valid
// X-API-Key header. Comparison is constant-time.
func apiKeyAuth(expectedKey string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if expectedKey == "" {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}

			key := r.Header.Get("X-API-Key")
			if subtle.ConstantTimeCompare([]byte(key), []byte(expectedKey)) != 1 {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// webhookTimestampTolerance is the maximum age of a HostBill webhook
// request before it is rejected. Matches the HostBill documentation
// recommendation of 60 seconds.
const webhookTimestampTolerance = 60 * time.Second

// webhookAuth returns middleware that validates HostBill webhook signatures.
// Each hook endpoint has its own secret. The signature is HMAC-SHA256 over
// the concatenation of the HB-Timestamp header value and the raw request body.
//
// HostBill headers:
//
//	HB-Hook       — system ID for the webhook (informational)
//	HB-Event      — event name, e.g. "after_clientadded"
//	HB-Timestamp  — Unix epoch seconds as a string
//	HB-Signature  — hex-encoded HMAC-SHA256 signature
//
// The request body is buffered and re-injected so downstream handlers
// can still read it.
func webhookAuth(secret string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if secret == "" {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}

			tsHeader := r.Header.Get("HB-Timestamp")
			sigHeader := r.Header.Get("HB-Signature")
			if tsHeader == "" || sigHeader == "" {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}

			// Validate timestamp freshness.
			ts, err := strconv.ParseInt(tsHeader, 10, 64)
			if err != nil {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			if time.Since(time.Unix(ts, 0)) > webhookTimestampTolerance {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}

			// Buffer the body so it can be re-read by downstream handlers.
			body, err := io.ReadAll(r.Body)
			_ = r.Body.Close()
			if err != nil {
				http.Error(w, "bad request", http.StatusBadRequest)
				return
			}
			r.Body = io.NopCloser(bytes.NewReader(body))

			// Compute HMAC-SHA256 over timestamp + body.
			mac := hmac.New(sha256.New, []byte(secret))
			mac.Write([]byte(tsHeader))
			mac.Write(body)
			expected := hex.EncodeToString(mac.Sum(nil))

			if subtle.ConstantTimeCompare([]byte(sigHeader), []byte(expected)) != 1 {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
