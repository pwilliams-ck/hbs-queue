package httpapi

import (
	"log/slog"
	"net/http"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"

	"github.com/CloudKey-io/hbs-queue/internal/clients/vcd"
	"github.com/CloudKey-io/hbs-queue/internal/jobs"
)

// handleOnboardOrg enqueues an onboard_customer River job from a
// script-provisioning request. Called by HostBill's Script Provisioner
// module when a new VCD tenant order is created.
//
// Flow: validate → VCD org lookup → enqueue River job → 202 Accepted.
//
// Request:  POST /api/v1/script/onboard-org (JSON body, API key auth)
// Response: 202 Accepted with job ID, or error.
func handleOnboardOrg(logger *slog.Logger, pool *pgxpool.Pool, riverClient *river.Client[pgx.Tx], vcdClient *vcd.Client) http.Handler {
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

		// Enqueue the onboard_customer job inside a transaction.
		tx, err := pool.Begin(r.Context())
		if err != nil {
			logger.Error("begin tx for enqueue failed", "err", err)
			encode(w, r, http.StatusInternalServerError, ErrorResponse{Error: "internal error"})
			return
		}
		defer tx.Rollback(r.Context()) //nolint:errcheck // rollback after commit is a no-op

		insertedJob, err := riverClient.InsertTx(r.Context(), tx, jobs.OnboardOrgArgs{
			ClientID:         req.ClientID,
			OrganizationName: req.OrganizationName,
			ClientUsername:   req.ClientUsername,
			ClientFirstName:  req.ClientFirstName,
			ClientLastName:   req.ClientLastName,
			ClientEmail:      req.ClientEmail,
			AccountID:        req.AccountID,
			Country:          req.Country,
			State:            req.State,
			PostalCode:       req.PostalCode,
			MaxZertoStorage:  req.MaxZertoStorage,
			MaxZertoVMs:      req.MaxZertoVMs,
			Bandwidth:        req.Bandwidth,
			ProductID:        req.ProductID,
		}, nil)
		if err != nil {
			logger.Error("enqueue onboard_customer failed", "crm_id", req.ClientID, "err", err)
			encode(w, r, http.StatusInternalServerError, ErrorResponse{Error: "failed to enqueue job"})
			return
		}

		if err := tx.Commit(r.Context()); err != nil {
			logger.Error("commit enqueue tx failed", "err", err)
			encode(w, r, http.StatusInternalServerError, ErrorResponse{Error: "internal error"})
			return
		}

		logger.Info("onboard_customer job enqueued",
			"job_id", insertedJob.Job.ID,
			"crm_id", req.ClientID,
		)

		encode(w, r, http.StatusAccepted, JobAcceptedResponse{
			JobID: insertedJob.Job.ID,
		})
	})
}
