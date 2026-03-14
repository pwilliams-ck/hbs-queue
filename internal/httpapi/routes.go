// Route surface:
//
//	GET  /ready         — readiness probe, pings DB (no auth)
//	GET  /health        — build info + DB status (no auth)
//	POST /api/v1/echo   — echo test (API key auth)
package httpapi

import (
	"log/slog"
	"net/http"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"

	"github.com/CloudKey-io/hbs-queue/internal/config"
)

func addRoutes(
	mux *http.ServeMux,
	logger *slog.Logger,
	cfg *config.Config,
	pool *pgxpool.Pool,
	_ *river.Client[pgx.Tx],
) {
	withAPIKey := apiKeyAuth(cfg.APIKey)

	// Health — no auth
	mux.Handle("GET /ready", handleReady(pool))
	mux.Handle("GET /health", handleHealth(cfg, pool))

	// API — API key auth
	mux.Handle("POST /api/v1/echo", withAPIKey(handleEcho(logger)))
}
