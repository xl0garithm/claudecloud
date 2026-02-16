package handler

import (
	"encoding/json"
	"net/http"

	"github.com/logan/cloudcode/internal/api/middleware"
	"github.com/logan/cloudcode/internal/api/response"
	"github.com/logan/cloudcode/internal/service"
)

// AuthHandler handles authentication endpoints.
type AuthHandler struct {
	auth        *service.AuthService
	frontendURL string
	devMode     bool
}

// NewAuthHandler creates a new AuthHandler.
func NewAuthHandler(auth *service.AuthService, frontendURL string, devMode bool) *AuthHandler {
	return &AuthHandler{auth: auth, frontendURL: frontendURL, devMode: devMode}
}

type loginRequest struct {
	Email string `json:"email"`
}

// Login handles POST /auth/login.
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Email == "" {
		response.Error(w, http.StatusBadRequest, "email is required")
		return
	}

	// Dev mode: skip email, issue session token directly
	if h.devMode {
		token, err := h.auth.DevLogin(r.Context(), w, req.Email)
		if err != nil {
			response.Error(w, http.StatusInternalServerError, "failed to login")
			return
		}
		response.JSON(w, http.StatusOK, map[string]string{
			"message": "dev login successful",
			"token":   token,
		})
		return
	}

	if err := h.auth.SendMagicLink(r.Context(), req.Email); err != nil {
		response.Error(w, http.StatusInternalServerError, "failed to send magic link")
		return
	}

	response.JSON(w, http.StatusOK, map[string]string{
		"message": "magic link sent",
	})
}

// Verify handles GET /auth/verify?token={token}.
func (h *AuthHandler) Verify(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	if token == "" {
		response.Error(w, http.StatusBadRequest, "missing token")
		return
	}

	sessionToken, err := h.auth.VerifyMagicLink(r.Context(), w, token)
	if err != nil {
		response.Error(w, http.StatusUnauthorized, "invalid or expired token")
		return
	}

	// If Accept header wants JSON, return token
	if r.Header.Get("Accept") == "application/json" {
		response.JSON(w, http.StatusOK, map[string]string{
			"token": sessionToken,
		})
		return
	}

	// Otherwise redirect to dashboard
	http.Redirect(w, r, h.frontendURL+"/dashboard", http.StatusFound)
}

// Me handles GET /auth/me.
func (h *AuthHandler) Me(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	if userID == 0 {
		response.Error(w, http.StatusUnauthorized, "not authenticated")
		return
	}

	user, err := h.auth.GetCurrentUser(r.Context(), userID)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "failed to get user")
		return
	}

	response.JSON(w, http.StatusOK, user)
}

type updateSettingsRequest struct {
	AnthropicAPIKey  *string `json:"anthropic_api_key"`
	ClaudeOAuthToken *string `json:"claude_oauth_token"`
}

// GetSettings handles GET /auth/settings.
func (h *AuthHandler) GetSettings(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	if userID == 0 {
		response.Error(w, http.StatusUnauthorized, "not authenticated")
		return
	}

	settings, err := h.auth.GetSettings(r.Context(), userID)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "failed to get settings")
		return
	}

	response.JSON(w, http.StatusOK, settings)
}

// UpdateSettings handles PUT /auth/settings.
func (h *AuthHandler) UpdateSettings(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	if userID == 0 {
		response.Error(w, http.StatusUnauthorized, "not authenticated")
		return
	}

	var req updateSettingsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := h.auth.UpdateSettings(r.Context(), userID, req.AnthropicAPIKey, req.ClaudeOAuthToken); err != nil {
		response.Error(w, http.StatusInternalServerError, "failed to update settings")
		return
	}

	response.JSON(w, http.StatusOK, map[string]string{"status": "updated"})
}
