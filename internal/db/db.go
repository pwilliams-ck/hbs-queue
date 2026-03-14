// Package db provides database connectivity for hbs-queue.
package db

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// NewPool creates a connection pool to Postgres. The pool is configured
// for a small service that spends most of its time waiting on external
// APIs rather than saturating the database.
func NewPool(ctx context.Context, databaseURL string) (*pgxpool.Pool, error) {
	config, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("parse database url: %w", err)
	}

	// Keep the pool small — River workers are I/O bound on external
	// HTTP calls, not DB queries.
	config.MaxConns = 10
	config.MinConns = 2

	// Don't hold idle connections open longer than necessary.
	config.MaxConnIdleTime = 5 * time.Minute
	config.MaxConnLifetime = 30 * time.Minute

	// Fail fast on startup rather than blocking indefinitely.
	config.HealthCheckPeriod = 30 * time.Second

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("create pool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}

	return pool, nil
}
