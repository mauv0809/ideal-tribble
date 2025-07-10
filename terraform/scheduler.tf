resource "google_project_service" "scheduler_api" {
  project = var.gcp_project_id
  service = "cloudscheduler.googleapis.com"
  disable_on_destroy = false
}

resource "google_cloud_scheduler_job" "check_job" {
  project          = var.gcp_project_id
  name             = "${var.service_name}-check"
  description      = "Triggers the /check endpoint of the ${var.service_name} service."
  schedule         = var.scheduler_cron
  time_zone        = "Etc/UTC"
  attempt_deadline = "320s"

  http_target {
    http_method = "GET"
    uri         = "${google_cloud_run_v2_service.main.uri}/check"

    oidc_token {
      service_account_email = google_service_account.scheduler_invoker.email
    }
  }

  depends_on = [google_project_service.scheduler_api]
} 
resource "google_cloud_scheduler_job" "populate-job" {
  project          = var.gcp_project_id
  name             = "${var.service_name}-populate-club"
  description      = "Triggers the /populate-club endpoint of the ${var.service_name} service."
  schedule         = var.scheduler_cron
  time_zone        = "Etc/UTC"
  attempt_deadline = "320s"

  http_target {
    http_method = "GET"
    uri         = "${google_cloud_run_v2_service.main.uri}/populate-club"

    oidc_token {
      service_account_email = google_service_account.scheduler_invoker.email
    }
  }

  depends_on = [google_project_service.scheduler_api]
} 