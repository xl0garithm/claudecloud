package hetzner

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/hashicorp/terraform-exec/tfexec"

	"github.com/logan/cloudcode/internal/provider"
)

// Provider implements provider.Provisioner using Hetzner Cloud via terraform-exec.
// Each user gets an isolated Terraform workspace directory for state isolation.
type Provider struct {
	tfBinary      string
	workspacesDir string
	hcloudToken   string
}

// New creates a new Hetzner provider.
func New(hcloudToken, tfBinary, workspacesDir string) (*Provider, error) {
	if hcloudToken == "" {
		return nil, fmt.Errorf("HCLOUD_TOKEN required: %w", provider.ErrProviderNotConfigured)
	}
	if tfBinary == "" {
		tfBinary = "terraform"
	}
	if workspacesDir == "" {
		workspacesDir = "terraform/workspaces"
	}

	if err := os.MkdirAll(workspacesDir, 0o755); err != nil {
		return nil, fmt.Errorf("create workspaces dir: %w", err)
	}

	return &Provider{
		tfBinary:      tfBinary,
		workspacesDir: workspacesDir,
		hcloudToken:   hcloudToken,
	}, nil
}

func (p *Provider) userDir(userID int) string {
	return filepath.Join(p.workspacesDir, fmt.Sprintf("user-%d", userID))
}

// Create provisions a new Hetzner server for the given user via Terraform.
// If opts.NetbirdSetupKey is set, it is passed to cloud-init for Netbird enrollment.
func (p *Provider) Create(ctx context.Context, userID int, opts provider.CreateOptions) (*provider.Instance, error) {
	dir := p.userDir(userID)

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create user dir: %w", err)
	}

	// Write tfvars
	vars := map[string]any{
		"user_id":      userID,
		"hcloud_token": p.hcloudToken,
	}
	if opts.NetbirdSetupKey != "" {
		vars["netbird_setup_key"] = opts.NetbirdSetupKey
	}
	varsJSON, _ := json.MarshalIndent(vars, "", "  ")
	varsFile := filepath.Join(dir, "terraform.tfvars.json")
	if err := os.WriteFile(varsFile, varsJSON, 0o600); err != nil {
		return nil, fmt.Errorf("write tfvars: %w", err)
	}

	// Copy module reference
	mainTF := fmt.Sprintf(`
module "instance" {
  source            = "../../modules/user_instance"
  user_id           = var.user_id
  hcloud_token      = var.hcloud_token
  netbird_setup_key = var.netbird_setup_key
}

variable "user_id" {
  type = number
}

variable "hcloud_token" {
  type      = string
  sensitive = true
}

variable "netbird_setup_key" {
  type      = string
  default   = ""
  sensitive = true
}

output "server_id" {
  value = module.instance.server_id
}

output "server_ip" {
  value = module.instance.private_ip
}

output "volume_id" {
  value = module.instance.volume_id
}
`)
	if err := os.WriteFile(filepath.Join(dir, "main.tf"), []byte(mainTF), 0o644); err != nil {
		return nil, fmt.Errorf("write main.tf: %w", err)
	}

	// Init and apply
	tf, err := tfexec.NewTerraform(dir, p.tfBinary)
	if err != nil {
		return nil, fmt.Errorf("terraform init client: %w", err)
	}

	if err := tf.Init(ctx); err != nil {
		return nil, fmt.Errorf("terraform init: %w", err)
	}

	if err := tf.Apply(ctx); err != nil {
		return nil, fmt.Errorf("terraform apply: %w", err)
	}

	// Read outputs
	output, err := tf.Output(ctx)
	if err != nil {
		return nil, fmt.Errorf("terraform output: %w", err)
	}

	serverID := ""
	if v, ok := output["server_id"]; ok {
		json.Unmarshal(v.Value, &serverID)
	}
	serverIP := ""
	if v, ok := output["server_ip"]; ok {
		json.Unmarshal(v.Value, &serverIP)
	}
	volumeID := ""
	if v, ok := output["volume_id"]; ok {
		json.Unmarshal(v.Value, &volumeID)
	}

	return &provider.Instance{
		ID:         fmt.Sprintf("hetzner-%d", userID),
		UserID:     userID,
		Provider:   "hetzner",
		ProviderID: serverID,
		Host:       serverIP,
		Status:     provider.StatusRunning,
		VolumeID:   volumeID,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}, nil
}

// Destroy tears down the Hetzner server via terraform destroy.
// The volume is preserved (Terraform module uses prevent_destroy on volume).
func (p *Provider) Destroy(ctx context.Context, instanceID string) error {
	// instanceID format: "hetzner-{userID}" — extract userID
	var userID int
	if _, err := fmt.Sscanf(instanceID, "hetzner-%d", &userID); err != nil {
		return fmt.Errorf("parse instance ID: %w", err)
	}

	dir := p.userDir(userID)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return provider.ErrNotFound
	}

	tf, err := tfexec.NewTerraform(dir, p.tfBinary)
	if err != nil {
		return fmt.Errorf("terraform client: %w", err)
	}

	if err := tf.Destroy(ctx); err != nil {
		return fmt.Errorf("terraform destroy: %w", err)
	}

	return nil
}

// Status returns the current state of the instance by checking Terraform state.
func (p *Provider) Status(ctx context.Context, instanceID string) (*provider.Instance, error) {
	var userID int
	if _, err := fmt.Sscanf(instanceID, "hetzner-%d", &userID); err != nil {
		return nil, fmt.Errorf("parse instance ID: %w", err)
	}

	dir := p.userDir(userID)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return nil, provider.ErrNotFound
	}

	tf, err := tfexec.NewTerraform(dir, p.tfBinary)
	if err != nil {
		return nil, fmt.Errorf("terraform client: %w", err)
	}

	state, err := tf.Show(ctx)
	if err != nil {
		return nil, fmt.Errorf("terraform show: %w", err)
	}

	status := provider.StatusDestroyed
	if state.Values != nil {
		status = provider.StatusRunning
	}

	return &provider.Instance{
		ID:       instanceID,
		UserID:   userID,
		Provider: "hetzner",
		Status:   status,
	}, nil
}

// Pause snapshots the server and destroys it (volume persists).
// In production this takes 30-120s. Synchronous for Phase 1.
func (p *Provider) Pause(ctx context.Context, instanceID string) error {
	// For Phase 1, Pause == Destroy (snapshot + destroy would need Hetzner API directly)
	// The volume persists via Terraform lifecycle rules
	return p.Destroy(ctx, instanceID)
}

// Activity checks if the Hetzner server is running (basic check for now).
func (p *Provider) Activity(ctx context.Context, instanceID string) (*provider.ActivityInfo, error) {
	inst, err := p.Status(ctx, instanceID)
	if err != nil {
		return nil, err
	}
	isActive := inst.Status == provider.StatusRunning
	return &provider.ActivityInfo{IsActive: isActive, IsHealthy: isActive}, nil
}

// Wake recreates the server from the latest snapshot.
func (p *Provider) Wake(ctx context.Context, instanceID string) error {
	var userID int
	if _, err := fmt.Sscanf(instanceID, "hetzner-%d", &userID); err != nil {
		return fmt.Errorf("parse instance ID: %w", err)
	}

	dir := p.userDir(userID)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return provider.ErrNotFound
	}

	tf, err := tfexec.NewTerraform(dir, p.tfBinary)
	if err != nil {
		return fmt.Errorf("terraform client: %w", err)
	}

	// Re-apply to recreate
	if err := tf.Apply(ctx); err != nil {
		return fmt.Errorf("terraform apply: %w", err)
	}

	return nil
}
