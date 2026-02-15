output "server_id" {
  value       = hcloud_server.instance.id
  description = "Hetzner server ID"
}

output "private_ip" {
  value       = local.private_ip
  description = "Private IP address on the Claude network"
}

output "public_ip" {
  value       = hcloud_server.instance.ipv4_address
  description = "Public IPv4 (temporary, removed once Netbird is set up)"
}

output "volume_id" {
  value       = hcloud_volume.data.id
  description = "Persistent volume ID"
}
