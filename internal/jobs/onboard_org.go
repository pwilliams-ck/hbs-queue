package jobs

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"

	"github.com/CloudKey-io/hbs-queue/internal/clients/vcd"
	"github.com/CloudKey-io/hbs-queue/internal/workflow"
)

// OnboardOrgWorker processes onboard_customer jobs enqueued by
// POST /api/v1/script/onboard-org. It uses the workflow runner to
// execute steps in order, resuming from current_step on retry.
//
// Each step is independently idempotent — it checks the actual state of
// the external system it talks to and either performs the operation or
// skips if already done. This means the same workflow handles both new
// and existing customers without branching.
//
// Step sequence:
//
//	Step 0: check_org       — look up org in VCD, store metadata in accumulator
//	Step 1: network_config  — configure VDC default network (always runs)
//	Step 2: saml_config     — check if Keycloak SAML client exists; skip if yes, create if no
//	Step 3: zerto_setup     — check if Zerto org exists; update resources if yes, create if no
//	Step 4: ldap_update     — check if LDAP attrs set; skip if yes, add if no
//	Step 5: keycloak_sync   — trigger Keycloak user sync (always safe to run)
//	Step 6: vapp_template   — provision vApp if product_id matches a configured template
//
// Steps are stubs in Task 4; real implementations land in Tasks 5-7.
type OnboardOrgWorker struct {
	river.WorkerDefaults[OnboardOrgArgs]
	pool      *pgxpool.Pool
	repo      workflow.Repository
	vcdClient *vcd.Client
	logger    *slog.Logger
}

// NewOnboardOrgWorker creates an OnboardOrgWorker.
func NewOnboardOrgWorker(pool *pgxpool.Pool, repo workflow.Repository, vcdClient *vcd.Client, logger *slog.Logger) *OnboardOrgWorker {
	return &OnboardOrgWorker{
		pool:      pool,
		repo:      repo,
		vcdClient: vcdClient,
		logger:    logger,
	}
}

// Work processes a single onboard_customer job. It opens a transaction,
// runs the workflow steps, and commits on success.
func (w *OnboardOrgWorker) Work(ctx context.Context, job *river.Job[OnboardOrgArgs]) error {
	tx, err := w.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // rollback after commit is a no-op

	// Convert job args to initial JSONB data.
	initialData, err := argsToData(job.Args)
	if err != nil {
		return fmt.Errorf("marshal args: %w", err)
	}

	steps := []workflow.Step{
		&checkOrgStep{},
		&networkConfigStep{},
		&samlConfigStep{},
		&zertoSetupStep{},
		&ldapUpdateStep{},
		&keycloakSyncStep{},
		&vappTemplateStep{},
	}

	runner := workflow.NewRunner(w.repo, steps, w.logger)
	if err := runner.Run(ctx, tx, job.ID, "onboard_customer", job.Args.ClientID, initialData); err != nil {
		return err
	}

	return tx.Commit(ctx)
}

// argsToData marshals job args into the initial JSONB accumulator.
func argsToData(args OnboardOrgArgs) (map[string]json.RawMessage, error) {
	raw, err := json.Marshal(args)
	if err != nil {
		return nil, err
	}
	var data map[string]json.RawMessage
	if err := json.Unmarshal(raw, &data); err != nil {
		return nil, err
	}
	return data, nil
}

// Stub steps — each returns nil (no-op). Real implementations will be
// added in Tasks 5-7 when the external API clients are built.

type checkOrgStep struct{}

func (s *checkOrgStep) Name() string                                           { return "check_org" }
func (s *checkOrgStep) Run(_ context.Context, _ *workflow.WorkflowState) error { return nil }

type networkConfigStep struct{}

func (s *networkConfigStep) Name() string                                           { return "network_config" }
func (s *networkConfigStep) Run(_ context.Context, _ *workflow.WorkflowState) error { return nil }

type samlConfigStep struct{}

func (s *samlConfigStep) Name() string                                           { return "saml_config" }
func (s *samlConfigStep) Run(_ context.Context, _ *workflow.WorkflowState) error { return nil }

type zertoSetupStep struct{}

func (s *zertoSetupStep) Name() string                                           { return "zerto_setup" }
func (s *zertoSetupStep) Run(_ context.Context, _ *workflow.WorkflowState) error { return nil }

type ldapUpdateStep struct{}

func (s *ldapUpdateStep) Name() string                                           { return "ldap_update" }
func (s *ldapUpdateStep) Run(_ context.Context, _ *workflow.WorkflowState) error { return nil }

type keycloakSyncStep struct{}

func (s *keycloakSyncStep) Name() string                                           { return "keycloak_sync" }
func (s *keycloakSyncStep) Run(_ context.Context, _ *workflow.WorkflowState) error { return nil }

type vappTemplateStep struct{}

func (s *vappTemplateStep) Name() string                                           { return "vapp_template" }
func (s *vappTemplateStep) Run(_ context.Context, _ *workflow.WorkflowState) error { return nil }
