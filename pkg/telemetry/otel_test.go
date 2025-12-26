package telemetry

import (
	"context"
	"testing"
	"time"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg == nil {
		t.Fatal("DefaultConfig() returned nil")
	}

	if cfg.Endpoint != "localhost:4317" {
		t.Errorf("Expected endpoint 'localhost:4317', got %s", cfg.Endpoint)
	}

	if !cfg.Insecure {
		t.Error("Expected Insecure to be true")
	}

	if cfg.SamplingRate != 1.0 {
		t.Errorf("Expected SamplingRate 1.0, got %f", cfg.SamplingRate)
	}

	if !cfg.Enabled {
		t.Error("Expected Enabled to be true")
	}
}

func TestNewProvider_Disabled(t *testing.T) {
	ctx := context.Background()
	cfg := &Config{
		Enabled: false,
	}

	provider, err := NewProvider(ctx, cfg)
	if err != nil {
		t.Fatalf("NewProvider() with disabled config failed: %v", err)
	}

	if provider == nil {
		t.Fatal("NewProvider() returned nil provider")
	}

	if provider.meterProvider == nil {
		t.Error("Provider meterProvider is nil")
	}

	if provider.tracerProvider == nil {
		t.Error("Provider tracerProvider is nil")
	}
}

func TestNewProvider_Enabled(t *testing.T) {
	// This test requires a real OTLP collector running locally.
	//
	// SETUP INSTRUCTIONS:
	// 1. Install and run an OTLP collector (Docker recommended):
	//    docker run -d --name otel-collector \
	//      -p 4317:4317 \
	//      otel/opentelemetry-collector:latest
	//
	// 2. Verify the collector is running:
	//    curl http://localhost:13133/  # Health check endpoint
	//
	// 3. Run this specific test:
	//    go test -v -run TestNewProvider_Enabled
	//
	// NOTE: The Forge controller currently uses Prometheus exporter directly
	// (see cmd/controller/main.go:startMetricsServer). To use OTLP instead:
	// - Modify main.go to call telemetry.NewProvider(ctx, config)
	// - Set OTLP collector endpoint via environment variable or config
	// - See otel.go for Config struct and DefaultConfig()
	//
	// To enable this test, comment out the t.Skip() line below.
	t.Skip("Requires OTLP collector at localhost:4317 - see test comments for setup instructions")

	ctx := context.Background()
	cfg := DefaultConfig()
	cfg.Enabled = true

	provider, err := NewProvider(ctx, cfg)
	if err != nil {
		t.Fatalf("NewProvider() with enabled config failed: %v", err)
	}

	if provider == nil {
		t.Fatal("NewProvider() returned nil provider")
	}

	if provider.meterProvider == nil {
		t.Error("Provider meterProvider is nil")
	}

	if provider.tracerProvider == nil {
		t.Error("Provider tracerProvider is nil")
	}

	if provider.resource == nil {
		t.Error("Provider resource is nil")
	}

	// Test meter creation
	meter := provider.Meter("test-meter")
	if meter == nil {
		t.Error("Meter() returned nil")
	}

	// Test tracer creation
	tracer := provider.Tracer("test-tracer")
	if tracer == nil {
		t.Error("Tracer() returned nil")
	}

	// Cleanup
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := provider.Shutdown(shutdownCtx); err != nil {
		t.Errorf("Shutdown() failed: %v", err)
	}
}

func TestProvider_Shutdown(t *testing.T) {
	ctx := context.Background()
	cfg := &Config{
		Enabled: false, // Use disabled mode for easier testing
	}

	provider, err := NewProvider(ctx, cfg)
	if err != nil {
		t.Fatalf("NewProvider() failed: %v", err)
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err = provider.Shutdown(shutdownCtx)
	if err != nil {
		t.Errorf("Shutdown() failed: %v", err)
	}
}

func TestProvider_Meter(t *testing.T) {
	ctx := context.Background()
	cfg := &Config{
		Enabled: false,
	}

	provider, err := NewProvider(ctx, cfg)
	if err != nil {
		t.Fatalf("NewProvider() failed: %v", err)
	}

	meter := provider.Meter("test-meter")
	if meter == nil {
		t.Error("Meter() returned nil")
	}
}

func TestProvider_Tracer(t *testing.T) {
	ctx := context.Background()
	cfg := &Config{
		Enabled: false,
	}

	provider, err := NewProvider(ctx, cfg)
	if err != nil {
		t.Fatalf("NewProvider() failed: %v", err)
	}

	tracer := provider.Tracer("test-tracer")
	if tracer == nil {
		t.Error("Tracer() returned nil")
	}
}

func TestCustomConfig(t *testing.T) {
	cfg := &Config{
		Endpoint:     "otel-collector.observability.svc:4317",
		Insecure:     false,
		SamplingRate: 0.1,
		Enabled:      true,
	}

	if cfg.Endpoint != "otel-collector.observability.svc:4317" {
		t.Errorf("Expected custom endpoint, got %s", cfg.Endpoint)
	}

	if cfg.Insecure {
		t.Error("Expected Insecure to be false")
	}

	if cfg.SamplingRate != 0.1 {
		t.Errorf("Expected SamplingRate 0.1, got %f", cfg.SamplingRate)
	}

	if !cfg.Enabled {
		t.Error("Expected Enabled to be true")
	}
}
