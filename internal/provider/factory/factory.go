package factory

import (
	"fmt"

	"github.com/logan/cloudcode/internal/config"
	"github.com/logan/cloudcode/internal/provider"
	"github.com/logan/cloudcode/internal/provider/docker"
)

// NewProvisioner creates a Provisioner based on the configured provider.
func NewProvisioner(cfg *config.Config) (provider.Provisioner, error) {
	switch cfg.Provider {
	case "docker":
		return docker.New()
	case "hetzner":
		return nil, fmt.Errorf("hetzner provider: %w", provider.ErrProviderNotConfigured)
	default:
		return nil, fmt.Errorf("unknown provider %q", cfg.Provider)
	}
}
