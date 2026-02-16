package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	otelTrace "go.opentelemetry.io/otel/trace"

	"github.com/logan/cloudcode/internal/ent"
	entinstance "github.com/logan/cloudcode/internal/ent/instance"
	entuser "github.com/logan/cloudcode/internal/ent/user"
	"github.com/logan/cloudcode/internal/provider"
)

var tracer = otel.Tracer("cloudcode/service/instance")

// InstanceService bridges HTTP handlers with the provider and database.
type InstanceService struct {
	db              *ent.Client
	provider        provider.Provisioner
	netbird         *NetbirdService // nil when PROVIDER=docker
	anthropicAPIKey string
}

// NewInstanceService creates a new InstanceService.
func NewInstanceService(db *ent.Client, prov provider.Provisioner, anthropicAPIKey string) *InstanceService {
	return &InstanceService{db: db, provider: prov, anthropicAPIKey: anthropicAPIKey}
}

// SetNetbirdService wires in the optional Netbird service for Hetzner mode.
func (s *InstanceService) SetNetbirdService(nb *NetbirdService) {
	s.netbird = nb
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

// ConnectInfo holds the data needed to generate a connect script.
type ConnectInfo struct {
	Provider     string
	Host         string
	ProviderID   string
	Status       string
	NetbirdConfig string
	UserID       int
}

// Create provisions a new instance for the given user.
func (s *InstanceService) Create(ctx context.Context, userID int) (*InstanceResponse, error) {
	ctx, span := tracer.Start(ctx, "instance.create",
		otelTrace.WithAttributes(attribute.Int("user_id", userID)))
	defer span.End()

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

	// Generate per-instance agent secret
	secretBytes := make([]byte, 32)
	if _, err := rand.Read(secretBytes); err != nil {
		return nil, fmt.Errorf("generate agent secret: %w", err)
	}
	agentSecret := hex.EncodeToString(secretBytes)

	var opts provider.CreateOptions
	opts.AgentSecret = agentSecret
	opts.AnthropicAPIKey = s.anthropicAPIKey
	var prep *NetbirdPrep

	// Phase 1: Prepare Netbird access (Hetzner only)
	if s.netbird != nil {
		prep, err = s.netbird.PrepareNetbirdAccess(ctx, userID)
		if err != nil {
			return nil, fmt.Errorf("netbird prepare: %w", err)
		}
		opts.NetbirdSetupKey = prep.SetupKey
	}

	// Call provider to create
	provInst, err := s.provider.Create(ctx, userID, opts)
	if err != nil {
		return nil, fmt.Errorf("provider create: %w", err)
	}

	// Phase 2: Finalize Netbird access (route + policy)
	var netbirdConfigStr string
	if s.netbird != nil && prep != nil {
		nbCfg, err := s.netbird.FinalizeNetbirdAccess(ctx, userID, prep)
		if err != nil {
			// Best-effort cleanup of provider instance
			_ = s.provider.Destroy(ctx, provInst.ID)
			return nil, fmt.Errorf("netbird finalize: %w", err)
		}
		netbirdConfigStr = MarshalNetbirdConfig(nbCfg)
	}

	// Save to DB
	create := s.db.Instance.Create().
		SetProvider(provInst.Provider).
		SetProviderID(provInst.ProviderID).
		SetHost(provInst.Host).
		SetPort(provInst.Port).
		SetStatus(string(provInst.Status)).
		SetVolumeID(provInst.VolumeID).
		SetAgentSecret(agentSecret).
		SetOwnerID(userID)

	if netbirdConfigStr != "" {
		create = create.SetNetbirdConfig(netbirdConfigStr)
	}

	inst, err := create.Save(ctx)
	if err != nil {
		// Best-effort cleanup on DB failure
		_ = s.provider.Destroy(ctx, provInst.ID)
		span.RecordError(err)
		span.SetStatus(codes.Error, "save instance failed")
		return nil, fmt.Errorf("save instance: %w", err)
	}

	span.SetAttributes(attribute.Int("instance_id", inst.ID), attribute.String("provider", inst.Provider))
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

	// Refresh status from provider
	if inst.Status != "destroyed" && inst.ProviderID != "" {
		provInst, err := s.provider.Status(ctx, inst.ProviderID)
		if err == nil && string(provInst.Status) != inst.Status {
			inst, _ = inst.Update().SetStatus(string(provInst.Status)).Save(ctx)
		} else if errors.Is(err, provider.ErrNotFound) {
			// Container was removed externally — mark as destroyed
			inst, _ = inst.Update().SetStatus("destroyed").Save(ctx)
		}
	}

	return toResponse(inst), nil
}

// Delete destroys the instance.
func (s *InstanceService) Delete(ctx context.Context, id int) error {
	ctx, span := tracer.Start(ctx, "instance.delete",
		otelTrace.WithAttributes(attribute.Int("instance_id", id)))
	defer span.End()

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

	// Teardown Netbird if configured
	if s.netbird != nil && inst.NetbirdConfig != "" {
		nbCfg, err := UnmarshalNetbirdConfig(inst.NetbirdConfig)
		if err == nil && nbCfg != nil {
			_ = s.netbird.TeardownUser(ctx, nbCfg)
		}
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
	ctx, span := tracer.Start(ctx, "instance.pause",
		otelTrace.WithAttributes(attribute.Int("instance_id", id)))
	defer span.End()

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
	ctx, span := tracer.Start(ctx, "instance.wake",
		otelTrace.WithAttributes(attribute.Int("instance_id", id)))
	defer span.End()

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

// GetByUserID looks up the active instance for a user.
func (s *InstanceService) GetByUserID(ctx context.Context, userID int) (*InstanceResponse, error) {
	inst, err := s.db.Instance.Query().
		Where(
			entinstance.HasOwnerWith(entuser.IDEQ(userID)),
			entinstance.StatusIn("provisioning", "running", "stopped"),
		).
		Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, provider.ErrNotFound
		}
		return nil, fmt.Errorf("query by user_id: %w", err)
	}
	return toResponse(inst), nil
}

// GetConnectInfo returns the data needed to generate a connect script.
func (s *InstanceService) GetConnectInfo(ctx context.Context, userID int) (*ConnectInfo, error) {
	inst, err := s.db.Instance.Query().
		Where(
			entinstance.HasOwnerWith(entuser.IDEQ(userID)),
			entinstance.StatusIn("running"),
		).
		Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, provider.ErrNotFound
		}
		return nil, fmt.Errorf("query connect info: %w", err)
	}

	owner, err := inst.QueryOwner().Only(ctx)
	if err != nil {
		return nil, fmt.Errorf("query owner: %w", err)
	}

	return &ConnectInfo{
		Provider:      inst.Provider,
		Host:          inst.Host,
		ProviderID:    inst.ProviderID,
		Status:        inst.Status,
		NetbirdConfig: inst.NetbirdConfig,
		UserID:        owner.ID,
	}, nil
}

// GetInstanceHost returns the host and agent secret for an instance, verifying user ownership.
func (s *InstanceService) GetInstanceHost(ctx context.Context, id int, userID int) (host string, agentSecret string, err error) {
	inst, err := s.db.Instance.Query().
		Where(
			entinstance.IDEQ(id),
			entinstance.HasOwnerWith(entuser.IDEQ(userID)),
			entinstance.StatusIn("running"),
		).
		Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return "", "", provider.ErrNotFound
		}
		return "", "", fmt.Errorf("query instance host: %w", err)
	}
	return inst.Host, inst.AgentSecret, nil
}

// ParseID converts a string ID from URL params to int.
func ParseID(s string) (int, error) {
	return strconv.Atoi(s)
}
