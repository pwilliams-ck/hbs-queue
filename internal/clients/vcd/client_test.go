package vcd

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// newTestServer creates an httptest server that handles VCD auth and custom routes.
// authCalls tracks how many times the auth endpoint was hit.
func newTestServer(t *testing.T, handler http.Handler) (*httptest.Server, *Client) {
	t.Helper()
	srv := httptest.NewTLSServer(handler)
	t.Cleanup(srv.Close)

	client := New(srv.URL, "38.0", "admin", "secret", "System", testLogger())
	client.httpClient = srv.Client()

	return srv, client
}

func TestLazyAuth(t *testing.T) {
	t.Parallel()

	var authCalls atomic.Int32

	mux := http.NewServeMux()
	mux.HandleFunc("POST /cloudapi/1.0.0/sessions/provider", func(w http.ResponseWriter, r *http.Request) {
		authCalls.Add(1)
		w.Header().Set("X-VMWARE-VCLOUD-ACCESS-TOKEN", "test-token-123")
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("GET /cloudapi/1.0.0/orgs", func(w http.ResponseWriter, r *http.Request) {
		// Verify the token is being sent.
		if r.Header.Get("Authorization") != "Bearer test-token-123" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(organizationResult{
			ResultTotal: 1,
			Values:      []Organization{{ID: "urn:vcloud:org:abc", Name: "TestOrg"}},
		})
	})

	_, client := newTestServer(t, mux)

	// First call should trigger auth.
	org, err := client.GetOrganization(context.Background(), "TestOrg")
	if err != nil {
		t.Fatalf("first call: %v", err)
	}
	if org.Name != "TestOrg" {
		t.Errorf("got name %q, want %q", org.Name, "TestOrg")
	}

	// Second call should reuse the token, not re-auth.
	_, err = client.GetOrganization(context.Background(), "TestOrg")
	if err != nil {
		t.Fatalf("second call: %v", err)
	}

	if got := authCalls.Load(); got != 1 {
		t.Errorf("got %d auth calls, want 1", got)
	}
}

func TestReAuthOn401(t *testing.T) {
	t.Parallel()

	var authCalls atomic.Int32
	var orgCalls atomic.Int32

	mux := http.NewServeMux()
	mux.HandleFunc("POST /cloudapi/1.0.0/sessions/provider", func(w http.ResponseWriter, r *http.Request) {
		authCalls.Add(1)
		w.Header().Set("X-VMWARE-VCLOUD-ACCESS-TOKEN", "new-token")
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("GET /cloudapi/1.0.0/orgs", func(w http.ResponseWriter, r *http.Request) {
		call := orgCalls.Add(1)

		// First call returns 401 (expired token).
		if call == 1 {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		// Second call succeeds with new token.
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(organizationResult{
			ResultTotal: 1,
			Values:      []Organization{{ID: "urn:vcloud:org:abc", Name: "TestOrg"}},
		})
	})

	_, client := newTestServer(t, mux)
	// Pre-set an expired token so the first org call gets 401.
	client.token = "expired-token"

	org, err := client.GetOrganization(context.Background(), "TestOrg")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if org.Name != "TestOrg" {
		t.Errorf("got name %q, want %q", org.Name, "TestOrg")
	}

	// Should have re-authenticated once.
	if got := authCalls.Load(); got != 1 {
		t.Errorf("got %d auth calls, want 1", got)
	}
}

func TestGetOrganization(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	mux.HandleFunc("POST /cloudapi/1.0.0/sessions/provider", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-VMWARE-VCLOUD-ACCESS-TOKEN", "tok")
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("GET /cloudapi/1.0.0/orgs", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(organizationResult{
			ResultTotal: 1,
			Values: []Organization{{
				ID:          "urn:vcloud:org:abc-123",
				Name:        "AcmeCorp",
				DisplayName: "Acme Corporation",
				IsEnabled:   true,
				OrgVdcCount: 2,
			}},
		})
	})

	_, client := newTestServer(t, mux)

	org, err := client.GetOrganization(context.Background(), "AcmeCorp")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if org.ID != "urn:vcloud:org:abc-123" {
		t.Errorf("got ID %q, want %q", org.ID, "urn:vcloud:org:abc-123")
	}
	if org.OrgVdcCount != 2 {
		t.Errorf("got OrgVdcCount %d, want 2", org.OrgVdcCount)
	}
}

func TestGetOrganizationNotFound(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	mux.HandleFunc("POST /cloudapi/1.0.0/sessions/provider", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-VMWARE-VCLOUD-ACCESS-TOKEN", "tok")
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("GET /cloudapi/1.0.0/orgs", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(organizationResult{ResultTotal: 0, Values: nil})
	})

	_, client := newTestServer(t, mux)

	_, err := client.GetOrganization(context.Background(), "NoSuchOrg")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestGetVDC(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	mux.HandleFunc("POST /cloudapi/1.0.0/sessions/provider", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-VMWARE-VCLOUD-ACCESS-TOKEN", "tok")
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("GET /cloudapi/1.0.0/vdcs", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(vdcResponse{
			ResultTotal: 1,
			Values: []VDC{{
				ID:   "urn:vcloud:vdc:def-456",
				Name: "TestVDC",
				Org:  OrgRef{Name: "AcmeCorp", ID: "urn:vcloud:org:abc"},
			}},
		})
	})

	_, client := newTestServer(t, mux)

	vdc, err := client.GetVDC(context.Background(), "TestVDC")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if vdc.ID != "urn:vcloud:vdc:def-456" {
		t.Errorf("got ID %q, want %q", vdc.ID, "urn:vcloud:vdc:def-456")
	}
	if vdc.Org.Name != "AcmeCorp" {
		t.Errorf("got org name %q, want %q", vdc.Org.Name, "AcmeCorp")
	}
}

func TestErrorClassification(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		err         *APIError
		isRetryable bool
		isNotFound  bool
	}{
		{"404", &APIError{StatusCode: 404}, false, true},
		{"400", &APIError{StatusCode: 400}, false, false},
		{"503", &APIError{StatusCode: 503}, true, false},
		{"429", &APIError{StatusCode: 429}, true, false},
		{"502", &APIError{StatusCode: 502}, true, false},
		{"504", &APIError{StatusCode: 504}, true, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := IsRetryable(tt.err); got != tt.isRetryable {
				t.Errorf("IsRetryable(%d) = %v, want %v", tt.err.StatusCode, got, tt.isRetryable)
			}
			if got := IsNotFound(tt.err); got != tt.isNotFound {
				t.Errorf("IsNotFound(%d) = %v, want %v", tt.err.StatusCode, got, tt.isNotFound)
			}
		})
	}
}

func TestExtractUUID(t *testing.T) {
	t.Parallel()

	uuid, err := ExtractUUID("urn:vcloud:org:a1b2c3d4-e5f6-7890-abcd-ef1234567890")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if uuid != "a1b2c3d4-e5f6-7890-abcd-ef1234567890" {
		t.Errorf("got %q, want %q", uuid, "a1b2c3d4-e5f6-7890-abcd-ef1234567890")
	}

	_, err = ExtractUUID("bad-format")
	if err == nil {
		t.Error("expected error for bad URN format")
	}
}
