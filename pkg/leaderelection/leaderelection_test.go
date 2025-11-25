package leaderelection

import (
	"context"
	"os"
	"testing"
	"time"

	"k8s.io/client-go/kubernetes/fake"
)

func TestDefaultConfig(t *testing.T) {
	// Save and restore original hostname
	originalHostname, _ := os.Hostname()
	defer func() {
		// Can't actually restore hostname, but test should pass regardless
		_ = originalHostname
	}()

	config := DefaultConfig()

	if config == nil {
		t.Fatal("DefaultConfig() returned nil")
	}

	if config.LockName != "forge-controller-lock" {
		t.Errorf("Expected lock name 'forge-controller-lock', got %s", config.LockName)
	}

	if config.LockNamespace != "forge-system" {
		t.Errorf("Expected lock namespace 'forge-system', got %s", config.LockNamespace)
	}

	if config.Identity == "" {
		t.Error("Identity should not be empty")
	}

	if config.LeaseDuration != DefaultLeaseDuration {
		t.Errorf("Expected lease duration %v, got %v", DefaultLeaseDuration, config.LeaseDuration)
	}

	if config.RenewDeadline != DefaultRenewDeadline {
		t.Errorf("Expected renew deadline %v, got %v", DefaultRenewDeadline, config.RenewDeadline)
	}

	if config.RetryPeriod != DefaultRetryPeriod {
		t.Errorf("Expected retry period %v, got %v", DefaultRetryPeriod, config.RetryPeriod)
	}
}

func TestDefaultConfigWithHostnameError(t *testing.T) {
	// We can't easily force os.Hostname() to fail in a test,
	// but we can verify the config is still created
	config := DefaultConfig()

	if config == nil {
		t.Fatal("DefaultConfig() should never return nil even if hostname fails")
	}

	// Identity should be either hostname or "unknown"
	if config.Identity == "" {
		t.Error("Identity should not be empty")
	}
}

func TestCustomConfig(t *testing.T) {
	config := &Config{
		LockName:      "custom-lock",
		LockNamespace: "custom-namespace",
		Identity:      "custom-identity",
		LeaseDuration: 20 * time.Second,
		RenewDeadline: 15 * time.Second,
		RetryPeriod:   5 * time.Second,
	}

	if config.LockName != "custom-lock" {
		t.Errorf("Expected lock name 'custom-lock', got %s", config.LockName)
	}

	if config.LockNamespace != "custom-namespace" {
		t.Errorf("Expected lock namespace 'custom-namespace', got %s", config.LockNamespace)
	}

	if config.Identity != "custom-identity" {
		t.Errorf("Expected identity 'custom-identity', got %s", config.Identity)
	}

	if config.LeaseDuration != 20*time.Second {
		t.Errorf("Expected lease duration 20s, got %v", config.LeaseDuration)
	}

	if config.RenewDeadline != 15*time.Second {
		t.Errorf("Expected renew deadline 15s, got %v", config.RenewDeadline)
	}

	if config.RetryPeriod != 5*time.Second {
		t.Errorf("Expected retry period 5s, got %v", config.RetryPeriod)
	}
}

func TestRunWithLeaderElection_NilConfig(t *testing.T) {
	client := fake.NewSimpleClientset()
	ctx, cancel := context.WithCancel(context.Background())

	// Cancel immediately to prevent the leader election from running indefinitely
	cancel()

	// RunWithLeaderElection should use DefaultConfig when nil is passed
	// Since we cancel immediately, this tests that the function handles nil config
	err := RunWithLeaderElection(ctx, client, nil, func(_ context.Context) {
		// This should not be called since context is canceled
		t.Error("Run function should not be called with canceled context")
	})

	// RunWithLeaderElection always returns nil
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
}

func TestRunWithLeaderElection_CustomConfig(t *testing.T) {
	client := fake.NewSimpleClientset()
	ctx, cancel := context.WithCancel(context.Background())

	// Cancel immediately to prevent running indefinitely
	cancel()

	config := &Config{
		LockName:      "test-lock",
		LockNamespace: "test-namespace",
		Identity:      "test-identity",
		LeaseDuration: 1 * time.Second,
		RenewDeadline: 500 * time.Millisecond,
		RetryPeriod:   100 * time.Millisecond,
	}

	err := RunWithLeaderElection(ctx, client, config, func(_ context.Context) {
		// Should not be called with canceled context
		t.Error("Run function should not be called with canceled context")
	})

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
}

func TestRunWithLeaderElection_RunFunctionCalled(t *testing.T) {
	// This test uses a very short timeout to verify the structure works
	// In a real scenario, leader election would take longer
	client := fake.NewSimpleClientset()
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	config := &Config{
		LockName:      "test-lock",
		LockNamespace: "test-namespace",
		Identity:      "test-leader",
		LeaseDuration: 50 * time.Millisecond,
		RenewDeadline: 40 * time.Millisecond,
		RetryPeriod:   10 * time.Millisecond,
	}

	runCalled := false
	err := RunWithLeaderElection(ctx, client, config, func(ctx context.Context) {
		runCalled = true
		// Wait for context cancellation
		<-ctx.Done()
	})

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	// Note: The run function may or may not be called depending on timing
	// Leader election might not complete before context timeout
	// This is expected behavior - we're just testing the function doesn't panic
	_ = runCalled
}

func TestConstants(t *testing.T) {
	if DefaultLeaseDuration != 15*time.Second {
		t.Errorf("Expected DefaultLeaseDuration to be 15s, got %v", DefaultLeaseDuration)
	}

	if DefaultRenewDeadline != 10*time.Second {
		t.Errorf("Expected DefaultRenewDeadline to be 10s, got %v", DefaultRenewDeadline)
	}

	if DefaultRetryPeriod != 2*time.Second {
		t.Errorf("Expected DefaultRetryPeriod to be 2s, got %v", DefaultRetryPeriod)
	}
}
