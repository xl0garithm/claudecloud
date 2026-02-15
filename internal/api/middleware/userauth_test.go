package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/logan/cloudcode/internal/auth"
)

func TestUserAuth_BearerJWT(t *testing.T) {
	secret := "test-secret"
	token, _ := auth.GenerateToken(secret, 42, "user@test.com", "session", time.Hour)

	handler := UserAuth(secret, "admin-key")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if uid := UserIDFromContext(r.Context()); uid != 42 {
			t.Errorf("userID = %d, want 42", uid)
		}
		if email := EmailFromContext(r.Context()); email != "user@test.com" {
			t.Errorf("email = %s, want user@test.com", email)
		}
		if IsAdminContext(r.Context()) {
			t.Error("should not be admin")
		}
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
}

func TestUserAuth_Cookie(t *testing.T) {
	secret := "test-secret"
	token, _ := auth.GenerateToken(secret, 42, "user@test.com", "session", time.Hour)

	handler := UserAuth(secret, "admin-key")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if uid := UserIDFromContext(r.Context()); uid != 42 {
			t.Errorf("userID = %d, want 42", uid)
		}
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	req.AddCookie(&http.Cookie{Name: "session", Value: token})
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
}

func TestUserAuth_APIKey(t *testing.T) {
	handler := UserAuth("secret", "admin-key")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !IsAdminContext(r.Context()) {
			t.Error("expected admin context")
		}
		if UserIDFromContext(r.Context()) != 0 {
			t.Error("expected no user ID for admin")
		}
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-API-Key", "admin-key")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
}

func TestUserAuth_InvalidAPIKey(t *testing.T) {
	handler := UserAuth("secret", "admin-key")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called")
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-API-Key", "wrong-key")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rec.Code)
	}
}

func TestUserAuth_NoAuth(t *testing.T) {
	handler := UserAuth("secret", "key")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called")
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rec.Code)
	}
}

func TestUserAuth_InvalidJWT(t *testing.T) {
	handler := UserAuth("secret", "key")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called")
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer invalid-token")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rec.Code)
	}
}

func TestUserAuth_MagicLinkTokenRejected(t *testing.T) {
	secret := "test-secret"
	token, _ := auth.GenerateToken(secret, 1, "user@test.com", "magic_link", time.Hour)

	handler := UserAuth(secret, "key")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called")
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rec.Code)
	}
}
