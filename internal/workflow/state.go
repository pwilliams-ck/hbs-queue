package workflow

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// Status is the lifecycle status of a workflow.
type Status string

const (
	StatusPending   Status = "pending"
	StatusRunning   Status = "running"
	StatusCompleted Status = "completed"
	StatusFailed    Status = "failed"
)

// ErrNotFound is returned when a workflow_state row does not exist.
var ErrNotFound = errors.New("workflow state not found")

// WorkflowState mirrors the workflow_state table. The Data field is the
// JSONB accumulator — it starts as the HostBill payload and grows as
// each step completes.
type WorkflowState struct {
	ID           string
	JobID        int64
	WorkflowType string
	ClientID     string
	OrderID      string // empty when NULL
	CurrentStep  int
	Status       Status
	Error        string // empty when NULL
	Data         map[string]json.RawMessage
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// GetString reads a string value from the JSONB accumulator.
// Returns ("", false) if the key does not exist or is not a string.
func (s *WorkflowState) GetString(key string) (string, bool) {
	raw, ok := s.Data[key]
	if !ok {
		return "", false
	}
	var v string
	if err := json.Unmarshal(raw, &v); err != nil {
		return "", false
	}
	return v, true
}

// GetInt reads an int value from the JSONB accumulator.
// Returns (0, false) if the key does not exist or is not a number.
func (s *WorkflowState) GetInt(key string) (int, bool) {
	raw, ok := s.Data[key]
	if !ok {
		return 0, false
	}
	var v int
	if err := json.Unmarshal(raw, &v); err != nil {
		return 0, false
	}
	return v, true
}

// Set writes a value into the JSONB accumulator under the given key.
// The value is marshalled to JSON.
func (s *WorkflowState) Set(key string, value any) error {
	raw, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("marshal %q: %w", key, err)
	}
	if s.Data == nil {
		s.Data = make(map[string]json.RawMessage)
	}
	s.Data[key] = raw
	return nil
}

// Repository abstracts workflow_state persistence. All methods accept a
// pgx.Tx so that callers can compose operations within a single
// transaction. This is critical for the River worker pattern where
// state updates must be atomic with job completion.
type Repository interface {
	// Create inserts a new workflow_state row within tx.
	Create(ctx context.Context, tx pgx.Tx, state *WorkflowState) error

	// Get fetches a workflow_state row by River job_id within tx.
	// Returns ErrNotFound if the row does not exist.
	Get(ctx context.Context, tx pgx.Tx, jobID int64) (*WorkflowState, error)

	// UpdateStep persists current_step, status, error, data, and
	// updated_at. Called by the runner after each step completes or fails.
	UpdateStep(ctx context.Context, tx pgx.Tx, state *WorkflowState) error
}

// PgxRepository is the production Repository backed by pgx transactions.
type PgxRepository struct{}

// NewPgxRepository creates a PgxRepository.
func NewPgxRepository() *PgxRepository {
	return &PgxRepository{}
}

// Create inserts a new workflow_state row.
func (r *PgxRepository) Create(ctx context.Context, tx pgx.Tx, state *WorkflowState) error {
	data, err := json.Marshal(state.Data)
	if err != nil {
		return fmt.Errorf("marshal data: %w", err)
	}

	var orderID *string
	if state.OrderID != "" {
		orderID = &state.OrderID
	}

	return tx.QueryRow(ctx,
		`INSERT INTO workflow_state (job_id, workflow_type, client_id, order_id, status, data)
		 VALUES ($1, $2, $3, $4, $5, $6)
		 RETURNING id, created_at, updated_at`,
		state.JobID,
		state.WorkflowType,
		state.ClientID,
		orderID,
		state.Status,
		data,
	).Scan(&state.ID, &state.CreatedAt, &state.UpdatedAt)
}

// Get fetches a workflow_state row by River job_id.
func (r *PgxRepository) Get(ctx context.Context, tx pgx.Tx, jobID int64) (*WorkflowState, error) {
	s := &WorkflowState{}
	var data []byte
	var orderID *string
	var errText *string

	err := tx.QueryRow(ctx,
		`SELECT id, job_id, workflow_type, client_id, order_id,
		        current_step, status, error, data, created_at, updated_at
		 FROM workflow_state
		 WHERE job_id = $1`,
		jobID,
	).Scan(
		&s.ID, &s.JobID, &s.WorkflowType, &s.ClientID, &orderID,
		&s.CurrentStep, &s.Status, &errText, &data, &s.CreatedAt, &s.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("query workflow state: %w", err)
	}

	if orderID != nil {
		s.OrderID = *orderID
	}
	if errText != nil {
		s.Error = *errText
	}
	if err := json.Unmarshal(data, &s.Data); err != nil {
		return nil, fmt.Errorf("unmarshal data: %w", err)
	}

	return s, nil
}

// UpdateStep persists step progress after each step completes or fails.
func (r *PgxRepository) UpdateStep(ctx context.Context, tx pgx.Tx, state *WorkflowState) error {
	data, err := json.Marshal(state.Data)
	if err != nil {
		return fmt.Errorf("marshal data: %w", err)
	}

	var errText *string
	if state.Error != "" {
		errText = &state.Error
	}

	tag, err := tx.Exec(ctx,
		`UPDATE workflow_state
		 SET current_step = $1, status = $2, error = $3, data = $4, updated_at = now()
		 WHERE job_id = $5`,
		state.CurrentStep,
		state.Status,
		errText,
		data,
		state.JobID,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("workflow state row not found for job_id %d", state.JobID)
	}
	return nil
}
