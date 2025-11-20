// Package telemetry provides OpenTelemetry integration for ScriptRunner observability.
//
// This package implements vendor-neutral metrics, tracing, and logging using the
// OpenTelemetry SDK. It provides:
//   - Provider initialization for OTLP exporters
//   - Metric collection for controller operations
//   - Distributed tracing with context propagation
//   - Helper functions for span creation and error recording
//
// The telemetry can be exported to any OTLP-compatible backend including
// Prometheus, Jaeger, Datadog, New Relic, and Honeycomb.
package telemetry

import (
	"context"
	"fmt"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/propagation"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
)

const (
	serviceName    = "scriptrunner-controller"
	serviceVersion = "v1alpha1"
)

// Provider holds the OpenTelemetry providers for metrics, traces, and logs
type Provider struct {
	meterProvider  *sdkmetric.MeterProvider
	tracerProvider *sdktrace.TracerProvider
	resource       *resource.Resource
}

// Config holds OpenTelemetry configuration
type Config struct {
	// Endpoint is the OTLP collector endpoint (e.g., "otel-collector:4317")
	Endpoint string
	// Insecure disables TLS for the OTLP connection
	Insecure bool
	// SamplingRate is the trace sampling rate (0.0 to 1.0)
	SamplingRate float64
	// Enabled controls whether telemetry is enabled
	Enabled bool
}

// DefaultConfig returns default OpenTelemetry configuration
func DefaultConfig() *Config {
	return &Config{
		Endpoint:     "localhost:4317",
		Insecure:     true,
		SamplingRate: 1.0, // Sample all traces by default
		Enabled:      true,
	}
}

// NewProvider creates a new OpenTelemetry provider with OTLP exporters
func NewProvider(ctx context.Context, cfg *Config) (*Provider, error) {
	if !cfg.Enabled {
		// Return no-op provider
		return &Provider{
			meterProvider:  sdkmetric.NewMeterProvider(),
			tracerProvider: sdktrace.NewTracerProvider(),
		}, nil
	}

	// Create resource with service information
	res, err := resource.Merge(
		resource.Default(),
		resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceName(serviceName),
			semconv.ServiceVersion(serviceVersion),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create resource: %w", err)
	}

	// Set up trace exporter
	traceExporter, err := otlptracegrpc.New(ctx,
		otlptracegrpc.WithEndpoint(cfg.Endpoint),
		otlptracegrpc.WithInsecure(),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create trace exporter: %w", err)
	}

	// Set up trace provider with sampling
	tracerProvider := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(traceExporter),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.TraceIDRatioBased(cfg.SamplingRate)),
	)

	// Set up metric exporter
	metricExporter, err := otlpmetricgrpc.New(ctx,
		otlpmetricgrpc.WithEndpoint(cfg.Endpoint),
		otlpmetricgrpc.WithInsecure(),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create metric exporter: %w", err)
	}

	// Set up metric provider with periodic reader
	meterProvider := sdkmetric.NewMeterProvider(
		sdkmetric.WithResource(res),
		sdkmetric.WithReader(sdkmetric.NewPeriodicReader(metricExporter,
			sdkmetric.WithInterval(10*time.Second))),
	)

	// Set global providers
	otel.SetTracerProvider(tracerProvider)
	otel.SetMeterProvider(meterProvider)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	return &Provider{
		meterProvider:  meterProvider,
		tracerProvider: tracerProvider,
		resource:       res,
	}, nil
}

// Shutdown gracefully shuts down the OpenTelemetry providers
func (p *Provider) Shutdown(ctx context.Context) error {
	var err error
	if p.tracerProvider != nil {
		if shutdownErr := p.tracerProvider.Shutdown(ctx); shutdownErr != nil {
			err = fmt.Errorf("failed to shutdown tracer provider: %w", shutdownErr)
		}
	}
	if p.meterProvider != nil {
		if shutdownErr := p.meterProvider.Shutdown(ctx); shutdownErr != nil {
			if err != nil {
				err = fmt.Errorf("%w; failed to shutdown meter provider: %w", err, shutdownErr)
			} else {
				err = fmt.Errorf("failed to shutdown meter provider: %w", shutdownErr)
			}
		}
	}
	return err
}

// Meter returns an OpenTelemetry meter for creating metrics
func (p *Provider) Meter(name string) metric.Meter {
	return otel.Meter(name)
}

// Tracer returns an OpenTelemetry tracer for creating spans
func (p *Provider) Tracer(name string) trace.Tracer {
	return otel.Tracer(name)
}
