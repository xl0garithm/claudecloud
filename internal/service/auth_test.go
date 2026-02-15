package service

import (
	"context"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/logan/cloudcode/internal/auth"
	"github.com/logan/cloudcode/internal/ent/enttest"

	_ "github.com/mattn/go-sqlite3"
)

type mockMailer struct {
	lastTo   string
	lastLink string
}

func (m *mockMailer) SendMagicLink(to, link string) error {
	m.lastTo = to
	m.lastLink = link
	return nil
}

func TestSendMagicLink_CreatesUser(t *testing.T) {
	client := enttest.Open(t, "sqlite3", "file:ent?mode=memory&_fk=1")
	defer client.Close()

	mailer := &mockMailer{}
	svc := NewAuthService(client, "test-secret", "http://localhost:8080", "http://localhost:3000", mailer)

	err := svc.SendMagicLink(context.Background(), "test@example.com")
	if err != nil {
		t.Fatalf("send: %v", err)
	}

	if mailer.lastTo != "test@example.com" {
		t.Errorf("to = %s, want test@example.com", mailer.lastTo)
	}
	if mailer.lastLink == "" {
		t.Error("link should not be empty")
	}

	// Verify user was created
	users, _ := client.User.Query().All(context.Background())
	if len(users) != 1 {
		t.Fatalf("users = %d, want 1", len(users))
	}
	if users[0].Email != "test@example.com" {
		t.Errorf("email = %s, want test@example.com", users[0].Email)
	}
}

func TestSendMagicLink_ExistingUser(t *testing.T) {
	client := enttest.Open(t, "sqlite3", "file:ent?mode=memory&_fk=1")
	defer client.Close()

	mailer := &mockMailer{}
	svc := NewAuthService(client, "test-secret", "http://localhost:8080", "http://localhost:3000", mailer)

	// First call creates user
	_ = svc.SendMagicLink(context.Background(), "test@example.com")

	// Second call finds existing user
	err := svc.SendMagicLink(context.Background(), "test@example.com")
	if err != nil {
		t.Fatalf("send again: %v", err)
	}

	// Still only one user
	users, _ := client.User.Query().All(context.Background())
	if len(users) != 1 {
		t.Fatalf("users = %d, want 1", len(users))
	}
}

func TestVerifyMagicLink(t *testing.T) {
	client := enttest.Open(t, "sqlite3", "file:ent?mode=memory&_fk=1")
	defer client.Close()

	secret := "test-secret"
	mailer := &mockMailer{}
	svc := NewAuthService(client, secret, "http://localhost:8080", "http://localhost:3000", mailer)

	// Create user via magic link
	_ = svc.SendMagicLink(context.Background(), "test@example.com")
	u, _ := client.User.Query().Only(context.Background())

	// Generate a magic link token directly
	token, _ := auth.GenerateToken(secret, u.ID, u.Email, "magic_link", 15*time.Minute)

	w := httptest.NewRecorder()
	sessionToken, err := svc.VerifyMagicLink(context.Background(), w, token)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}

	// Validate session token
	claims, err := auth.ValidateToken(secret, sessionToken)
	if err != nil {
		t.Fatalf("validate session: %v", err)
	}
	if claims.Purpose != "session" {
		t.Errorf("purpose = %s, want session", claims.Purpose)
	}
	if claims.UserID != u.ID {
		t.Errorf("user_id = %d, want %d", claims.UserID, u.ID)
	}

	// Check cookie was set
	cookies := w.Result().Cookies()
	found := false
	for _, c := range cookies {
		if c.Name == "session" {
			found = true
			if !c.HttpOnly {
				t.Error("cookie should be HttpOnly")
			}
		}
	}
	if !found {
		t.Error("session cookie not set")
	}
}

func TestVerifyMagicLink_SessionTokenRejected(t *testing.T) {
	client := enttest.Open(t, "sqlite3", "file:ent?mode=memory&_fk=1")
	defer client.Close()

	secret := "test-secret"
	mailer := &mockMailer{}
	svc := NewAuthService(client, secret, "http://localhost:8080", "http://localhost:3000", mailer)

	// Create user
	_ = svc.SendMagicLink(context.Background(), "test@example.com")
	u, _ := client.User.Query().Only(context.Background())

	// Try to verify with a session token (wrong purpose)
	token, _ := auth.GenerateToken(secret, u.ID, u.Email, "session", time.Hour)

	w := httptest.NewRecorder()
	_, err := svc.VerifyMagicLink(context.Background(), w, token)
	if err == nil {
		t.Fatal("expected error for session token used as magic link")
	}
}

func TestGetCurrentUser(t *testing.T) {
	client := enttest.Open(t, "sqlite3", "file:ent?mode=memory&_fk=1")
	defer client.Close()

	mailer := &mockMailer{}
	svc := NewAuthService(client, "test-secret", "http://localhost:8080", "http://localhost:3000", mailer)

	_ = svc.SendMagicLink(context.Background(), "test@example.com")
	u, _ := client.User.Query().Only(context.Background())

	resp, err := svc.GetCurrentUser(context.Background(), u.ID)
	if err != nil {
		t.Fatalf("get user: %v", err)
	}
	if resp.Email != "test@example.com" {
		t.Errorf("email = %s, want test@example.com", resp.Email)
	}
	if resp.Plan != "free" {
		t.Errorf("plan = %s, want free", resp.Plan)
	}
	if resp.SubscriptionStatus != "inactive" {
		t.Errorf("subscription_status = %s, want inactive", resp.SubscriptionStatus)
	}
}
