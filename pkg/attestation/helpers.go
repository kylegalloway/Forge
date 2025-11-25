package attestation

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Constants for attestation annotations
const (
	// AnnotationGenerateAttestation enables attestation generation
	AnnotationGenerateAttestation = "forge.forge.dev/generate-attestation"

	// AnnotationTrackProvenance enables provenance tracking
	AnnotationTrackProvenance = "forge.forge.dev/track-provenance"

	// AnnotationAttestationReference stores the OCI reference to the attestation
	AnnotationAttestationReference = "forge.forge.dev/attestation-reference"

	// AnnotationAttestationDigest stores the digest of the attestation
	AnnotationAttestationDigest = "forge.forge.dev/attestation-digest"
)

// Status field keys for attestation
const (
	// StatusAttestationGenerated indicates if attestation was generated
	StatusAttestationGenerated = "attestationGenerated"

	// StatusAttestationLocation stores where the attestation is stored
	StatusAttestationLocation = "attestationLocation"

	// StatusAttestationDigest stores the attestation digest
	StatusAttestationDigest = "attestationDigest"
)

// ShouldGenerateAttestation checks if attestation should be generated for a resource
func ShouldGenerateAttestation(annotations map[string]string) bool {
	if annotations == nil {
		return false
	}

	val, ok := annotations[AnnotationGenerateAttestation]
	return ok && val == "true"
}

// ShouldTrackProvenance checks if provenance tracking is enabled
func ShouldTrackProvenance(annotations map[string]string) bool {
	if annotations == nil {
		return false
	}

	val, ok := annotations[AnnotationTrackProvenance]
	return ok && val == "true"
}

// GenerateInvocationID generates a unique invocation ID
func GenerateInvocationID(namespace, name string, timestamp time.Time) string {
	data := fmt.Sprintf("%s/%s@%d", namespace, name, timestamp.Unix())
	hash := sha256.Sum256([]byte(data))
	return hex.EncodeToString(hash[:16])
}

// SourceInfoFromSpec extracts source info from ZarfPackageJob spec
// This is a helper that would be called from the controller
func SourceInfoFromGitSpec(url, ref, commitSHA, path string) *SourceInfo {
	return &SourceInfo{
		Type: "Git",
		Git: &GitSourceInfo{
			URL:       url,
			Ref:       ref,
			CommitSHA: commitSHA,
			Path:      path,
		},
	}
}

// SourceInfoFromS3Spec creates source info from S3 spec
func SourceInfoFromS3Spec(bucket, key, region, versionID string) *SourceInfo {
	return &SourceInfo{
		Type: "S3",
		S3: &S3SourceInfo{
			Bucket:    bucket,
			Key:       key,
			Region:    region,
			VersionID: versionID,
		},
	}
}

// SourceInfoFromOCISpec creates source info from OCI spec
func SourceInfoFromOCISpec(registry, repository, tag, digest string) *SourceInfo {
	return &SourceInfo{
		Type: "OCI",
		OCI: &OCISourceInfo{
			Registry:   registry,
			Repository: repository,
			Tag:        tag,
			Digest:     digest,
		},
	}
}

// DestinationInfoFromS3Spec creates destination info from S3 spec
func DestinationInfoFromS3Spec(bucket, key, region, versionID string) *DestinationInfo {
	return &DestinationInfo{
		Type: "S3",
		S3: &S3DestinationInfo{
			Bucket:    bucket,
			Key:       key,
			Region:    region,
			VersionID: versionID,
		},
	}
}

// DestinationInfoFromOCISpec creates destination info from OCI spec
func DestinationInfoFromOCISpec(registry, repository, tag, digest string) *DestinationInfo {
	return &DestinationInfo{
		Type: "OCI",
		OCI: &OCIDestinationInfo{
			Registry:   registry,
			Repository: repository,
			Tag:        tag,
			Digest:     digest,
		},
	}
}

// DeployTargetInfoFromSpec creates deploy target info from spec
func DeployTargetInfoFromSpec(targetType, namespace, clusterName, endpoint string) *DeployTargetInfo {
	return &DeployTargetInfo{
		Type:            targetType,
		Namespace:       namespace,
		ClusterName:     clusterName,
		ClusterEndpoint: endpoint,
	}
}

// AddAttestationAnnotations adds attestation metadata to resource annotations
func AddAttestationAnnotations(obj metav1.Object, reference, digest string) {
	annotations := obj.GetAnnotations()
	if annotations == nil {
		annotations = make(map[string]string)
	}

	annotations[AnnotationAttestationReference] = reference
	annotations[AnnotationAttestationDigest] = digest

	obj.SetAnnotations(annotations)
}

// GetAttestationReference retrieves the attestation reference from annotations
func GetAttestationReference(annotations map[string]string) string {
	if annotations == nil {
		return ""
	}
	return annotations[AnnotationAttestationReference]
}

// GetAttestationDigest retrieves the attestation digest from annotations
func GetAttestationDigest(annotations map[string]string) string {
	if annotations == nil {
		return ""
	}
	return annotations[AnnotationAttestationDigest]
}

// ValidateAttestationBundle validates an attestation bundle structure
func ValidateAttestationBundle(bundle *AttestationBundle) error {
	if bundle == nil {
		return fmt.Errorf("attestation bundle is nil")
	}

	if bundle.Statement.Type != "https://in-toto.io/Statement/v1" {
		return fmt.Errorf("invalid statement type: %s", bundle.Statement.Type)
	}

	if len(bundle.Statement.Subject) == 0 {
		return fmt.Errorf("statement has no subjects")
	}

	if bundle.Statement.PredicateType == "" {
		return fmt.Errorf("predicate type is empty")
	}

	if bundle.Statement.Predicate == nil {
		return fmt.Errorf("predicate is nil")
	}

	return nil
}

// ComputeAttestationDigest computes the digest of an attestation bundle
func ComputeAttestationDigest(bundle *AttestationBundle) (string, error) {
	data, err := SerializeAttestation(bundle)
	if err != nil {
		return "", err
	}

	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:]), nil
}

// SerializeAttestation serializes an attestation bundle to JSON bytes
func SerializeAttestation(bundle *AttestationBundle) ([]byte, error) {
	return json.Marshal(bundle)
}

// DeserializeAttestation deserializes JSON bytes to an attestation bundle
func DeserializeAttestation(data []byte) (*AttestationBundle, error) {
	var bundle AttestationBundle
	if err := json.Unmarshal(data, &bundle); err != nil {
		return nil, fmt.Errorf("failed to unmarshal attestation: %w", err)
	}
	return &bundle, nil
}
