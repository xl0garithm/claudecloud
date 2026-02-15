package netbird

import (
	"context"
	"fmt"
	"net/http"
)

// ListGroups returns all peer groups.
func (c *Client) ListGroups(ctx context.Context) ([]Group, error) {
	var groups []Group
	if err := c.do(ctx, http.MethodGet, "/api/groups", nil, &groups); err != nil {
		return nil, fmt.Errorf("list groups: %w", err)
	}
	return groups, nil
}

// CreateGroup creates a new peer group.
func (c *Client) CreateGroup(ctx context.Context, name string) (*Group, error) {
	body := map[string]string{"name": name}
	var group Group
	if err := c.do(ctx, http.MethodPost, "/api/groups", body, &group); err != nil {
		return nil, fmt.Errorf("create group: %w", err)
	}
	return &group, nil
}

// DeleteGroup deletes a peer group by ID.
func (c *Client) DeleteGroup(ctx context.Context, id string) error {
	if err := c.do(ctx, http.MethodDelete, "/api/groups/"+id, nil, nil); err != nil {
		return fmt.Errorf("delete group %s: %w", id, err)
	}
	return nil
}
