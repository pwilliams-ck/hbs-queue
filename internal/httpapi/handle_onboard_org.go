package httpapi

import (
	"net/http"
)

// handleOnboardOrg enqueues an onboard_customer River job from a
// script-provisioning request. Called by HostBill's Script Provisioner
// module when a new VCD tenant order is created.
//
// Request:  POST /api/v1/script/onboard-org (JSON body, API key auth)
// Response: 202 Accepted with job ID, or error.
//
// Stub — returns 501 until wired to the workflow engine.
func handleOnboardOrg() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, http.StatusText(http.StatusNotImplemented), http.StatusNotImplemented)
	})
}
