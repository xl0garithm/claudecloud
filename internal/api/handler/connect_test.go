package handler

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	_ "github.com/mattn/go-sqlite3"

	"github.com/logan/cloudcode/internal/ent/enttest"
	"github.com/logan/cloudcode/internal/provider"
	"github.com/logan/cloudcode/internal/service"
)

func setupConnectTest(t *testing.T) (*ConnectHandler, *service.InstanceService, int) {
	t.Helper()
	client := enttest.Open(t, "sqlite3", "file:ent_connect?mode=memory&_fk=1")
	t.Cleanup(func() { client.Close() })

	mock := provider.NewMock()
	svc := service.NewInstanceService(client, mock, "")
	ch := NewConnectHandler(svc, "test-jwt-secret")

	// Create test user
	u, err := client.User.Create().
		SetEmail("connect-test@example.com").
		Save(context.Background())
	if err != nil {
		t.Fatalf("create test user: %v", err)
	}

	return ch, svc, u.ID
}

func TestConnectScript_Docker(t *testing.T) {
	ch, svc, userID := setupConnectTest(t)

	// Create instance
	_, err := svc.Create(context.Background(), userID)
	if err != nil {
		t.Fatalf("create instance: %v", err)
	}

	// Request connect script
	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/connect.sh?user_id=%d", userID), nil)
	rec := httptest.NewRecorder()
	ch.ServeScript(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "text/x-shellscript" {
		t.Errorf("expected text/x-shellscript, got %s", ct)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "docker exec") {
		t.Errorf("expected docker exec in script, got:\n%s", body)
	}
	if !strings.Contains(body, "zellij attach claude") {
		t.Errorf("expected zellij attach in script, got:\n%s", body)
	}
}

func TestConnectScript_MissingUserID(t *testing.T) {
	ch, _, _ := setupConnectTest(t)

	req := httptest.NewRequest(http.MethodGet, "/connect.sh", nil)
	rec := httptest.NewRecorder()
	ch.ServeScript(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}

func TestConnectScript_NoInstance(t *testing.T) {
	ch, _, _ := setupConnectTest(t)

	req := httptest.NewRequest(http.MethodGet, "/connect.sh?user_id=9999", nil)
	rec := httptest.NewRecorder()
	ch.ServeScript(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Error:") {
		t.Errorf("expected error script, got:\n%s", body)
	}
}

func TestConnectScript_InvalidUserID(t *testing.T) {
	ch, _, _ := setupConnectTest(t)

	req := httptest.NewRequest(http.MethodGet, "/connect.sh?user_id=abc", nil)
	rec := httptest.NewRecorder()
	ch.ServeScript(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}
