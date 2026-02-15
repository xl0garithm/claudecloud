variable "environment" {
  type        = string
  default     = "dev"
  description = "Environment name (dev, staging, prod)"
}

variable "region" {
  type        = string
  default     = "nbg1"
  description = "Hetzner datacenter region"
}
