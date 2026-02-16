package config

import (
	"fmt"
	"os"
	"strings"
)

// Config holds application configuration loaded from environment variables.
type Config struct {
	Provider    string
	Environment string // "development" or "production"
	DatabaseURL string
	APIKey      string
	ListenAddr  string
	HCloudToken string

	// Netbird (only needed when PROVIDER=hetzner)
	NetbirdAPIURL   string
	NetbirdAPIToken string

	// Activity detection
	ActivityCheckInterval string
	IdleThreshold         string

	// JWT auth
	JWTSecret string

	// URLs
	BaseURL     string // Backend URL (e.g., http://localhost:8080)
	FrontendURL string // Frontend URL (e.g., http://localhost:3000)

	// SMTP
	SMTPHost     string
	SMTPPort     string
	SMTPUsername string
	SMTPPassword string
	SMTPFrom     string

	// Stripe
	StripeSecretKey     string
	StripeWebhookSecret string
	StripePriceStarter  string
	StripePricePro      string

	// Anthropic
	AnthropicAPIKey string

	// OpenTelemetry
	OTELEndpoint string // OTLP HTTP endpoint (empty = no export in dev)
}

// Load reads configuration from environment variables with sensible defaults.
func Load() *Config {
	return &Config{
		Provider:    envOrDefault("PROVIDER", "docker"),
		Environment: envOrDefault("ENVIRONMENT", "development"),
		DatabaseURL: envOrDefault("DATABASE_URL", "postgres://cloudcode:cloudcode@localhost:5432/cloudcode?sslmode=disable"),
		APIKey:      envOrDefault("API_KEY", "dev-api-key"),
		ListenAddr:  envOrDefault("LISTEN_ADDR", ":8080"),
		HCloudToken: os.Getenv("HCLOUD_TOKEN"),

		NetbirdAPIURL:   envOrDefault("NETBIRD_API_URL", "https://api.netbird.io"),
		NetbirdAPIToken: os.Getenv("NETBIRD_API_TOKEN"),

		ActivityCheckInterval: envOrDefault("ACTIVITY_CHECK_INTERVAL", "5m"),
		IdleThreshold:         envOrDefault("IDLE_THRESHOLD", "2h"),

		JWTSecret: envOrDefault("JWT_SECRET", "dev-jwt-secret-change-in-production"),

		BaseURL:     envOrDefault("BASE_URL", "http://localhost:8080"),
		FrontendURL: envOrDefault("FRONTEND_URL", "http://localhost:3000"),

		SMTPHost:     os.Getenv("SMTP_HOST"),
		SMTPPort:     envOrDefault("SMTP_PORT", "587"),
		SMTPUsername: os.Getenv("SMTP_USERNAME"),
		SMTPPassword: os.Getenv("SMTP_PASSWORD"),
		SMTPFrom:     envOrDefault("SMTP_FROM", "noreply@claudecloud.dev"),

		StripeSecretKey:     os.Getenv("STRIPE_SECRET_KEY"),
		StripeWebhookSecret: os.Getenv("STRIPE_WEBHOOK_SECRET"),
		StripePriceStarter:  os.Getenv("STRIPE_PRICE_STARTER"),
		StripePricePro:      os.Getenv("STRIPE_PRICE_PRO"),

		AnthropicAPIKey: os.Getenv("ANTHROPIC_API_KEY"),

		OTELEndpoint: os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT"),
	}
}

// Validate checks that critical configuration is set for production mode.
// Returns nil if everything is okay, or an error describing what's missing.
func (c *Config) Validate() error {
	if c.Environment != "production" {
		return nil
	}

	var errs []string

	if c.JWTSecret == "dev-jwt-secret-change-in-production" || c.JWTSecret == "" {
		errs = append(errs, "JWT_SECRET must be set to a secure value in production")
	}
	if c.DatabaseURL == "" {
		errs = append(errs, "DATABASE_URL is required in production")
	}
	if c.StripeSecretKey == "" {
		errs = append(errs, "STRIPE_SECRET_KEY is recommended in production (billing disabled)")
	}

	if len(errs) > 0 {
		return fmt.Errorf("config validation failed:\n  - %s", strings.Join(errs, "\n  - "))
	}
	return nil
}

func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
