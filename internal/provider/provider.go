package provider

import (
	"context"
	"time"
)

// Status represents the lifecycle state of an instance.
type Status string

const (
	StatusProvisioning Status = "provisioning"
	StatusRunning      Status = "running"
	StatusStopped      Status = "stopped"
	StatusDestroyed    Status = "destroyed"
	StatusError        Status = "error"
)

// Instance represents a provisioned Claude Code instance.
type Instance struct {
	ID         string    `json:"id"`
	UserID     int       `json:"user_id"`
	Provider   string    `json:"provider"`
	ProviderID string    `json:"provider_id"`
	Host       string    `json:"host"`
	Port       int       `json:"port"`
	Status     Status    `json:"status"`
	VolumeID   string    `json:"volume_id"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// Provisioner defines the interface for instance lifecycle management.
// Both Docker (local dev) and Hetzner (production) implement this interface.
type Provisioner interface {
	// Create provisions a new instance for the given user.
	Create(ctx context.Context, userID int) (*Instance, error)

	// Destroy tears down the instance but preserves the data volume.
	Destroy(ctx context.Context, instanceID string) error

	// Status returns the current state of an instance.
	Status(ctx context.Context, instanceID string) (*Instance, error)

	// Pause stops the instance without destroying it.
	Pause(ctx context.Context, instanceID string) error

	// Wake starts a previously paused instance.
	Wake(ctx context.Context, instanceID string) error
}
