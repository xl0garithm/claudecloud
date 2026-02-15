package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/logan/cloudcode/internal/api/response"
	"github.com/logan/cloudcode/internal/auth"
)

type contextKey string

const (
	userIDKey  contextKey = "user_id"
	emailKey   contextKey = "email"
	isAdminKey contextKey = "is_admin"
)

// UserAuth returns middleware that supports dual-mode authentication:
//  1. Bearer JWT token (Authorization header or "session" cookie)
//  2. X-API-Key header (admin/backwards compat)
func UserAuth(jwtSecret, adminAPIKey string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Try Bearer JWT first
			if authHeader := r.Header.Get("Authorization"); strings.HasPrefix(authHeader, "Bearer ") {
				tokenStr := strings.TrimPrefix(authHeader, "Bearer ")
				claims, err := auth.ValidateToken(jwtSecret, tokenStr)
				if err != nil {
					response.Error(w, http.StatusUnauthorized, "invalid token")
					return
				}
				if claims.Purpose != "session" {
					response.Error(w, http.StatusUnauthorized, "invalid token purpose")
					return
				}
				ctx := context.WithValue(r.Context(), userIDKey, claims.UserID)
				ctx = context.WithValue(ctx, emailKey, claims.Email)
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}

			// Try session cookie
			if cookie, err := r.Cookie("session"); err == nil {
				claims, err := auth.ValidateToken(jwtSecret, cookie.Value)
				if err == nil && claims.Purpose == "session" {
					ctx := context.WithValue(r.Context(), userIDKey, claims.UserID)
					ctx = context.WithValue(ctx, emailKey, claims.Email)
					next.ServeHTTP(w, r.WithContext(ctx))
					return
				}
			}

			// Try X-API-Key (admin)
			if apiKey := r.Header.Get("X-API-Key"); apiKey != "" {
				if apiKey != adminAPIKey {
					response.Error(w, http.StatusUnauthorized, "invalid API key")
					return
				}
				ctx := context.WithValue(r.Context(), isAdminKey, true)
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}

			response.Error(w, http.StatusUnauthorized, "authentication required")
		})
	}
}

// UserIDFromContext returns the authenticated user's ID, or 0 if not set.
func UserIDFromContext(ctx context.Context) int {
	if id, ok := ctx.Value(userIDKey).(int); ok {
		return id
	}
	return 0
}

// EmailFromContext returns the authenticated user's email, or empty string.
func EmailFromContext(ctx context.Context) string {
	if email, ok := ctx.Value(emailKey).(string); ok {
		return email
	}
	return ""
}

// IsAdminContext returns true if the request was authenticated via X-API-Key.
func IsAdminContext(ctx context.Context) bool {
	if isAdmin, ok := ctx.Value(isAdminKey).(bool); ok {
		return isAdmin
	}
	return false
}

// TestUserIDKey returns the context key for user_id (for testing only).
func TestUserIDKey() contextKey { return userIDKey }
