package vcd

import (
	"fmt"
	"strings"
)

// Organization represents a VCD organization returned by the Cloud API.
type Organization struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	DisplayName    string `json:"displayName"`
	Description    string `json:"description"`
	IsEnabled      bool   `json:"isEnabled"`
	OrgVdcCount    int    `json:"orgVdcCount"`
	CatalogCount   int    `json:"catalogCount"`
	VappCount      int    `json:"vappCount"`
	RunningVMCount int    `json:"runningVMCount"`
	UserCount      int    `json:"userCount"`
	DiskCount      int    `json:"diskCount"`
	CanPublish     bool   `json:"canPublish"`
}

// organizationResult is the paginated response from GET /cloudapi/1.0.0/orgs.
type organizationResult struct {
	ResultTotal int            `json:"resultTotal"`
	PageCount   int            `json:"pageCount"`
	Page        int            `json:"page"`
	PageSize    int            `json:"pageSize"`
	Values      []Organization `json:"values"`
}

// VDC represents a Virtual Data Center returned by the Cloud API.
type VDC struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	Description    string `json:"description"`
	AllocationType string `json:"allocationType"`
	Org            OrgRef `json:"org"`
}

// OrgRef is a reference to an organization, embedded in other types.
type OrgRef struct {
	Name string `json:"name"`
	ID   string `json:"id"`
}

// vdcResponse is the paginated response from GET /cloudapi/1.0.0/vdcs.
type vdcResponse struct {
	ResultTotal int   `json:"resultTotal"`
	PageCount   int   `json:"pageCount"`
	Page        int   `json:"page"`
	PageSize    int   `json:"pageSize"`
	Values      []VDC `json:"values"`
}

// ExtractUUID returns the UUID portion of a VCD URN.
// For example, "urn:vcloud:org:a1b2c3d4-..." returns "a1b2c3d4-...".
func ExtractUUID(urn string) (string, error) {
	parts := strings.Split(urn, ":")
	if len(parts) < 4 {
		return "", fmt.Errorf("invalid URN format: %s", urn)
	}
	return parts[len(parts)-1], nil
}
