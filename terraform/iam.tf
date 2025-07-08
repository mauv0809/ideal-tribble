# Service account for the Cloud Run service itself
resource "google_service_account" "cloud_run" {
  account_id   = "${var.service_name}-run"
  display_name = "Service Account for ${var.service_name} Cloud Run service"
}

# Service account for the Cloud Scheduler job to invoke Cloud Run
resource "google_service_account" "scheduler_invoker" {
  account_id   = "${var.service_name}-invoker"
  display_name = "Service Account for invoking ${var.service_name}"
}

# Grant the scheduler's service account the permission to invoke the Cloud Run service.
resource "google_cloud_run_service_iam_member" "invoker" {
  location = google_cloud_run_v2_service.main.location
  project  = google_cloud_run_v2_service.main.project
  name     = google_cloud_run_v2_service.main.name
  role     = "roles/run.invoker"
  member   = "serviceAccount:${google_service_account.scheduler_invoker.email}"
}

# Grant the Cloud Run service account access to the necessary secrets in Secret Manager.
resource "google_secret_manager_secret_iam_member" "secret_accessor" {
  project   = var.gcp_project_id
  for_each  = toset(var.secret_names)
  secret_id = each.key
  role      = "roles/secretmanager.secretAccessor"
  member    = "serviceAccount:${google_service_account.cloud_run.email}"
} 