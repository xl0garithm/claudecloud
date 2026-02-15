package handler

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/logan/cloudcode/internal/api/middleware"
	"github.com/logan/cloudcode/internal/api/response"
	"github.com/logan/cloudcode/internal/service"
)

// BillingHandler handles billing endpoints.
type BillingHandler struct {
	billing *service.BillingService
}

// NewBillingHandler creates a new BillingHandler.
func NewBillingHandler(billing *service.BillingService) *BillingHandler {
	return &BillingHandler{billing: billing}
}

type checkoutRequest struct {
	Plan string `json:"plan"`
}

// CreateCheckout handles POST /billing/checkout.
func (h *BillingHandler) CreateCheckout(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	if userID == 0 {
		response.Error(w, http.StatusUnauthorized, "not authenticated")
		return
	}

	var req checkoutRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Plan == "" {
		req.Plan = "starter"
	}

	url, err := h.billing.CreateCheckoutSession(r.Context(), userID, req.Plan)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "failed to create checkout session")
		return
	}

	response.JSON(w, http.StatusOK, map[string]string{"url": url})
}

// Webhook handles POST /billing/webhook.
func (h *BillingHandler) Webhook(w http.ResponseWriter, r *http.Request) {
	payload, err := io.ReadAll(io.LimitReader(r.Body, 65536))
	if err != nil {
		response.Error(w, http.StatusBadRequest, "failed to read body")
		return
	}

	sigHeader := r.Header.Get("Stripe-Signature")
	if err := h.billing.HandleWebhookEvent(payload, sigHeader); err != nil {
		response.Error(w, http.StatusBadRequest, "webhook error")
		return
	}

	response.JSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// GetPortal handles GET /billing/portal.
func (h *BillingHandler) GetPortal(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	if userID == 0 {
		response.Error(w, http.StatusUnauthorized, "not authenticated")
		return
	}

	url, err := h.billing.GetBillingPortalURL(r.Context(), userID)
	if err != nil {
		response.Error(w, http.StatusBadRequest, "no billing account")
		return
	}

	response.JSON(w, http.StatusOK, map[string]string{"url": url})
}

// GetUsage handles GET /billing/usage.
func (h *BillingHandler) GetUsage(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	if userID == 0 {
		response.Error(w, http.StatusUnauthorized, "not authenticated")
		return
	}

	summary, err := h.billing.GetUsageSummary(r.Context(), userID)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "failed to get usage")
		return
	}

	response.JSON(w, http.StatusOK, summary)
}
