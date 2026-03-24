package vcd

import (
	"context"
	"fmt"
	"net/url"
)

// GetVDC retrieves a Virtual Data Center by name.
// Returns an error if no VDC matches the given name.
func (c *Client) GetVDC(ctx context.Context, vdcName string) (*VDC, error) {
	params := url.Values{}
	params.Set("filter", fmt.Sprintf("name==%s", vdcName))
	params.Set("page", "1")
	params.Set("pageSize", "25")

	path := "/cloudapi/1.0.0/vdcs?" + params.Encode()

	var result vdcResponse
	if err := c.do(ctx, "GET", path, nil, &result); err != nil {
		return nil, fmt.Errorf("get vdc %q: %w", vdcName, err)
	}

	if len(result.Values) == 0 {
		return nil, fmt.Errorf("vdc %q not found", vdcName)
	}

	return &result.Values[0], nil
}
