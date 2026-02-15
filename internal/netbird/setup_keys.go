package netbird

import (
	"context"
	"fmt"
	"net/http"
)

// ListSetupKeys returns all setup keys.
func (c *Client) ListSetupKeys(ctx context.Context) ([]SetupKey, error) {
	var keys []SetupKey
	if err := c.do(ctx, http.MethodGet, "/api/setup-keys", nil, &keys); err != nil {
		return nil, fmt.Errorf("list setup keys: %w", err)
	}
	return keys, nil
}

// CreateSetupKey creates a new setup key.
func (c *Client) CreateSetupKey(ctx context.Context, req *CreateSetupKeyRequest) (*SetupKey, error) {
	var key SetupKey
	if err := c.do(ctx, http.MethodPost, "/api/setup-keys", req, &key); err != nil {
		return nil, fmt.Errorf("create setup key: %w", err)
	}
	return &key, nil
}

// RevokeSetupKey revokes a setup key by ID.
func (c *Client) RevokeSetupKey(ctx context.Context, id string) error {
	body := map[string]bool{"revoked": true}
	if err := c.do(ctx, http.MethodPut, "/api/setup-keys/"+id, body, nil); err != nil {
		return fmt.Errorf("revoke setup key %s: %w", id, err)
	}
	return nil
}
