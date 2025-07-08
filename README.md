# Playtomic to Slack Notifier

This is a Go application designed to run as a serverless container on Google Cloud Run. It periodically fetches upcoming bookings from a Playtomic account, filters for specific club matches, and posts notifications to a designated Slack channel.

## Features

- Fetches upcoming bookings using the [go-playtomic-api](https://github.com/rafa-garcia/go-playtomic-api).
- Intelligently filters for "club matches" based on the number of known members participating.
- Discovers and saves new club members automatically.
- Assigns a "ball boy" for each match to ensure fairness.
- Posts formatted notifications to a Slack channel.
- Infrastructure is managed via Terraform for consistent, repeatable deployments.
- Includes a simple hot-reloading setup for easy local development.

## Technology Stack

- **Language:** Go
- **Local Development:** Air
- **Infrastructure as Code:** Terraform
- **Platform:** Docker
- **Deployment:** Google Cloud Run, Google Cloud Scheduler
- **CI/CD:** Google Cloud Build

## Local Development

A simple hot-reloading environment is configured using [Air](https://github.com/cosmtrek/air).

### Prerequisites

1.  Install [Go](https://golang.org/doc/install).
2.  Install [Air](https://github.com/cosmtrek/air#installation).

### Setup & Running

1.  **Set up Environment Variables:**
    Copy the environment variable template to a new `.env` file.

    ```bash
    cp .env.example .env
    ```

    Now, open the `.env` file and fill in your actual credentials (e.g., `SLACK_BOT_TOKEN`, `SLACK_CHANNEL_ID`, `PLAYER_IDS`).

2.  **Run the Application:**
    Start the application using `air`. It will automatically watch for file changes and rebuild/restart the server.
    ```bash
    air
    ```
    The server will be running on the port specified in your `.env` file (default: `8080`).

## Deployment with Terraform

This project uses Terraform to manage all required Google Cloud infrastructure as code. This is the **only supported way** to deploy and manage the application's resources.

### Prerequisites

1.  [Install Terraform](https://learn.hashicorp.com/tutorials/terraform/install-cli).
2.  [Install the gcloud CLI](https://cloud.google.com/sdk/docs/install) and authenticate:
    ```bash
    gcloud auth application-default login
    ```
3.  Ensure your user account has the necessary permissions in the GCP project (e.g., `Owner`, or a custom role with permissions for Cloud Run, Cloud Scheduler, IAM, Secret Manager, and Cloud Storage).
4.  Create a [Google Cloud Storage bucket](https://cloud.google.com/storage/docs/creating-buckets) to store the Terraform state remotely. This is a critical best practice.

### Managing Secrets in GCP

Hardcoding secrets (like API tokens) is a major security risk. The recommended approach is to use [Google Secret Manager](https://cloud.google.com/secret-manager). The Terraform configuration is set up to automatically handle creating, securing, and providing access to these secrets.

**1. Create the Secrets in Google Cloud:**
For each environment variable your application needs (see `.env.example`), create a corresponding secret in Google Secret Manager. You can do this via the GCP Console or the `gcloud` CLI.

**Example using `gcloud`:**

```bash
# Create the secret
gcloud secrets create SLACK_BOT_TOKEN --replication-policy="automatic"

# Add the first version of the secret value
printf "your-xoxb-slack-token-here" | gcloud secrets versions add SLACK_BOT_TOKEN --data-file=-
```

Repeat this process for `SLACK_CHANNEL_ID`, `PLAYER_IDS`, etc.

**2. Mount Secrets in Terraform:**
The final step is to tell Terraform to mount these secrets as environment variables in the Cloud Run container. Open `terraform/cloud_run.tf` and add an `env` block for each secret.

```terraform
# In terraform/cloud_run.tf inside the container definition

# ...
      env {
        name = "SLACK_BOT_TOKEN"
        value_source {
          secret_key_ref {
            secret  = "SLACK_BOT_TOKEN" # The name of the secret in Secret Manager
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
      # Add blocks for PLAYER_IDS, TENANT_ID, etc.
# ...
```

Our Terraform script (`iam.tf`) automatically handles granting the Cloud Run service the necessary `Secret Manager Secret Accessor` role for all secrets listed in the `secret_names` variable. There are no manual IAM permissions to configure.

### Deployment Steps

1.  **Build and Push the Docker Image:**
    First, build your application's Docker image and push it to the Google Artifact Registry (or Container Registry).

    ```bash
    # Replace YOUR_PROJECT_ID with your actual GCP Project ID
    gcloud builds submit --config cloudbuild.yaml . --substitutions=_IMAGE_NAME="gcr.io/YOUR_PROJECT_ID/ideal-tribble"
    ```

2.  **Initialize Terraform:**
    Navigate to the `terraform` directory and run `init`, providing the name of your GCS bucket for the backend state.

    ```bash
    cd terraform
    terraform init \
      -backend-config="bucket=your-terraform-state-bucket-name"
    ```

3.  **Create and Apply a Plan:**
    Create a `terraform.tfvars` file to specify your project and image name.
    ```hcl
    # terraform/terraform.tfvars
    gcp_project_id = "your-gcp-project-id"
    image_name     = "gcr.io/your-gcp-project-id/ideal-tribble"
    ```
    Now, create and apply the plan.
    ```bash
    terraform plan -out=tfplan
    terraform apply "tfplan"
    ```

Once applied, Terraform will create all resources and output the URL of your new Cloud Run service. The Cloud Scheduler job will already be configured to trigger it automatically.

## API Endpoints

- `GET /check`: Manually triggers a check for new matches.
- `GET /health`: A simple health check endpoint that returns `OK!`.
- `GET /members`: Returns a JSON list of all known club members.
- `GET /matches`: Returns a JSON list of all processed matches.
- `POST /clear`: Clears the internal store. Can accept a `matchID` query param to clear a specific match.

## Roadmap

Here's a look at our future development plans:

- **Automated Weekly Match Generation:**
  - On a schedule (e.g., every Sunday), the application will send a message to the Slack channel asking club members to indicate their availability for the upcoming week.
  - It will collect and parse player responses over a configured period.
  - Based on player availability and skill levels, the system will automatically propose a set of matches for the week, including suggested player pairings.
  - For each proposed match, it will assign one player to be responsible for booking the court and another to be responsible for bringing balls, ensuring fairness.
  - These "proposed" matches will be stored in the database. When a real booking from Playtomic matches a proposed match (based on players, date, and booking owner), the system will automatically link them, tracking the match from proposal to completion.
- **GitHub Actions CI/CD:** Implement a full CI/CD pipeline using GitHub Actions to automate testing and deployment to Google Cloud Run.
- **Unit & Integration Testing:** Introduce a robust testing suite to ensure code quality and reliability.

## License

This project is licensed under the MIT License. See the [LICENSE](LICENSE) file for details.
