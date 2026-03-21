package db

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"log/slog"
	"sort"

	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed migrations/*.sql
var migrations embed.FS

// MigrateUp runs all .up.sql files from the embedded migrations directory
// in sorted order. Each file runs in its own implicit transaction. The SQL
// files use IF NOT EXISTS to stay idempotent.
func MigrateUp(ctx context.Context, pool *pgxpool.Pool, logger *slog.Logger) error {
	files, err := fs.Glob(migrations, "migrations/*.up.sql")
	if err != nil {
		return fmt.Errorf("glob migrations: %w", err)
	}
	sort.Strings(files)

	for _, name := range files {
		sql, err := fs.ReadFile(migrations, name)
		if err != nil {
			return fmt.Errorf("read %s: %w", name, err)
		}

		if _, err := pool.Exec(ctx, string(sql)); err != nil {
			return fmt.Errorf("exec %s: %w", name, err)
		}
	}

	logger.Debug("app migrations up to date", "count", len(files))

	return nil
}
