package httpapi

import (
	"net/http"
)

// handleBandwidthUpdate processes a bandwidth-update webhook from HostBill.
// Enqueues an update_bandwidth River job that adjusts the tenant's
// network rate limit in VCD.
//
// Request:  POST /hooks/update-bandwidth (JSON body, webhook HMAC auth)
// Response: 202 Accepted with job ID, or error.
//
// Stub — returns 501 until wired to the workflow engine.
func handleBandwidthUpdate() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, http.StatusText(http.StatusNotImplemented), http.StatusNotImplemented)
	})
}
