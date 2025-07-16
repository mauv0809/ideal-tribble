resource "google_project_service" "scheduler_api" {
  project = var.gcp_project_id
  service = "cloudscheduler.googleapis.com"
  disable_on_destroy = false
}

resource "google_cloud_scheduler_job" "fetch_job" {
  project          = var.gcp_project_id
  name             = "${var.service_name}-fetch"
  description      = "Triggers the ${var.fetch_path} endpoint to get new matches from Playtomic."
  schedule         = var.fetch_cron_schedule
  time_zone        = "Europe/Copenhagen"
  attempt_deadline = "320s"
  paused           = false

  http_target {
    http_method = "POST"
    uri         = "${google_cloud_run_v2_service.main.uri}${var.fetch_path}"

    oidc_token {
      service_account_email = google_service_account.scheduler_invoker.email
    }
  }

  depends_on = [
    google_project_service.scheduler_api,
    google_cloud_run_v2_service.main,
    google_service_account.scheduler_invoker
  ]
}

resource "google_cloud_scheduler_job" "process_job" {
  project          = var.gcp_project_id
  name             = "${var.service_name}-process"
  description      = "Triggers the ${var.process_path} endpoint to handle fetched matches."
  schedule         = var.process_cron_schedule
  time_zone        = "Europe/Copenhagen"
  attempt_deadline = "320s"
  paused           = true

  http_target {
    http_method = "POST"
    uri         = "${google_cloud_run_v2_service.main.uri}${var.process_path}"

    oidc_token {
      service_account_email = google_service_account.scheduler_invoker.email
    }
  }

  depends_on = [
    google_project_service.scheduler_api,
    google_cloud_run_v2_service.main,
    google_service_account.scheduler_invoker
  ]
}
