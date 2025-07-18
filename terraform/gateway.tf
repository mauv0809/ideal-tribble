locals {
  openapi_spec_path = "${path.module}/../openapi.yaml"
  openapi_hash      = filesha256(local.openapi_spec_path)
  api_config_id     = "v-${replace(local.openapi_hash, "/[^a-zA-Z0-9]/", "")}"
}
resource "google_project_service" "service_control" {
  project = var.gcp_project_id
  service = "servicecontrol.googleapis.com"
}
resource "google_project_service" "apigateway" {
  project = var.gcp_project_id
  service = "apigateway.googleapis.com"
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
      path     = local.openapi_spec_path
      contents = filebase64(local.openapi_spec_path)
    }
  }
}

resource "google_api_gateway_gateway" "wally_gateway" {
  gateway_id = "wally-gateway"
  api_config = google_api_gateway_api_config.wally_api_config.name
  provider = google-beta
}