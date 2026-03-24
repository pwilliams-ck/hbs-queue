package httpapi

import "context"

// --- Health ---

// ReadyResponse is returned by the readiness probe.
type ReadyResponse struct {
	Status string `json:"status"`
}

// HealthResponse is returned by the health endpoint with build info.
type HealthResponse struct {
	Status    string `json:"status"`
	Version   string `json:"version"`
	Commit    string `json:"commit"`
	BuildTime string `json:"build_time"`
	Database  string `json:"database"`
}

// --- Echo ---

// EchoRequest is the payload for the echo endpoint.
type EchoRequest struct {
	Message string `json:"message"`
}

// Valid returns nil if the request is valid, or a map of field-level problems.
func (r EchoRequest) Valid(ctx context.Context) map[string]string {
	if r.Message == "" {
		return map[string]string{"message": "required"}
	}
	if len(r.Message) > 1000 {
		return map[string]string{"message": "max 1000 characters"}
	}
	return nil
}

// EchoResponse is returned by the echo endpoint.
type EchoResponse struct {
	Echo      string `json:"echo"`
	RequestID string `json:"request_id"`
}

// --- Job Accepted ---

// JobAcceptedResponse is returned when a handler successfully enqueues a River job.
type JobAcceptedResponse struct {
	JobID      int64  `json:"job_id"`
	WorkflowID string `json:"workflow_id"`
}

// --- Script Provisioning ---

// OnboardOrgRequest is the payload for POST /api/v1/script/onboard-org.
// Fields match the HostBill Script Provisioner payload.
type OnboardOrgRequest struct {
	OrganizationName string `json:"organization_name"`
	ClientUsername   string `json:"client_username"`
	ClientFirstName  string `json:"client_first_name"`
	ClientLastName   string `json:"client_last_name"`
	ClientEmail      string `json:"client_email"`
	ClientID         string `json:"crm_id"`
	AccountID        int    `json:"account_id"`
	Country          string `json:"country"`
	State            string `json:"state"`
	PostalCode       string `json:"postal_code"`
	MaxZertoStorage  int    `json:"max_zerto_storage"`
	MaxZertoVMs      int    `json:"max_zerto_vms"`
	Bandwidth        string `json:"bandwidth"`
	ProductID        string `json:"product_id"`
}

// Valid returns nil if the request is valid, or a map of field-level problems.
func (r OnboardOrgRequest) Valid(ctx context.Context) map[string]string {
	problems := make(map[string]string)
	if r.ClientID == "" {
		problems["crm_id"] = "required"
	}
	if r.ClientFirstName == "" {
		problems["client_first_name"] = "required"
	}
	if r.ClientLastName == "" {
		problems["client_last_name"] = "required"
	}
	if r.ClientEmail == "" {
		problems["client_email"] = "required"
	}
	if r.Bandwidth == "" {
		problems["bandwidth"] = "required"
	}
	if r.AccountID <= 0 {
		problems["account_id"] = "must be a positive integer"
	}
	if r.MaxZertoStorage < 0 {
		problems["max_zerto_storage"] = "must be zero or greater"
	}
	if r.MaxZertoVMs < 0 {
		problems["max_zerto_vms"] = "must be zero or greater"
	}
	if len(problems) > 0 {
		return problems
	}
	return nil
}

// --- Hooks ---

// DeboardOrgRequest is the payload for POST /hooks/deboard-org.
type DeboardOrgRequest struct {
	ClientID         string `json:"crm_id"`
	OrganizationName string `json:"organization_name"`
}

// Valid returns nil if the request is valid, or a map of field-level problems.
func (r DeboardOrgRequest) Valid(ctx context.Context) map[string]string {
	problems := make(map[string]string)
	if r.ClientID == "" {
		problems["crm_id"] = "required"
	}
	if r.OrganizationName == "" {
		problems["organization_name"] = "required"
	}
	if len(problems) > 0 {
		return problems
	}
	return nil
}

// OnboardContactRequest is the payload for POST /hooks/onboard-contact.
type OnboardContactRequest struct {
	ClientID  string `json:"crm_id"`
	ContactID string `json:"contact_id"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Email     string `json:"email"`
}

// Valid returns nil if the request is valid, or a map of field-level problems.
func (r OnboardContactRequest) Valid(ctx context.Context) map[string]string {
	problems := make(map[string]string)
	if r.ClientID == "" {
		problems["crm_id"] = "required"
	}
	if r.ContactID == "" {
		problems["contact_id"] = "required"
	}
	if r.Email == "" {
		problems["email"] = "required"
	}
	if len(problems) > 0 {
		return problems
	}
	return nil
}

// DeboardContactRequest is the payload for POST /hooks/deboard-contact.
type DeboardContactRequest struct {
	ClientID  string `json:"crm_id"`
	ContactID string `json:"contact_id"`
}

// Valid returns nil if the request is valid, or a map of field-level problems.
func (r DeboardContactRequest) Valid(ctx context.Context) map[string]string {
	problems := make(map[string]string)
	if r.ClientID == "" {
		problems["crm_id"] = "required"
	}
	if r.ContactID == "" {
		problems["contact_id"] = "required"
	}
	if len(problems) > 0 {
		return problems
	}
	return nil
}

// UpdatePwRequest is the payload for POST /hooks/update-pw.
type UpdatePwRequest struct {
	ClientID    string `json:"crm_id"`
	ContactID   string `json:"contact_id"`
	NewPassword string `json:"new_password"`
}

// Valid returns nil if the request is valid, or a map of field-level problems.
func (r UpdatePwRequest) Valid(ctx context.Context) map[string]string {
	problems := make(map[string]string)
	if r.ClientID == "" {
		problems["crm_id"] = "required"
	}
	if r.ContactID == "" {
		problems["contact_id"] = "required"
	}
	if r.NewPassword == "" {
		problems["new_password"] = "required"
	}
	if len(problems) > 0 {
		return problems
	}
	return nil
}

// UpdateBandwidthRequest is the payload for POST /hooks/update-bandwidth.
type UpdateBandwidthRequest struct {
	ClientID  string `json:"crm_id"`
	OrderID   string `json:"order_id"`
	Bandwidth string `json:"bandwidth"`
}

// Valid returns nil if the request is valid, or a map of field-level problems.
func (r UpdateBandwidthRequest) Valid(ctx context.Context) map[string]string {
	problems := make(map[string]string)
	if r.ClientID == "" {
		problems["crm_id"] = "required"
	}
	if r.OrderID == "" {
		problems["order_id"] = "required"
	}
	if r.Bandwidth == "" {
		problems["bandwidth"] = "required"
	}
	if len(problems) > 0 {
		return problems
	}
	return nil
}

// --- Errors ---

// ErrorResponse is the standard error envelope.
type ErrorResponse struct {
	Error    string            `json:"error"`
	Problems map[string]string `json:"problems,omitempty"`
}
