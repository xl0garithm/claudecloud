package netbird

import "time"

// Group represents a Netbird peer group.
type Group struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Peers []Peer `json:"peers,omitempty"`
}

// Peer is a minimal peer reference within a group.
type Peer struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// SetupKey is a one-time or reusable key for peer enrollment.
type SetupKey struct {
	ID        string    `json:"id"`
	Key       string    `json:"key"`
	Name      string    `json:"name"`
	Type      string    `json:"type"` // "one-off" or "reusable"
	Revoked   bool      `json:"revoked"`
	UsedTimes int       `json:"used_times"`
	ExpiresIn int       `json:"expires_in"` // seconds, used in create request
	Expires   time.Time `json:"expires"`
	AutoGroups []string `json:"auto_groups"`
	Valid     bool      `json:"valid"`
}

// CreateSetupKeyRequest is the request body for creating a setup key.
type CreateSetupKeyRequest struct {
	Name       string   `json:"name"`
	Type       string   `json:"type"`
	ExpiresIn  int      `json:"expires_in"`
	AutoGroups []string `json:"auto_groups"`
	UsageLimit int      `json:"usage_limit,omitempty"`
}

// Route represents a Netbird network route.
type Route struct {
	ID          string   `json:"id"`
	Description string   `json:"description"`
	NetworkID   string   `json:"network_id"`
	Network     string   `json:"network"`
	Peer        string   `json:"peer,omitempty"`
	PeerGroups  []string `json:"peer_groups,omitempty"`
	Groups      []string `json:"groups"`
	Enabled     bool     `json:"enabled"`
	Masquerade  bool     `json:"masquerade"`
	Metric      int      `json:"metric"`
	NetworkType string   `json:"network_type"`
}

// CreateRouteRequest is the request body for creating a route.
type CreateRouteRequest struct {
	Description string   `json:"description"`
	NetworkID   string   `json:"network_id"`
	Network     string   `json:"network"`
	PeerGroups  []string `json:"peer_groups,omitempty"`
	Groups      []string `json:"groups"`
	Enabled     bool     `json:"enabled"`
	Masquerade  bool     `json:"masquerade"`
	Metric      int      `json:"metric"`
	NetworkType string   `json:"network_type"`
}

// Policy controls access between peer groups.
type Policy struct {
	ID          string       `json:"id"`
	Name        string       `json:"name"`
	Description string       `json:"description"`
	Enabled     bool         `json:"enabled"`
	Rules       []PolicyRule `json:"rules"`
}

// PolicyRule defines a single rule within a policy.
type PolicyRule struct {
	ID            string   `json:"id,omitempty"`
	Name          string   `json:"name"`
	Description   string   `json:"description,omitempty"`
	Enabled       bool     `json:"enabled"`
	Action        string   `json:"action"` // "accept" or "drop"
	Bidirectional bool     `json:"bidirectional"`
	Protocol      string   `json:"protocol"` // "all", "tcp", "udp", "icmp"
	Ports         []string `json:"ports,omitempty"`
	Sources       []string `json:"sources"`
	Destinations  []string `json:"destinations"`
}

// CreatePolicyRequest is the request body for creating a policy.
type CreatePolicyRequest struct {
	Name        string       `json:"name"`
	Description string       `json:"description"`
	Enabled     bool         `json:"enabled"`
	Rules       []PolicyRule `json:"rules"`
}

// APIError represents an error response from the Netbird API.
type APIError struct {
	StatusCode int    `json:"status_code"`
	Message    string `json:"message"`
	Code       int    `json:"code"`
}

func (e *APIError) Error() string {
	return e.Message
}
