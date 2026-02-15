terraform {
  required_providers {
    hcloud = {
      source  = "hetznercloud/hcloud"
      version = "~> 1.48"
    }
  }
}

variable "user_id" {
  type        = number
  description = "User ID for this instance"
}

variable "hcloud_token" {
  type        = string
  sensitive   = true
  description = "Hetzner Cloud API token"
}

variable "server_type" {
  type        = string
  default     = "cx22"
  description = "Hetzner server type"
}

variable "location" {
  type        = string
  default     = "nbg1"
  description = "Hetzner datacenter location"
}

variable "image" {
  type        = string
  default     = "ubuntu-24.04"
  description = "Server OS image"
}

variable "netbird_setup_key" {
  type        = string
  default     = ""
  sensitive   = true
  description = "Netbird setup key for zero-trust enrollment (empty = skip)"
}

provider "hcloud" {
  token = var.hcloud_token
}

# Dynamic subnet: 10.100.{user_id % 250 + 1}.0/24
locals {
  subnet_octet = (var.user_id % 250) + 1
  private_ip   = "10.100.${local.subnet_octet}.10"
}

# Persistent volume for user data
resource "hcloud_volume" "data" {
  name      = "claude-data-${var.user_id}"
  size      = 20
  location  = var.location
  format    = "ext4"

  lifecycle {
    prevent_destroy = false # Set to true in production
  }
}

# User instance server
resource "hcloud_server" "instance" {
  name        = "claude-${var.user_id}"
  server_type = var.server_type
  image       = var.image
  location    = var.location

  user_data = templatefile("${path.module}/cloud-init.yaml.tpl", {
    user_id           = var.user_id
    netbird_setup_key = var.netbird_setup_key
  })

  labels = {
    managed_by = "cloudcode"
    user_id    = tostring(var.user_id)
  }
}

# Attach volume to server
resource "hcloud_volume_attachment" "data" {
  volume_id = hcloud_volume.data.id
  server_id = hcloud_server.instance.id
  automount = true
}
