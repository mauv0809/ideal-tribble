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
- Assigns a "ball boy" for each match atomically to ensure fairness and prevent race conditions, making the assignment idempotent.
- Posts formatted Slack notifications for match bookings and results idempotently, preventing duplicate notifications.
- Tracks player statistics (win/loss records, sets/games won) and provides a leaderboard.
- Provides two leaderboards accessible via Slack commands: `/leaderboard` (sorted by win percentage) and `/level-leaderboard` (sorted by player level).
- Allows looking up individual player stats via the `/padel-stats [name]` command.
- Resiliently processes matches through a state machine, leveraging PubSub for asynchronous processing and ensuring status updates and notifications are handled reliably and idempotently across various stages.
- Secures Slack command endpoints (e.g., `/command/leaderboard`) by verifying the `X-Slack-Signature` header, ensuring requests originate genuinely from Slack.
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
- **Database Migrations:** Goose

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

## Cloud Deployment with Terraform and GitHub Actions

This guide provides a complete walkthrough for deploying the application to Google Cloud Run using Terraform for infrastructure management and GitHub Actions for continuous deployment.

#### Prerequisites

1.  **Google Cloud Project:** You must have a GCP project with billing enabled.
2.  **`gcloud` CLI:** [Install](https://cloud.google.com/sdk/docs/install) and authenticate the CLI to your account:
    ```bash
    gcloud auth login
    gcloud auth application-default login
    ```
3.  **Terraform:** [Install the Terraform CLI](https://learn.hashicorp.com/tutorials/terraform/install-cli).
4.  **A Fork of This Repository:** You should be working from your own fork of the project.

---

#### Step 1: Configure Your GCP Project

First, set your project context for the `gcloud` CLI and enable all the necessary APIs.

```bash
# Replace "your-gcp-project-id" with your actual project ID
export GCP_PROJECT_ID="your-gcp-project-id"
gcloud config set project $GCP_PROJECT_ID

# Enable the required APIs for the project
gcloud services enable \
  run.googleapis.com \
  cloudscheduler.googleapis.com \
  secretmanager.googleapis.com \
  iam.googleapis.com \
  artifactregistry.googleapis.com \
  cloudresourcemanager.googleapis.com
```

---

#### Step 2: Create a Bucket for Terraform State

It is a critical best practice to store your Terraform state file remotely.

```bash
# Replace "your-unique-bucket-name" with a globally unique name
export TF_STATE_BUCKET="your-unique-bucket-name"
gcloud storage buckets create gs://$TF_STATE_BUCKET --project=$GCP_PROJECT_ID --location=europe-west3
```

**Note:** You must update the `ci-cd.yml` workflow later to use this bucket name.

---

#### Step 3: Create Secrets in Secret Manager

Our application loads its configuration from secrets. Create a secret for each required environment variable.

**Example for one secret:**

```bash
# 1. Create the secret container
gcloud secrets create SLACK_BOT_TOKEN --replication-policy="automatic"

# 2. Add the secret value (this reads from your terminal)
printf "your-xoxb-slack-token-here" | gcloud secrets versions add SLACK_BOT_TOKEN --data-file=-
```

**Repeat the process above for all of the following secrets:**

- `DB_NAME`
- `SLACK_BOT_TOKEN`
- `SLACK_CHANNEL_ID`
- `TENANT_ID`
- `TURSO_PRIMARY_URL`
- `TURSO_AUTH_TOKEN`

---

#### Step 4: Configure Workload Identity Federation

This is the key step to allow GitHub Actions to securely authenticate with GCP.

**4a. Create a GCP Service Account for GitHub Actions**
This service account is what GitHub will impersonate.

```bash
export GITHUB_SA="github-actions-runner"
gcloud iam service-accounts create $GITHUB_SA \
  --display-name="GitHub Actions Runner SA"
```

**4b. Grant the Service Account Permissions**
This service account needs permission to manage the resources defined in our Terraform files.

```bash
# Get the full email of the service account
export GITHUB_SA_EMAIL=$(gcloud iam service-accounts list --filter="displayName:GitHub Actions Runner SA" --format="value(email)")

# Grant permissions
gcloud projects add-iam-policy-binding $GCP_PROJECT_ID \
  --member="serviceAccount:$GITHUB_SA_EMAIL" \
  --role="roles/run.admin"
gcloud projects add-iam-policy-binding $GCP_PROJECT_ID \
  --member="serviceAccount:$GITHUB_SA_EMAIL" \
  --role="roles/iam.serviceAccountUser"
gcloud projects add-iam-policy-binding $GCP_PROJECT_ID \
  --member="serviceAccount:$GITHUB_SA_EMAIL" \
  --role="roles/secretmanager.admin"
```

You also need to grant it permission to write to the Terraform state bucket:

```bash
gcloud storage buckets add-iam-member gs://$TF_STATE_BUCKET \
  --member="serviceAccount:$GITHUB_SA_EMAIL" \
  --role="roles/storage.objectAdmin"
```

**4c. Create the Workload Identity Pool and Provider**
This creates the trust relationship between GCP and GitHub.

```bash
gcloud iam workload-identity-pools create "github-pool" \
  --location="global" \
  --display-name="GitHub Actions Pool"

# Get the full ID of the new pool
export WORKLOAD_POOL_ID=$(gcloud iam workload-identity-pools list --location="global" --filter="displayName:GitHub Actions Pool" --format="value(name)")

# Create the provider, restricting it to your repository
export GITHUB_REPO="your-github-username/ideal-tribble"
gcloud iam workload-identity-pools providers create-oidc "github-provider" \
  --workload-identity-pool-id="$WORKLOAD_POOL_ID" \
  --location="global" \
  --issuer-uri="https://token.actions.githubusercontent.com" \
  --attribute-mapping="google.subject=assertion.sub,attribute.actor=assertion.actor,attribute.repository=assertion.repository"
```

**4d. Link the GCP Service Account to the GitHub Identity**
This is the final step that allows an action running in your repo to impersonate the service account.

```bash
gcloud iam service-accounts add-iam-policy-binding "$GITHUB_SA_EMAIL" \
  --role="roles/iam.workloadIdentityUser" \
  --member="principalSet://iam.googleapis.com/$WORKLOAD_POOL_ID/attribute.repository/$GITHUB_REPO"
```

---

#### Step 5: Configure Your GitHub Repository

1.  **Update the CI/CD Workflow:**

    - Open the `.github/workflows/ci-cd.yml` file.
    - Find the `TERRAFORM_STATE_BUCKET` environment variable and replace the placeholder with the unique bucket name you created in Step 2.

2.  **Add Secrets to GitHub Actions:**
    - In your forked GitHub repository, go to `Settings` > `Secrets and variables` > `Actions`.
    - Create the following three secrets:

| Secret Name                      | Value                                                                                                                                                                                      |
| :------------------------------- | :----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `GCP_PROJECT_ID`                 | Your GCP Project ID (e.g., `your-gcp-project-id`).                                                                                                                                         |
| `GCP_SERVICE_ACCOUNT`            | The full email of the service account you created in Step 4a.                                                                                                                              |
| `GCP_WORKLOAD_IDENTITY_PROVIDER` | The full name of the provider. Get this by running: <br> `gcloud iam workload-identity-pools providers list --location=global --workload-identity-pool=github-pool --format="value(name)"` |

---

#### Step 6: Deploy!

With the setup complete, simply **push a commit to the `main` branch** of your forked repository.

The GitHub Actions workflow will automatically trigger. It will:

1.  Authenticate to Google Cloud using Workload Identity Federation.
2.  Build the application's Docker image.
3.  Push the image to Google Artifact Registry.
4.  Run `terraform apply` to provision the Cloud Run service, IAM roles, and Cloud Scheduler jobs with the new image.

The URL of your live service will be available in the output of the "Deploy with Terraform" step in the GitHub Actions log.

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
  - Introduce slash command `/upcoming-matches` for access to upcoming matches.
- **Remote Metrics & Monitoring:**
  - Export the application's operational metrics to a dedicated monitoring system for advanced visualization, alerting, and long-term storage.
  - Potential tools for this include Google Cloud Monitoring, Prometheus with Grafana, or Datadog.
- **Guest Player Management:**
  - Add a way to include guest players in a match without permanently adding them to the club's member list.
- **Weekly Stats Notification:**
  - Send a Slack notification every Sunday with the statistics (wins/losses, sets/games won) from matches played in the last 7 days. This will involve querying the `matches` table and processing the data to generate a summary of recent player performance, without storing this weekly data persistently.

## License

This project is licensed under the MIT License. See the [LICENSE](LICENSE) file for details.
