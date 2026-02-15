package service

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	"github.com/logan/cloudcode/internal/ent"
	entinstance "github.com/logan/cloudcode/internal/ent/instance"
	entuser "github.com/logan/cloudcode/internal/ent/user"
	"github.com/logan/cloudcode/internal/provider"
)

// InstanceService bridges HTTP handlers with the provider and database.
type InstanceService struct {
	db       *ent.Client
	provider provider.Provisioner
}

// NewInstanceService creates a new InstanceService.
func NewInstanceService(db *ent.Client, prov provider.Provisioner) *InstanceService {
	return &InstanceService{db: db, provider: prov}
}

// InstanceResponse is the API response for an instance.
type InstanceResponse struct {
	ID         int    `json:"id"`
	Provider   string `json:"provider"`
	ProviderID string `json:"provider_id"`
	Host       string `json:"host"`
	Port       int    `json:"port"`
	Status     string `json:"status"`
	VolumeID   string `json:"volume_id"`
}

func toResponse(inst *ent.Instance) *InstanceResponse {
	return &InstanceResponse{
		ID:         inst.ID,
		Provider:   inst.Provider,
		ProviderID: inst.ProviderID,
		Host:       inst.Host,
		Port:       inst.Port,
		Status:     inst.Status,
		VolumeID:   inst.VolumeID,
	}
}

// Create provisions a new instance for the given user.
func (s *InstanceService) Create(ctx context.Context, userID int) (*InstanceResponse, error) {
	// Check for existing active instance
	exists, err := s.db.Instance.Query().
		Where(
			entinstance.HasOwnerWith(entuser.IDEQ(userID)),
			entinstance.StatusIn("provisioning", "running", "stopped"),
		).
		Exist(ctx)
	if err != nil {
		return nil, fmt.Errorf("query existing: %w", err)
	}
	if exists {
		return nil, provider.ErrAlreadyExists
	}

	// Call provider to create
	provInst, err := s.provider.Create(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("provider create: %w", err)
	}

	// Save to DB
	inst, err := s.db.Instance.Create().
		SetProvider(provInst.Provider).
		SetProviderID(provInst.ProviderID).
		SetHost(provInst.Host).
		SetPort(provInst.Port).
		SetStatus(string(provInst.Status)).
		SetVolumeID(provInst.VolumeID).
		SetOwnerID(userID).
		Save(ctx)
	if err != nil {
		// Best-effort cleanup on DB failure
		_ = s.provider.Destroy(ctx, provInst.ID)
		return nil, fmt.Errorf("save instance: %w", err)
	}

	return toResponse(inst), nil
}

// Get returns instance details by DB ID.
func (s *InstanceService) Get(ctx context.Context, id int) (*InstanceResponse, error) {
	inst, err := s.db.Instance.Get(ctx, id)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, provider.ErrNotFound
		}
		return nil, fmt.Errorf("get instance: %w", err)
	}

	// Optionally refresh status from provider
	if inst.Status != "destroyed" && inst.ProviderID != "" {
		provInst, err := s.provider.Status(ctx, inst.ProviderID)
		if err == nil && string(provInst.Status) != inst.Status {
			inst, _ = inst.Update().SetStatus(string(provInst.Status)).Save(ctx)
		}
	}

	return toResponse(inst), nil
}

// Delete destroys the instance.
func (s *InstanceService) Delete(ctx context.Context, id int) error {
	inst, err := s.db.Instance.Get(ctx, id)
	if err != nil {
		if ent.IsNotFound(err) {
			return provider.ErrNotFound
		}
		return fmt.Errorf("get instance: %w", err)
	}

	if inst.Status == "destroyed" {
		return provider.ErrInvalidState
	}

	// Destroy via provider using the container/instance name
	provID := inst.ProviderID
	if err := s.provider.Destroy(ctx, provID); err != nil && !errors.Is(err, provider.ErrNotFound) {
		return fmt.Errorf("provider destroy: %w", err)
	}

	_, err = inst.Update().SetStatus("destroyed").Save(ctx)
	return err
}

// Pause pauses the instance.
func (s *InstanceService) Pause(ctx context.Context, id int) error {
	inst, err := s.db.Instance.Get(ctx, id)
	if err != nil {
		if ent.IsNotFound(err) {
			return provider.ErrNotFound
		}
		return fmt.Errorf("get instance: %w", err)
	}

	if inst.Status != "running" {
		return provider.ErrInvalidState
	}

	if err := s.provider.Pause(ctx, inst.ProviderID); err != nil {
		return fmt.Errorf("provider pause: %w", err)
	}

	_, err = inst.Update().SetStatus("stopped").Save(ctx)
	return err
}

// Wake wakes a paused instance.
func (s *InstanceService) Wake(ctx context.Context, id int) error {
	inst, err := s.db.Instance.Get(ctx, id)
	if err != nil {
		if ent.IsNotFound(err) {
			return provider.ErrNotFound
		}
		return fmt.Errorf("get instance: %w", err)
	}

	if inst.Status != "stopped" {
		return provider.ErrInvalidState
	}

	if err := s.provider.Wake(ctx, inst.ProviderID); err != nil {
		return fmt.Errorf("provider wake: %w", err)
	}

	_, err = inst.Update().SetStatus("running").Save(ctx)
	return err
}

// GetByProviderID looks up an instance by its provider-side ID.
func (s *InstanceService) GetByProviderID(ctx context.Context, providerID string) (*InstanceResponse, error) {
	inst, err := s.db.Instance.Query().
		Where(entinstance.ProviderID(providerID)).
		Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, provider.ErrNotFound
		}
		return nil, fmt.Errorf("query by provider_id: %w", err)
	}
	return toResponse(inst), nil
}

// ParseID converts a string ID from URL params to int.
func ParseID(s string) (int, error) {
	return strconv.Atoi(s)
}
