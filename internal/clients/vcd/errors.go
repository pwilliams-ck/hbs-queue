package vcd

import (
	"errors"
	"fmt"
	"strings"
)

// APIError represents a non-2xx response from the VCD API.
type APIError struct {
	StatusCode int
	Body       string
	Method     string
	Path       string
}

// Error returns a formatted error message including the HTTP method, path,
// status code, and a truncated response body. HTML responses are summarized
// to avoid dumping full error pages into log lines.
func (e *APIError) Error() string {
	return fmt.Sprintf("vcd: %s %s returned %d: %s", e.Method, e.Path, e.StatusCode, summarizeBody(e.Body))
}

// maxBodyLen is the maximum number of characters included in error messages.
const maxBodyLen = 200

// summarizeBody returns a short, log-friendly version of a response body.
// HTML pages are reduced to their <title> or first <h3> text; other bodies
// are truncated to maxBodyLen characters.
func summarizeBody(body string) string {
	// VCD and its load balancer (VMware Avi LB) return full HTML error
	// pages on 403/502/etc. Logging the raw HTML produces multi-KB single
	// log lines that break log aggregators and are unreadable. Pull out
	// just the title or heading so the error is still useful.
	if strings.Contains(body, "<html") || strings.Contains(body, "<HTML") {
		if t := extractTag(body, "title"); t != "" {
			return t
		}
		if t := extractTag(body, "h3"); t != "" {
			return t
		}
		return "(HTML error page)"
	}
	if len(body) > maxBodyLen {
		return body[:maxBodyLen] + "..."
	}
	return body
}

// extractTag returns the trimmed text content of the first occurrence of
// the given HTML tag, or empty string if not found.
func extractTag(html, tag string) string {
	open := "<" + tag + ">"
	closeTag := "</" + tag + ">"
	i := strings.Index(strings.ToLower(html), strings.ToLower(open))
	if i < 0 {
		return ""
	}
	start := i + len(open)
	j := strings.Index(strings.ToLower(html[start:]), strings.ToLower(closeTag))
	if j < 0 {
		return ""
	}
	return strings.TrimSpace(html[start : start+j])
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
