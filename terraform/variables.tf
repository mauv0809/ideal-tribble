variable "gcp_project_id" {
  description = "The GCP project ID to deploy to."
  type        = string
}

variable "gcp_region" {
  description = "The GCP region to deploy resources in."
  type        = string
  default     = "europe-west3"
}

variable "service_name" {
  description = "The name of the Cloud Run service."
  type        = string
  default     = "ideal-tribble"
}

variable "image_name" {
  description = "The full name of the container image to deploy (e.g., 'gcr.io/your-project/ideal-tribble')."
  type        = string
}

variable "fetch_cron_schedule" {
  description = "The cron schedule for the fetch job."
  type        = string
  default     = "*/5 * * * *" # Every 5 minutes
}

variable "process_cron_schedule" {
  description = "The cron schedule for the process job."
  type        = string
  default     = "* * * * *" # Every minute
}

variable "secret_names" {
  description = "A list of secret names to grant the Cloud Run service access to."
  type        = list(string)
  default = [
    "DB_NAME",
    "SLACK_BOT_TOKEN",
    "SLACK_CHANNEL_ID",
    "BOOKING_FILTER",
    "TENANT_ID",
    "TURSO_PRIMARY_URL",
    "TURSO_AUTH_TOKEN"
  ]
} 