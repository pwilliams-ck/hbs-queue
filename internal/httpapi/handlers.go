package httpapi

import (
	"log/slog"
	"net/http"

	"github.com/CloudKey-io/hbs-queue/internal/config"
)

// handleReady returns 200 when the service can accept traffic.
// Used by load balancers and orchestrators as a readiness probe.
func handleReady() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Future: add pool.Ping(r.Context()) check
		encode(w, r, http.StatusOK, ReadyResponse{Status: "ok"})
	})
}

// handleHealth returns build and version info for diagnostics.
// The response is built once at handler creation time.
func handleHealth(cfg *config.Config) http.Handler {
	resp := HealthResponse{
		Status:    "healthy",
		Version:   cfg.Version,
		Commit:    cfg.Commit,
		BuildTime: cfg.BuildTime,
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		encode(w, r, http.StatusOK, resp)
	})
}

// handleEcho demonstrates the full request lifecycle:
// decode → validate → process → encode.
func handleEcho(logger *slog.Logger) http.Handler {
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
				encode(w, r, http.StatusUnprocessableEntity, ErrorResponse{
					Error:    "validation failed",
					Problems: problems,
				})
				return
			}
			logger.Debug("decode failed", "request_id", reqID, "err", err)
			encode(w, r, http.StatusBadRequest, ErrorResponse{Error: "invalid json"})
			return
		}

		logger.Debug("echo request",
			"request_id", reqID,
			"message", req.Message,
		)

		encode(w, r, http.StatusOK, EchoResponse{
			Echo:      req.Message,
			RequestID: reqID,
		})
	})
}
