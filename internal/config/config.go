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
	}
}

func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
