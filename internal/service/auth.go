package service

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/logan/cloudcode/internal/auth"
	"github.com/logan/cloudcode/internal/ent"
	entuser "github.com/logan/cloudcode/internal/ent/user"
)

// AuthService handles user authentication via magic links.
type AuthService struct {
	db          *ent.Client
	jwtSecret   string
	baseURL     string
	frontendURL string
	mailer      Mailer
}

// NewAuthService creates a new AuthService.
func NewAuthService(db *ent.Client, jwtSecret, baseURL, frontendURL string, mailer Mailer) *AuthService {
	return &AuthService{
		db:          db,
		jwtSecret:   jwtSecret,
		baseURL:     baseURL,
		frontendURL: frontendURL,
		mailer:      mailer,
	}
}

// SendMagicLink finds or creates a user by email and sends a magic link.
func (s *AuthService) SendMagicLink(ctx context.Context, email string) error {
	// Find or create user
	u, err := s.db.User.Query().Where(entuser.EmailEQ(email)).Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			u, err = s.db.User.Create().
				SetEmail(email).
				Save(ctx)
			if err != nil {
				return fmt.Errorf("create user: %w", err)
			}
		} else {
			return fmt.Errorf("query user: %w", err)
		}
	}

	// Generate magic link JWT (15 min expiry)
	token, err := auth.GenerateToken(s.jwtSecret, u.ID, u.Email, "magic_link", 15*time.Minute)
	if err != nil {
		return fmt.Errorf("generate token: %w", err)
	}

	link := fmt.Sprintf("%s/auth/verify?token=%s", s.baseURL, token)
	return s.mailer.SendMagicLink(email, link)
}

// VerifyMagicLink validates a magic link token and returns a session JWT.
// It also sets an HttpOnly cookie on the response.
func (s *AuthService) VerifyMagicLink(ctx context.Context, w http.ResponseWriter, tokenStr string) (string, error) {
	claims, err := auth.ValidateToken(s.jwtSecret, tokenStr)
	if err != nil {
		return "", fmt.Errorf("invalid token: %w", err)
	}
	if claims.Purpose != "magic_link" {
		return "", fmt.Errorf("invalid token purpose")
	}

	// Verify user still exists
	u, err := s.db.User.Get(ctx, claims.UserID)
	if err != nil {
		return "", fmt.Errorf("user not found: %w", err)
	}

	// Generate session JWT (24h)
	sessionToken, err := auth.GenerateToken(s.jwtSecret, u.ID, u.Email, "session", 24*time.Hour)
	if err != nil {
		return "", fmt.Errorf("generate session: %w", err)
	}

	// Set HttpOnly cookie
	http.SetCookie(w, &http.Cookie{
		Name:     "session",
		Value:    sessionToken,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   86400, // 24 hours
	})

	return sessionToken, nil
}

// UserResponse is the API response for user info.
type UserResponse struct {
	ID                 int     `json:"id"`
	Email              string  `json:"email"`
	Name               string  `json:"name"`
	Plan               string  `json:"plan"`
	SubscriptionStatus string  `json:"subscription_status"`
	UsageHours         float64 `json:"usage_hours"`
}

// DevLogin finds or creates a user by email, then issues a session token
// and sets the session cookie directly — skipping the magic link email.
// Only use this in development mode.
func (s *AuthService) DevLogin(ctx context.Context, w http.ResponseWriter, email string) (string, error) {
	// Find or create user
	u, err := s.db.User.Query().Where(entuser.EmailEQ(email)).Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			u, err = s.db.User.Create().
				SetEmail(email).
				Save(ctx)
			if err != nil {
				return "", fmt.Errorf("create user: %w", err)
			}
		} else {
			return "", fmt.Errorf("query user: %w", err)
		}
	}

	// Generate session JWT (24h)
	sessionToken, err := auth.GenerateToken(s.jwtSecret, u.ID, u.Email, "session", 24*time.Hour)
	if err != nil {
		return "", fmt.Errorf("generate session: %w", err)
	}

	// Set HttpOnly cookie
	http.SetCookie(w, &http.Cookie{
		Name:     "session",
		Value:    sessionToken,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   86400,
	})

	return sessionToken, nil
}

// GetCurrentUser returns the user info for the given user ID.
func (s *AuthService) GetCurrentUser(ctx context.Context, userID int) (*UserResponse, error) {
	u, err := s.db.User.Get(ctx, userID)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, fmt.Errorf("user not found")
		}
		return nil, fmt.Errorf("get user: %w", err)
	}

	return &UserResponse{
		ID:                 u.ID,
		Email:              u.Email,
		Name:               u.Name,
		Plan:               u.Plan,
		SubscriptionStatus: u.SubscriptionStatus,
		UsageHours:         u.UsageHours,
	}, nil
}
