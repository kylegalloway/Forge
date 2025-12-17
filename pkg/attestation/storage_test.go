package attestation

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLocalStorage_Store(t *testing.T) {
	// Create temporary directory
	tmpDir := t.TempDir()

	storage, err := NewLocalStorage(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create local storage: %v", err)
	}

	bundle := createTestAttestationBundle()
	opts := StoreOptions{
		ZarfPackageJob: "test-pkg",
		Namespace:      "default",
		Operation:      "Build",
		ArtifactDigest: "sha256:abcdef123456",
	}

	err = storage.Store(context.Background(), bundle, opts)
	if err != nil {
		t.Fatalf("Failed to store attestation: %v", err)
	}

	// Verify file was created
	expectedPath := filepath.Join(tmpDir, "default", "test-pkg-Build-sha256:abcde.json")
	if _, statErr := os.Stat(expectedPath); os.IsNotExist(statErr) {
		t.Errorf("Attestation file not created at %s", expectedPath)
	}

	// Verify content
	data, err := os.ReadFile(expectedPath)
	if err != nil {
		t.Fatalf("Failed to read attestation file: %v", err)
	}

	var storedBundle AttestationBundle
	if err := json.Unmarshal(data, &storedBundle); err != nil {
		t.Fatalf("Failed to unmarshal stored attestation: %v", err)
	}

	if len(storedBundle.Statement.Subject) != len(bundle.Statement.Subject) {
		t.Errorf("Subject count mismatch: expected %d, got %d",
			len(bundle.Statement.Subject), len(storedBundle.Statement.Subject))
	}
}

func TestLocalStorage_Retrieve(t *testing.T) {
	tmpDir := t.TempDir()
	storage, err := NewLocalStorage(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create local storage: %v", err)
	}

	// Store an attestation first
	bundle := createTestAttestationBundle()
	opts := StoreOptions{
		ZarfPackageJob: "test-pkg",
		Namespace:      "default",
		Operation:      "Build",
		ArtifactDigest: "sha256:abcdef123456",
	}

	if storeErr := storage.Store(context.Background(), bundle, opts); storeErr != nil {
		t.Fatalf("Failed to store attestation: %v", storeErr)
	}

	// Retrieve by full digest
	retrieved, err := storage.Retrieve(context.Background(), "sha256:abcdef123456")
	if err != nil {
		t.Fatalf("Failed to retrieve attestation: %v", err)
	}

	if retrieved == nil {
		t.Fatal("Retrieved attestation is nil")
	}

	if len(retrieved.Statement.Subject) == 0 {
		t.Error("Retrieved attestation has no subjects")
	}

	// Retrieve by short digest
	retrieved, err = storage.Retrieve(context.Background(), "sha256:abcde")
	if err != nil {
		t.Fatalf("Failed to retrieve attestation by short digest: %v", err)
	}

	if retrieved == nil {
		t.Fatal("Retrieved attestation is nil")
	}

	// Try to retrieve non-existent attestation
	_, err = storage.Retrieve(context.Background(), "sha256:nonexistent")
	if err == nil {
		t.Error("Expected error for non-existent attestation, got nil")
	}
}

func TestLocalStorage_List(t *testing.T) {
	tmpDir := t.TempDir()
	storage, err := NewLocalStorage(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create local storage: %v", err)
	}

	// Store multiple attestations
	for i := 0; i < 5; i++ {
		digest := fmt.Sprintf("sha256:abcdef%d", i)
		bundle := createTestAttestationBundleWithDigest(digest)
		// Modify predicate for each attestation
		if forgePred, ok := bundle.Statement.Predicate.(ForgeOperationPredicate); ok {
			forgePred.Operation = fmt.Sprintf("Build-%d", i)
			bundle.Statement.Predicate = forgePred
		}

		opts := StoreOptions{
			ZarfPackageJob: "test-pkg",
			Namespace:      "default",
			Operation:      fmt.Sprintf("Build-%d", i),
			ArtifactDigest: digest,
		}

		if err := storage.Store(context.Background(), bundle, opts); err != nil {
			t.Fatalf("Failed to store attestation %d: %v", i, err)
		}
	}

	tests := []struct {
		name          string
		opts          ListOptions
		expectedCount int
	}{
		{
			name:          "list all",
			opts:          ListOptions{},
			expectedCount: 5,
		},
		{
			name: "list with namespace filter",
			opts: ListOptions{
				Namespace: "default",
			},
			expectedCount: 5,
		},
		{
			name: "list with limit",
			opts: ListOptions{
				Limit: 3,
			},
			expectedCount: 3,
		},
		{
			name: "list with zarfpackagejob filter",
			opts: ListOptions{
				ZarfPackageJob: "test-pkg",
			},
			expectedCount: 5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results, err := storage.List(context.Background(), tt.opts)
			if err != nil {
				t.Fatalf("Failed to list attestations: %v", err)
			}

			if len(results) != tt.expectedCount {
				t.Errorf("Expected %d results, got %d", tt.expectedCount, len(results))
			}
		})
	}
}

func TestConfigMapStorage_Store(t *testing.T) {
	fakeClient := &FakeKubeClient{
		configMaps: make(map[string]*ConfigMap),
	}

	storage := NewConfigMapStorage("default", fakeClient)

	bundle := createTestAttestationBundle()
	opts := StoreOptions{
		ZarfPackageJob: "test-pkg",
		Namespace:      "default",
		Operation:      "Build",
		ArtifactDigest: "sha256:abcdef123456",
	}

	err := storage.Store(context.Background(), bundle, opts)
	if err != nil {
		t.Fatalf("Failed to store attestation: %v", err)
	}

	// Verify ConfigMap was created
	expectedName := "attestation-test-pkg-Build-sha256:abcde"
	if _, exists := fakeClient.configMaps[expectedName]; !exists {
		t.Errorf("ConfigMap %s not created", expectedName)
	}

	cm := fakeClient.configMaps[expectedName]
	if cm.Labels["forge.dev/attestation"] != "true" {
		t.Error("ConfigMap missing attestation label")
	}

	if cm.Labels["forge.dev/zarfpackagejob"] != "test-pkg" {
		t.Error("ConfigMap missing zarfpackagejob label")
	}

	if _, ok := cm.Data["attestation.json"]; !ok {
		t.Error("ConfigMap missing attestation.json data")
	}
}

func TestConfigMapStorage_Retrieve(t *testing.T) {
	fakeClient := &FakeKubeClient{
		configMaps: make(map[string]*ConfigMap),
	}

	storage := NewConfigMapStorage("default", fakeClient)

	// Store an attestation first
	bundle := createTestAttestationBundle()
	opts := StoreOptions{
		ZarfPackageJob: "test-pkg",
		Namespace:      "default",
		Operation:      "Build",
		ArtifactDigest: "sha256:abcdef123456",
	}

	if err := storage.Store(context.Background(), bundle, opts); err != nil {
		t.Fatalf("Failed to store attestation: %v", err)
	}

	// Retrieve by digest
	retrieved, err := storage.Retrieve(context.Background(), "sha256:abcdef123456")
	if err != nil {
		t.Fatalf("Failed to retrieve attestation: %v", err)
	}

	if retrieved == nil {
		t.Fatal("Retrieved attestation is nil")
	}

	// Try to retrieve non-existent attestation
	_, err = storage.Retrieve(context.Background(), "sha256:nonexistent")
	if err == nil {
		t.Error("Expected error for non-existent attestation, got nil")
	}
}

func TestConfigMapStorage_List(t *testing.T) {
	fakeClient := &FakeKubeClient{
		configMaps: make(map[string]*ConfigMap),
	}

	storage := NewConfigMapStorage("default", fakeClient)

	// Store multiple attestations
	for i := 0; i < 3; i++ {
		digest := fmt.Sprintf("sha256:abcdef%d", i)
		bundle := createTestAttestationBundleWithDigest(digest)
		opts := StoreOptions{
			ZarfPackageJob: fmt.Sprintf("test-pkg-%d", i),
			Namespace:      "default",
			Operation:      "Build",
			ArtifactDigest: digest,
		}

		if err := storage.Store(context.Background(), bundle, opts); err != nil {
			t.Fatalf("Failed to store attestation %d: %v", i, err)
		}
	}

	tests := []struct {
		name          string
		opts          ListOptions
		expectedCount int
	}{
		{
			name:          "list all",
			opts:          ListOptions{},
			expectedCount: 3,
		},
		{
			name: "list with limit",
			opts: ListOptions{
				Limit: 2,
			},
			expectedCount: 2,
		},
		{
			name: "list with zarfpackagejob filter",
			opts: ListOptions{
				ZarfPackageJob: "test-pkg-0",
			},
			expectedCount: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results, err := storage.List(context.Background(), tt.opts)
			if err != nil {
				t.Fatalf("Failed to list attestations: %v", err)
			}

			if len(results) != tt.expectedCount {
				t.Errorf("Expected %d results, got %d", tt.expectedCount, len(results))
			}
		})
	}
}

// Helper functions

func createTestAttestationBundle() *AttestationBundle {
	return createTestAttestationBundleWithDigest("sha256:abcdef123456")
}

func createTestAttestationBundleWithDigest(digest string) *AttestationBundle {
	now := time.Now()
	return &AttestationBundle{
		Statement: Statement{
			Type: "https://in-toto.io/Statement/v1",
			Subject: []Subject{
				{
					Name: "test-package",
					Digest: map[string]string{
						"sha256": digest,
					},
				},
			},
			PredicateType: PredicateTypeForgeOperation,
			Predicate: ForgeOperationPredicate{
				Operation:      "Build",
				ZarfPackageJob: "test-pkg",
				Namespace:      "default",
				ServiceAccount: "test-sa",
				StartTime:      now,
				EndTime:        now.Add(5 * time.Minute),
				Status:         "Completed",
				JobName:        "test-job",
				Controller: ControllerInfo{
					Name:      "forge-controller",
					Namespace: "forge-system",
					Version:   "v0.1.0",
				},
			},
		},
	}
}

// FakeKubeClient implements KubeClient for testing
type FakeKubeClient struct {
	configMaps map[string]*ConfigMap
}

func (f *FakeKubeClient) CreateConfigMap(_ context.Context, _ string, cm *ConfigMap) error {
	if _, exists := f.configMaps[cm.Name]; exists {
		return fmt.Errorf("configmap %s already exists", cm.Name)
	}
	f.configMaps[cm.Name] = cm
	return nil
}

func (f *FakeKubeClient) GetConfigMap(_ context.Context, _ string, name string) (*ConfigMap, error) {
	cm, exists := f.configMaps[name]
	if !exists {
		return nil, fmt.Errorf("configmap %s not found", name)
	}
	return cm, nil
}

func (f *FakeKubeClient) ListConfigMaps(_ context.Context, _ string, labels map[string]string) ([]*ConfigMap, error) {
	var results []*ConfigMap

	for _, cm := range f.configMaps {
		// Check if all required labels match
		matches := true
		for key, value := range labels {
			if cm.Labels[key] != value {
				matches = false
				break
			}
		}

		if matches {
			results = append(results, cm)
		}
	}

	return results, nil
}

func (f *FakeKubeClient) UpdateConfigMap(_ context.Context, _ string, cm *ConfigMap) error {
	if _, exists := f.configMaps[cm.Name]; !exists {
		return fmt.Errorf("configmap %s not found", cm.Name)
	}
	f.configMaps[cm.Name] = cm
	return nil
}
