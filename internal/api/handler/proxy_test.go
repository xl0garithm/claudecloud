package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	_ "github.com/mattn/go-sqlite3"

	"github.com/logan/cloudcode/internal/api/middleware"
	"github.com/logan/cloudcode/internal/ent/enttest"
	"github.com/logan/cloudcode/internal/provider"
	"github.com/logan/cloudcode/internal/service"
)

func setupProxyTest(t *testing.T) (*ProxyHandler, *service.InstanceService, int) {
	t.Helper()
	client := enttest.Open(t, "sqlite3", "file:ent_proxy?mode=memory&_fk=1")
	t.Cleanup(func() { client.Close() })

	mock := provider.NewMock()
	svc := service.NewInstanceService(client, mock, "")
	ph := NewProxyHandler(svc, "test-jwt-secret")

	u, err := client.User.Create().
		SetEmail("proxy-test@example.com").
		Save(context.Background())
	if err != nil {
		t.Fatalf("create test user: %v", err)
	}

	return ph, svc, u.ID
}

func TestGetMine_NoInstance(t *testing.T) {
	client := enttest.Open(t, "sqlite3", "file:ent_mine?mode=memory&_fk=1")
	t.Cleanup(func() { client.Close() })

	mock := provider.NewMock()
	svc := service.NewInstanceService(client, mock, "")

	u, err := client.User.Create().
		SetEmail("mine-test@example.com").
		Save(context.Background())
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	handler := GetMine(svc)

	req := httptest.NewRequest("GET", "/instances/mine", nil)
	ctx := context.WithValue(req.Context(), middleware.TestUserIDKey(), u.ID)
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestGetMine_WithInstance(t *testing.T) {
	client := enttest.Open(t, "sqlite3", "file:ent_mine2?mode=memory&_fk=1")
	t.Cleanup(func() { client.Close() })

	mock := provider.NewMock()
	svc := service.NewInstanceService(client, mock, "")

	u, err := client.User.Create().
		SetEmail("mine-test2@example.com").
		Save(context.Background())
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	// Create an instance
	_, err = svc.Create(context.Background(), u.ID)
	if err != nil {
		t.Fatalf("create instance: %v", err)
	}

	handler := GetMine(svc)

	req := httptest.NewRequest("GET", "/instances/mine", nil)
	ctx := context.WithValue(req.Context(), middleware.TestUserIDKey(), u.ID)
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestProxy_Unauthorized(t *testing.T) {
	ph, _, _ := setupProxyTest(t)

	// Request without auth context
	req := httptest.NewRequest("GET", "/instances/1/files", nil)

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "1")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	rr := httptest.NewRecorder()
	ph.Files(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rr.Code)
	}
}
