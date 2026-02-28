package telemetry

import (
	"context"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/log"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// InitOtel initializes OpenTelemetry with OTLP gRPC exporter.
// Set OTEL_EXPORTER_OTLP_ENDPOINT to "disabled" or "none" to skip initialization.
func InitOtel(ctx context.Context) (func(), error) {
	// Get the OTel Collector endpoint from environment
	endpoint := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")

	// Allow disabling OTel for local development
	if endpoint == "" || endpoint == "disabled" || endpoint == "none" {
		log.Info("OpenTelemetry disabled (OTEL_EXPORTER_OTLP_ENDPOINT not set or disabled)")
		return func() {}, nil
	}

	// Initialize the OTLP gRPC trace exporter
	conn, err := grpc.NewClient(endpoint,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, err
	}

	traceExporter, err := otlptracegrpc.New(ctx, otlptracegrpc.WithGRPCConn(conn))
	if err != nil {
		return nil, err
	}

	// Define OpenTelemetry Resource
	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceNameKey.String("ideal-tribble"),
			semconv.ServiceVersionKey.String(getEnvOrDefault("APP_VERSION", "unknown")),
			semconv.DeploymentEnvironmentKey.String(getEnvOrDefault("APP_ENV", "development")),
		),
	)
	if err != nil {
		return nil, err
	}

	// Create and set the global TracerProvider
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(traceExporter),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.AlwaysSample()), // 100% sampling
	)

	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.TraceContext{})

	// Return shutdown function
	return func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := tp.Shutdown(shutdownCtx); err != nil {
			slog.Error("Error shutting down tracer provider", "error", err)
		}
	}, nil
}

// LogWithTrace returns a charmbracelet/log.Logger enriched with trace and span IDs from the context
func LogWithTrace(ctx context.Context) *log.Logger {
	span := trace.SpanFromContext(ctx)
	if !span.IsRecording() {
		return log.Default()
	}

	spanContext := span.SpanContext()
	return log.Default().With(
		"trace_id", spanContext.TraceID().String(),
		"span_id", spanContext.SpanID().String(),
	)
}

// getEnvOrDefault returns the value of the environment variable or a default value
func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// InitLogging configures the charmbracelet/log logger based on environment variables.
//
// Environment variables:
//   - LOG_FORMAT: "json" (default) or "text" - controls output format
//   - LOG_LEVEL: "debug", "info" (default), "warn", or "error"
//
// JSON format is recommended for production as it enables:
//   - Structured log parsing in Loki/Grafana
//   - Easy correlation with trace_id and span_id
//   - Machine-readable log aggregation
func InitLogging() {
	// Configure log format
	format := strings.ToLower(getEnvOrDefault("LOG_FORMAT", "json"))
	switch format {
	case "text":
		log.SetFormatter(log.TextFormatter)
	default:
		log.SetFormatter(log.JSONFormatter)
	}

	// Configure log level
	level := strings.ToLower(getEnvOrDefault("LOG_LEVEL", "info"))
	switch level {
	case "debug":
		log.SetLevel(log.DebugLevel)
	case "warn", "warning":
		log.SetLevel(log.WarnLevel)
	case "error":
		log.SetLevel(log.ErrorLevel)
	default:
		log.SetLevel(log.InfoLevel)
	}

	// Log the configuration (only visible if level allows)
	log.Debug("Logging initialized", "format", format, "level", level)
}