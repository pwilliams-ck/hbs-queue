package httpapi

import (
	"net/http"
)

// handleDeboardOrg processes a deboard_org webhook from HostBill.
// Enqueues a deboard_org River job that tears down the tenant's
// resources across VCD, Zerto, Keycloak, and Active Directory.
//
// Request:  POST /hooks/deboard-org (JSON body, webhook HMAC auth)
// Response: 202 Accepted with job ID, or error.
//
// Stub — returns 501 until wired to the workflow engine.
func handleDeboardOrg() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, http.StatusText(http.StatusNotImplemented), http.StatusNotImplemented)
	})
}
