package docker

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/api/types/volume"
	"github.com/docker/docker/client"

	"github.com/logan/cloudcode/internal/provider"
)

const (
	networkName = "claude-net"
	imageTag    = "claude-instance:latest"
	labelPrefix = "cloudcode."
)

// Provider implements provider.Provisioner using the local Docker daemon.
type Provider struct {
	cli *client.Client
}

// New creates a new Docker provider.
func New() (*Provider, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, fmt.Errorf("docker client: %w", err)
	}
	return &Provider{cli: cli}, nil
}

func containerName(userID int) string {
	return fmt.Sprintf("claude-%d", userID)
}

func volumeName(userID int) string {
	return fmt.Sprintf("claude-data-%d", userID)
}

// Create provisions a new Claude instance for the given user.
// Docker ignores opts (no Netbird in local dev).
func (p *Provider) Create(ctx context.Context, userID int, opts provider.CreateOptions) (*provider.Instance, error) {
	name := containerName(userID)
	volName := volumeName(userID)

	// Check if container already exists
	existing, err := p.findContainer(ctx, name)
	if err != nil {
		return nil, err
	}
	if existing != "" {
		return nil, provider.ErrAlreadyExists
	}

	// Ensure network exists
	if err := p.ensureNetwork(ctx); err != nil {
		return nil, fmt.Errorf("ensure network: %w", err)
	}

	// Ensure volume exists
	if err := p.ensureVolume(ctx, volName, userID); err != nil {
		return nil, fmt.Errorf("ensure volume: %w", err)
	}

	// Build env vars for the container
	var envVars []string
	if opts.AgentSecret != "" {
		envVars = append(envVars, "AGENT_SECRET="+opts.AgentSecret)
	}
	if opts.AnthropicAPIKey != "" {
		envVars = append(envVars, "ANTHROPIC_API_KEY="+opts.AnthropicAPIKey)
	}

	// Create container
	resp, err := p.cli.ContainerCreate(ctx,
		&container.Config{
			Image: imageTag,
			Env:   envVars,
			Labels: map[string]string{
				labelPrefix + "managed": "true",
				labelPrefix + "user_id": strconv.Itoa(userID),
			},
		},
		&container.HostConfig{
			Binds:       []string{volName + ":/claude-data"},
			NetworkMode: container.NetworkMode(networkName),
			RestartPolicy: container.RestartPolicy{
				Name: container.RestartPolicyUnlessStopped,
			},
		},
		&network.NetworkingConfig{},
		nil,
		name,
	)
	if err != nil {
		return nil, fmt.Errorf("create container: %w", err)
	}

	// Start the container
	if err := p.cli.ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {
		return nil, fmt.Errorf("start container: %w", err)
	}

	return &provider.Instance{
		ID:         name,
		UserID:     userID,
		Provider:   "docker",
		ProviderID: resp.ID,
		Host:       name, // reachable by container name on the bridge network
		Status:     provider.StatusRunning,
		VolumeID:   volName,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}, nil
}

// Destroy removes the container but preserves the data volume.
func (p *Provider) Destroy(ctx context.Context, instanceID string) error {
	info, err := p.cli.ContainerInspect(ctx, instanceID)
	if err != nil {
		if client.IsErrNotFound(err) {
			return provider.ErrNotFound
		}
		return fmt.Errorf("inspect: %w", err)
	}

	if info.State.Running {
		timeout := 10
		if err := p.cli.ContainerStop(ctx, instanceID, container.StopOptions{Timeout: &timeout}); err != nil {
			return fmt.Errorf("stop: %w", err)
		}
	}

	if err := p.cli.ContainerRemove(ctx, instanceID, container.RemoveOptions{}); err != nil {
		return fmt.Errorf("remove: %w", err)
	}
	return nil
}

// Status returns the current state of the instance.
func (p *Provider) Status(ctx context.Context, instanceID string) (*provider.Instance, error) {
	info, err := p.cli.ContainerInspect(ctx, instanceID)
	if err != nil {
		if client.IsErrNotFound(err) {
			return nil, provider.ErrNotFound
		}
		return nil, fmt.Errorf("inspect: %w", err)
	}

	userID := 0
	if v, ok := info.Config.Labels[labelPrefix+"user_id"]; ok {
		userID, _ = strconv.Atoi(v)
	}

	status := mapDockerState(info.State.Status)

	return &provider.Instance{
		ID:         info.Name[1:], // strip leading '/'
		UserID:     userID,
		Provider:   "docker",
		ProviderID: info.ID,
		Host:       info.Name[1:],
		Status:     status,
		VolumeID:   volumeName(userID),
	}, nil
}

// Pause stops the container without removing it.
func (p *Provider) Pause(ctx context.Context, instanceID string) error {
	info, err := p.cli.ContainerInspect(ctx, instanceID)
	if err != nil {
		if client.IsErrNotFound(err) {
			return provider.ErrNotFound
		}
		return fmt.Errorf("inspect: %w", err)
	}

	if !info.State.Running {
		return provider.ErrInvalidState
	}

	timeout := 30
	if err := p.cli.ContainerStop(ctx, instanceID, container.StopOptions{Timeout: &timeout}); err != nil {
		return fmt.Errorf("stop: %w", err)
	}
	return nil
}

// Wake starts a previously stopped container.
func (p *Provider) Wake(ctx context.Context, instanceID string) error {
	info, err := p.cli.ContainerInspect(ctx, instanceID)
	if err != nil {
		if client.IsErrNotFound(err) {
			return provider.ErrNotFound
		}
		return fmt.Errorf("inspect: %w", err)
	}

	if info.State.Running {
		return provider.ErrInvalidState
	}

	if err := p.cli.ContainerStart(ctx, instanceID, container.StartOptions{}); err != nil {
		return fmt.Errorf("start: %w", err)
	}
	return nil
}

// Activity checks if the container has active processes beyond the base set.
// Docker: active if process count > 6 (entrypoint + tail + zellij + ttyd + node agent + shell).
func (p *Provider) Activity(ctx context.Context, instanceID string) (*provider.ActivityInfo, error) {
	top, err := p.cli.ContainerTop(ctx, instanceID, nil)
	if err != nil {
		if client.IsErrNotFound(err) {
			return nil, provider.ErrNotFound
		}
		return nil, fmt.Errorf("container top: %w", err)
	}

	processCount := len(top.Processes)
	return &provider.ActivityInfo{
		IsActive:     processCount > 6,
		ProcessCount: processCount,
	}, nil
}

func (p *Provider) ensureNetwork(ctx context.Context) error {
	nets, err := p.cli.NetworkList(ctx, network.ListOptions{
		Filters: filters.NewArgs(filters.Arg("name", networkName)),
	})
	if err != nil {
		return err
	}
	for _, n := range nets {
		if n.Name == networkName {
			return nil
		}
	}

	_, err = p.cli.NetworkCreate(ctx, networkName, network.CreateOptions{
		Driver: "bridge",
		Labels: map[string]string{
			labelPrefix + "managed": "true",
		},
	})
	return err
}

func (p *Provider) ensureVolume(ctx context.Context, name string, userID int) error {
	_, err := p.cli.VolumeInspect(ctx, name)
	if err == nil {
		return nil // already exists
	}

	_, err = p.cli.VolumeCreate(ctx, volume.CreateOptions{
		Name: name,
		Labels: map[string]string{
			labelPrefix + "managed": "true",
			labelPrefix + "user_id": strconv.Itoa(userID),
		},
	})
	return err
}

func (p *Provider) findContainer(ctx context.Context, name string) (string, error) {
	containers, err := p.cli.ContainerList(ctx, container.ListOptions{
		All:     true,
		Filters: filters.NewArgs(filters.Arg("name", "^/"+name+"$")),
	})
	if err != nil {
		return "", err
	}
	if len(containers) > 0 {
		return containers[0].ID, nil
	}
	return "", nil
}

func mapDockerState(state string) provider.Status {
	switch state {
	case "running":
		return provider.StatusRunning
	case "created", "restarting":
		return provider.StatusProvisioning
	case "paused", "exited", "dead":
		return provider.StatusStopped
	default:
		return provider.StatusError
	}
}
