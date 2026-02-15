terraform {
  required_providers {
    hcloud = {
      source  = "hetznercloud/hcloud"
      version = "~> 1.48"
    }
  }
  required_version = ">= 1.5"
}

variable "hcloud_token" {
  type        = string
  sensitive   = true
  description = "Hetzner Cloud API token"
}

provider "hcloud" {
  token = var.hcloud_token
}

# Shared private network for all Claude instances
resource "hcloud_network" "claude_private" {
  name     = "claude-private-net"
  ip_range = "10.100.0.0/16"
}

resource "hcloud_network_subnet" "proxy_subnet" {
  network_id   = hcloud_network.claude_private.id
  type         = "cloud"
  network_zone = "eu-central"
  ip_range     = "10.100.0.0/24"
}
