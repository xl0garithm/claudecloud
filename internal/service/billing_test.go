package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"testing"

	"github.com/stripe/stripe-go/v82"

	"github.com/logan/cloudcode/internal/ent/enttest"
	"github.com/logan/cloudcode/internal/provider"

	_ "github.com/mattn/go-sqlite3"
)

func newTestBillingService(t *testing.T) (*BillingService, *provider.MockProvisioner) {
	t.Helper()
	client := enttest.Open(t, "sqlite3", "file:ent?mode=memory&_fk=1")
	t.Cleanup(func() { client.Close() })

	mock := provider.NewMock()
	instanceSvc := NewInstanceService(client, mock, "")
	logger := log.New(os.Stderr, "test: ", 0)

	billing := &BillingService{
		db:            client,
		instanceSvc:   instanceSvc,
		webhookSecret: "whsec_test",
		priceStarter:  "price_starter",
		pricePro:      "price_pro",
		frontendURL:   "http://localhost:3000",
		logger:        logger,
	}

	return billing, mock
}

func TestProcessEvent_CheckoutCompleted(t *testing.T) {
	svc, _ := newTestBillingService(t)
	ctx := context.Background()

	// Create a user
	u, err := svc.db.User.Create().SetEmail("test@example.com").Save(ctx)
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	// Construct checkout.session.completed event
	sessData, _ := json.Marshal(map[string]interface{}{
		"id":           "cs_test_123",
		"customer":     map[string]interface{}{"id": "cus_123"},
		"subscription": map[string]interface{}{"id": "sub_123"},
		"metadata": map[string]string{
			"user_id": formatID(u.ID),
			"plan":    "starter",
		},
	})

	event := stripe.Event{
		Type: "checkout.session.completed",
		Data: &stripe.EventData{
			Raw: json.RawMessage(sessData),
		},
	}

	if err := svc.processEvent(event); err != nil {
		t.Fatalf("processEvent: %v", err)
	}

	// Verify user was updated
	u, _ = svc.db.User.Get(ctx, u.ID)
	if u.SubscriptionStatus != "active" {
		t.Errorf("subscription_status = %s, want active", u.SubscriptionStatus)
	}
	if u.Plan != "starter" {
		t.Errorf("plan = %s, want starter", u.Plan)
	}
	if u.StripeCustomerID == nil || *u.StripeCustomerID != "cus_123" {
		t.Errorf("stripe_customer_id = %v, want cus_123", u.StripeCustomerID)
	}
	if u.StripeSubscriptionID == nil || *u.StripeSubscriptionID != "sub_123" {
		t.Errorf("stripe_subscription_id = %v, want sub_123", u.StripeSubscriptionID)
	}
}

func TestProcessEvent_SubscriptionDeleted(t *testing.T) {
	svc, _ := newTestBillingService(t)
	ctx := context.Background()

	// Create user with active subscription
	customerID := "cus_456"
	u, _ := svc.db.User.Create().
		SetEmail("test@example.com").
		SetStripeCustomerID(customerID).
		SetSubscriptionStatus("active").
		SetPlan("starter").
		Save(ctx)

	// Create an instance for the user (so we can test pause)
	_, err := svc.instanceSvc.Create(ctx, u.ID)
	if err != nil {
		t.Fatalf("create instance: %v", err)
	}

	// Construct customer.subscription.deleted event
	subData, _ := json.Marshal(map[string]interface{}{
		"id":       "sub_456",
		"customer": map[string]interface{}{"id": customerID},
		"status":   "canceled",
	})

	event := stripe.Event{
		Type: "customer.subscription.deleted",
		Data: &stripe.EventData{
			Raw: json.RawMessage(subData),
		},
	}

	if err := svc.processEvent(event); err != nil {
		t.Fatalf("processEvent: %v", err)
	}

	// Verify user was updated
	u, _ = svc.db.User.Get(ctx, u.ID)
	if u.SubscriptionStatus != "canceled" {
		t.Errorf("subscription_status = %s, want canceled", u.SubscriptionStatus)
	}

	// Verify instance was paused
	inst, _ := svc.instanceSvc.GetByUserID(ctx, u.ID)
	if inst != nil && inst.Status != "stopped" {
		t.Errorf("instance status = %s, want stopped", inst.Status)
	}
}

func TestProcessEvent_PaymentFailed(t *testing.T) {
	svc, _ := newTestBillingService(t)
	ctx := context.Background()

	customerID := "cus_789"
	u, _ := svc.db.User.Create().
		SetEmail("test@example.com").
		SetStripeCustomerID(customerID).
		SetSubscriptionStatus("active").
		Save(ctx)

	invoiceData, _ := json.Marshal(map[string]interface{}{
		"id":       "in_789",
		"customer": map[string]interface{}{"id": customerID},
	})

	event := stripe.Event{
		Type: "invoice.payment_failed",
		Data: &stripe.EventData{
			Raw: json.RawMessage(invoiceData),
		},
	}

	if err := svc.processEvent(event); err != nil {
		t.Fatalf("processEvent: %v", err)
	}

	u, _ = svc.db.User.Get(ctx, u.ID)
	if u.SubscriptionStatus != "past_due" {
		t.Errorf("subscription_status = %s, want past_due", u.SubscriptionStatus)
	}
}

func TestReportUsage(t *testing.T) {
	svc, _ := newTestBillingService(t)
	ctx := context.Background()

	u, _ := svc.db.User.Create().SetEmail("test@example.com").Save(ctx)

	if err := svc.ReportUsage(ctx, u.ID, 1.5); err != nil {
		t.Fatalf("report: %v", err)
	}

	u, _ = svc.db.User.Get(ctx, u.ID)
	if u.UsageHours != 1.5 {
		t.Errorf("usage_hours = %f, want 1.5", u.UsageHours)
	}

	// Add more
	if err := svc.ReportUsage(ctx, u.ID, 0.5); err != nil {
		t.Fatalf("report: %v", err)
	}

	u, _ = svc.db.User.Get(ctx, u.ID)
	if u.UsageHours != 2.0 {
		t.Errorf("usage_hours = %f, want 2.0", u.UsageHours)
	}
}

func formatID(id int) string {
	return fmt.Sprintf("%d", id)
}
