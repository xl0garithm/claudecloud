package provider

import (
	"fmt"

	"github.com/logan/cloudcode/internal/config"
)

// NewProvisioner creates a Provisioner based on the configured provider.
func NewProvisioner(cfg *config.Config) (Provisioner, error) {
	switch cfg.Provider {
	case "docker":
		return nil, fmt.Errorf("docker provider: %w", ErrProviderNotConfigured)
	case "hetzner":
		return nil, fmt.Errorf("hetzner provider: %w", ErrProviderNotConfigured)
	default:
		return nil, fmt.Errorf("unknown provider %q", cfg.Provider)
	}
}
