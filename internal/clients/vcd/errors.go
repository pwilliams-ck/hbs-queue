package vcd

import (
	"errors"
	"fmt"
)

// APIError represents a non-2xx response from the VCD API.
type APIError struct {
	StatusCode int
	Body       string
	Method     string
	Path       string
}

// Error returns a formatted error message including the HTTP method, path,
// status code, and response body.
func (e *APIError) Error() string {
	return fmt.Sprintf("vcd: %s %s returned %d: %s", e.Method, e.Path, e.StatusCode, e.Body)
}

// IsRetryable returns true for status codes that indicate a transient failure.
func (e *APIError) IsRetryable() bool {
	switch e.StatusCode {
	case 429, 502, 503, 504:
		return true
	default:
		return false
	}
}

// IsNotFound returns true if the VCD API returned 404.
func (e *APIError) IsNotFound() bool {
	return e.StatusCode == 404
}

// IsRetryable unwraps err and returns true if the underlying APIError
// is retryable. Returns true for non-APIError errors (e.g. network errors)
// since those are typically transient.
func IsRetryable(err error) bool {
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		return apiErr.IsRetryable()
	}
	return true // network errors are retryable
}

// IsNotFound unwraps err and returns true if the underlying APIError
// has a 404 status code.
func IsNotFound(err error) bool {
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		return apiErr.IsNotFound()
	}
	return false
}
