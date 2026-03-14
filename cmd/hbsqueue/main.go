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

	"github.com/CloudKey-io/hbs-queue/internal/config"
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

	logLevel := slog.LevelInfo
	if cfg.Env == "dev" {
		logLevel = slog.LevelDebug
	}
	logger := slog.New(slog.NewJSONHandler(stdout, &slog.HandlerOptions{Level: logLevel}))

	srv := httpapi.NewServer(logger, cfg)

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

	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("http shutdown: %w", err)
	}

	logger.Info("shutdown complete")
	return nil
}
