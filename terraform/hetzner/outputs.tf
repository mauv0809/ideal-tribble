output "server_ip" {
  description = "Public IP address of the server"
  value       = hcloud_server.tribble.ipv4_address
}

output "server_name" {
  description = "Name of the server"
  value       = hcloud_server.tribble.name
}

output "ssh_command" {
  description = "SSH command to connect to the server"
  value       = "ssh root@${hcloud_server.tribble.ipv4_address}"
}

output "dns_instructions" {
  description = "DNS configuration instructions"
  value = <<-EOT
    Configure your DNS to point to the server:
    
    A Record: wally-api.${var.domain} -> ${hcloud_server.tribble.ipv4_address}
    
    Then run these commands on the server:
    1. ./scripts/setup-secrets.sh
    2. ./scripts/install-systemd-service.sh  
    3. ./scripts/setup-cron.sh
    4. sudo systemctl start ideal-tribble
  EOT
}