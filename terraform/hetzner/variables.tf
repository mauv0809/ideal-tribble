variable "hcloud_token" {
  description = "Hetzner Cloud API token"
  type        = string
  sensitive   = true
}

variable "ssh_public_key" {
  description = "SSH public key for server access"
  type        = string
}

variable "server_name" {
  description = "Name of the Hetzner server"
  type        = string
  default     = "ideal-tribble"
}

variable "domain" {
  description = "Domain name for the application (e.g., 'utiger.dk' for 'wally-api.utiger.dk')"
  type        = string
  default     = "utiger.dk"
}