package service

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/logan/cloudcode/internal/netbird"
)

// mockNetbirdHandler tracks created resources and serves the Netbird API.
type mockNetbirdHandler struct {
	mu       sync.Mutex
	groups   map[string]netbird.Group
	keys     map[string]netbird.SetupKey
	routes   map[string]netbird.Route
	policies map[string]netbird.Policy
	nextID   int
}

func newMockNetbirdHandler() *mockNetbirdHandler {
	return &mockNetbirdHandler{
		groups:   make(map[string]netbird.Group),
		keys:     make(map[string]netbird.SetupKey),
		routes:   make(map[string]netbird.Route),
		policies: make(map[string]netbird.Policy),
	}
}

func (m *mockNetbirdHandler) genID(prefix string) string {
	m.nextID++
	return prefix + "-" + strings.Repeat("x", 3) + string(rune('0'+m.nextID))
}

func (m *mockNetbirdHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	m.mu.Lock()
	defer m.mu.Unlock()

	path := r.URL.Path

	switch {
	// Groups
	case path == "/api/groups" && r.Method == http.MethodPost:
		var body map[string]string
		json.NewDecoder(r.Body).Decode(&body)
		id := m.genID("grp")
		g := netbird.Group{ID: id, Name: body["name"]}
		m.groups[id] = g
		json.NewEncoder(w).Encode(g)

	case path == "/api/groups" && r.Method == http.MethodGet:
		var groups []netbird.Group
		for _, g := range m.groups {
			groups = append(groups, g)
		}
		json.NewEncoder(w).Encode(groups)

	case strings.HasPrefix(path, "/api/groups/") && r.Method == http.MethodDelete:
		id := strings.TrimPrefix(path, "/api/groups/")
		delete(m.groups, id)
		w.WriteHeader(http.StatusOK)

	// Setup Keys
	case path == "/api/setup-keys" && r.Method == http.MethodPost:
		var body netbird.CreateSetupKeyRequest
		json.NewDecoder(r.Body).Decode(&body)
		id := m.genID("key")
		k := netbird.SetupKey{ID: id, Key: "nb-setup-" + id, Name: body.Name, Type: body.Type, AutoGroups: body.AutoGroups, Valid: true}
		m.keys[id] = k
		json.NewEncoder(w).Encode(k)

	case path == "/api/setup-keys" && r.Method == http.MethodGet:
		var keys []netbird.SetupKey
		for _, k := range m.keys {
			keys = append(keys, k)
		}
		json.NewEncoder(w).Encode(keys)

	case strings.HasPrefix(path, "/api/setup-keys/") && r.Method == http.MethodPut:
		id := strings.TrimPrefix(path, "/api/setup-keys/")
		if k, ok := m.keys[id]; ok {
			k.Revoked = true
			m.keys[id] = k
		}
		w.WriteHeader(http.StatusOK)

	// Routes
	case path == "/api/routes" && r.Method == http.MethodPost:
		var body netbird.CreateRouteRequest
		json.NewDecoder(r.Body).Decode(&body)
		id := m.genID("rt")
		rt := netbird.Route{ID: id, NetworkID: body.NetworkID, Network: body.Network, Enabled: body.Enabled}
		m.routes[id] = rt
		json.NewEncoder(w).Encode(rt)

	case path == "/api/routes" && r.Method == http.MethodGet:
		var routes []netbird.Route
		for _, rt := range m.routes {
			routes = append(routes, rt)
		}
		json.NewEncoder(w).Encode(routes)

	case strings.HasPrefix(path, "/api/routes/") && r.Method == http.MethodDelete:
		id := strings.TrimPrefix(path, "/api/routes/")
		delete(m.routes, id)
		w.WriteHeader(http.StatusOK)

	// Policies
	case path == "/api/policies" && r.Method == http.MethodPost:
		var body netbird.CreatePolicyRequest
		json.NewDecoder(r.Body).Decode(&body)
		id := m.genID("pol")
		p := netbird.Policy{ID: id, Name: body.Name, Enabled: body.Enabled, Rules: body.Rules}
		m.policies[id] = p
		json.NewEncoder(w).Encode(p)

	case path == "/api/policies" && r.Method == http.MethodGet:
		var policies []netbird.Policy
		for _, p := range m.policies {
			policies = append(policies, p)
		}
		json.NewEncoder(w).Encode(policies)

	case strings.HasPrefix(path, "/api/policies/") && r.Method == http.MethodDelete:
		id := strings.TrimPrefix(path, "/api/policies/")
		delete(m.policies, id)
		w.WriteHeader(http.StatusOK)

	default:
		w.WriteHeader(http.StatusNotFound)
	}
}

func setupNetbirdTest(t *testing.T) (*NetbirdService, *mockNetbirdHandler, *httptest.Server) {
	t.Helper()
	handler := newMockNetbirdHandler()
	server := httptest.NewServer(handler)
	client := netbird.New(server.URL, "test-token")
	logger := log.New(os.Stderr, "test: ", 0)
	svc := NewNetbirdService(client, logger)
	return svc, handler, server
}

func TestPrepareNetbirdAccess(t *testing.T) {
	svc, handler, server := setupNetbirdTest(t)
	defer server.Close()

	prep, err := svc.PrepareNetbirdAccess(context.Background(), 42)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if prep.GroupID == "" {
		t.Error("expected non-empty group ID")
	}
	if prep.KeyID == "" {
		t.Error("expected non-empty key ID")
	}
	if prep.SetupKey == "" {
		t.Error("expected non-empty setup key")
	}

	// Verify resources were created
	handler.mu.Lock()
	defer handler.mu.Unlock()
	if len(handler.groups) != 1 {
		t.Errorf("expected 1 group, got %d", len(handler.groups))
	}
	if len(handler.keys) != 1 {
		t.Errorf("expected 1 key, got %d", len(handler.keys))
	}
}

func TestFinalizeNetbirdAccess(t *testing.T) {
	svc, handler, server := setupNetbirdTest(t)
	defer server.Close()

	prep, err := svc.PrepareNetbirdAccess(context.Background(), 42)
	if err != nil {
		t.Fatalf("prepare: %v", err)
	}

	cfg, err := svc.FinalizeNetbirdAccess(context.Background(), 42, prep)
	if err != nil {
		t.Fatalf("finalize: %v", err)
	}

	if cfg.GroupID != prep.GroupID {
		t.Errorf("group ID mismatch: %s vs %s", cfg.GroupID, prep.GroupID)
	}
	if cfg.RouteID == "" {
		t.Error("expected non-empty route ID")
	}
	if cfg.PolicyID == "" {
		t.Error("expected non-empty policy ID")
	}

	handler.mu.Lock()
	defer handler.mu.Unlock()
	if len(handler.routes) != 1 {
		t.Errorf("expected 1 route, got %d", len(handler.routes))
	}
	if len(handler.policies) != 1 {
		t.Errorf("expected 1 policy, got %d", len(handler.policies))
	}
}

func TestTeardownUser(t *testing.T) {
	svc, handler, server := setupNetbirdTest(t)
	defer server.Close()

	// Full provision cycle
	prep, _ := svc.PrepareNetbirdAccess(context.Background(), 42)
	cfg, _ := svc.FinalizeNetbirdAccess(context.Background(), 42, prep)

	// Teardown
	if err := svc.TeardownUser(context.Background(), cfg); err != nil {
		t.Fatalf("teardown: %v", err)
	}

	handler.mu.Lock()
	defer handler.mu.Unlock()
	if len(handler.groups) != 0 {
		t.Errorf("expected 0 groups after teardown, got %d", len(handler.groups))
	}
	if len(handler.routes) != 0 {
		t.Errorf("expected 0 routes after teardown, got %d", len(handler.routes))
	}
	if len(handler.policies) != 0 {
		t.Errorf("expected 0 policies after teardown, got %d", len(handler.policies))
	}
}

func TestCleanupExpiredKeys(t *testing.T) {
	svc, handler, server := setupNetbirdTest(t)
	defer server.Close()

	// Create a key and mark it as invalid (expired/used)
	prep, _ := svc.PrepareNetbirdAccess(context.Background(), 42)

	handler.mu.Lock()
	k := handler.keys[prep.KeyID]
	k.Valid = false
	k.Revoked = false
	handler.keys[prep.KeyID] = k
	handler.mu.Unlock()

	// Run cleanup
	if err := svc.CleanupExpiredKeys(context.Background()); err != nil {
		t.Fatalf("cleanup: %v", err)
	}

	// Verify it was revoked
	handler.mu.Lock()
	defer handler.mu.Unlock()
	if !handler.keys[prep.KeyID].Revoked {
		t.Error("expected key to be revoked after cleanup")
	}
}

func TestMarshalUnmarshalNetbirdConfig(t *testing.T) {
	original := &UserNetbirdConfig{
		GroupID:  "grp-1",
		KeyID:    "key-1",
		RouteID:  "rt-1",
		PolicyID: "pol-1",
	}

	data := MarshalNetbirdConfig(original)
	if data == "" {
		t.Fatal("expected non-empty JSON")
	}

	parsed, err := UnmarshalNetbirdConfig(data)
	if err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if parsed.GroupID != original.GroupID || parsed.PolicyID != original.PolicyID {
		t.Error("round-trip mismatch")
	}
}

func TestUnmarshalNetbirdConfigEmpty(t *testing.T) {
	cfg, err := UnmarshalNetbirdConfig("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg != nil {
		t.Error("expected nil for empty string")
	}
}

func TestPrepareRollbackOnKeyFailure(t *testing.T) {
	// Use a server that fails on setup-key creation
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/groups" && r.Method == http.MethodPost {
			json.NewEncoder(w).Encode(netbird.Group{ID: "grp-rollback", Name: "test"})
			return
		}
		if r.URL.Path == "/api/setup-keys" && r.Method == http.MethodPost {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(netbird.APIError{StatusCode: 500, Message: "internal error"})
			return
		}
		if strings.HasPrefix(r.URL.Path, "/api/groups/") && r.Method == http.MethodDelete {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusOK)
	})
	server := httptest.NewServer(handler)
	defer server.Close()

	client := netbird.New(server.URL, "test-token")
	logger := log.New(os.Stderr, "test: ", 0)
	svc := NewNetbirdService(client, logger)

	_, err := svc.PrepareNetbirdAccess(context.Background(), 99)
	if err == nil {
		t.Fatal("expected error when setup key creation fails")
	}
}
