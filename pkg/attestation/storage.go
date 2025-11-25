package attestation

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"k8s.io/klog/v2"
)

// Storage defines the interface for storing attestations
type Storage interface {
	// Store saves an attestation bundle
	Store(ctx context.Context, bundle *AttestationBundle, opts StoreOptions) error

	// Retrieve retrieves an attestation by digest
	Retrieve(ctx context.Context, digest string) (*AttestationBundle, error)

	// List lists attestations matching the given criteria
	List(ctx context.Context, opts ListOptions) ([]*AttestationBundle, error)
}

// StoreOptions contains options for storing attestations
type StoreOptions struct {
	// ZarfPackageJob is the name of the ZarfPackageJob
	ZarfPackageJob string

	// Namespace is the Kubernetes namespace
	Namespace string

	// Operation is the operation type (Build, Publish, Deploy)
	Operation string

	// ArtifactDigest is the digest of the primary artifact
	ArtifactDigest string
}

// ListOptions contains options for listing attestations
type ListOptions struct {
	// ZarfPackageJob filters by ZarfPackageJob name
	ZarfPackageJob string

	// Namespace filters by namespace
	Namespace string

	// Operation filters by operation type
	Operation string

	// Limit limits the number of results
	Limit int
}

// LocalStorage stores attestations on the local filesystem
// This is primarily for development and testing
type LocalStorage struct {
	// BasePath is the base directory for storing attestations
	BasePath string
}

// NewLocalStorage creates a new local storage backend
func NewLocalStorage(basePath string) (*LocalStorage, error) {
	// Create base directory if it doesn't exist
	if err := os.MkdirAll(basePath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create attestation storage directory: %w", err)
	}

	return &LocalStorage{
		BasePath: basePath,
	}, nil
}

// Store saves an attestation to the local filesystem
func (s *LocalStorage) Store(ctx context.Context, bundle *AttestationBundle, opts StoreOptions) error {
	klog.InfoS("Storing attestation locally", "zarfPackageJob", opts.ZarfPackageJob, "operation", opts.Operation)

	// Create namespace directory
	nsDir := filepath.Join(s.BasePath, opts.Namespace)
	if err := os.MkdirAll(nsDir, 0755); err != nil {
		return fmt.Errorf("failed to create namespace directory: %w", err)
	}

	// Generate filename
	filename := fmt.Sprintf("%s-%s-%s.json", opts.ZarfPackageJob, opts.Operation, opts.ArtifactDigest[:12])
	filePath := filepath.Join(nsDir, filename)

	// Marshal to JSON
	data, err := json.MarshalIndent(bundle, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal attestation: %w", err)
	}

	// Write to file
	if err := os.WriteFile(filePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write attestation file: %w", err)
	}

	klog.InfoS("Attestation stored", "path", filePath)
	return nil
}

// Retrieve retrieves an attestation by digest
func (s *LocalStorage) Retrieve(ctx context.Context, digest string) (*AttestationBundle, error) {
	// TODO: Implement retrieve by walking directories
	return nil, fmt.Errorf("retrieve not yet implemented for local storage")
}

// List lists attestations matching the given criteria
func (s *LocalStorage) List(ctx context.Context, opts ListOptions) ([]*AttestationBundle, error) {
	// TODO: Implement list by walking directories
	return nil, fmt.Errorf("list not yet implemented for local storage")
}

// OCIStorage stores attestations in an OCI registry
// This is the recommended production storage backend
type OCIStorage struct {
	// Registry is the OCI registry URL
	Registry string

	// Repository is the repository path for attestations
	Repository string

	// TODO: Add OCI client for pushing attestations
}

// NewOCIStorage creates a new OCI storage backend
func NewOCIStorage(registry, repository string) *OCIStorage {
	return &OCIStorage{
		Registry:   registry,
		Repository: repository,
	}
}

// Store saves an attestation to an OCI registry
func (s *OCIStorage) Store(ctx context.Context, bundle *AttestationBundle, opts StoreOptions) error {
	klog.InfoS("Storing attestation in OCI registry", "registry", s.Registry, "zarfPackageJob", opts.ZarfPackageJob)

	// TODO: Implement OCI storage
	// 1. Marshal attestation to JSON
	// 2. Create OCI artifact with attestation as layer
	// 3. Tag with artifact digest
	// 4. Push to registry

	return fmt.Errorf("OCI storage not yet implemented")
}

// Retrieve retrieves an attestation from an OCI registry
func (s *OCIStorage) Retrieve(ctx context.Context, digest string) (*AttestationBundle, error) {
	// TODO: Implement OCI retrieval
	return nil, fmt.Errorf("OCI retrieval not yet implemented")
}

// List lists attestations from an OCI registry
func (s *OCIStorage) List(ctx context.Context, opts ListOptions) ([]*AttestationBundle, error) {
	// TODO: Implement OCI list
	return nil, fmt.Errorf("OCI list not yet implemented")
}

// ConfigMapStorage stores attestations in Kubernetes ConfigMaps
// This is useful for small attestations but not recommended for production
type ConfigMapStorage struct {
	// Namespace is the Kubernetes namespace for ConfigMaps
	Namespace string

	// TODO: Add Kubernetes client
}

// NewConfigMapStorage creates a new ConfigMap storage backend
func NewConfigMapStorage(namespace string) *ConfigMapStorage {
	return &ConfigMapStorage{
		Namespace: namespace,
	}
}

// Store saves an attestation to a ConfigMap
func (s *ConfigMapStorage) Store(ctx context.Context, bundle *AttestationBundle, opts StoreOptions) error {
	klog.InfoS("Storing attestation in ConfigMap", "namespace", s.Namespace, "zarfPackageJob", opts.ZarfPackageJob)

	// TODO: Implement ConfigMap storage
	// 1. Marshal attestation to JSON
	// 2. Create or update ConfigMap with attestation data
	// 3. Add labels for querying

	return fmt.Errorf("ConfigMap storage not yet implemented")
}

// Retrieve retrieves an attestation from a ConfigMap
func (s *ConfigMapStorage) Retrieve(ctx context.Context, digest string) (*AttestationBundle, error) {
	// TODO: Implement ConfigMap retrieval
	return nil, fmt.Errorf("ConfigMap retrieval not yet implemented")
}

// List lists attestations from ConfigMaps
func (s *ConfigMapStorage) List(ctx context.Context, opts ListOptions) ([]*AttestationBundle, error) {
	// TODO: Implement ConfigMap list
	return nil, fmt.Errorf("ConfigMap list not yet implemented")
}
