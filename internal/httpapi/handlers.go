package httpapi

import (
	"log/slog"
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/CloudKey-io/hbs-queue/internal/config"
)

// handleReady returns 200 when the service can accept traffic.
// The DB is pinged on every request — if it fails, the service
// reports not ready so load balancers stop sending traffic.
func handleReady(pool *pgxpool.Pool) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := pool.Ping(r.Context()); err != nil {
			encode(w, r, http.StatusServiceUnavailable, ReadyResponse{Status: "unavailable"})
			return
		}
		encode(w, r, http.StatusOK, ReadyResponse{Status: "ok"})
	})
}

// handleHealth returns build info and database status for diagnostics.
// Build info is computed once at handler creation time; DB status is
// checked per request.
func handleHealth(cfg *config.Config, pool *pgxpool.Pool) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		dbStatus := "up"
		if err := pool.Ping(r.Context()); err != nil {
			dbStatus = "down"
		}

		encode(w, r, http.StatusOK, HealthResponse{
			Status:    "healthy",
			Version:   cfg.Version,
			Commit:    cfg.Commit,
			BuildTime: cfg.BuildTime,
			Database:  dbStatus,
		})
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
