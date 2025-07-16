resource "google_project_service" "run_api" {
  project = var.gcp_project_id
  service = "run.googleapis.com"
  disable_on_destroy = false
}

resource "google_cloud_run_v2_service" "main" {
  project  = var.gcp_project_id
  name     = var.service_name
  location = var.gcp_region

  template {
    service_account = google_service_account.cloud_run.email

    containers {
      image = var.image_name
      startup_probe {
        initial_delay_seconds = 5
        timeout_seconds = 240 
        period_seconds = 240
        failure_threshold = 1
        tcp_socket {
          port = 8080
        }
      }
      liveness_probe {
        tcp_socket {
          port = 8080
        }
      }
      resources {
        limits = {
          cpu    = "1"
          memory = "512Mi"
        }
      }
      env {
        name = "DB_NAME"
        value_source {
          secret_key_ref {
            secret  = "DB_NAME"
            version = "latest"
          }
        }
      }
      env {
        name = "SLACK_BOT_TOKEN"
        value_source {
          secret_key_ref {
            secret  = "SLACK_BOT_TOKEN"
            version = "latest"
          }
        }
      }
      env {
        name = "SLACK_CHANNEL_ID"
        value_source {
          secret_key_ref {
            secret  = "SLACK_CHANNEL_ID"
            version = "latest"
          }
        }
      }
      env {
        name = "SLACK_SIGNING_SECRET"
        value_source {
          secret_key_ref {
            secret  = "SLACK_SIGNING_SECRET"
            version = "latest"
          }
        }
      }
      env {
        name = "TENANT_ID"
        value_source {
          secret_key_ref {
            secret  = "TENANT_ID"
            version = "latest"
          }
        }
      }
      env {
        name = "TURSO_PRIMARY_URL"
        value_source {
          secret_key_ref {
            secret  = "TURSO_PRIMARY_URL"
            version = "latest"
          }
        }
      }
      env {
        name = "TURSO_AUTH_TOKEN"
        value_source {
          secret_key_ref {
            secret  = "TURSO_AUTH_TOKEN"
            version = "latest"
          }
        }
      }
       env {
        name  = "GCP_PROJECT"
        value = var.gcp_project_id
      }
    }
    scaling {
      max_instance_count = 1
    }
    
  }
  traffic {
  type     = "TRAFFIC_TARGET_ALLOCATION_TYPE_REVISION"
  revision = var.stable_revision
  percent  = 100
}

traffic {
  type    = "TRAFFIC_TARGET_ALLOCATION_TYPE_LATEST"
  percent = 0
}
  # Make the service private, only allowing authenticated invocations
  ingress = "INGRESS_TRAFFIC_INTERNAL_ONLY"

  depends_on = [google_project_service.run_api]
}
