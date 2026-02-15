package netbird

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func setupMockServer(t *testing.T, handler http.HandlerFunc) (*Client, *httptest.Server) {
	t.Helper()
	server := httptest.NewServer(handler)
	client := New(server.URL, "test-token")
	return client, server
}

func TestCreateGroup(t *testing.T) {
	client, server := setupMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/api/groups" {
			t.Errorf("expected /api/groups, got %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Token test-token" {
			t.Errorf("missing or wrong auth header")
		}

		var body map[string]string
		json.NewDecoder(r.Body).Decode(&body)
		if body["name"] != "user-42" {
			t.Errorf("expected name user-42, got %s", body["name"])
		}

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(Group{ID: "grp-1", Name: "user-42"})
	})
	defer server.Close()

	group, err := client.CreateGroup(context.Background(), "user-42")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if group.ID != "grp-1" {
		t.Errorf("expected ID grp-1, got %s", group.ID)
	}
	if group.Name != "user-42" {
		t.Errorf("expected name user-42, got %s", group.Name)
	}
}

func TestListGroups(t *testing.T) {
	client, server := setupMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("expected GET, got %s", r.Method)
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode([]Group{{ID: "grp-1", Name: "test"}})
	})
	defer server.Close()

	groups, err := client.ListGroups(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(groups) != 1 {
		t.Fatalf("expected 1 group, got %d", len(groups))
	}
}

func TestDeleteGroup(t *testing.T) {
	client, server := setupMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("expected DELETE, got %s", r.Method)
		}
		if r.URL.Path != "/api/groups/grp-1" {
			t.Errorf("expected /api/groups/grp-1, got %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
	})
	defer server.Close()

	if err := client.DeleteGroup(context.Background(), "grp-1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCreateSetupKey(t *testing.T) {
	client, server := setupMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/api/setup-keys" {
			t.Errorf("expected /api/setup-keys, got %s", r.URL.Path)
		}

		var body CreateSetupKeyRequest
		json.NewDecoder(r.Body).Decode(&body)
		if body.Type != "one-off" {
			t.Errorf("expected type one-off, got %s", body.Type)
		}

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(SetupKey{ID: "key-1", Key: "secret-key", Type: "one-off"})
	})
	defer server.Close()

	key, err := client.CreateSetupKey(context.Background(), &CreateSetupKeyRequest{
		Name:       "user-42",
		Type:       "one-off",
		ExpiresIn:  3600,
		AutoGroups: []string{"grp-1"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if key.Key != "secret-key" {
		t.Errorf("expected key secret-key, got %s", key.Key)
	}
}

func TestRevokeSetupKey(t *testing.T) {
	client, server := setupMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("expected PUT, got %s", r.Method)
		}
		if r.URL.Path != "/api/setup-keys/key-1" {
			t.Errorf("expected /api/setup-keys/key-1, got %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
	})
	defer server.Close()

	if err := client.RevokeSetupKey(context.Background(), "key-1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCreateRoute(t *testing.T) {
	client, server := setupMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}

		var body CreateRouteRequest
		json.NewDecoder(r.Body).Decode(&body)
		if body.NetworkID != "net-42" {
			t.Errorf("expected network_id net-42, got %s", body.NetworkID)
		}

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(Route{ID: "route-1", NetworkID: "net-42", Enabled: true})
	})
	defer server.Close()

	route, err := client.CreateRoute(context.Background(), &CreateRouteRequest{
		Description: "test route",
		NetworkID:   "net-42",
		Network:     "10.100.1.0/24",
		PeerGroups:  []string{"grp-1"},
		Groups:      []string{"grp-1"},
		Enabled:     true,
		Masquerade:  true,
		Metric:      9999,
		NetworkType: "IPv4",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if route.ID != "route-1" {
		t.Errorf("expected ID route-1, got %s", route.ID)
	}
}

func TestDeleteRoute(t *testing.T) {
	client, server := setupMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("expected DELETE, got %s", r.Method)
		}
		w.WriteHeader(http.StatusOK)
	})
	defer server.Close()

	if err := client.DeleteRoute(context.Background(), "route-1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCreatePolicy(t *testing.T) {
	client, server := setupMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}

		var body CreatePolicyRequest
		json.NewDecoder(r.Body).Decode(&body)
		if body.Name != "allow-user-42" {
			t.Errorf("expected name allow-user-42, got %s", body.Name)
		}
		if len(body.Rules) != 1 {
			t.Fatalf("expected 1 rule, got %d", len(body.Rules))
		}

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(Policy{ID: "pol-1", Name: body.Name, Enabled: true, Rules: body.Rules})
	})
	defer server.Close()

	policy, err := client.CreatePolicy(context.Background(), &CreatePolicyRequest{
		Name:        "allow-user-42",
		Description: "Allow traffic for user 42",
		Enabled:     true,
		Rules: []PolicyRule{
			{
				Name:          "allow-all",
				Enabled:       true,
				Action:        "accept",
				Bidirectional: true,
				Protocol:      "all",
				Sources:       []string{"grp-1"},
				Destinations:  []string{"grp-1"},
			},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if policy.ID != "pol-1" {
		t.Errorf("expected ID pol-1, got %s", policy.ID)
	}
}

func TestDeletePolicy(t *testing.T) {
	client, server := setupMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("expected DELETE, got %s", r.Method)
		}
		w.WriteHeader(http.StatusOK)
	})
	defer server.Close()

	if err := client.DeletePolicy(context.Background(), "pol-1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAPIError(t *testing.T) {
	client, server := setupMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		json.NewEncoder(w).Encode(APIError{StatusCode: 403, Message: "forbidden"})
	})
	defer server.Close()

	_, err := client.ListGroups(context.Background())
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	apiErr, ok := err.(*APIError)
	if !ok {
		// The error is wrapped, check the message
		if err.Error() == "" {
			t.Error("expected non-empty error message")
		}
		return
	}
	if apiErr.StatusCode != 403 {
		t.Errorf("expected status 403, got %d", apiErr.StatusCode)
	}
}

func TestListSetupKeys(t *testing.T) {
	client, server := setupMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode([]SetupKey{{ID: "key-1", Valid: true}})
	})
	defer server.Close()

	keys, err := client.ListSetupKeys(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(keys) != 1 {
		t.Fatalf("expected 1 key, got %d", len(keys))
	}
}

func TestListRoutes(t *testing.T) {
	client, server := setupMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode([]Route{{ID: "route-1"}})
	})
	defer server.Close()

	routes, err := client.ListRoutes(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(routes) != 1 {
		t.Fatalf("expected 1 route, got %d", len(routes))
	}
}

func TestListPolicies(t *testing.T) {
	client, server := setupMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode([]Policy{{ID: "pol-1"}})
	})
	defer server.Close()

	policies, err := client.ListPolicies(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(policies) != 1 {
		t.Fatalf("expected 1 policy, got %d", len(policies))
	}
}
