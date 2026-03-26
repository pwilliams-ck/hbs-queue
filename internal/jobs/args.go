// Package jobs defines River job argument types and workers for hbs-queue
// workflows. Each args struct implements river.JobArgs and maps to a
// workflow type in the workflow_state table.
package jobs

// OnboardOrgArgs is the payload for the onboard_org job.
// Enqueued by POST /api/v1/script/onboard-org.
// Maps to workflow type "onboard_org".
type OnboardOrgArgs struct {
	ClientID         string `json:"crm_id"`
	OrganizationName string `json:"organization_name"`
	ClientUsername   string `json:"client_username"`
	ClientFirstName  string `json:"client_first_name"`
	ClientLastName   string `json:"client_last_name"`
	ClientEmail      string `json:"client_email"`
	AccountID        int    `json:"account_id"`
	Country          string `json:"country"`
	State            string `json:"state"`
	PostalCode       string `json:"postal_code"`
	MaxZertoStorage  int    `json:"max_zerto_storage"`
	MaxZertoVMs      int    `json:"max_zerto_vms"`
	Bandwidth        string `json:"bandwidth"`
	ProductID        string `json:"product_id"`
}

// Kind returns the River job kind for onboard_org jobs.
func (OnboardOrgArgs) Kind() string { return "onboard_org" }

// DeboardOrgArgs is the payload for the deboard_org job.
// Enqueued by POST /hooks/deboard-org.
// Maps to workflow type "deboard_org".
type DeboardOrgArgs struct {
	ClientID         string `json:"client_id"`
	OrganizationName string `json:"organization_name"`
}

// Kind returns the River job kind for deboard_org jobs.
func (DeboardOrgArgs) Kind() string { return "deboard_org" }

// AddContactArgs is the payload for the onboard_contact job.
// Enqueued by POST /hooks/onboard-contact.
// Maps to workflow type "onboard_contact".
type AddContactArgs struct {
	ClientID  string `json:"client_id"`
	ContactID string `json:"contact_id"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Email     string `json:"email"`
}

// Kind returns the River job kind for onboard_contact jobs.
func (AddContactArgs) Kind() string { return "onboard_contact" }

// DeleteContactArgs is the payload for the deboard_contact job.
// Enqueued by POST /hooks/deboard-contact.
// Maps to workflow type "deboard_contact".
type DeleteContactArgs struct {
	ClientID  string `json:"client_id"`
	ContactID string `json:"contact_id"`
}

// Kind returns the River job kind for deboard_contact jobs.
func (DeleteContactArgs) Kind() string { return "deboard_contact" }

// UpdatePwArgs is the payload for the update_pw job.
// Enqueued by POST /hooks/update-pw.
// Maps to workflow type "update_pw".
type UpdatePwArgs struct {
	ClientID    string `json:"client_id"`
	ContactID   string `json:"contact_id"`
	NewPassword string `json:"new_password"`
}

// Kind returns the River job kind for update_pw jobs.
func (UpdatePwArgs) Kind() string { return "update_pw" }

// UpdateBandwidthArgs is the payload for the update_bandwidth job.
// Enqueued by POST /hooks/update-bandwidth.
// Maps to workflow type "update_bandwidth".
type UpdateBandwidthArgs struct {
	ClientID  string `json:"client_id"`
	OrderID   string `json:"order_id"`
	Bandwidth string `json:"bandwidth"`
}

// Kind returns the River job kind for update_bandwidth jobs.
func (UpdateBandwidthArgs) Kind() string { return "update_bandwidth" }
