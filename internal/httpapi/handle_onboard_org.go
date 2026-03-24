package httpapi

import (
	"log/slog"
	"net/http"

	"github.com/CloudKey-io/hbs-queue/internal/clients/vcd"
)

// handleOnboardOrg enqueues an onboard_customer River job from a
// script-provisioning request. Called by HostBill's Script Provisioner
// module when a new VCD tenant order is created.
//
// Request:  POST /api/v1/script/onboard-org (JSON body, API key auth)
// Response: 202 Accepted with job ID, or error.
func handleOnboardOrg(logger *slog.Logger, vcdClient *vcd.Client) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		req, problems, err := decodeValid[OnboardOrgRequest](r)
		if err != nil {
			if problems != nil {
				encode(w, r, http.StatusUnprocessableEntity, ErrorResponse{
					Error:    "validation failed",
					Problems: problems,
				})
				return
			}
			encode(w, r, http.StatusBadRequest, ErrorResponse{Error: err.Error()})
			return
		}

		// Look up the organization in VCD using the crm_id.
		if vcdClient == nil {
			encode(w, r, http.StatusServiceUnavailable, ErrorResponse{Error: "vcd client not configured"})
			return
		}

		org, err := vcdClient.GetOrganization(r.Context(), req.ClientID)
		if err != nil {
			logger.Error("vcd get organization failed",
				"crm_id", req.ClientID,
				"err", err,
			)
			encode(w, r, http.StatusBadGateway, ErrorResponse{Error: "vcd lookup failed: " + err.Error()})
			return
		}

		logger.Info("vcd organization found",
			"crm_id", req.ClientID,
			"vcd_org_id", org.ID,
			"vcd_org_name", org.Name,
		)

		// TODO: enqueue River job instead of returning the org directly.
		// For now, return the org lookup result to prove the flow works.
		encode(w, r, http.StatusOK, org)
	})
}
