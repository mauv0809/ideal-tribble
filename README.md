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
- Assigns a "ball boy" for each match using time-based rotation to ensure fairness, with atomic assignment preventing race conditions and making the process idempotent.
- Posts formatted Slack notifications for match bookings and results idempotently, preventing duplicate notifications.
- Tracks player statistics (win/loss records, sets/games won) and provides a leaderboard.
- Provides two leaderboards accessible via Slack commands: `/leaderboard` (sorted by win percentage) and `/level-leaderboard` (sorted by player level).
- Allows looking up individual player stats via the `/player-stats [name]` command.
- Provides automated matchmaking via the `/match` command with intelligent player mapping using fuzzy search.
- Integrates with Slack Events API to welcome new members automatically.
- Resiliently processes matches through a state machine, leveraging PubSub for asynchronous processing and ensuring status updates and notifications are handled reliably and idempotently across various stages.
- Secures Slack command endpoints (e.g., `/slack/command/leaderboard`) by verifying the `X-Slack-Signature` header, ensuring requests originate genuinely from Slack.
- Supports match type separation (singles/doubles) with dedicated statistics and ball-bringing tracking.
- **Web dashboard** for tracking pairing analytics, opponent breakdowns, and match history with session-based authentication and optional TOTP 2FA.
- Infrastructure is managed via Terraform for consistent, repeatable deployments.
- Includes a simple hot-reloading setup for easy local development.

## Technology Stack

- **Language:** Go
- **Web UI:** Templ templates, htmx, Pico CSS
- **Local Development:** Air
- **Infrastructure as Code:** Terraform with Terraform Cloud
- **Platform:** Docker (observability stack)
- **Deployment:** Hetzner Cloud Server, systemd service with cron scheduling
- **CI/CD:** GitHub Actions
- **Testing:** Go standard library, Testify
- **Database Migrations:** Goose
- **SSL:** Let's Encrypt via Cloudflare DNS-01 challenge

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

## Cloud Deployment with Hetzner and GitHub Actions

This guide provides a complete walkthrough for deploying the application to Hetzner Cloud using Terraform Cloud and GitHub Actions for automated continuous deployment.

#### Prerequisites

1.  **Hetzner Cloud Account:** You must have a Hetzner Cloud account with a project created.
2.  **Terraform Cloud Account:** Infrastructure state is managed through Terraform Cloud.
3.  **Domain Name:** A domain name pointing to your server (e.g., `wally-api.utiger.dk`).
4.  **A Fork of This Repository:** You should be working from your own fork of the project.

---

#### Step 1: Automated Infrastructure & Application Deployment

**Everything is automatically managed by GitHub Actions:**
- The unified deployment pipeline runs when you push to `main`
- Tests run first, then infrastructure and application build happen in parallel
- Once infrastructure is ready, the application is deployed automatically
- The infrastructure includes:
  - Hetzner Cloud server (CX22: 2 vCPU, 4GB RAM)
  - Firewall configuration for HTTP/HTTPS/SSH access
  - Cloud-init setup with nginx reverse proxy
  - Automated observability stack setup

---

#### Step 2: Configure Environment Variables

Create a `.env` file on your server with the required secrets:

```bash
# Application secrets (stored securely on server)
SLACK_BOT_TOKEN=xoxb-your-slack-token
SLACK_CHANNEL_ID=C1234567890
SLACK_SIGNING_SECRET=your-signing-secret
TENANT_ID=your-playtomic-tenant
TURSO_PRIMARY_URL=your-turso-database-url
TURSO_AUTH_TOKEN=your-turso-auth-token
DB_NAME=ideal-tribble
PORT=8080
```

---

#### Step 3: Configure Your GitHub Repository

1.  **Add Secrets to GitHub Actions:**
    - In your forked GitHub repository, go to `Settings` > `Secrets and variables` > `Actions`.
    - Create the following secrets:

| Secret Name | Value |
| :---------- | :---- |
| `TF_API_TOKEN` | Terraform Cloud API token |
| `HCLOUD_TOKEN` | Hetzner Cloud API token |
| `SSH_PRIVATE_KEY` | Private SSH key for server access |
| `SSH_PUBLIC_KEY` | Public SSH key for server configuration |
| `DB_NAME` | Database name |
| `TURSO_PRIMARY_URL` | Turso database URL |
| `TURSO_AUTH_TOKEN` | Turso database auth token |
| `SLACK_BOT_TOKEN` | Slack bot token (xoxb-*) |
| `SLACK_CHANNEL_ID` | Slack channel ID |
| `SLACK_SIGNING_SECRET` | Slack webhook signing secret |
| `TENANT_ID` | Playtomic tenant ID |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | OpenTelemetry endpoint (e.g., localhost:4317) |
| `GRAFANA_ADMIN_PASSWORD` | Grafana admin password |
| `WEB_SESSION_SECRET` | Session cookie secret (32+ characters) |
| `WEB_TOTP_ENCRYPTION_KEY` | TOTP encryption key (exactly 32 characters) |
| `CLOUDFLARE_API_TOKEN` | Cloudflare API token for DNS-01 SSL (Zone:DNS:Edit) |

---

#### Step 4: Configure DNS (Cloudflare)

1.  **Point your domains to the server:**
    - Configure A records in Cloudflare pointing to your server IP
    - `wally-api.utiger.dk` → `YOUR_SERVER_IP` (API endpoints)
    - `wally.utiger.dk` → `YOUR_SERVER_IP` (Web dashboard)
    - Both can be **proxied** (orange cloud) - DNS-01 challenge works with proxied domains
    - Set SSL/TLS mode to **Full (strict)** in Cloudflare settings

---

#### Step 5: Deploy!

With the setup complete, simply **push a commit to the `main` branch** of your forked repository.

The unified GitHub Actions workflow will automatically:

1.  **Test**: Run the full test suite with race condition detection
2.  **Infrastructure**: Deploy/update Hetzner Cloud server via Terraform Cloud
3.  **Build**: Compile the Go binary for Linux deployment
4.  **Deploy**: Copy application, scripts, and configuration to the server
5.  **Setup**: Install systemd service, observability stack, and cron jobs
6.  **Health Check**: Verify the application is running correctly

Your application will be available at:
- **API**: `https://wally-api.utiger.dk` (Slack, health, Grafana)
- **Web Dashboard**: `https://wally.utiger.dk` (pairing analytics)

SSL certificates are automatically obtained via Cloudflare DNS-01 challenge on first deploy.

#### Post-Deployment Setup

After the first successful deployment:

1.  **Create the initial admin user:**
    ```bash
    ssh root@YOUR_SERVER_IP
    cd /opt/ideal-tribble
    ./tribble-admin create-user --email admin@example.com --admin
    ```

2.  **Access the web dashboard:**
    - URL: `https://wally.utiger.dk`
    - Login with the email and password from step 1
    - Optionally enable TOTP 2FA in your profile

3.  **Access observability:**
    - Grafana: `https://wally-api.utiger.dk/grafana`
    - Username: `admin`
    - Password: Set via `GRAFANA_ADMIN_PASSWORD` secret

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

### Core API Endpoints
- `POST /fetch`: Manually triggers a fetch for new matches from Playtomic.
- `POST /process`: Manually triggers the processing of fetched matches (sending notifications, updating stats, etc.).
- `GET /health`: A simple health check endpoint that returns `OK!`.
- `GET /members`: Returns a JSON list of all known club members.
- `GET /matches`: Returns a JSON list of all processed matches.
- `GET /leaderboard`: Returns a JSON object with the current player statistics.
- `GET /metrics`: Returns a JSON object with operational metrics.
- `POST /clear`: Clears the internal store. Can accept a `matchID` query param to clear a specific match.
- `POST /test/react`: Development endpoint for testing emoji reactions (development only).


### Slack Integration Endpoints
- `POST /slack/command/leaderboard`: Responds with the formatted player leaderboard (by win %).
- `POST /slack/command/level-leaderboard`: Responds with the formatted player leaderboard (by level).
- `POST /slack/command/player-stats`: Responds with the stats for a specific player.
- `POST /slack/command/match`: Initiates the matchmaking process for the requesting player.
- `POST /slack/events`: Handles Slack Events API callbacks (member joins, reactions, etc.).

## Roadmap

Here's a look at our future development plans:

- **Automated Weekly Match Generation:**
  - On a schedule (e.g., every Sunday), the application will send a message to the Slack channel asking club members to indicate their availability for the upcoming week.
  - It will collect and parse player responses over a configured period.
  - Based on player availability and skill levels, the system will automatically propose a set of matches for the week, including suggested player pairings.
  - For each proposed match, it will assign one player to be responsible for booking the court and another to be responsible for bringing balls, ensuring fairness.
  - These "proposed" matches will be stored in the database. When a real booking from Playtomic matches a proposed match (based on players, date, and booking owner), the system will automatically link them, tracking the match from proposal to completion.
- **Endpoint Authentication:** Secure the `/fetch` and `/process` endpoints to prevent unauthorized access, ensuring that only trusted sources like cron jobs or authorized users can trigger them.

  - **Strategy 1: API Key for Service-to-Service (Recommended for Prod):**
    - Secure the `/fetch` and `/process` endpoints so they can only be invoked by authorized sources.
    - Implement middleware that checks for a secret `X-API-Key` header.
    - Store the API key securely in environment variables on the server.
  - **Strategy 2: API Key for Manual/Admin Access:**
    - Secure administrative endpoints like `/clear`, `/matches`, and `/members`.
    - Implement a middleware that checks for a secret `X-API-Key` header.
    - The API key will be stored securely in server environment variables.
  - **Strategy 3: Slack Request Signing for Commands:**
    - Secure the `/command/leaderboard` endpoint by verifying the `X-Slack-Signature` header.
    - This is a standard security practice to ensure that incoming webhook requests are genuinely from Slack.

- **API Documentation:** Create comprehensive API documentation using an OpenAPI (Swagger) specification and update this README with endpoint details.
- **Enhanced Slack Interactivity & Commands:**
  - Use interactive buttons and modals for setting availability or confirming match participation.
  - Introduce slash command `/upcoming-matches` for access to upcoming matches.
- **Remote Metrics & Monitoring:**
  - Export the application's operational metrics to a dedicated monitoring system for advanced visualization, alerting, and long-term storage.
  - Potential tools for this include Prometheus with Grafana, Datadog, or other self-hosted monitoring solutions.
- **Guest Player Management:**
  - Add a way to include guest players in a match without permanently adding them to the club's member list.
- **Weekly Stats Notification:**
  - Send a Slack notification every Sunday with the statistics (wins/losses, sets/games won) from matches played in the last 7 days. This will involve querying the `matches` table and processing the data to generate a summary of recent player performance, without storing this weekly data persistently.

- **Doubles vs Singles Separation:**
  - **Problem:** Singles matches are currently tainting doubles statistics, and players who play singles are less likely to bring balls to doubles matches due to inflated ball-bringing counts.
  - **Solution Ideas:**
    - Separate match types in database schema (add `match_type` field: "doubles" or "singles")
    - Maintain separate statistics tables/views for doubles vs singles
    - Separate ball-bringing counts per match type
    - Update matchmaking service to handle match type preferences
    - Modify Slack commands to specify match type: `/match doubles` or `/match singles`
    - Add separate leaderboards: `/leaderboard doubles` and `/leaderboard singles`

- **Time-Based Ball Boy Assignment:** ✅ **Completed**
  - Implemented a fairness-based ball-bringing assignment system that considers time since last assignment
  - Players who haven't brought balls recently are prioritized for assignment
  - Ensures fair rotation even when player groups change or new members join
  - Prevents the issue where frequent players with new members never get assigned

- **Enhanced Player Statistics System:** ✅ **Completed**
  - Web dashboard for tracking pairing analytics with session-based authentication
  - Track partner-specific statistics (win rate, streaks, opponent breakdowns)
  - Individual player performance against different opponents
  - Match history with filtering by wins/losses
  - Head-to-head records against specific opponent pairs

## License

This project is licensed under the MIT License. See the [LICENSE](LICENSE) file for details.
