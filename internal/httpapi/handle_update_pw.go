package httpapi

import (
	"net/http"
)

// handlePWChange processes a password-change webhook from HostBill.
// Enqueues an update_pw River job that synchronises the new password
// to Keycloak and Active Directory.
//
// Request:  POST /hooks/update-pw (JSON body, webhook HMAC auth)
// Response: 202 Accepted with job ID, or error.
//
// Stub — returns 501 until wired to the workflow engine.
func handlePWChange() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, http.StatusText(http.StatusNotImplemented), http.StatusNotImplemented)
	})
}
