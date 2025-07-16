# Enable Pub/Sub API
resource "google_project_service" "pubsub_api" {
  project = var.gcp_project_id
  service = "pubsub.googleapis.com"
  disable_on_destroy = false
}

# Create all Pub/Sub topics
resource "google_pubsub_topic" "events" {
  for_each   = var.pubsub_topics
  name       = each.key
  project    = var.gcp_project_id
  depends_on = [google_project_service.pubsub_api]
}

# Create push subscriptions for each topic
resource "google_pubsub_subscription" "event_push_subs" {
  for_each = var.pubsub_topics

  name  = "${each.key}-sub"
  topic = google_pubsub_topic.events[each.key].id

  push_config {
    push_endpoint = "${google_cloud_run_v2_service.main.uri}${each.value}"

    oidc_token {
      service_account_email = google_service_account.cloud_run.email
    }
  }

  ack_deadline_seconds         = 30
  message_retention_duration   = "600s"  # Optional: 10 minutes
  retain_acked_messages        = false  # Optional
}
