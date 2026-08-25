// Package observability wires warehouse-ops-agent's OpenTelemetry
// pipeline: a TracerProvider and MeterProvider exporting over OTLP/gRPC to
// a Collector, plus a slog handler that stamps log records with the active
// trace/span id. It mirrors fulfillment-execution's
// internal/observability package (the pilot) exactly, minus the
// Kafka-specific trace carrier this agent has no use for (it never
// produces or consumes a Kafka message).
package observability

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"strings"

	"go.opentelemetry.io/contrib/instrumentation/runtime"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.43.0"
)

const (
	// DefaultServiceName is this service's name in traces and metrics when
	// OTEL_SERVICE_NAME is not set.
	DefaultServiceName = "warehouse-ops-agent"

	// DefaultOTLPEndpoint is the OTel Collector's standard gRPC receiver
	// address.
	DefaultOTLPEndpoint = "localhost:4317"

	defaultServiceVersion = "dev"
	defaultEnvironment    = "local"

	// InstrumentationName is the scope every instrument created by this
	// package is attributed to.
	InstrumentationName = "github.com/claudioed/warehouse-ops-agent"
)

// ServiceName resolves the service name from OTEL_SERVICE_NAME, falling
// back to DefaultServiceName.
func ServiceName() string { return getenv("OTEL_SERVICE_NAME", DefaultServiceName) }

// ServiceVersion resolves the service version from SERVICE_VERSION.
func ServiceVersion() string { return getenv("SERVICE_VERSION", defaultServiceVersion) }

// Endpoint resolves the Collector's OTLP/gRPC address from
// OTEL_EXPORTER_OTLP_ENDPOINT.
func Endpoint() string { return getenv("OTEL_EXPORTER_OTLP_ENDPOINT", DefaultOTLPEndpoint) }

// Setup builds and installs the global TracerProvider, MeterProvider and
// trace-context propagator, and starts Go runtime metrics collection. It
// returns a shutdown func that flushes and closes both providers.
//
// Export is deliberately non-blocking: the OTLP gRPC exporters dial lazily
// and no blocking dial option is set, so an unreachable Collector degrades
// to "telemetry silently dropped", never to a service that will not start
// or requests that hang.
func Setup(ctx context.Context, serviceName, serviceVersion, otlpEndpoint string) (func(context.Context) error, error) {
	if serviceName == "" {
		serviceName = DefaultServiceName
	}
	if serviceVersion == "" {
		serviceVersion = defaultServiceVersion
	}
	if otlpEndpoint == "" {
		otlpEndpoint = DefaultOTLPEndpoint
	}

	endpointURL := normalizeEndpoint(otlpEndpoint)

	res, err := newResource(serviceName, serviceVersion)
	if err != nil {
		return nil, err
	}

	traceExporter, err := otlptracegrpc.New(ctx,
		otlptracegrpc.WithEndpointURL(endpointURL),
		otlptracegrpc.WithInsecure(),
	)
	if err != nil {
		return nil, err
	}
	tracerProvider := sdktrace.NewTracerProvider(
		sdktrace.WithResource(res),
		sdktrace.WithBatcher(traceExporter),
	)

	metricExporter, err := otlpmetricgrpc.New(ctx,
		otlpmetricgrpc.WithEndpointURL(endpointURL),
		otlpmetricgrpc.WithInsecure(),
	)
	if err != nil {
		_ = tracerProvider.Shutdown(ctx)
		return nil, err
	}
	meterProvider := sdkmetric.NewMeterProvider(
		sdkmetric.WithResource(res),
		sdkmetric.WithReader(sdkmetric.NewPeriodicReader(metricExporter)),
	)

	otel.SetErrorHandler(otel.ErrorHandlerFunc(func(err error) {
		slog.Warn("opentelemetry error", "error", err)
	}))

	otel.SetTracerProvider(tracerProvider)
	otel.SetMeterProvider(meterProvider)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	runtimeErr := runtime.Start(runtime.WithMeterProvider(meterProvider))

	shutdown := func(ctx context.Context) error {
		return errors.Join(
			tracerProvider.Shutdown(ctx),
			meterProvider.Shutdown(ctx),
		)
	}
	return shutdown, runtimeErr
}

// normalizeEndpoint turns this platform's host:port convention into the
// URL form the OTLP exporters expect, leaving a value that already carries
// a scheme alone.
func normalizeEndpoint(endpoint string) string {
	url := endpoint
	if !strings.Contains(url, "://") {
		url = "http://" + url
	}
	if raw := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT"); raw != "" && !strings.Contains(raw, "://") {
		_ = os.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "http://"+raw)
	}
	return url
}

// newResource describes this service to the Collector.
func newResource(serviceName, serviceVersion string) (*resource.Resource, error) {
	return resource.Merge(
		resource.Default(),
		resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceName(serviceName),
			semconv.ServiceVersion(serviceVersion),
			semconv.DeploymentEnvironmentNameKey.String(getenv("ENVIRONMENT", defaultEnvironment)),
		),
	)
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
