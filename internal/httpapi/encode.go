package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
)

// encode writes v as JSON with the given status code.
// If encoding fails the error is logged — there is nothing useful
// the caller can do once headers are on the wire.
func encode(w http.ResponseWriter, r *http.Request, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.ErrorContext(r.Context(), "encode json", "err", err)
	}
}

// Validator is implemented by request types that can self-validate.
// Valid returns nil when the request is valid, or a map of field names
// to human-readable problem descriptions.
type Validator interface {
	Valid(ctx context.Context) map[string]string
}

// decodeValid decodes JSON from the request body and validates it.
// On success problems is nil. On validation failure problems is non-nil
// and err is non-nil. On decode failure problems is nil and err is non-nil.
func decodeValid[T Validator](r *http.Request) (T, map[string]string, error) {
	var v T
	if err := json.NewDecoder(r.Body).Decode(&v); err != nil {
		return v, nil, fmt.Errorf("decode json: %w", err)
	}
	if problems := v.Valid(r.Context()); len(problems) > 0 {
		return v, problems, fmt.Errorf("invalid %T", v)
	}
	return v, nil, nil
}
