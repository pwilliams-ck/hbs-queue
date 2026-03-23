// Package workflow implements the step-based workflow engine for hbs-queue.
// A workflow is a sequence of steps that execute in order, with each step's
// results accumulated in a JSONB column. The engine supports crash-safe
// resume: on retry, it picks up from the last completed step.
package workflow

import (
	"context"
	"errors"
)

// Step is a single unit of work within a workflow. Implementations must
// be idempotent — the runner may call Run again for the same step if
// the worker is retried after a crash.
//
// Accumulator contract:
//   - Steps read what they need from state.Data using GetString/GetInt.
//   - Steps write their results using Set before returning nil.
//   - The runner persists state.Data after every successful step.
//
// Error contract:
//   - Return nil on success.
//   - Return a *PermanentError to signal a non-retryable failure. The runner
//     marks the workflow as failed and tells River to discard the job.
//   - Return any other error to signal a retryable failure. River will retry
//     the job; the runner resumes at the current step.
type Step interface {
	// Name returns a stable, human-readable identifier used in logs.
	Name() string

	// Run executes the step. It may mutate state.Data to record output.
	// It must not mutate CurrentStep or Status — the runner owns those.
	Run(ctx context.Context, state *WorkflowState) error
}

// PermanentError wraps an underlying error and signals that the job
// should not be retried.
type PermanentError struct {
	Cause error
}

func (e *PermanentError) Error() string { return "permanent: " + e.Cause.Error() }
func (e *PermanentError) Unwrap() error { return e.Cause }

// IsPermanent reports whether err is or wraps a *PermanentError.
func IsPermanent(err error) bool {
	var pe *PermanentError
	return errors.As(err, &pe)
}
