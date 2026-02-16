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
	inactive  map[string]bool // tracks instances marked as inactive for testing
}

// NewMock creates a new MockProvisioner.
func NewMock() *MockProvisioner {
	return &MockProvisioner{
		instances: make(map[string]*Instance),
		inactive:  make(map[string]bool),
	}
}

func (m *MockProvisioner) Create(ctx context.Context, userID int, opts CreateOptions) (*Instance, error) {
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

func (m *MockProvisioner) Activity(ctx context.Context, instanceID string) (*ActivityInfo, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	_, ok := m.instances[instanceID]
	if !ok {
		return nil, ErrNotFound
	}
	if m.inactive[instanceID] {
		return &ActivityInfo{IsActive: false, IsHealthy: true, ProcessCount: 2}, nil
	}
	return &ActivityInfo{IsActive: true, IsHealthy: true, ProcessCount: 5}, nil
}

// SetInactive marks an instance as inactive for testing.
func (m *MockProvisioner) SetInactive(instanceID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.inactive[instanceID] = true
}
