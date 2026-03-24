package workflow

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5"
)

// Runner executes a workflow's steps in order against a Postgres transaction.
//
// Resume semantics: on startup, Runner loads the workflow_state row and
// skips all steps with index < state.CurrentStep (already durably complete).
// Execution begins at state.CurrentStep. If the process crashes mid-step,
// the next retry re-runs that step from scratch — steps MUST be idempotent.
//
// Failure behavior: if a step returns a retryable error, Runner sets
// status = "failed", writes the error text, and returns the error (River
// will schedule a retry). If a step returns a *PermanentError, Runner sets
// status = "failed" and wraps the error for River to discard the job.
type Runner struct {
	repo   Repository
	steps  []Step
	logger *slog.Logger
}

// NewRunner creates a Runner with the given repository and ordered step list.
func NewRunner(repo Repository, steps []Step, logger *slog.Logger) *Runner {
	return &Runner{
		repo:   repo,
		steps:  steps,
		logger: logger,
	}
}

// Run loads or creates the workflow_state row for jobID, then executes
// steps starting from current_step. All state mutations happen within tx
// so they are atomic with the caller's transaction.
func (r *Runner) Run(
	ctx context.Context,
	tx pgx.Tx,
	jobID int64,
	workflowType string,
	clientID string,
	initialData map[string]json.RawMessage,
) error {
	// Load existing state or create a new row.
	state, err := r.repo.Get(ctx, tx, jobID)
	if err != nil {
		if !errors.Is(err, ErrNotFound) {
			return fmt.Errorf("load workflow state: %w", err)
		}

		state = &WorkflowState{
			JobID:        jobID,
			WorkflowType: workflowType,
			ClientID:     clientID,
			Status:       StatusPending,
			Data:         initialData,
		}
		if err := r.repo.Create(ctx, tx, state); err != nil {
			return fmt.Errorf("create workflow state: %w", err)
		}
	}

	// Mark as running.
	state.Status = StatusRunning
	state.Error = ""
	if err := r.repo.UpdateStep(ctx, tx, state); err != nil {
		return fmt.Errorf("update status to running: %w", err)
	}

	// Execute steps from current position.
	for i, step := range r.steps {
		if i < state.CurrentStep {
			continue
		}

		r.logger.Debug("running workflow step",
			"job_id", jobID,
			"workflow_type", workflowType,
			"step", i,
			"step_name", step.Name(),
		)

		if err := step.Run(ctx, state); err != nil {
			state.Status = StatusFailed
			state.Error = err.Error()
			if updateErr := r.repo.UpdateStep(ctx, tx, state); updateErr != nil {
				return fmt.Errorf("update failed state: %w (step error: %w)", updateErr, err)
			}
			return err
		}

		// Advance step counter and persist.
		state.CurrentStep = i + 1
		if err := r.repo.UpdateStep(ctx, tx, state); err != nil {
			return fmt.Errorf("update step %d: %w", i, err)
		}

		r.logger.Debug("workflow step complete",
			"job_id", jobID,
			"step", i,
			"step_name", step.Name(),
		)
	}

	// All steps done.
	state.Status = StatusCompleted
	if err := r.repo.UpdateStep(ctx, tx, state); err != nil {
		return fmt.Errorf("update status to completed: %w", err)
	}

	r.logger.Info("workflow completed",
		"job_id", jobID,
		"workflow_type", workflowType,
		"client_id", clientID,
		"steps", len(r.steps),
	)

	return nil
}
