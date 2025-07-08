# Playtomic to Slack Notifier

This is a Go application designed to run as a serverless container on Google Cloud Run. It periodically fetches upcoming bookings from a Playtomic account, filters for specific bookings, and posts a notification to a designated Slack channel.

## Features

- Fetches upcoming bookings using the [go-playtomic-api](https://github.com/rafa-garcia/go-playtomic-api) library.
- Posts formatted notifications to a Slack channel using [slack-go](https://github.com/slack-go/slack).
- Configurable via environment variables, making it perfect for cloud environments.
- Packaged as a lightweight, secure Docker container.
- Includes a `cloudbuild.yaml` for easy, repeatable deployments to Google Cloud Run.

## Technology Stack

- **Language:** Go
- **Platform:** Docker
- **Deployment:** Google Cloud Run
- **CI/CD:** Google Cloud Build

## Setup & Configuration

The application is configured entirely through environment variables.

| Variable           | Description                                                                                                                  | Default | Required |
| ------------------ | ---------------------------------------------------------------------------------------------------------------------------- | ------- | :------: |
| `PLAYTOMIC_USER`   | Your Playtomic account email/username.                                                                                       | -       |   Yes    |
| `PLAYTOMIC_PASS`   | Your Playtomic account password.                                                                                             | -       |   Yes    |
| `SLACK_BOT_TOKEN`  | The bot token for your Slack App (starts with `xoxb-`).                                                                      | -       |   Yes    |
| `SLACK_CHANNEL_ID` | The ID of the Slack channel to post notifications to (e.g., `C12345678`).                                                    | -       |   Yes    |
| `BOOKING_FILTER`   | A case-insensitive string to filter bookings. The app will notify if this string appears in the court name or resource type. | `Padel` |    No    |
| `PORT`             | The port the internal web server listens on. Cloud Run sets this automatically.                                              | `8080`  |    No    |

**Security Note:** For sensitive values like passwords and tokens, it is **highly recommended** to use [Google Secret Manager](https://cloud.google.com/secret-manager) instead of putting them directly in the `cloudbuild.yaml` file. You can grant your Cloud Run service access to these secrets.

## Deployment

This project is set up for easy deployment using Google Cloud Build.

1.  **Prerequisites:**

    - A Google Cloud Project with the **Cloud Run API** and **Cloud Build API** enabled.
    - The `gcloud` CLI installed and authenticated.
    - Your project source code (including `main.go`, `Dockerfile`, and `cloudbuild.yaml`) pushed to a repository like GitHub or Cloud Source Repositories, or available locally.

2.  **Configure `cloudbuild.yaml`:**

    - Open the `cloudbuild.yaml` file.
    - In the final `deploy` step, replace the placeholder values in the `--set-env-vars` flag with your actual credentials and configuration.

3.  **Submit the Build:**
    - Navigate to the root directory of the project in your terminal.
    - Run the following command:
      ```bash
      gcloud builds submit --config cloudbuild.yaml .
      ```
    - This command will build the Docker image, push it to the Google Container Registry, and deploy it as a new service on Cloud Run.

## Usage

The application runs an HTTP server and exposes a single endpoint: `/check`.

When this endpoint receives an HTTP GET request, it triggers the process of fetching bookings and sending notifications.

### Automation with Cloud Scheduler

To run this check automatically, you can use [Google Cloud Scheduler](https://cloud.google.com/scheduler):

1.  Create a new Cloud Scheduler job.
2.  Set the **Frequency** to your desired interval (e.g., `*/15 * * * *` for every 15 minutes).
3.  Set the **Target type** to `HTTP`.
4.  Set the **URL** to the URL of your deployed Cloud Run service, followed by `/check`.
5.  Set the **HTTP method** to `GET`.
6.  If your Cloud Run service requires authentication, configure the **Auth** section accordingly.

## Roadmap

Here's a look at our future development plans:

- **Endpoint Authentication:** Secure the `/check` endpoint to prevent unauthorized access.
- **GitHub Actions CI/CD:** Implement a full CI/CD pipeline using GitHub Actions to automate testing and deployment to Google Cloud Run.
- **Unit & Integration Testing:** Introduce a robust testing suite to ensure code quality and reliability.
- **API Documentation:** Create comprehensive API documentation using an OpenAPI (Swagger) specification and update this README with endpoint details.

## License

This project is licensed under the MIT License. See the [LICENSE](LICENSE) file for details.
