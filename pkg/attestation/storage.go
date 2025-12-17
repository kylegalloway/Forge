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
func (s *LocalStorage) Store(_ context.Context, bundle *AttestationBundle, opts StoreOptions) error {
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
	if err := os.WriteFile(filePath, data, 0600); err != nil {
		return fmt.Errorf("failed to write attestation file: %w", err)
	}

	klog.InfoS("Attestation stored", "path", filePath)
	return nil
}

// Retrieve retrieves an attestation by digest
func (s *LocalStorage) Retrieve(_ context.Context, digest string) (*AttestationBundle, error) {
	klog.V(4).InfoS("Retrieving attestation by digest", "digest", digest)

	// Walk all namespace directories looking for matching digest
	entries, err := os.ReadDir(s.BasePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read base directory: %w", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		nsDir := filepath.Join(s.BasePath, entry.Name())
		files, err := os.ReadDir(nsDir)
		if err != nil {
			klog.V(4).InfoS("Failed to read namespace directory", "namespace", entry.Name(), "error", err)
			continue
		}

		for _, file := range files {
			if file.IsDir() || filepath.Ext(file.Name()) != ".json" {
				continue
			}

			filePath := filepath.Join(nsDir, file.Name())
			bundle, err := s.readAttestationFile(filePath)
			if err != nil {
				klog.V(4).InfoS("Failed to read attestation file", "path", filePath, "error", err)
				continue
			}

			// Check if any subject digest matches
			for _, subject := range bundle.Statement.Subject {
				for alg, subjectDigest := range subject.Digest {
					fullDigest := fmt.Sprintf("%s:%s", alg, subjectDigest)
					// Match full digest or short digest (first 12 chars after algorithm)
					if fullDigest == digest || subjectDigest == digest {
						klog.InfoS("Attestation found", "path", filePath, "digest", digest)
						return bundle, nil
					}
					// Check short digest match (at least 12 chars)
					if len(digest) >= 12 && len(subjectDigest) >= 12 && subjectDigest[:12] == digest[:12] {
						klog.InfoS("Attestation found", "path", filePath, "digest", digest)
						return bundle, nil
					}
					if len(digest) >= 12 && len(fullDigest) >= 12 && fullDigest[:12] == digest[:12] {
						klog.InfoS("Attestation found", "path", filePath, "digest", digest)
						return bundle, nil
					}
				}
			}
		}
	}

	return nil, fmt.Errorf("attestation not found for digest: %s", digest)
}

// List lists attestations matching the given criteria
func (s *LocalStorage) List(_ context.Context, opts ListOptions) ([]*AttestationBundle, error) {
	klog.V(4).InfoS("Listing attestations", "options", opts)

	var results []*AttestationBundle

	// Determine which directories to search
	var namespaceDirs []string
	if opts.Namespace != "" {
		namespaceDirs = []string{opts.Namespace}
	} else {
		// List all namespace directories
		entries, err := os.ReadDir(s.BasePath)
		if err != nil {
			return nil, fmt.Errorf("failed to read base directory: %w", err)
		}
		for _, entry := range entries {
			if entry.IsDir() {
				namespaceDirs = append(namespaceDirs, entry.Name())
			}
		}
	}

	// Walk each namespace directory
	for _, ns := range namespaceDirs {
		nsDir := filepath.Join(s.BasePath, ns)
		files, err := os.ReadDir(nsDir)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, fmt.Errorf("failed to read namespace directory %s: %w", ns, err)
		}

		for _, file := range files {
			if file.IsDir() || filepath.Ext(file.Name()) != ".json" {
				continue
			}

			filePath := filepath.Join(nsDir, file.Name())
			bundle, err := s.readAttestationFile(filePath)
			if err != nil {
				klog.V(4).InfoS("Failed to read attestation file", "path", filePath, "error", err)
				continue
			}

			// Apply filters
			if !s.matchesFilters(bundle, opts) {
				continue
			}

			results = append(results, bundle)

			// Check limit
			if opts.Limit > 0 && len(results) >= opts.Limit {
				klog.InfoS("Attestations listed", "count", len(results), "limit", opts.Limit)
				return results, nil
			}
		}
	}

	klog.InfoS("Attestations listed", "count", len(results))
	return results, nil
}

// readAttestationFile reads and unmarshals an attestation from a file
func (s *LocalStorage) readAttestationFile(filePath string) (*AttestationBundle, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	var bundle AttestationBundle
	if err := json.Unmarshal(data, &bundle); err != nil {
		return nil, fmt.Errorf("failed to unmarshal attestation: %w", err)
	}

	return &bundle, nil
}

// matchesFilters checks if an attestation matches the given filters
func (s *LocalStorage) matchesFilters(bundle *AttestationBundle, opts ListOptions) bool {
	// Extract Forge-specific predicate if present
	var forgePredicate *ForgeOperationPredicate
	if bundle.Statement.PredicateType == PredicateTypeForgeOperation {
		if pred, ok := bundle.Statement.Predicate.(map[string]interface{}); ok {
			// Convert map to struct
			predJSON, err := json.Marshal(pred)
			if err != nil {
				return false
			}
			forgePredicate = &ForgeOperationPredicate{}
			if err := json.Unmarshal(predJSON, forgePredicate); err != nil {
				return false
			}
		}
	}

	// Filter by ZarfPackageJob
	if opts.ZarfPackageJob != "" && forgePredicate != nil {
		if forgePredicate.ZarfPackageJob != opts.ZarfPackageJob {
			return false
		}
	}

	// Filter by Operation
	if opts.Operation != "" && forgePredicate != nil {
		if forgePredicate.Operation != opts.Operation {
			return false
		}
	}

	return true
}

// OCIStorage stores attestations in an OCI registry
// This is the recommended production storage backend
type OCIStorage struct {
	// Registry is the OCI registry URL
	Registry string

	// Repository is the repository path for attestations
	Repository string

	// OCIClient will be added when OCI storage is implemented
}

// NewOCIStorage creates a new OCI storage backend
func NewOCIStorage(registry, repository string) *OCIStorage {
	return &OCIStorage{
		Registry:   registry,
		Repository: repository,
	}
}

// Store saves an attestation to an OCI registry
// OCI storage is not yet implemented. When complete, this will marshal the attestation
// to JSON, create an OCI artifact with the attestation as a layer, tag it with the
// artifact digest, and push to the registry.
func (s *OCIStorage) Store(_ context.Context, _ *AttestationBundle, _ StoreOptions) error {
	klog.InfoS("Storing attestation in OCI registry", "registry", s.Registry)
	return fmt.Errorf("OCI storage not yet implemented")
}

// Retrieve retrieves an attestation from an OCI registry
// OCI retrieval is not yet implemented.
func (s *OCIStorage) Retrieve(_ context.Context, _ string) (*AttestationBundle, error) {
	return nil, fmt.Errorf("OCI retrieval not yet implemented")
}

// List lists attestations from an OCI registry
// OCI list is not yet implemented.
func (s *OCIStorage) List(_ context.Context, _ ListOptions) ([]*AttestationBundle, error) {
	return nil, fmt.Errorf("OCI list not yet implemented")
}

// ConfigMapStorage stores attestations in Kubernetes ConfigMaps
// This is useful for small attestations but not recommended for large-scale production
type ConfigMapStorage struct {
	// Namespace is the Kubernetes namespace for ConfigMaps
	Namespace string

	// kubeClient is the Kubernetes client interface
	kubeClient KubeClient
}

// KubeClient is an interface for Kubernetes operations (for testing)
type KubeClient interface {
	CreateConfigMap(ctx context.Context, namespace string, cm *ConfigMap) error
	GetConfigMap(ctx context.Context, namespace, name string) (*ConfigMap, error)
	ListConfigMaps(ctx context.Context, namespace string, labels map[string]string) ([]*ConfigMap, error)
	UpdateConfigMap(ctx context.Context, namespace string, cm *ConfigMap) error
}

// ConfigMap represents a Kubernetes ConfigMap
type ConfigMap struct {
	Name   string
	Labels map[string]string
	Data   map[string]string
}

// NewConfigMapStorage creates a new ConfigMap storage backend
func NewConfigMapStorage(namespace string, kubeClient KubeClient) *ConfigMapStorage {
	return &ConfigMapStorage{
		Namespace:  namespace,
		kubeClient: kubeClient,
	}
}

// Store saves an attestation to a ConfigMap
func (s *ConfigMapStorage) Store(ctx context.Context, bundle *AttestationBundle, opts StoreOptions) error {
	klog.InfoS("Storing attestation in ConfigMap", "namespace", s.Namespace, "zarfPackageJob", opts.ZarfPackageJob)

	// Marshal attestation to JSON
	data, err := json.MarshalIndent(bundle, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal attestation: %w", err)
	}

	// Generate ConfigMap name
	cmName := fmt.Sprintf("attestation-%s-%s-%s", opts.ZarfPackageJob, opts.Operation, opts.ArtifactDigest[:12])

	// Create ConfigMap
	cm := &ConfigMap{
		Name: cmName,
		Labels: map[string]string{
			"forge.dev/attestation":     "true",
			"forge.dev/zarfpackagejob":  opts.ZarfPackageJob,
			"forge.dev/operation":       opts.Operation,
			"forge.dev/artifact-digest": opts.ArtifactDigest[:12],
		},
		Data: map[string]string{
			"attestation.json": string(data),
		},
	}

	// Try to create the ConfigMap
	if err := s.kubeClient.CreateConfigMap(ctx, s.Namespace, cm); err != nil {
		// If it already exists, update it
		if err := s.kubeClient.UpdateConfigMap(ctx, s.Namespace, cm); err != nil {
			return fmt.Errorf("failed to update ConfigMap: %w", err)
		}
	}

	klog.InfoS("Attestation stored in ConfigMap", "name", cmName, "namespace", s.Namespace)
	return nil
}

// Retrieve retrieves an attestation from a ConfigMap
func (s *ConfigMapStorage) Retrieve(ctx context.Context, digest string) (*AttestationBundle, error) {
	klog.V(4).InfoS("Retrieving attestation from ConfigMap", "digest", digest)

	// List all attestation ConfigMaps
	labels := map[string]string{
		"forge.dev/attestation": "true",
	}

	configMaps, err := s.kubeClient.ListConfigMaps(ctx, s.Namespace, labels)
	if err != nil {
		return nil, fmt.Errorf("failed to list ConfigMaps: %w", err)
	}

	// Search for matching digest
	for _, cm := range configMaps {
		attestationData, ok := cm.Data["attestation.json"]
		if !ok {
			continue
		}

		var bundle AttestationBundle
		if err := json.Unmarshal([]byte(attestationData), &bundle); err != nil {
			klog.V(4).InfoS("Failed to unmarshal attestation from ConfigMap", "name", cm.Name, "error", err)
			continue
		}

		// Check if any subject digest matches
		for _, subject := range bundle.Statement.Subject {
			for alg, subjectDigest := range subject.Digest {
				fullDigest := fmt.Sprintf("%s:%s", alg, subjectDigest)
				// Match full digest or short digest
				if fullDigest == digest || subjectDigest == digest {
					klog.InfoS("Attestation found in ConfigMap", "name", cm.Name, "digest", digest)
					return &bundle, nil
				}
				// Check short digest match (at least 12 chars)
				if len(digest) >= 12 && len(subjectDigest) >= 12 && subjectDigest[:12] == digest[:12] {
					klog.InfoS("Attestation found in ConfigMap", "name", cm.Name, "digest", digest)
					return &bundle, nil
				}
				if len(digest) >= 12 && len(fullDigest) >= 12 && fullDigest[:12] == digest[:12] {
					klog.InfoS("Attestation found in ConfigMap", "name", cm.Name, "digest", digest)
					return &bundle, nil
				}
			}
		}
	}

	return nil, fmt.Errorf("attestation not found for digest: %s", digest)
}

// List lists attestations from ConfigMaps
func (s *ConfigMapStorage) List(ctx context.Context, opts ListOptions) ([]*AttestationBundle, error) {
	klog.V(4).InfoS("Listing attestations from ConfigMaps", "options", opts)

	// Build label selector
	labels := map[string]string{
		"forge.dev/attestation": "true",
	}

	if opts.ZarfPackageJob != "" {
		labels["forge.dev/zarfpackagejob"] = opts.ZarfPackageJob
	}

	if opts.Operation != "" {
		labels["forge.dev/operation"] = opts.Operation
	}

	configMaps, err := s.kubeClient.ListConfigMaps(ctx, s.Namespace, labels)
	if err != nil {
		return nil, fmt.Errorf("failed to list ConfigMaps: %w", err)
	}

	var results []*AttestationBundle

	for _, cm := range configMaps {
		attestationData, ok := cm.Data["attestation.json"]
		if !ok {
			klog.V(4).InfoS("ConfigMap missing attestation.json", "name", cm.Name)
			continue
		}

		var bundle AttestationBundle
		if err := json.Unmarshal([]byte(attestationData), &bundle); err != nil {
			klog.V(4).InfoS("Failed to unmarshal attestation from ConfigMap", "name", cm.Name, "error", err)
			continue
		}

		results = append(results, &bundle)

		// Check limit
		if opts.Limit > 0 && len(results) >= opts.Limit {
			klog.InfoS("Attestations listed from ConfigMaps", "count", len(results), "limit", opts.Limit)
			return results, nil
		}
	}

	klog.InfoS("Attestations listed from ConfigMaps", "count", len(results))
	return results, nil
}
