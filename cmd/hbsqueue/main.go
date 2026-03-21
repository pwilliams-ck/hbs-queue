// hbsqueue is the HostBill Queue Service. It orchestrates customer
// onboarding and offboarding workflows across VCD, Zerto, Keycloak,
// HostBill, and Active Directory using River as a Postgres-backed job queue.
package main

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/riverqueue/river"

	"github.com/CloudKey-io/hbs-queue/internal/config"
	"github.com/CloudKey-io/hbs-queue/internal/db"
	"github.com/CloudKey-io/hbs-queue/internal/httpapi"
)

func main() {
	ctx := context.Background()
	if err := run(ctx, os.Args, os.Getenv, os.Stdout, os.Stderr); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

// run is the real entry point. It accepts explicit dependencies so the
// entire server lifecycle — startup, serve, shutdown — is testable.
func run(
	ctx context.Context,
	args []string,
	getenv func(string) string,
	stdout, stderr io.Writer,
) error {
	ctx, cancel := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer cancel()

	cfg := config.Load(getenv)

	opts := &slog.HandlerOptions{Level: slog.LevelInfo}
	var handler slog.Handler = slog.NewJSONHandler(stdout, opts)
	if cfg.Env == "dev" {
		opts.Level = slog.LevelDebug
		handler = slog.NewTextHandler(stdout, opts)
	}
	logger := slog.New(handler)

	// Database
	pool, err := db.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("db pool: %w", err)
	}
	defer pool.Close()

	// Migrations — River's own tables, then our app schema.
	if err := db.MigrateRiver(ctx, pool, logger); err != nil {
		return fmt.Errorf("river migrations: %w", err)
	}
	if err := db.MigrateUp(ctx, pool, logger); err != nil {
		return fmt.Errorf("app migrations: %w", err)
	}

	// River client — no workers registered yet, will be added in Task 5.
	workers := river.NewWorkers()
	riverClient, err := db.NewRiverClient(pool, workers)
	if err != nil {
		return fmt.Errorf("river client: %w", err)
	}

	// Server
	srv := httpapi.NewServer(logger, cfg, pool, riverClient)

	httpServer := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           srv,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	// Start listener in a goroutine and surface bind errors immediately.
	serverErr := make(chan error, 1)
	go func() {
		logger.Info("server starting", "addr", httpServer.Addr, "env", cfg.Env)
		serverErr <- httpServer.ListenAndServe()
	}()

	// Block until shutdown signal or server failure.
	select {
	case err := <-serverErr:
		if err != nil && err != http.ErrServerClosed {
			return fmt.Errorf("http server: %w", err)
		}
	case <-ctx.Done():
	}

	logger.Info("shutdown signal received")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	// Shutdown order matters:
	// 1. HTTP — stop accepting new requests, drain in-flight
	// 2. River — stop fetching jobs, let active jobs finish
	// 3. Pool — closed via defer above after everything else is done
	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		logger.Error("http shutdown error", "err", err)
	}

	if err := riverClient.Stop(shutdownCtx); err != nil {
		logger.Error("river shutdown error", "err", err)
	}

	logger.Info("shutdown complete")
	return nil
}
