package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/stripe/stripe-go/v82"
	portalsession "github.com/stripe/stripe-go/v82/billingportal/session"
	"github.com/stripe/stripe-go/v82/checkout/session"
	"github.com/stripe/stripe-go/v82/customer"
	"github.com/stripe/stripe-go/v82/webhook"

	"github.com/logan/cloudcode/internal/ent"
	entuser "github.com/logan/cloudcode/internal/ent/user"
)

// BillingService handles Stripe billing and subscription management.
type BillingService struct {
	db            *ent.Client
	instanceSvc   *InstanceService
	webhookSecret string
	priceStarter  string
	pricePro      string
	frontendURL   string
	logger        *slog.Logger
}

// NewBillingService creates a new BillingService and sets the Stripe API key.
func NewBillingService(
	db *ent.Client,
	instanceSvc *InstanceService,
	stripeKey string,
	webhookSecret string,
	priceStarter string,
	pricePro string,
	frontendURL string,
	logger *slog.Logger,
) *BillingService {
	stripe.Key = stripeKey
	return &BillingService{
		db:            db,
		instanceSvc:   instanceSvc,
		webhookSecret: webhookSecret,
		priceStarter:  priceStarter,
		pricePro:      pricePro,
		frontendURL:   frontendURL,
		logger:        logger,
	}
}

// UsageSummary is the API response for billing usage.
type UsageSummary struct {
	Plan               string  `json:"plan"`
	SubscriptionStatus string  `json:"subscription_status"`
	UsageHours         float64 `json:"usage_hours"`
}

// CreateCheckoutSession creates a Stripe Checkout session for the given user and plan.
func (s *BillingService) CreateCheckoutSession(ctx context.Context, userID int, plan string) (string, error) {
	u, err := s.db.User.Get(ctx, userID)
	if err != nil {
		return "", fmt.Errorf("get user: %w", err)
	}

	// Create or get Stripe customer
	customerID := ""
	if u.StripeCustomerID != nil {
		customerID = *u.StripeCustomerID
	}
	if customerID == "" {
		params := &stripe.CustomerParams{
			Email: stripe.String(u.Email),
		}
		params.AddMetadata("user_id", fmt.Sprintf("%d", u.ID))
		c, err := customer.New(params)
		if err != nil {
			return "", fmt.Errorf("create customer: %w", err)
		}
		customerID = c.ID
		_, err = u.Update().SetStripeCustomerID(customerID).Save(ctx)
		if err != nil {
			return "", fmt.Errorf("save customer id: %w", err)
		}
	}

	// Select price
	priceID := s.priceStarter
	if plan == "pro" {
		priceID = s.pricePro
	}

	params := &stripe.CheckoutSessionParams{
		Customer: stripe.String(customerID),
		Mode:     stripe.String(string(stripe.CheckoutSessionModeSubscription)),
		LineItems: []*stripe.CheckoutSessionLineItemParams{
			{
				Price:    stripe.String(priceID),
				Quantity: stripe.Int64(1),
			},
		},
		SuccessURL: stripe.String(s.frontendURL + "/dashboard?checkout=success"),
		CancelURL:  stripe.String(s.frontendURL + "/dashboard?checkout=cancel"),
	}
	params.AddMetadata("user_id", fmt.Sprintf("%d", userID))
	params.AddMetadata("plan", plan)

	sess, err := session.New(params)
	if err != nil {
		return "", fmt.Errorf("create checkout: %w", err)
	}

	return sess.URL, nil
}

// GetUsageSummary returns billing usage stats for the user.
func (s *BillingService) GetUsageSummary(ctx context.Context, userID int) (*UsageSummary, error) {
	u, err := s.db.User.Get(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("get user: %w", err)
	}

	return &UsageSummary{
		Plan:               u.Plan,
		SubscriptionStatus: u.SubscriptionStatus,
		UsageHours:         u.UsageHours,
	}, nil
}

// GetBillingPortalURL creates a Stripe billing portal session.
func (s *BillingService) GetBillingPortalURL(ctx context.Context, userID int) (string, error) {
	u, err := s.db.User.Get(ctx, userID)
	if err != nil {
		return "", fmt.Errorf("get user: %w", err)
	}

	if u.StripeCustomerID == nil || *u.StripeCustomerID == "" {
		return "", fmt.Errorf("no billing account")
	}

	params := &stripe.BillingPortalSessionParams{
		Customer:  u.StripeCustomerID,
		ReturnURL: stripe.String(s.frontendURL + "/dashboard"),
	}

	sess, err := portalsession.New(params)
	if err != nil {
		return "", fmt.Errorf("create portal: %w", err)
	}

	return sess.URL, nil
}

// HandleWebhookEvent verifies and processes a Stripe webhook event.
func (s *BillingService) HandleWebhookEvent(payload []byte, sigHeader string) error {
	event, err := webhook.ConstructEvent(payload, sigHeader, s.webhookSecret)
	if err != nil {
		return fmt.Errorf("verify signature: %w", err)
	}

	return s.processEvent(event)
}

func (s *BillingService) processEvent(event stripe.Event) error {
	ctx := context.Background()

	switch event.Type {
	case "checkout.session.completed":
		return s.handleCheckoutCompleted(ctx, event)
	case "customer.subscription.updated":
		return s.handleSubscriptionUpdated(ctx, event)
	case "customer.subscription.deleted":
		return s.handleSubscriptionDeleted(ctx, event)
	case "invoice.payment_failed":
		return s.handlePaymentFailed(ctx, event)
	default:
		s.logger.Info("unhandled billing event", "event_type", event.Type)
		return nil
	}
}

func (s *BillingService) handleCheckoutCompleted(ctx context.Context, event stripe.Event) error {
	var sess stripe.CheckoutSession
	if err := json.Unmarshal(event.Data.Raw, &sess); err != nil {
		return fmt.Errorf("parse checkout session: %w", err)
	}

	if sess.Metadata == nil {
		return fmt.Errorf("missing metadata")
	}

	userIDStr := sess.Metadata["user_id"]
	plan := sess.Metadata["plan"]
	if plan == "" {
		plan = "starter"
	}

	userID, err := ParseID(userIDStr)
	if err != nil {
		return fmt.Errorf("parse user_id: %w", err)
	}

	u, err := s.db.User.Get(ctx, userID)
	if err != nil {
		return fmt.Errorf("get user: %w", err)
	}

	update := u.Update().
		SetSubscriptionStatus("active").
		SetPlan(plan)

	if sess.Subscription != nil {
		update = update.SetStripeSubscriptionID(sess.Subscription.ID)
	}
	if sess.Customer != nil {
		update = update.SetStripeCustomerID(sess.Customer.ID)
	}

	if _, err = update.Save(ctx); err != nil {
		return fmt.Errorf("update user: %w", err)
	}

	// Auto-provision instance
	if _, provErr := s.instanceSvc.Create(ctx, userID); provErr != nil {
		s.logger.Error("auto-provision failed after checkout", "user_id", userID, "error", provErr)
	}

	s.logger.Info("checkout completed", "user_id", userID, "plan", plan)
	return nil
}

func (s *BillingService) handleSubscriptionUpdated(ctx context.Context, event stripe.Event) error {
	var sub stripe.Subscription
	if err := json.Unmarshal(event.Data.Raw, &sub); err != nil {
		return fmt.Errorf("parse subscription: %w", err)
	}

	if sub.Customer == nil {
		return fmt.Errorf("missing customer in subscription")
	}

	u, err := s.db.User.Query().
		Where(entuser.StripeCustomerID(sub.Customer.ID)).
		Only(ctx)
	if err != nil {
		return fmt.Errorf("find user by customer: %w", err)
	}

	_, err = u.Update().
		SetSubscriptionStatus(string(sub.Status)).
		SetStripeSubscriptionID(sub.ID).
		Save(ctx)
	if err != nil {
		return fmt.Errorf("update subscription status: %w", err)
	}

	s.logger.Info("subscription updated", "user_id", u.ID, "status", sub.Status)
	return nil
}

func (s *BillingService) handleSubscriptionDeleted(ctx context.Context, event stripe.Event) error {
	var sub stripe.Subscription
	if err := json.Unmarshal(event.Data.Raw, &sub); err != nil {
		return fmt.Errorf("parse subscription: %w", err)
	}

	if sub.Customer == nil {
		return fmt.Errorf("missing customer in subscription")
	}

	u, err := s.db.User.Query().
		Where(entuser.StripeCustomerID(sub.Customer.ID)).
		Only(ctx)
	if err != nil {
		return fmt.Errorf("find user by customer: %w", err)
	}

	_, err = u.Update().
		SetSubscriptionStatus("canceled").
		ClearStripeSubscriptionID().
		Save(ctx)
	if err != nil {
		return fmt.Errorf("update subscription status: %w", err)
	}

	// Pause active instance
	inst, err := s.instanceSvc.GetByUserID(ctx, u.ID)
	if err == nil && inst.Status == "running" {
		if pauseErr := s.instanceSvc.Pause(ctx, inst.ID); pauseErr != nil {
			s.logger.Error("failed to pause instance on subscription delete", "user_id", u.ID, "error", pauseErr)
		}
	}

	s.logger.Info("subscription deleted", "user_id", u.ID)
	return nil
}

func (s *BillingService) handlePaymentFailed(ctx context.Context, event stripe.Event) error {
	var invoice stripe.Invoice
	if err := json.Unmarshal(event.Data.Raw, &invoice); err != nil {
		return fmt.Errorf("parse invoice: %w", err)
	}

	if invoice.Customer == nil {
		return nil
	}

	u, err := s.db.User.Query().
		Where(entuser.StripeCustomerID(invoice.Customer.ID)).
		Only(ctx)
	if err != nil {
		return fmt.Errorf("find user by customer: %w", err)
	}

	_, err = u.Update().
		SetSubscriptionStatus("past_due").
		Save(ctx)
	if err != nil {
		return fmt.Errorf("update subscription status: %w", err)
	}

	s.logger.Warn("payment failed", "user_id", u.ID)
	return nil
}

// ReportUsage adds usage hours to a user's total.
func (s *BillingService) ReportUsage(ctx context.Context, userID int, hours float64) error {
	_, err := s.db.User.UpdateOneID(userID).
		AddUsageHours(hours).
		Save(ctx)
	return err
}
