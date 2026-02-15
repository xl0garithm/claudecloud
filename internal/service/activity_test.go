package service

import (
	"context"
	"log/slog"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"

	"github.com/logan/cloudcode/internal/ent"
	"github.com/logan/cloudcode/internal/ent/enttest"
	"github.com/logan/cloudcode/internal/provider"
)

func setupActivityTest(t *testing.T) (*ActivityService, *InstanceService, *ent.Client, *provider.MockProvisioner) {
	t.Helper()
	client := enttest.Open(t, "sqlite3", "file:ent_activity?mode=memory&_fk=1")
	mock := provider.NewMock()
	logger := slog.Default()

	instSvc := NewInstanceService(client, mock, "")
	actSvc := NewActivityService(client, mock, logger, time.Minute, 2*time.Hour)

	return actSvc, instSvc, client, mock
}

func TestActivityService_ActiveNotPaused(t *testing.T) {
	actSvc, instSvc, client, _ := setupActivityTest(t)
	defer client.Close()

	userID := createTestUser(t, client)
	inst, err := instSvc.Create(context.Background(), userID)
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	// Get the ent instance for checking
	entInst, _ := client.Instance.Get(context.Background(), inst.ID)

	// Mock returns active = true, so instance should NOT be paused
	actSvc.CheckInstance(context.Background(), entInst, time.Now())

	got, _ := instSvc.Get(context.Background(), inst.ID)
	if got.Status != "running" {
		t.Errorf("expected running, got %s", got.Status)
	}

	// Verify last_activity_at was updated
	entInst, _ = client.Instance.Get(context.Background(), inst.ID)
	if entInst.LastActivityAt == nil {
		t.Error("expected last_activity_at to be set after active check")
	}
}

func TestActivityService_IdleAutoPaused(t *testing.T) {
	actSvc, instSvc, client, mock := setupActivityTest(t)
	defer client.Close()

	userID := createTestUser(t, client)
	inst, err := instSvc.Create(context.Background(), userID)
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	// Set last activity to 3 hours ago (past the 2h threshold)
	threeHoursAgo := time.Now().Add(-3 * time.Hour)
	_, err = client.Instance.UpdateOneID(inst.ID).
		SetLastActivityAt(threeHoursAgo).
		Save(context.Background())
	if err != nil {
		t.Fatalf("update last_activity_at: %v", err)
	}

	// Make mock return inactive
	mock.SetInactive(inst.ProviderID)

	entInst, _ := client.Instance.Get(context.Background(), inst.ID)
	actSvc.CheckInstance(context.Background(), entInst, time.Now())

	got, _ := instSvc.Get(context.Background(), inst.ID)
	if got.Status != "stopped" {
		t.Errorf("expected stopped (auto-paused), got %s", got.Status)
	}
}

func TestActivityService_RecentlyActiveNotPaused(t *testing.T) {
	actSvc, instSvc, client, mock := setupActivityTest(t)
	defer client.Close()

	userID := createTestUser(t, client)
	inst, err := instSvc.Create(context.Background(), userID)
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	// Set last activity to 30 minutes ago (within 2h threshold)
	recentActivity := time.Now().Add(-30 * time.Minute)
	_, err = client.Instance.UpdateOneID(inst.ID).
		SetLastActivityAt(recentActivity).
		Save(context.Background())
	if err != nil {
		t.Fatalf("update last_activity_at: %v", err)
	}

	// Make mock return inactive but recently active
	mock.SetInactive(inst.ProviderID)

	entInst, _ := client.Instance.Get(context.Background(), inst.ID)
	actSvc.CheckInstance(context.Background(), entInst, time.Now())

	got, _ := instSvc.Get(context.Background(), inst.ID)
	if got.Status != "running" {
		t.Errorf("expected running (not idle enough), got %s", got.Status)
	}
}
