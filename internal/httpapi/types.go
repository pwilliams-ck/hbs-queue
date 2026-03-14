package httpapi

import "context"

// --- Health ---

// ReadyResponse is returned by the readiness probe.
type ReadyResponse struct {
	Status string `json:"status"`
}

// HealthResponse is returned by the health endpoint with build info.
type HealthResponse struct {
	Status    string `json:"status"`
	Version   string `json:"version"`
	Commit    string `json:"commit"`
	BuildTime string `json:"build_time"`
}

// --- Echo ---

// EchoRequest is the payload for the echo endpoint.
type EchoRequest struct {
	Message string `json:"message"`
}

// Valid returns nil if the request is valid, or a map of field-level problems.
func (r EchoRequest) Valid(ctx context.Context) map[string]string {
	if r.Message == "" {
		return map[string]string{"message": "required"}
	}
	if len(r.Message) > 1000 {
		return map[string]string{"message": "max 1000 characters"}
	}
	return nil
}

// EchoResponse is returned by the echo endpoint.
type EchoResponse struct {
	Echo      string `json:"echo"`
	RequestID string `json:"request_id"`
}

// --- Errors ---

// ErrorResponse is the standard error envelope.
type ErrorResponse struct {
	Error    string            `json:"error"`
	Problems map[string]string `json:"problems,omitempty"`
}
