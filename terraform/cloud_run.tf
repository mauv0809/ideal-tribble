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
      ports {
        container_port = 8080
      }
      # Note: Environment variables should be configured here.
      # For sensitive values like API keys and tokens, it is highly
      # recommended to use Google Cloud Secret Manager.
      #
      # example:
      # env {
      #   name = "SLACK_BOT_TOKEN"
      #   value_source {
      #     secret_key_ref {
      #       secret  = "your-slack-token-secret-name"
      #       version = "latest"
      #     }
      #   }
      # }
    }
  }

  # Make the service private, only allowing authenticated invocations
  ingress = "INGRESS_TRAFFIC_INTERNAL_ONLY"

  depends_on = [google_project_service.run_api]
} 