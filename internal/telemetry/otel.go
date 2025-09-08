package telemetry

import (
	"context"
	"log/slog"
	"os"
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

// InitOtel initializes OpenTelemetry with OTLP gRPC exporter
func InitOtel(ctx context.Context) (func(), error) {
	// Get the OTel Collector endpoint from environment
	endpoint := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")
	if endpoint == "" {
		endpoint = "localhost:4317"
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