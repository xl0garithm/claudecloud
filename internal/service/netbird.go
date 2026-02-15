package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/logan/cloudcode/internal/netbird"
)

// NetbirdService orchestrates Netbird zero-trust networking for user instances.
// It provides two-phase provisioning: PrepareNetbirdAccess creates a group and
// setup key before server creation, and FinalizeNetbirdAccess creates the route
// and policy after the server is up.
type NetbirdService struct {
	client *netbird.Client
	logger *slog.Logger
}

// NewNetbirdService creates a new NetbirdService.
func NewNetbirdService(client *netbird.Client, logger *slog.Logger) *NetbirdService {
	return &NetbirdService{client: client, logger: logger}
}

// NetbirdPrep holds the resources created during the prepare phase.
// The setup key is needed during server creation (cloud-init).
type NetbirdPrep struct {
	GroupID  string `json:"group_id"`
	KeyID    string `json:"key_id"`
	SetupKey string `json:"setup_key"`
}

// UserNetbirdConfig holds all Netbird resource IDs for a user.
// Serialized to JSON and stored in the instance's netbird_config field.
type UserNetbirdConfig struct {
	GroupID  string `json:"group_id"`
	KeyID    string `json:"key_id"`
	RouteID  string `json:"route_id"`
	PolicyID string `json:"policy_id"`
}

// PrepareNetbirdAccess creates a peer group and one-off setup key for the user.
// This must be called before the server is created, because cloud-init needs
// the setup key to enroll the instance into Netbird.
func (s *NetbirdService) PrepareNetbirdAccess(ctx context.Context, userID int) (*NetbirdPrep, error) {
	groupName := fmt.Sprintf("user-%d", userID)

	// Create user-specific peer group
	group, err := s.client.CreateGroup(ctx, groupName)
	if err != nil {
		return nil, fmt.Errorf("create group: %w", err)
	}
	s.logger.Info("netbird group created", "group_id", group.ID, "user_id", userID)

	// Create one-off setup key that auto-assigns to the group
	key, err := s.client.CreateSetupKey(ctx, &netbird.CreateSetupKeyRequest{
		Name:       fmt.Sprintf("setup-%d", userID),
		Type:       "one-off",
		ExpiresIn:  3600, // 1 hour
		AutoGroups: []string{group.ID},
		UsageLimit: 1,
	})
	if err != nil {
		// Rollback: delete the group we just created
		_ = s.client.DeleteGroup(ctx, group.ID)
		return nil, fmt.Errorf("create setup key: %w", err)
	}
	s.logger.Info("netbird setup key created", "key_id", key.ID, "user_id", userID)

	return &NetbirdPrep{
		GroupID:  group.ID,
		KeyID:    key.ID,
		SetupKey: key.Key,
	}, nil
}

// FinalizeNetbirdAccess creates the route and policy after the server has
// registered with Netbird using the setup key. Returns the full config to
// be stored in the instance's netbird_config field.
func (s *NetbirdService) FinalizeNetbirdAccess(ctx context.Context, userID int, prep *NetbirdPrep) (*UserNetbirdConfig, error) {
	subnetOctet := (userID % 250) + 1
	network := fmt.Sprintf("10.100.%d.0/24", subnetOctet)

	// Create route so the user's Netbird peer can reach the instance subnet
	route, err := s.client.CreateRoute(ctx, &netbird.CreateRouteRequest{
		Description: fmt.Sprintf("Route for user %d", userID),
		NetworkID:   fmt.Sprintf("user-%d-net", userID),
		Network:     network,
		PeerGroups:  []string{prep.GroupID},
		Groups:      []string{prep.GroupID},
		Enabled:     true,
		Masquerade:  true,
		Metric:      9999,
		NetworkType: "IPv4",
	})
	if err != nil {
		return nil, fmt.Errorf("create route: %w", err)
	}
	s.logger.Info("netbird route created", "route_id", route.ID, "user_id", userID)

	// Create policy allowing bidirectional traffic within the user's group
	policy, err := s.client.CreatePolicy(ctx, &netbird.CreatePolicyRequest{
		Name:        fmt.Sprintf("allow-user-%d", userID),
		Description: fmt.Sprintf("Allow traffic for user %d instances", userID),
		Enabled:     true,
		Rules: []netbird.PolicyRule{
			{
				Name:          fmt.Sprintf("user-%d-all", userID),
				Enabled:       true,
				Action:        "accept",
				Bidirectional: true,
				Protocol:      "all",
				Sources:       []string{prep.GroupID},
				Destinations:  []string{prep.GroupID},
			},
		},
	})
	if err != nil {
		// Rollback: delete the route
		_ = s.client.DeleteRoute(ctx, route.ID)
		return nil, fmt.Errorf("create policy: %w", err)
	}
	s.logger.Info("netbird policy created", "policy_id", policy.ID, "user_id", userID)

	return &UserNetbirdConfig{
		GroupID:  prep.GroupID,
		KeyID:    prep.KeyID,
		RouteID:  route.ID,
		PolicyID: policy.ID,
	}, nil
}

// TeardownUser removes all Netbird resources for a user in reverse order.
func (s *NetbirdService) TeardownUser(ctx context.Context, cfg *UserNetbirdConfig) error {
	var firstErr error

	// Delete policy first
	if cfg.PolicyID != "" {
		if err := s.client.DeletePolicy(ctx, cfg.PolicyID); err != nil {
			s.logger.Error("failed to delete netbird policy", "policy_id", cfg.PolicyID, "error", err)
			if firstErr == nil {
				firstErr = err
			}
		}
	}

	// Delete route
	if cfg.RouteID != "" {
		if err := s.client.DeleteRoute(ctx, cfg.RouteID); err != nil {
			s.logger.Error("failed to delete netbird route", "route_id", cfg.RouteID, "error", err)
			if firstErr == nil {
				firstErr = err
			}
		}
	}

	// Delete group last
	if cfg.GroupID != "" {
		if err := s.client.DeleteGroup(ctx, cfg.GroupID); err != nil {
			s.logger.Error("failed to delete netbird group", "group_id", cfg.GroupID, "error", err)
			if firstErr == nil {
				firstErr = err
			}
		}
	}

	return firstErr
}

// CleanupExpiredKeys revokes expired or used setup keys.
func (s *NetbirdService) CleanupExpiredKeys(ctx context.Context) error {
	keys, err := s.client.ListSetupKeys(ctx)
	if err != nil {
		return fmt.Errorf("list setup keys: %w", err)
	}

	for _, key := range keys {
		if !key.Valid && !key.Revoked {
			if err := s.client.RevokeSetupKey(ctx, key.ID); err != nil {
				s.logger.Error("failed to revoke expired key", "key_id", key.ID, "error", err)
				continue
			}
			s.logger.Info("revoked expired netbird key", "key_id", key.ID)
		}
	}

	return nil
}

// MarshalConfig serializes a UserNetbirdConfig to JSON for DB storage.
func MarshalNetbirdConfig(cfg *UserNetbirdConfig) string {
	b, _ := json.Marshal(cfg)
	return string(b)
}

// UnmarshalNetbirdConfig deserializes a UserNetbirdConfig from a JSON string.
func UnmarshalNetbirdConfig(data string) (*UserNetbirdConfig, error) {
	if data == "" {
		return nil, nil
	}
	var cfg UserNetbirdConfig
	if err := json.Unmarshal([]byte(data), &cfg); err != nil {
		return nil, fmt.Errorf("unmarshal netbird config: %w", err)
	}
	return &cfg, nil
}
