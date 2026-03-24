package vcd

import (
	"encoding/json"
	"fmt"
	"io"
)

// decodeJSON reads from r and unmarshals JSON into v.
func decodeJSON(r io.Reader, v any) error {
	data, err := io.ReadAll(r)
	if err != nil {
		return fmt.Errorf("read response: %w", err)
	}
	if err := json.Unmarshal(data, v); err != nil {
		return fmt.Errorf("decode json: %w", err)
	}
	return nil
}
