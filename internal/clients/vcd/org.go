package vcd

import (
	"context"
	"fmt"
	"net/url"
)

// GetOrganization retrieves a VCD organization by name.
// Returns an error if no organization matches the given name.
func (c *Client) GetOrganization(ctx context.Context, orgName string) (*Organization, error) {
	params := url.Values{}
	params.Set("filter", fmt.Sprintf("name==%s", orgName))

	path := "/cloudapi/1.0.0/orgs?" + params.Encode()

	var result organizationResult
	if err := c.do(ctx, "GET", path, nil, &result); err != nil {
		return nil, fmt.Errorf("get organization %q: %w", orgName, err)
	}

	if len(result.Values) == 0 {
		return nil, fmt.Errorf("organization %q not found", orgName)
	}

	return &result.Values[0], nil
}
