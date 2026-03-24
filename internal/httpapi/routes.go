// Package httpapi Route surface:
//
//	GET  /ready                         — readiness probe, pings DB (no auth)
//	GET  /health                        — build info + DB status (no auth)
//	POST /api/v1/echo                   — echo test (API key auth)
//	POST /api/v1/script/onboard-org     — enqueue tenant onboarding (API key auth)
//	POST /hooks/deboard-org             — deboard tenant (webhook HMAC auth)
//	POST /hooks/onboard-contact         — add contact (webhook HMAC auth)
//	POST /hooks/deboard-contact         — remove contact (webhook HMAC auth)
//	POST /hooks/update-pw               — password change (webhook HMAC auth)
//	POST /hooks/update-bandwidth        — bandwidth update (webhook HMAC auth)
package httpapi

import (
	"log/slog"
	"net/http"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"

	"github.com/CloudKey-io/hbs-queue/internal/clients/vcd"
	"github.com/CloudKey-io/hbs-queue/internal/config"
)

func addRoutes(
	mux *http.ServeMux,
	logger *slog.Logger,
	cfg *config.Config,
	pool *pgxpool.Pool,
	_ *river.Client[pgx.Tx],
	vcdClient *vcd.Client,
) {
	withAPIKey := apiKeyAuth(cfg.APIKey)

	// Health — no auth
	mux.Handle("GET /ready", handleReady(pool))
	mux.Handle("GET /health", handleHealth(cfg, pool))

	// API — API key auth
	mux.Handle("POST /api/v1/echo", withAPIKey(handleEcho(logger)))
	mux.Handle("POST /api/v1/script/onboard-org", withAPIKey(handleOnboardOrg(logger, vcdClient)))

	// Hooks — per-endpoint webhook HMAC auth
	mux.Handle("POST /hooks/deboard-org", webhookAuth(cfg.Hooks.DeboardOrg)(handleDeboardOrg()))
	mux.Handle("POST /hooks/onboard-contact", webhookAuth(cfg.Hooks.OnboardContact)(handleOnboardContact()))
	mux.Handle("POST /hooks/deboard-contact", webhookAuth(cfg.Hooks.DeboardContact)(handleDeboardContact()))
	mux.Handle("POST /hooks/update-pw", webhookAuth(cfg.Hooks.UpdatePW)(handlePWChange()))
	mux.Handle("POST /hooks/update-bandwidth", webhookAuth(cfg.Hooks.UpdateBandwidth)(handleBandwidthUpdate()))
}
