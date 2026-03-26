package jobs

import (
	"testing"
)

func TestJobKinds(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		kind string
	}{
		{"OnboardOrgArgs", OnboardOrgArgs{}.Kind()},
		{"DeboardOrgArgs", DeboardOrgArgs{}.Kind()},
		{"AddContactArgs", AddContactArgs{}.Kind()},
		{"DeleteContactArgs", DeleteContactArgs{}.Kind()},
		{"UpdatePwArgs", UpdatePwArgs{}.Kind()},
		{"UpdateBandwidthArgs", UpdateBandwidthArgs{}.Kind()},
	}

	// Verify all kinds are unique and non-empty.
	seen := make(map[string]bool)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if tt.kind == "" {
				t.Errorf("%s.Kind() is empty", tt.name)
			}
		})
		if seen[tt.kind] {
			t.Errorf("duplicate Kind %q", tt.kind)
		}
		seen[tt.kind] = true
	}
}

func TestArgsToData(t *testing.T) {
	t.Parallel()

	args := OnboardOrgArgs{
		ClientID:         "test-001",
		OrganizationName: "TestCorp",
		ClientUsername:   "testuser",
		ClientFirstName:  "Test",
		ClientLastName:   "User",
		ClientEmail:      "test@example.com",
		AccountID:        42,
		Country:          "US",
		State:            "Texas",
		PostalCode:       "75074",
		MaxZertoStorage:  50,
		MaxZertoVMs:      5,
		Bandwidth:        "100",
		ProductID:        "web-1",
	}

	data, err := argsToData(args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify all JSON fields from the struct tags are present.
	wantKeys := []string{
		"crm_id", "organization_name", "client_username",
		"client_first_name", "client_last_name", "client_email",
		"account_id", "country", "state", "postal_code",
		"max_zerto_storage", "max_zerto_vms", "bandwidth", "product_id",
	}

	for _, key := range wantKeys {
		if _, ok := data[key]; !ok {
			t.Errorf("missing key %q in data", key)
		}
	}
}
