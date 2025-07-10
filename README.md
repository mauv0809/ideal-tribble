# ideal-tribble

`ideal-tribble` is the project name for a backend service that helps a private Padel club manage their match bookings and player coordination.

## Meet Wally 👋

The primary user interface for the club members is a Slack bot named **Wally**. Wally is responsible for:

- Announcing new matches as they are booked.
- Reporting the results of finished matches.
- Helping manage who is bringing balls to the game.

The name "Wally" is inspired by the helpful robot and the glass walls of the padel court.

## Features

- Fetches upcoming bookings using the [go-playtomic-api](https://github.com/rafa-garcia/go-playtomic-api).
- Intelligently filters for "club matches" based on the number of known members participating.
- Discovers and saves new club members automatically.
- Assigns a "ball boy" for each match to ensure fairness.
- Posts formatted notifications to a Slack channel.
- Tracks player statistics (win/loss records, sets/games won) and provides a leaderboard.
- Provides two leaderboards accessible via Slack commands: `/leaderboard` (sorted by win percentage) and `/level-leaderboard` (sorted by player level).
- Allows looking up individual player stats via the `/padel-stats [name]` command.
- Resiliently processes matches through a state machine to ensure notifications are sent and stats are updated reliably.
- Infrastructure is managed via Terraform for consistent, repeatable deployments.
- Includes a simple hot-reloading setup for easy local development.

## Technology Stack

- **Language:** Go
- **Local Development:** Air
- **Infrastructure as Code:** Terraform
- **Platform:** Docker
- **Deployment:** Google Cloud Run, Google Cloud Scheduler
- **CI/CD:** GitHub Actions
- **Testing:** Go standard library, Testify

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

### Automated Deployment with GitHub Actions

This project uses a GitHub Actions workflow to automate the entire testing, building, and deployment process. The workflow is defined in `.github/workflows/ci-cd.yml`.

**Workflow Trigger:**
The CI/CD pipeline is automatically triggered on every `push` to the `main` branch.

**Workflow Steps:**

1.  **Build and Push:** The workflow builds the application's Docker image and pushes it to the Google Artifact Registry, tagged with the Git commit SHA.
2.  **Deploy with Terraform:** It then uses Terraform to deploy the new image to Google Cloud Run. It passes the new image tag to the Terraform configuration, ensuring that your infrastructure state is always in sync.

**Setup for Your Fork:**
To get the automated deployment working on your own fork of this repository, you must configure the following secrets in your GitHub repository's settings (`Settings` > `Secrets and variables` > `Actions`):

- `GCP_PROJECT_ID`: Your Google Cloud project ID.
- `GCP_WORKLOAD_IDENTITY_PROVIDER`: The full name of your Workload Identity Provider (e.g., `projects/123.../locations/global/workloadIdentityPools/my-pool/providers/my-provider`). This is the recommended, most secure way to authenticate.
- `GCP_SERVICE_ACCOUNT`: The email of the GCP service account that you've granted permissions to deploy to Cloud Run and push to Artifact Registry.

You must also update the `TERRAFORM_STATE_BUCKET` environment variable in the `.github/workflows/ci-cd.yml` file to point to your own GCS bucket.

Once applied, Terraform will create all resources and output the URL of your new Cloud Run service. The Cloud Scheduler job will already be configured to trigger it automatically.

## Testing

This project includes a comprehensive test suite covering core business logic, API handlers, and client wrappers.

The application also tracks key operational metrics (e.g., number of checks run, Playtomic API calls, Slack notifications sent) and exposes them via a dedicated endpoint.

To run all tests locally, use the following command:

```bash
go test -v -race ./...
```

The tests are also automatically executed by the GitHub Actions workflow on every push to the `main` branch.

## API Endpoints

The application exposes the following HTTP endpoints:

- `POST /fetch`: Manually triggers a fetch for new matches from Playtomic.
- `POST /process`: Manually triggers the processing of fetched matches (sending notifications, updating stats, etc.).
- `GET /health`: A simple health check endpoint that returns `OK!`.
- `GET /members`: Returns a JSON list of all known club members.
- `GET /matches`: Returns a JSON list of all processed matches.
- `GET /leaderboard`: Returns a JSON object with the current player statistics.
- `GET /metrics`: Returns a JSON object with operational metrics.
- `POST /clear`: Clears the internal store. Can accept a `matchID` query param to clear a specific match.

The application also exposes an endpoint to be used with a Slack slash command:

- `POST /command/leaderboard`: Responds with the formatted player leaderboard (by win %).
- `POST /command/level-leaderboard`: Responds with the formatted player leaderboard (by level).
- `POST /command/player-stats`: Responds with the stats for a specific player.

## Roadmap

Here's a look at our future development plans:

- **Automated Weekly Match Generation:**
  - On a schedule (e.g., every Sunday), the application will send a message to the Slack channel asking club members to indicate their availability for the upcoming week.
  - It will collect and parse player responses over a configured period.
  - Based on player availability and skill levels, the system will automatically propose a set of matches for the week, including suggested player pairings.
  - For each proposed match, it will assign one player to be responsible for booking the court and another to be responsible for bringing balls, ensuring fairness.
  - These "proposed" matches will be stored in the database. When a real booking from Playtomic matches a proposed match (based on players, date, and booking owner), the system will automatically link them, tracking the match from proposal to completion.
- **Endpoint Authentication:** Secure the `/fetch` and `/process` endpoints to prevent unauthorized access, ensuring that only trusted sources like Google Cloud Scheduler or authorized users can trigger them.

  - **Strategy 1: OIDC for Service-to-Service (Recommended for Prod):**
    - Secure the `/fetch` and `/process` endpoints so they can only be invoked by Google Cloud Scheduler.
    - Configure the Cloud Run service to only accept requests with a valid OIDC token from a specific service account.
    - Configure the Cloud Scheduler job to use this service account to authenticate its requests.
  - **Strategy 2: API Key for Manual/Admin Access:**
    - Secure administrative endpoints like `/clear`, `/matches`, and `/members`.
    - Implement a middleware that checks for a secret `X-API-Key` header.
    - The API key will be stored securely in Google Secret Manager.
  - **Strategy 3: Slack Request Signing for Commands:**
    - Secure the `/command/leaderboard` endpoint by verifying the `X-Slack-Signature` header.
    - This is a standard security practice to ensure that incoming webhook requests are genuinely from Slack.

- **API Documentation:** Create comprehensive API documentation using an OpenAPI (Swagger) specification and update this README with endpoint details.
- **Enhanced Slack Interactivity & Commands:**
  - Use interactive buttons and modals for setting availability or confirming match participation.
  - Introduce slash commands like `/padel-availability` for quick access to information.
- **Remote Metrics & Monitoring:**
  - Export the application's operational metrics to a dedicated monitoring system for advanced visualization, alerting, and long-term storage.
  - Potential tools for this include Google Cloud Monitoring, Prometheus with Grafana, or Datadog.
- **Guest Player Management:**
  - Add a way to include guest players in a match without permanently adding them to the club's member list.
- **Update CLI testing tool to support all API endpoints with dryRun and verbose options**

## License

This project is licensed under the MIT License. See the [LICENSE](LICENSE) file for details.
