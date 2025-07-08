terraform {
  required_providers {
    google = {
      source  = "hashicorp/google"
      version = ">= 4.0.0"
    }
  }

  backend "gcs" {
    # This will be configured via the CLI or a backend.hcl file
    # for security and flexibility. For example:
    # terraform init -backend-config="bucket=your-tf-state-bucket"
  }
}

provider "google" {
  project = var.gcp_project_id
  region  = var.gcp_region
} 