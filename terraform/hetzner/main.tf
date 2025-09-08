terraform {
  required_providers {
    hcloud = {
      source  = "hetznercloud/hcloud"
      version = "~> 1.45"
    }
  }
  required_version = ">= 1.0"
  
  # Spacelift manages the backend automatically
  # No backend configuration needed
}

provider "hcloud" {
  token = var.hcloud_token
}

# SSH key for server access
resource "hcloud_ssh_key" "default" {
  name       = "ideal-tribble-deploy"
  public_key = var.ssh_public_key
}

# Server instance
resource "hcloud_server" "tribble" {
  name        = var.server_name
  image       = "ubuntu-22.04"
  server_type = "cpx22"  # 2 vCPU, 4GB RAM, €3.29/month
  location    = "fsn1"   # Falkenstein, Germany (closest to Denmark)

  ssh_keys = [hcloud_ssh_key.default.id]

  # Cloud-init user data for initial setup
  user_data = templatefile("${path.module}/cloud-init.yml", {
    APP_USER = "tribble"
    APP_DIR  = "/opt/ideal-tribble"
  })

  labels = {
    environment = "production"
    service     = "ideal-tribble"
  }
}

# Firewall for security
resource "hcloud_firewall" "tribble_firewall" {
  name = "tribble-firewall"

  # SSH access
  rule {
    direction = "in"
    port      = "22"
    protocol  = "tcp"
    source_ips = [
      "0.0.0.0/0",
      "::/0"
    ]
  }

  # HTTP/HTTPS for the application
  rule {
    direction = "in"
    port      = "80"
    protocol  = "tcp" 
    source_ips = [
      "0.0.0.0/0",
      "::/0"
    ]
  }

  rule {
    direction = "in"
    port      = "443"
    protocol  = "tcp"
    source_ips = [
      "0.0.0.0/0", 
      "::/0"
    ]
  }

  # Application port (if needed for direct access)
  rule {
    direction = "in"
    port      = "8080"
    protocol  = "tcp"
    source_ips = [
      "0.0.0.0/0",
      "::/0" 
    ]
  }
}

# Attach firewall to server
resource "hcloud_firewall_attachment" "tribble_firewall_attachment" {
  firewall_id = hcloud_firewall.tribble_firewall.id
  server_ids  = [hcloud_server.tribble.id]
}