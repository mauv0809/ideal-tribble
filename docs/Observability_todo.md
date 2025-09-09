Of course. Here is the updated observability plan, revised to reflect the new self-hosted strategy using the Grafana stack. You can give this directly to another AI for implementation.

Guide: Instrumenting ideal-tribble with OpenTelemetry and a Self-Hosted Grafana Stack
Objective
This guide details the process for integrating a comprehensive telemetry solution into the ideal-tribble Go application running on a Hetzner VPS. The goal is to gain deep insights into application performance and correlate traces with logs using a free, open-source, and self-hosted stack.

We will use the following technology stack for this task:

Instrumentation Standard: OpenTelemetry (OTel) for vendor-neutral collection of traces and logs.

Telemetry Backend:

Collection: OpenTelemetry Collector

Traces: Grafana Tempo

Logs: Grafana Loki

Visualization: Grafana

Step 0: Set Up the Observability Backend on the VPS
Before instrumenting the application, set up the backend services that will receive and store telemetry data. The easiest method is using Docker Compose on the Hetzner VPS.

Create a docker-compose.yml file to define the Grafana, Tempo, Loki, and OTel Collector services with the following specifications:
- **Resource Limits**: Set memory limits (Grafana: 256MB, Tempo: 512MB, Loki: 256MB, OTel Collector: 128MB)
- **Storage**: Use bind mounts to `/var/lib/observability/` for persistence
- **Networking**: Internal network, only Grafana exposed to nginx proxy

Create the necessary configuration files for each service:

otel-collector-config.yaml: To configure the collector's receivers (OTLP) and exporters (to Tempo and Loki).

tempo.yaml: To configure Tempo for local storage with 7-day retention.

loki.yaml: To configure Loki with 7-day retention and automatic cleanup.

grafana-datasources.yaml: To automatically provision Grafana with datasources for Tempo and Loki.

grafana.ini: Configure Grafana to run at `/grafana` path and set custom admin password.

Create observability.service systemd unit file to manage docker-compose.

Update nginx configuration to proxy `/grafana/*` to `http://localhost:3000`.

Launch the entire stack using systemctl start observability.

Step 1: Add Necessary Go Dependencies
First, update the project's dependencies to include the standard OTLP exporter, which will send data to the OTel Collector. Remove any Google Cloud-specific exporters.

Run the appropriate go get command to add the modules for the OpenTelemetry base SDK, trace, contrib http instrumentation, the OTLP gRPC trace exporter, and semantic conventions.

Step 2: Create a Centralized Telemetry Package
To keep the code organized, create a new package internal/telemetry. Inside this package, create the file otel.go.

File: internal/telemetry/otel.go
In this file, implement the initialization logic for OpenTelemetry.

Create a function named InitOtel that accepts a context.Context and returns a shutdown function (func()) and an error.

Inside this function, retrieve the OTel Collector endpoint from the OTEL_EXPORTER_OTLP_ENDPOINT environment variable. Default to localhost:4317 if it is not set.

Initialize the OTLP gRPC trace exporter, configuring it to connect to the collector's endpoint. For a simple VPS setup, this connection can be insecure (no TLS).

Define an OpenTelemetry Resource that identifies the service with the name ideal-tribble and includes other relevant attributes like service.version and deployment.environment from environment variables.

Create and set the global TracerProvider using the OTLP exporter and the defined resource. Configure with 100% sampling rate (no sampling) to trace every request.

Return a shutdown function that calls the TracerProvider.Shutdown method to ensure telemetry is flushed before the app exits.

Also, create a helper function SlogWithTrace that accepts a context.Context. This function should extract the trace and span ID from the context and return a slog.Logger instance enriched with these fields for correlated logging. This function remains unchanged from the previous plan.

Step 3: Integrate Telemetry into the Application Entrypoint (main.go)
Modify your main application entrypoint, main.go, to use the new telemetry package. No changes are required here from the original plan, as the application code is decoupled from the backend.

At the start of your main function, set up a structured JSON logger (slog) as the application default.

Call your telemetry.InitOtel() function at the start of main, handling any potential error. Defer the returned shutdown function.

After setting up your main HTTP router (e.g., mux), wrap it with the OpenTelemetry middleware (otelhttp.NewHandler) to enable tracing on all incoming requests.

Use this final, wrapped handler when you call http.ListenAndServe.

Inside one of your existing HTTP handlers (like /health), add an example of using your new telemetry.SlogWithTrace(r.Context()) helper to demonstrate correlated logging.

Step 4: Update Environment and Verification Steps
To complete the task, you must update the configuration and verify that the integration is working.

1. Update Environment Variables
Add the following new variables to your .env.example file. The Google Cloud-specific variables are no longer needed.

OTEL_EXPORTER_OTLP_ENDPOINT=localhost:4317
APP_ENV=production
APP_VERSION=v1.0.2
GRAFANA_ADMIN_PASSWORD=your-secure-password-here

2. Update Production Secrets
This step is no longer necessary as we are not connecting to a cloud provider's services that require authentication.

3. Verification Plan
After deploying these changes, follow these steps to verify that everything is working:

Trigger an Endpoint: Make a request to one of the application's endpoints (e.g., /health).

Check Grafana for Traces:

Navigate to your Grafana instance (https://wally-api.utiger.dk/grafana).

Go to Explore and select the Tempo data source.

You should be able to search for and find a new trace for the request you just made, tagged with the service name ideal-tribble.

Check Grafana for Logs and Correlation:

In the Explore view, switch the data source to Loki.

You should see your slog JSON output. The log entry must contain trace_id and span_id fields.

Verify that you can click the trace_id in the log entry to pivot directly to the corresponding trace in Tempo.

## Detailed Implementation Specifications

### Docker Compose Configuration
- Use `/var/lib/observability/` for all persistent data
- Set restart policies to `unless-stopped` for all services
- Configure internal docker network `observability` 
- Only Grafana container exposes ports (3000 internal only)

### Systemd Service Configuration
Create `/etc/systemd/system/observability.service`:
- Runs docker-compose up/down
- Starts before ideal-tribble.service (ordering only, no dependency)
- Independent restart behavior
- Includes ExecStop for graceful shutdown

### Nginx Configuration Updates
Add to existing `/etc/nginx/sites-available/ideal-tribble`:
```
location /grafana/ {
    proxy_pass http://localhost:3000/;
    proxy_set_header Host $host;
    proxy_set_header X-Real-IP $remote_addr;
}
```

### Resource Monitoring
- Total observability stack memory: ~1.2GB of 4GB available
- Expected disk usage: <1GB for 7-day retention
- No CPU limits needed for low-traffic application

### Security Configuration
- Grafana admin password via GRAFANA_ADMIN_PASSWORD env var
- No additional authentication initially (handled by nginx SSL)
- All services on internal docker network only
- No additional firewall rules needed (everything via nginx)