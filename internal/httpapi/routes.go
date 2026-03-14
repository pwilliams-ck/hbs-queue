// Route surface:
//
//	GET  /ready         — readiness probe (no auth)
//	GET  /health        — build info (no auth)
//	POST /api/v1/echo   — echo test (API key auth)
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
) {
	withAPIKey := apiKeyAuth(cfg.APIKey)

	// Health — no auth
	mux.Handle("GET /ready", handleReady())
	mux.Handle("GET /health", handleHealth(cfg))

	// API — API key auth
	mux.Handle("POST /api/v1/echo", withAPIKey(handleEcho(logger)))
}
