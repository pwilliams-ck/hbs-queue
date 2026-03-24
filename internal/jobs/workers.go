package jobs

import (
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"

	"github.com/CloudKey-io/hbs-queue/internal/clients/vcd"
	"github.com/CloudKey-io/hbs-queue/internal/workflow"
)

// Register creates a *river.Workers registry with all job workers.
// Called from run() before NewRiverClient so workers are available
// when River starts processing.
//
// Registered workers:
//   - OnboardOrgWorker → onboard_customer jobs
//
// Additional workers will be registered as their workflows are
// implemented in Tasks 5-7.
func Register(pool *pgxpool.Pool, repo workflow.Repository, vcdClient *vcd.Client, logger *slog.Logger) *river.Workers {
	workers := river.NewWorkers()
	river.AddWorker(workers, NewOnboardOrgWorker(pool, repo, vcdClient, logger))
	return workers
}
