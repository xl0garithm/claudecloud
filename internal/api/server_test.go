package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/logan/cloudcode/internal/config"
	"github.com/logan/cloudcode/internal/service"
)

func TestRoutes(t *testing.T) {
	cfg := &config.Config{APIKey: "test-key"}
	// nil service — we're only testing routing and auth, not handler logic
	var svc *service.InstanceService
	router := NewRouter(cfg, svc)

	t.Run("healthz returns 200", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/healthz", nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("got %d, want %d", rec.Code, http.StatusOK)
		}

		var body map[string]string
		json.NewDecoder(rec.Body).Decode(&body)
		if body["status"] != "ok" {
			t.Errorf("got %q, want %q", body["status"], "ok")
		}
	})

	t.Run("instances without key returns 401", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/instances/", nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("got %d, want %d", rec.Code, http.StatusUnauthorized)
		}
	})
}
