package service

import (
	"context"
	"testing"

	_ "github.com/mattn/go-sqlite3"

	"github.com/logan/cloudcode/internal/ent"
	"github.com/logan/cloudcode/internal/ent/enttest"
	"github.com/logan/cloudcode/internal/provider"
)

func setupTest(t *testing.T) (*InstanceService, *ent.Client) {
	t.Helper()
	client := enttest.Open(t, "sqlite3", "file:ent?mode=memory&_fk=1")
	mock := provider.NewMock()
	svc := NewInstanceService(client, mock, "")
	return svc, client
}

func createTestUser(t *testing.T, client *ent.Client) int {
	t.Helper()
	u, err := client.User.Create().
		SetEmail("test@example.com").
		SetAPIKey("test-key-123").
		Save(context.Background())
	if err != nil {
		t.Fatalf("create test user: %v", err)
	}
	return u.ID
}

func TestInstanceService_Create(t *testing.T) {
	svc, client := setupTest(t)
	defer client.Close()

	userID := createTestUser(t, client)

	inst, err := svc.Create(context.Background(), userID)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if inst.Status != "running" {
		t.Errorf("got status %q, want %q", inst.Status, "running")
	}
	if inst.Provider != "mock" {
		t.Errorf("got provider %q, want %q", inst.Provider, "mock")
	}
}

func TestInstanceService_DuplicateCreate(t *testing.T) {
	svc, client := setupTest(t)
	defer client.Close()

	userID := createTestUser(t, client)

	_, err := svc.Create(context.Background(), userID)
	if err != nil {
		t.Fatalf("first create: %v", err)
	}

	_, err = svc.Create(context.Background(), userID)
	if err == nil {
		t.Fatal("expected error on duplicate create, got nil")
	}
}

func TestInstanceService_GetNotFound(t *testing.T) {
	svc, client := setupTest(t)
	defer client.Close()

	_, err := svc.Get(context.Background(), 9999)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestInstanceService_PauseWake(t *testing.T) {
	svc, client := setupTest(t)
	defer client.Close()

	userID := createTestUser(t, client)

	inst, err := svc.Create(context.Background(), userID)
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	// Pause
	if err := svc.Pause(context.Background(), inst.ID); err != nil {
		t.Fatalf("pause: %v", err)
	}

	got, _ := svc.Get(context.Background(), inst.ID)
	if got.Status != "stopped" {
		t.Errorf("after pause: got status %q, want %q", got.Status, "stopped")
	}

	// Wake
	if err := svc.Wake(context.Background(), inst.ID); err != nil {
		t.Fatalf("wake: %v", err)
	}

	got, _ = svc.Get(context.Background(), inst.ID)
	if got.Status != "running" {
		t.Errorf("after wake: got status %q, want %q", got.Status, "running")
	}
}

func TestInstanceService_Delete(t *testing.T) {
	svc, client := setupTest(t)
	defer client.Close()

	userID := createTestUser(t, client)

	inst, err := svc.Create(context.Background(), userID)
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	if err := svc.Delete(context.Background(), inst.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}

	got, _ := svc.Get(context.Background(), inst.ID)
	if got.Status != "destroyed" {
		t.Errorf("after delete: got status %q, want %q", got.Status, "destroyed")
	}
}

func TestInstanceService_InvalidStateTransitions(t *testing.T) {
	svc, client := setupTest(t)
	defer client.Close()

	userID := createTestUser(t, client)

	inst, _ := svc.Create(context.Background(), userID)

	// Can't wake a running instance
	if err := svc.Wake(context.Background(), inst.ID); err == nil {
		t.Error("expected error waking running instance")
	}

	// Pause it, then can't pause again
	svc.Pause(context.Background(), inst.ID)
	if err := svc.Pause(context.Background(), inst.ID); err == nil {
		t.Error("expected error pausing stopped instance")
	}
}
