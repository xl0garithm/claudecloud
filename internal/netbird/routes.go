package netbird

import (
	"context"
	"fmt"
	"net/http"
)

// ListRoutes returns all network routes.
func (c *Client) ListRoutes(ctx context.Context) ([]Route, error) {
	var routes []Route
	if err := c.do(ctx, http.MethodGet, "/api/routes", nil, &routes); err != nil {
		return nil, fmt.Errorf("list routes: %w", err)
	}
	return routes, nil
}

// CreateRoute creates a new network route.
func (c *Client) CreateRoute(ctx context.Context, req *CreateRouteRequest) (*Route, error) {
	var route Route
	if err := c.do(ctx, http.MethodPost, "/api/routes", req, &route); err != nil {
		return nil, fmt.Errorf("create route: %w", err)
	}
	return &route, nil
}

// DeleteRoute deletes a route by ID.
func (c *Client) DeleteRoute(ctx context.Context, id string) error {
	if err := c.do(ctx, http.MethodDelete, "/api/routes/"+id, nil, nil); err != nil {
		return fmt.Errorf("delete route %s: %w", id, err)
	}
	return nil
}
