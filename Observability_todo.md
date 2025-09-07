# Guide: Instrumenting `ideal-tribble` with OpenTelemetry and Google Cloud

## **Objective**

This guide details the process for integrating a comprehensive telemetry solution into the `ideal-tribble` Go application using OpenTelemetry. The goal is to gain deep insights into application performance and trace requests across services.

We will use the following technology stack for this task:
1.  **Instrumentation Standard:** OpenTelemetry (OTel) for vendor-neutral collection of traces and logs.
2.  **Telemetry Backend:** Google Cloud's Operations Suite (Cloud Trace, Cloud Monitoring, Cloud Logging).

---

## **Step 1: Add Necessary Go Dependencies**

First, update the project's dependencies by running the appropriate `go get` command. You will need to add the modules for OpenTelemetry (base SDK, trace, contrib http instrumentation), the Google Cloud Exporter for OpenTelemetry, and semantic conventions.

---

## **Step 2: Create a Centralized Telemetry Package**

To keep the code organized, create a new package `internal/telemetry`. Inside this package, create the file `otel.go`.

### **File: `internal/telemetry/otel.go`**

In this file, implement the initialization logic for OpenTelemetry and its connection to Google Cloud Trace.

1.  Create a function named `InitOtel` that accepts a `context.Context` and returns a shutdown function (`func()`) and an error.
2.  Inside this function, retrieve the `GCP_PROJECT_ID` from the environment. If it's not set, log a message and return a no-op shutdown function.
3.  Initialize the Google Cloud Trace exporter using the project ID.
4.  Define an OpenTelemetry `Resource` that identifies the service with the name `ideal-tribble`.
5.  Create and set the global `TracerProvider` using the exporter and the resource.
6.  Return a shutdown function that calls the `TracerProvider.Shutdown` method to ensure telemetry is flushed before the app exits.
7.  Also, create a helper function `SlogWithTrace` that accepts a `context.Context`. This function should extract the trace and span ID from the context and return a `slog.Logger` instance enriched with these fields for correlated logging.

---

## **Step 3: Integrate Telemetry into the Application Entrypoint (`main.go`)**

Modify your main application entrypoint, **`main.go`**, to use the new telemetry package.

1.  At the start of your `main` function, set up a structured JSON logger (`slog`) as the application default.
2.  Call your `telemetry.InitOtel()` function at the start of `main`, handling any potential error. Defer the returned shutdown function.
3.  After setting up your main HTTP router (e.g., `mux`), wrap it with the OpenTelemetry middleware (`otelhttp.NewHandler`) to enable tracing on all incoming requests.
4.  Use this final, wrapped handler when you call `http.ListenAndServe`.
5.  Inside one of your existing HTTP handlers (like `/health`), add an example of using your new `telemetry.SlogWithTrace(r.Context())` helper to demonstrate correlated logging.

---

## **Step 4: Update Environment and Verification Steps**

To complete the task, you must update the configuration and verify that the integration is working.

### **1. Update Environment Variables**

Add the following new variables to your **`.env.example`** file. These will be required for local development and production.

-   `GCP_PROJECT_ID`
-   `APP_ENV`
-   `APP_VERSION`

### **2. Update Production Secrets**

The secret `GCP_PROJECT_ID` must be added to Google Secret Manager, and the Cloud Run service must be configured to load it as an environment variable.

### **3. Verification Plan**

After deploying these changes, follow these steps to verify that everything is working:

-   **Trigger an Endpoint:** Make a request to one of the application's endpoints (e.g., `/health`).
-   **Check Google Cloud Trace:**
    -   Navigate to the Google Cloud Console -> Trace -> Trace list.
    -   You should see a new trace for the request you just made, with the service name `ideal-tribble`.
    -   Clicking on it should show the full span of the HTTP request.
-   **Check Google Cloud Logging:**
    -   Navigate to the Logs Explorer.
    -   You should see your `slog` JSON output. For any request that is traced, the log entry must contain `trace_id` and `span_id` fields, linking it directly to the trace you saw in the previous step.