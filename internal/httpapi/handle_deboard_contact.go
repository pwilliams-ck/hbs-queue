package httpapi

import (
	"net/http"
)

// handleDeboardContact processes a deboard-contact webhook from HostBill.
// Enqueues a delete_contact River job that removes the contact's accounts
// from Keycloak and Active Directory.
//
// Request:  POST /hooks/deboard-contact (JSON body, webhook HMAC auth)
// Response: 202 Accepted with job ID, or error.
//
// Stub — returns 501 until wired to the workflow engine.
func handleDeboardContact() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, http.StatusText(http.StatusNotImplemented), http.StatusNotImplemented)
	})
}
