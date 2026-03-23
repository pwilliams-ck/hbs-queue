package httpapi

import (
	"net/http"
)

// handleProvisionVDC enqueues a provision_vdc River job from a
// script-provisioning request. Called by HostBill's Script Provisioner
// module when a VDC needs to be provisioned for an existing tenant.
//
// Request:  POST /api/v1/script/provision-vdc (JSON body, API key auth)
// Response: 202 Accepted with job ID, or error.
//
// Stub — returns 501 until wired to the workflow engine.
func handleProvisionVDC() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, http.StatusText(http.StatusNotImplemented), http.StatusNotImplemented)
	})
}
