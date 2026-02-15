package config

import "os"

// Config holds application configuration loaded from environment variables.
type Config struct {
	Provider    string
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
}

// Load reads configuration from environment variables with sensible defaults.
func Load() *Config {
	return &Config{
		Provider:    envOrDefault("PROVIDER", "docker"),
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
	}
}

func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
