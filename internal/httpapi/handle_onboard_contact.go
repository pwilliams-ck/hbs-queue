package httpapi

import (
	"net/http"
)

// handleOnboardContact processes an onboard-contact webhook from HostBill.
// Enqueues an add_contact River job that creates the contact's accounts
// in Keycloak and Active Directory under the parent tenant.
//
// Request:  POST /hooks/onboard-contact (JSON body, webhook HMAC auth)
// Response: 202 Accepted with job ID, or error.
//
// Stub — returns 501 until wired to the workflow engine.
func handleOnboardContact() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, http.StatusText(http.StatusNotImplemented), http.StatusNotImplemented)
	})
}
