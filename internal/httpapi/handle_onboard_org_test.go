package httpapi

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/CloudKey-io/hbs-queue/internal/clients/vcd"
)

// validOnboardBody is a minimal valid OnboardOrgRequest JSON body.
const validOnboardBody = `{"crm_id":"167","client_first_name":"Test","client_last_name":"User","client_email":"test@example.com","account_id":1,"bandwidth":"100"}`

// newVCDMock creates a TLS httptest server with VCD auth and a custom org
// handler, returning a *vcd.Client wired to the mock.
func newVCDMock(t *testing.T, orgHandler http.HandlerFunc) *vcd.Client {
	t.Helper()

	mux := http.NewServeMux()
	mux.HandleFunc("POST /cloudapi/1.0.0/sessions/provider", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-VMWARE-VCLOUD-ACCESS-TOKEN", "test-token")
		w.WriteHeader(http.StatusOK)
	})
	if orgHandler != nil {
		mux.HandleFunc("GET /cloudapi/1.0.0/orgs", orgHandler)
	}

	srv := httptest.NewTLSServer(mux)
	t.Cleanup(srv.Close)

	client := vcd.New(srv.URL, "38.0", "admin", "secret", "System",
		slog.New(slog.NewTextHandler(io.Discard, nil)))
	client.SetHTTPClient(srv.Client())

	return client
}

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestHandleOnboardOrgValidation(t *testing.T) {
	t.Parallel()

	logger := discardLogger()
	handler := handleOnboardOrg(logger, nil, nil, nil)

	tests := []struct {
		name       string
		body       string
		wantStatus int
		wantError  string
	}{
		{
			name:       "invalid json",
			body:       `{bad`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "empty object",
			body:       `{}`,
			wantStatus: http.StatusUnprocessableEntity,
			wantError:  "validation failed",
		},
		{
			name:       "missing crm_id",
			body:       `{"client_first_name":"Test","client_last_name":"User","client_email":"test@example.com","account_id":1,"bandwidth":"100"}`,
			wantStatus: http.StatusUnprocessableEntity,
			wantError:  "validation failed",
		},
		{
			name:       "missing email",
			body:       `{"crm_id":"167","client_first_name":"Test","client_last_name":"User","account_id":1,"bandwidth":"100"}`,
			wantStatus: http.StatusUnprocessableEntity,
			wantError:  "validation failed",
		},
		{
			name:       "zero account_id",
			body:       `{"crm_id":"167","client_first_name":"Test","client_last_name":"User","client_email":"test@example.com","account_id":0,"bandwidth":"100"}`,
			wantStatus: http.StatusUnprocessableEntity,
			wantError:  "validation failed",
		},
		{
			name:       "negative max_zerto_storage",
			body:       `{"crm_id":"167","client_first_name":"Test","client_last_name":"User","client_email":"test@example.com","account_id":1,"bandwidth":"100","max_zerto_storage":-1}`,
			wantStatus: http.StatusUnprocessableEntity,
			wantError:  "validation failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequest(http.MethodPost, "/api/v1/script/onboard-org",
				bytes.NewBufferString(tt.body))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Errorf("got status %d, want %d", rec.Code, tt.wantStatus)
			}

			if tt.wantError != "" {
				var resp ErrorResponse
				json.NewDecoder(rec.Body).Decode(&resp)
				if resp.Error != tt.wantError {
					t.Errorf("got error %q, want %q", resp.Error, tt.wantError)
				}
			}
		})
	}
}

func TestHandleOnboardOrgAllFieldProblems(t *testing.T) {
	t.Parallel()

	handler := handleOnboardOrg(discardLogger(), nil, nil, nil)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/script/onboard-org",
		bytes.NewBufferString(`{}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("got %d, want 422", rec.Code)
	}

	var resp ErrorResponse
	json.NewDecoder(rec.Body).Decode(&resp)

	for _, field := range []string{"crm_id", "client_first_name", "client_last_name", "client_email", "bandwidth", "account_id"} {
		if _, ok := resp.Problems[field]; !ok {
			t.Errorf("expected problem for %q, got %v", field, resp.Problems)
		}
	}
}

func TestHandleOnboardOrgNoVCDClient(t *testing.T) {
	t.Parallel()

	handler := handleOnboardOrg(discardLogger(), nil, nil, nil)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/script/onboard-org",
		bytes.NewBufferString(validOnboardBody))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("got %d, want 503", rec.Code)
	}

	var resp ErrorResponse
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp.Error != "vcd client not configured" {
		t.Errorf("got error %q, want %q", resp.Error, "vcd client not configured")
	}
}

func TestHandleOnboardOrgVCDError(t *testing.T) {
	t.Parallel()

	vcdClient := newVCDMock(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"message":"bad request"}`))
	})

	handler := handleOnboardOrg(discardLogger(), nil, nil, vcdClient)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/script/onboard-org",
		bytes.NewBufferString(validOnboardBody))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadGateway {
		t.Errorf("got %d, want 502", rec.Code)
	}

	var resp ErrorResponse
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp.Error == "" {
		t.Error("expected non-empty error message")
	}
}

func TestHandleOnboardOrgVCDOrgNotFound(t *testing.T) {
	t.Parallel()

	vcdClient := newVCDMock(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"resultTotal": 0,
			"values":      []any{},
		})
	})

	handler := handleOnboardOrg(discardLogger(), nil, nil, vcdClient)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/script/onboard-org",
		bytes.NewBufferString(validOnboardBody))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadGateway {
		t.Errorf("got %d, want 502", rec.Code)
	}
}
