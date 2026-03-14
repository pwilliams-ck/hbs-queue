package httpapi

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHandleEcho(t *testing.T) {
	t.Parallel()

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	tests := []struct {
		name       string
		body       string
		wantStatus int
		wantEcho   string
	}{
		{
			name:       "valid",
			body:       `{"message":"hello"}`,
			wantStatus: http.StatusOK,
			wantEcho:   "hello",
		},
		{
			name:       "empty message",
			body:       `{"message":""}`,
			wantStatus: http.StatusUnprocessableEntity,
		},
		{
			name:       "message too long",
			body:       `{"message":"` + strings.Repeat("a", 1001) + `"}`,
			wantStatus: http.StatusUnprocessableEntity,
		},
		{
			name:       "invalid json",
			body:       `{bad`,
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			handler := handleEcho(logger)

			req := httptest.NewRequest(http.MethodPost, "/api/v1/echo",
				bytes.NewBufferString(tt.body))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Errorf("got %d, want %d", rec.Code, tt.wantStatus)
			}

			if tt.wantEcho != "" {
				var resp EchoResponse
				json.NewDecoder(rec.Body).Decode(&resp)
				if resp.Echo != tt.wantEcho {
					t.Errorf("got %q, want %q", resp.Echo, tt.wantEcho)
				}
			}
		})
	}
}
