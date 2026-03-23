package jobs

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"

	"github.com/CloudKey-io/hbs-queue/internal/workflow"
)

// OnboardOrgWorker processes onboard_customer jobs enqueued by
// POST /api/v1/script/onboard-org. It uses the workflow runner to
// execute steps in order, resuming from current_step on retry.
//
// Step sequence:
//
//	Step 0: check_tenant    — determine if org is new or existing in VCD
//	Step 1: network_config  — configure virtual DC and default network
//	Step 2: saml_config     — establish SAML federation between VCD and Keycloak
//	Step 3: zerto_setup     — register org in Zerto, configure limits
//	Step 4: ldap_update     — add LDAP entry for zorg user
//	Step 5: keycloak_sync   — trigger Keycloak federation sync
//	Step 6: vapp_template   — deploy vApp template to org VDC
//
// Steps are stubs in Task 4; real implementations land in Tasks 5-7.
type OnboardOrgWorker struct {
	river.WorkerDefaults[OnboardOrgArgs]
	pool   *pgxpool.Pool
	repo   workflow.Repository
	logger *slog.Logger
}

// NewOnboardOrgWorker creates an OnboardOrgWorker.
func NewOnboardOrgWorker(pool *pgxpool.Pool, repo workflow.Repository, logger *slog.Logger) *OnboardOrgWorker {
	return &OnboardOrgWorker{
		pool:   pool,
		repo:   repo,
		logger: logger,
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
		&checkTenantStep{},
		&networkConfigStep{},
		&samlConfigStep{},
		&zertoSetupStep{},
		&ldapUpdateStep{},
		&keycloakSyncStep{},
		&vappTemplateStep{},
	}

	runner := workflow.NewRunner(w.repo, steps, w.logger)
	if err := runner.Run(ctx, tx, job.ID, "onboard_customer", job.Args.ClientID, job.Args.OrderID, initialData); err != nil {
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

type checkTenantStep struct{}

func (s *checkTenantStep) Name() string                                           { return "check_tenant" }
func (s *checkTenantStep) Run(_ context.Context, _ *workflow.WorkflowState) error { return nil }

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
