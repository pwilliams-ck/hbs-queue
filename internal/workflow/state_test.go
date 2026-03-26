package workflow

import (
	"encoding/json"
	"errors"
	"fmt"
	"testing"
)

func TestGetString(t *testing.T) {
	t.Parallel()

	state := &WorkflowState{
		Data: map[string]json.RawMessage{
			"name":   json.RawMessage(`"alice"`),
			"count":  json.RawMessage(`42`),
			"broken": json.RawMessage(`{invalid`),
		},
	}

	tests := []struct {
		key     string
		wantVal string
		wantOK  bool
	}{
		{"name", "alice", true},
		{"missing", "", false},
		{"count", "", false},  // not a string
		{"broken", "", false}, // invalid JSON
	}

	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			t.Parallel()
			got, ok := state.GetString(tt.key)
			if got != tt.wantVal || ok != tt.wantOK {
				t.Errorf("GetString(%q) = (%q, %v), want (%q, %v)", tt.key, got, ok, tt.wantVal, tt.wantOK)
			}
		})
	}
}

func TestGetInt(t *testing.T) {
	t.Parallel()

	state := &WorkflowState{
		Data: map[string]json.RawMessage{
			"count":  json.RawMessage(`42`),
			"name":   json.RawMessage(`"alice"`),
			"broken": json.RawMessage(`{invalid`),
		},
	}

	tests := []struct {
		key     string
		wantVal int
		wantOK  bool
	}{
		{"count", 42, true},
		{"missing", 0, false},
		{"name", 0, false},   // not an int
		{"broken", 0, false}, // invalid JSON
	}

	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			t.Parallel()
			got, ok := state.GetInt(tt.key)
			if got != tt.wantVal || ok != tt.wantOK {
				t.Errorf("GetInt(%q) = (%d, %v), want (%d, %v)", tt.key, got, ok, tt.wantVal, tt.wantOK)
			}
		})
	}
}

func TestSet(t *testing.T) {
	t.Parallel()

	t.Run("nil data map", func(t *testing.T) {
		t.Parallel()
		state := &WorkflowState{}
		if err := state.Set("key", "value"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		got, ok := state.GetString("key")
		if !ok || got != "value" {
			t.Errorf("after Set: GetString = (%q, %v), want (%q, true)", got, ok, "value")
		}
	})

	t.Run("overwrite", func(t *testing.T) {
		t.Parallel()
		state := &WorkflowState{
			Data: map[string]json.RawMessage{
				"key": json.RawMessage(`"old"`),
			},
		}
		if err := state.Set("key", "new"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		got, _ := state.GetString("key")
		if got != "new" {
			t.Errorf("after overwrite: got %q, want %q", got, "new")
		}
	})

	t.Run("unmarshalable value", func(t *testing.T) {
		t.Parallel()
		state := &WorkflowState{}
		err := state.Set("bad", make(chan int))
		if err == nil {
			t.Error("expected error for unmarshalable value")
		}
	})
}

func TestIsPermanent(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"permanent", &PermanentError{Cause: errors.New("fail")}, true},
		{"wrapped permanent", fmt.Errorf("wrap: %w", &PermanentError{Cause: errors.New("fail")}), true},
		{"regular error", errors.New("transient"), false},
		{"nil", nil, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := IsPermanent(tt.err); got != tt.want {
				t.Errorf("IsPermanent() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPermanentErrorMessage(t *testing.T) {
	t.Parallel()

	err := &PermanentError{Cause: errors.New("bad input")}
	want := "permanent: bad input"
	if got := err.Error(); got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}
}
