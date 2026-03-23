package workflow

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"testing"

	"github.com/jackc/pgx/v5"
)

// stubRepo is a minimal in-memory Repository for testing the Runner
// without a real database. It ignores the pgx.Tx parameter.
type stubRepo struct {
	states map[int64]*WorkflowState
}

func newStubRepo() *stubRepo {
	return &stubRepo{states: make(map[int64]*WorkflowState)}
}

func (r *stubRepo) Create(_ context.Context, _ pgx.Tx, state *WorkflowState) error {
	cp := *state
	cp.ID = "test-uuid"
	// Deep copy data map.
	cp.Data = make(map[string]json.RawMessage, len(state.Data))
	for k, v := range state.Data {
		cp.Data[k] = v
	}
	r.states[state.JobID] = &cp
	state.ID = cp.ID
	return nil
}

func (r *stubRepo) Get(_ context.Context, _ pgx.Tx, jobID int64) (*WorkflowState, error) {
	s, ok := r.states[jobID]
	if !ok {
		return nil, ErrNotFound
	}
	return s, nil
}

func (r *stubRepo) UpdateStep(_ context.Context, _ pgx.Tx, state *WorkflowState) error {
	existing, ok := r.states[state.JobID]
	if !ok {
		return errors.New("not found")
	}
	existing.CurrentStep = state.CurrentStep
	existing.Status = state.Status
	existing.Error = state.Error
	existing.Data = make(map[string]json.RawMessage, len(state.Data))
	for k, v := range state.Data {
		existing.Data[k] = v
	}
	return nil
}

// stubStep is a test Step that records whether it was called.
type stubStep struct {
	name string
	err  error
}

func (s *stubStep) Name() string                                  { return s.name }
func (s *stubStep) Run(_ context.Context, _ *WorkflowState) error { return s.err }

// writingStep writes a value to the accumulator when run.
type writingStep struct {
	name  string
	key   string
	value string
}

func (s *writingStep) Name() string { return s.name }
func (s *writingStep) Run(_ context.Context, state *WorkflowState) error {
	return state.Set(s.key, s.value)
}

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestRunnerAllStepsComplete(t *testing.T) {
	t.Parallel()

	repo := newStubRepo()
	steps := []Step{
		&stubStep{name: "step_0"},
		&stubStep{name: "step_1"},
		&stubStep{name: "step_2"},
	}
	runner := NewRunner(repo, steps, testLogger())

	err := runner.Run(context.Background(), nil, 1, "test_workflow", "client1", "order1", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	state := repo.states[1]
	if state.Status != StatusCompleted {
		t.Errorf("got status %q, want %q", state.Status, StatusCompleted)
	}
	if state.CurrentStep != 3 {
		t.Errorf("got current_step %d, want 3", state.CurrentStep)
	}
}

func TestRunnerResumesFromCurrentStep(t *testing.T) {
	t.Parallel()

	repo := newStubRepo()

	// Pre-populate state as if steps 0 and 1 already completed.
	repo.states[1] = &WorkflowState{
		ID:           "test-uuid",
		JobID:        1,
		WorkflowType: "test_workflow",
		ClientID:     "client1",
		CurrentStep:  2,
		Status:       StatusFailed,
		Data:         make(map[string]json.RawMessage),
	}

	var calledSteps []string
	steps := []Step{
		&stubStep{name: "step_0"},
		&stubStep{name: "step_1"},
		&recordingStep{name: "step_2", called: &calledSteps},
		&recordingStep{name: "step_3", called: &calledSteps},
	}
	runner := NewRunner(repo, steps, testLogger())

	err := runner.Run(context.Background(), nil, 1, "test_workflow", "client1", "", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Only steps 2 and 3 should have run.
	if len(calledSteps) != 2 {
		t.Fatalf("got %d called steps, want 2: %v", len(calledSteps), calledSteps)
	}
	if calledSteps[0] != "step_2" || calledSteps[1] != "step_3" {
		t.Errorf("got steps %v, want [step_2 step_3]", calledSteps)
	}

	state := repo.states[1]
	if state.Status != StatusCompleted {
		t.Errorf("got status %q, want %q", state.Status, StatusCompleted)
	}
	if state.CurrentStep != 4 {
		t.Errorf("got current_step %d, want 4", state.CurrentStep)
	}
}

func TestRunnerStepFailure(t *testing.T) {
	t.Parallel()

	repo := newStubRepo()
	steps := []Step{
		&stubStep{name: "step_0"},
		&stubStep{name: "step_1", err: errors.New("network timeout")},
		&stubStep{name: "step_2"},
	}
	runner := NewRunner(repo, steps, testLogger())

	err := runner.Run(context.Background(), nil, 1, "test_workflow", "client1", "", nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if err.Error() != "network timeout" {
		t.Errorf("got error %q, want %q", err, "network timeout")
	}

	state := repo.states[1]
	if state.Status != StatusFailed {
		t.Errorf("got status %q, want %q", state.Status, StatusFailed)
	}
	// Step 0 completed, step 1 failed — current_step stays at 1.
	if state.CurrentStep != 1 {
		t.Errorf("got current_step %d, want 1", state.CurrentStep)
	}
}

func TestRunnerPermanentError(t *testing.T) {
	t.Parallel()

	repo := newStubRepo()
	steps := []Step{
		&stubStep{name: "step_0"},
		&stubStep{name: "step_1", err: &PermanentError{Cause: errors.New("invalid config")}},
	}
	runner := NewRunner(repo, steps, testLogger())

	err := runner.Run(context.Background(), nil, 1, "test_workflow", "client1", "", nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !IsPermanent(err) {
		t.Errorf("expected permanent error, got %T: %v", err, err)
	}

	state := repo.states[1]
	if state.Status != StatusFailed {
		t.Errorf("got status %q, want %q", state.Status, StatusFailed)
	}
}

func TestRunnerAccumulatesData(t *testing.T) {
	t.Parallel()

	repo := newStubRepo()
	initial := map[string]json.RawMessage{
		"org_name": json.RawMessage(`"AcmeCorp"`),
	}
	steps := []Step{
		&writingStep{name: "step_0", key: "tenant_type", value: "new"},
		&writingStep{name: "step_1", key: "vdc_name", value: "789-site1"},
	}
	runner := NewRunner(repo, steps, testLogger())

	err := runner.Run(context.Background(), nil, 1, "test_workflow", "client1", "", initial)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	state := repo.states[1]

	// Original data preserved.
	if v, ok := state.GetString("org_name"); !ok || v != "AcmeCorp" {
		t.Errorf("got org_name %q, want %q", v, "AcmeCorp")
	}
	// Step 0 added tenant_type.
	if v, ok := state.GetString("tenant_type"); !ok || v != "new" {
		t.Errorf("got tenant_type %q, want %q", v, "new")
	}
	// Step 1 added vdc_name.
	if v, ok := state.GetString("vdc_name"); !ok || v != "789-site1" {
		t.Errorf("got vdc_name %q, want %q", v, "789-site1")
	}
}

// recordingStep tracks which steps were called.
type recordingStep struct {
	name   string
	called *[]string
}

func (s *recordingStep) Name() string { return s.name }
func (s *recordingStep) Run(_ context.Context, _ *WorkflowState) error {
	*s.called = append(*s.called, s.name)
	return nil
}
