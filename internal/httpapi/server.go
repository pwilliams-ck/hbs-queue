// Package httpapi implements the HTTP layer for hbs-queue.
package httpapi

import (
	"log/slog"
	"net/http"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"

	"github.com/CloudKey-io/hbs-queue/internal/config"
)

// NewServer returns an http.Handler with all routes registered and
// global middleware applied. Middleware executes in this order:
// requestID → requestLogger → panicRecovery → route handler.
func NewServer(
	logger *slog.Logger,
	cfg *config.Config,
	pool *pgxpool.Pool,
	riverClient *river.Client[pgx.Tx],
) http.Handler {
	mux := http.NewServeMux()

	addRoutes(mux, logger, cfg, pool, riverClient)

	var handler http.Handler = mux
	handler = panicRecovery(handler, logger)
	handler = requestLogger(handler, logger)
	handler = requestID(handler)

	return handler
}
