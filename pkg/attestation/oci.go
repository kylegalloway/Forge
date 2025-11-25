package attestation

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/static"
	"github.com/google/go-containerregistry/pkg/v1/types"
	"k8s.io/klog/v2"
)

// OCIStorageImpl provides OCI registry storage for attestations
type OCIStorageImpl struct {
	*OCIStorage

	// Authenticator provides authentication for the registry
	Authenticator authn.Authenticator

	// Options for remote operations
	Options []remote.Option
}

// NewOCIStorageImpl creates a new OCI storage implementation
func NewOCIStorageImpl(registry, repository string, auth authn.Authenticator) *OCIStorageImpl {
	opts := []remote.Option{
		remote.WithAuth(auth),
	}

	return &OCIStorageImpl{
		OCIStorage: &OCIStorage{
			Registry:   registry,
			Repository: repository,
		},
		Authenticator: auth,
		Options:       opts,
	}
}

// Store saves an attestation to an OCI registry as an artifact
func (s *OCIStorageImpl) Store(ctx context.Context, bundle *AttestationBundle, opts StoreOptions) error {
	klog.InfoS("Storing attestation in OCI registry",
		"registry", s.Registry,
		"repository", s.Repository,
		"zarfPackageJob", opts.ZarfPackageJob,
		"operation", opts.Operation,
	)

	// Marshal attestation to JSON
	attestationJSON, err := json.Marshal(bundle)
	if err != nil {
		return fmt.Errorf("failed to marshal attestation: %w", err)
	}

	// Create attestation layer
	layer := static.NewLayer(attestationJSON, types.MediaType("application/vnd.in-toto+json"))

	// Create base image with layer
	img, err := mutate.AppendLayers(empty.Image, layer)
	if err != nil {
		return fmt.Errorf("failed to append layer: %w", err)
	}

	// Get config file and add labels (annotations on the config, not manifest)
	configFile, err := img.ConfigFile()
	if err != nil {
		return fmt.Errorf("failed to get config file: %w", err)
	}

	// Add labels to config
	if configFile.Config.Labels == nil {
		configFile.Config.Labels = make(map[string]string)
	}
	configFile.Config.Labels["dev.forge.attestation.zarfPackageJob"] = opts.ZarfPackageJob
	configFile.Config.Labels["dev.forge.attestation.namespace"] = opts.Namespace
	configFile.Config.Labels["dev.forge.attestation.operation"] = opts.Operation
	configFile.Config.Labels["dev.forge.attestation.digest"] = opts.ArtifactDigest
	configFile.Config.Labels["dev.forge.attestation.predicateType"] = string(bundle.Statement.PredicateType)

	img, err = mutate.ConfigFile(img, configFile)
	if err != nil {
		return fmt.Errorf("failed to set config file: %w", err)
	}

	// Generate reference
	// Format: registry/repository:attestation-{namespace}-{zpj}-{operation}-{digest}
	tag := fmt.Sprintf("attestation-%s-%s-%s-%s",
		opts.Namespace,
		opts.ZarfPackageJob,
		opts.Operation,
		opts.ArtifactDigest[:12],
	)

	ref, err := name.NewTag(fmt.Sprintf("%s/%s:%s", s.Registry, s.Repository, tag))
	if err != nil {
		return fmt.Errorf("failed to create reference: %w", err)
	}

	// Push to registry
	if err := remote.Write(ref, img, s.Options...); err != nil {
		return fmt.Errorf("failed to push attestation: %w", err)
	}

	klog.InfoS("Attestation stored successfully",
		"reference", ref.String(),
		"digest", opts.ArtifactDigest[:12],
	)

	return nil
}

// Retrieve retrieves an attestation from an OCI registry by digest
func (s *OCIStorageImpl) Retrieve(ctx context.Context, digest string) (*AttestationBundle, error) {
	klog.InfoS("Retrieving attestation from OCI registry",
		"registry", s.Registry,
		"digest", digest[:12],
	)

	// In a real implementation, we'd need to:
	// 1. Search for attestations by digest annotation
	// 2. Pull the matching image
	// 3. Extract the attestation layer
	// 4. Unmarshal to AttestationBundle

	return nil, fmt.Errorf("retrieve by digest not yet fully implemented")
}

// List lists attestations from an OCI registry
func (s *OCIStorageImpl) List(ctx context.Context, opts ListOptions) ([]*AttestationBundle, error) {
	klog.InfoS("Listing attestations from OCI registry",
		"registry", s.Registry,
		"zarfPackageJob", opts.ZarfPackageJob,
	)

	// In a real implementation, we'd need to:
	// 1. List tags in the repository
	// 2. Filter by attestation prefix and criteria
	// 3. Pull matching images
	// 4. Extract and return attestation bundles

	return nil, fmt.Errorf("list not yet fully implemented")
}

// RetrieveByReference retrieves an attestation by full OCI reference
func (s *OCIStorageImpl) RetrieveByReference(ctx context.Context, reference string) (*AttestationBundle, error) {
	klog.InfoS("Retrieving attestation by reference", "reference", reference)

	ref, err := name.ParseReference(reference)
	if err != nil {
		return nil, fmt.Errorf("failed to parse reference: %w", err)
	}

	// Pull image
	img, err := remote.Image(ref, s.Options...)
	if err != nil {
		return nil, fmt.Errorf("failed to pull attestation image: %w", err)
	}

	// Get layers
	layers, err := img.Layers()
	if err != nil {
		return nil, fmt.Errorf("failed to get layers: %w", err)
	}

	if len(layers) == 0 {
		return nil, fmt.Errorf("no layers found in attestation image")
	}

	// Get first layer (attestation data)
	layer := layers[0]
	reader, err := layer.Uncompressed()
	if err != nil {
		return nil, fmt.Errorf("failed to read layer: %w", err)
	}
	defer reader.Close()

	// Unmarshal attestation
	var bundle AttestationBundle
	if err := json.NewDecoder(reader).Decode(&bundle); err != nil {
		return nil, fmt.Errorf("failed to decode attestation: %w", err)
	}

	klog.InfoS("Attestation retrieved successfully", "reference", reference)

	return &bundle, nil
}

// GetReferenceForAttestation generates an OCI reference for an attestation
func (s *OCIStorageImpl) GetReferenceForAttestation(opts StoreOptions) (string, error) {
	tag := fmt.Sprintf("attestation-%s-%s-%s-%s",
		opts.Namespace,
		opts.ZarfPackageJob,
		opts.Operation,
		opts.ArtifactDigest[:12],
	)

	return fmt.Sprintf("%s/%s:%s", s.Registry, s.Repository, tag), nil
}
