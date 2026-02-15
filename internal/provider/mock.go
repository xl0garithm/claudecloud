package provider

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// MockProvisioner is a test double for the Provisioner interface.
type MockProvisioner struct {
	mu        sync.Mutex
	instances map[string]*Instance
}

// NewMock creates a new MockProvisioner.
func NewMock() *MockProvisioner {
	return &MockProvisioner{
		instances: make(map[string]*Instance),
	}
}

func (m *MockProvisioner) Create(ctx context.Context, userID int) (*Instance, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	id := fmt.Sprintf("mock-%d", userID)
	if _, exists := m.instances[id]; exists {
		return nil, ErrAlreadyExists
	}

	inst := &Instance{
		ID:         id,
		UserID:     userID,
		Provider:   "mock",
		ProviderID: id,
		Host:       "localhost",
		Port:       8080,
		Status:     StatusRunning,
		VolumeID:   fmt.Sprintf("mock-vol-%d", userID),
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}
	m.instances[id] = inst
	return inst, nil
}

func (m *MockProvisioner) Destroy(ctx context.Context, instanceID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	inst, ok := m.instances[instanceID]
	if !ok {
		return ErrNotFound
	}
	inst.Status = StatusDestroyed
	delete(m.instances, instanceID)
	return nil
}

func (m *MockProvisioner) Status(ctx context.Context, instanceID string) (*Instance, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	inst, ok := m.instances[instanceID]
	if !ok {
		return nil, ErrNotFound
	}
	return inst, nil
}

func (m *MockProvisioner) Pause(ctx context.Context, instanceID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	inst, ok := m.instances[instanceID]
	if !ok {
		return ErrNotFound
	}
	if inst.Status != StatusRunning {
		return ErrInvalidState
	}
	inst.Status = StatusStopped
	return nil
}

func (m *MockProvisioner) Wake(ctx context.Context, instanceID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	inst, ok := m.instances[instanceID]
	if !ok {
		return ErrNotFound
	}
	if inst.Status != StatusStopped {
		return ErrInvalidState
	}
	inst.Status = StatusRunning
	return nil
}
