locals {
  openapi_spec_path     = "${path.module}/../openapi.yaml"
  openapi_spec_template = templatefile(local.openapi_spec_path, {
    CLOUD_RUN_URL = google_cloud_run_v2_service.main.uri
    DOMAIN = var.domain
  })
  openapi_hash     = sha256(local.openapi_spec_template)
  api_config_id    = "v-${replace(local.openapi_hash, "/[^a-zA-Z0-9]/", "")}"
}
# Enable required APIs
resource "google_project_service" "service_control" {
  project = var.gcp_project_id
  service = "servicecontrol.googleapis.com"
}

resource "google_project_service" "apigateway" {
  project = var.gcp_project_id
  service = "apigateway.googleapis.com"
}

resource "google_project_service" "compute" {
  project = var.gcp_project_id
  service = "compute.googleapis.com"
}
resource "google_api_gateway_api" "wally_api" {
  provider = google-beta
  api_id = "wally-api"
}

resource "google_api_gateway_api_config" "wally_api_config" {
  provider = google-beta
  api       = google_api_gateway_api.wally_api.name
  api_config_id = local.api_config_id

  openapi_documents {
    document {
      path     = "openapi.yaml"
      contents = base64encode(local.openapi_spec_template)
    }
  }

  depends_on = [google_api_gateway_api.wally_api]
}

resource "google_api_gateway_gateway" "wally_gateway" {
  gateway_id = "wally-gateway"
  api_config = google_api_gateway_api_config.wally_api_config.name
  provider   = google-beta

  depends_on = [
    google_project_service.apigateway,
    google_project_service.service_control
  ]
}

# Cloud Armor Security Policy for DDoS protection and rate limiting
resource "google_compute_security_policy" "wally_security_policy" {
  name        = "wally-security-policy"
  description = "Security policy for Wally API Gateway - DDoS protection and rate limiting"

  depends_on = [google_project_service.compute]

  # Default rule - allow all traffic (Slack signature verification handles auth)
  rule {
    action   = "allow"
    priority = 2147483647
    match {
      versioned_expr = "SRC_IPS_V1"
      config {
        src_ip_ranges = ["*"]
      }
    }
    description = "Default allow rule"
  }

  # Rate limiting rule - 100 requests per minute per IP
  rule {
    action   = "rate_based_ban"
    priority = 1000
    match {
      versioned_expr = "SRC_IPS_V1"
      config {
        src_ip_ranges = ["*"]
      }
    }
    rate_limit_options {
      conform_action = "allow"
      exceed_action  = "deny(429)"
      enforce_on_key = "IP"
      rate_limit_threshold {
        count        = 100
        interval_sec = 60
      }
      ban_duration_sec = 300 # 5 minute ban for rate limit violations
    }
    description = "Rate limit rule: 100 requests per minute per IP"
  }

  # Block common attack patterns
  rule {
    action   = "deny(403)"
    priority = 500
    match {
      expr {
        expression = "request.path.contains('..') || request.path.contains('etc/passwd') || request.path.contains('wp-admin')"
      }
    }
    description = "Block common attack patterns"
  }

  # Geo-blocking rule (optional - block high-risk countries)
  rule {
    action   = "deny(403)"
    priority = 300
    match {
      expr {
        expression = "origin.region_code == 'CN' || origin.region_code == 'RU' || origin.region_code == 'KP'"
      }
    }
    description = "Block requests from high-risk countries"
  }
}

# Backend service for the API Gateway
resource "google_compute_backend_service" "wally_backend" {
  name                  = "wally-api-backend"
  description           = "Backend service for Wally API Gateway"
  protocol              = "HTTPS"
  timeout_sec           = 30
  enable_cdn            = false
  load_balancing_scheme = "EXTERNAL_MANAGED"

  # Point to the API Gateway
  backend {
    group = google_api_gateway_gateway.wally_gateway.default_hostname
  }

  security_policy = google_compute_security_policy.wally_security_policy.id

  log_config {
    enable      = true
    sample_rate = 1.0
  }
}

# URL map for the load balancer
resource "google_compute_url_map" "wally_url_map" {
  name            = "wally-url-map"
  description     = "URL map for Wally API Gateway"
  default_service = google_compute_backend_service.wally_backend.id

  host_rule {
    hosts        = ["wally-api.${var.domain}"]
    path_matcher = "allpaths"
  }

  path_matcher {
    name            = "allpaths"
    default_service = google_compute_backend_service.wally_backend.id

    path_rule {
      paths   = ["/slack/*"]
      service = google_compute_backend_service.wally_backend.id
    }
  }
}

# HTTP(S) Load Balancer
resource "google_compute_target_https_proxy" "wally_https_proxy" {
  name             = "wally-https-proxy"
  url_map          = google_compute_url_map.wally_url_map.id
  ssl_certificates = [google_compute_managed_ssl_certificate.wally_ssl_cert.id]
}

# Managed SSL Certificate
resource "google_compute_managed_ssl_certificate" "wally_ssl_cert" {
  name = "wally-ssl-cert"

  depends_on = [google_project_service.compute]

  managed {
    domains = ["wally-api.${var.domain}"]
  }
}

# Global forwarding rule (external IP)
resource "google_compute_global_forwarding_rule" "wally_forwarding_rule" {
  name                  = "wally-forwarding-rule"
  ip_protocol           = "TCP"
  load_balancing_scheme = "EXTERNAL_MANAGED"
  port_range           = "443"
  target               = google_compute_target_https_proxy.wally_https_proxy.id
  ip_address           = google_compute_global_address.wally_ip.id
}

# Reserve a static external IP
resource "google_compute_global_address" "wally_ip" {
  name         = "wally-external-ip"
  address_type = "EXTERNAL"
  ip_version   = "IPV4"

  depends_on = [google_project_service.compute]
}

# Output the external IP and gateway URL
output "wally_external_ip" {
  description = "External IP address for Wally API Gateway"
  value       = google_compute_global_address.wally_ip.address
}

output "wally_gateway_url" {
  description = "API Gateway URL"
  value       = google_api_gateway_gateway.wally_gateway.default_hostname
}

output "wally_load_balancer_url" {
  description = "Load Balancer URL (use this for Slack webhooks)"
  value       = "https://wally-api.${var.domain}"
}