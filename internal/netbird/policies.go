package netbird

import (
	"context"
	"fmt"
	"net/http"
)

// ListPolicies returns all access control policies.
func (c *Client) ListPolicies(ctx context.Context) ([]Policy, error) {
	var policies []Policy
	if err := c.do(ctx, http.MethodGet, "/api/policies", nil, &policies); err != nil {
		return nil, fmt.Errorf("list policies: %w", err)
	}
	return policies, nil
}

// CreatePolicy creates a new access control policy.
func (c *Client) CreatePolicy(ctx context.Context, req *CreatePolicyRequest) (*Policy, error) {
	var policy Policy
	if err := c.do(ctx, http.MethodPost, "/api/policies", req, &policy); err != nil {
		return nil, fmt.Errorf("create policy: %w", err)
	}
	return &policy, nil
}

// DeletePolicy deletes a policy by ID.
func (c *Client) DeletePolicy(ctx context.Context, id string) error {
	if err := c.do(ctx, http.MethodDelete, "/api/policies/"+id, nil, nil); err != nil {
		return fmt.Errorf("delete policy %s: %w", id, err)
	}
	return nil
}
