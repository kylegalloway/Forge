package attestation

import (
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestShouldGenerateAttestation(t *testing.T) {
	tests := []struct {
		name        string
		annotations map[string]string
		want        bool
	}{
		{
			name:        "nil annotations",
			annotations: nil,
			want:        false,
		},
		{
			name:        "empty annotations",
			annotations: map[string]string{},
			want:        false,
		},
		{
			name: "annotation set to true",
			annotations: map[string]string{
				AnnotationGenerateAttestation: "true",
			},
			want: true,
		},
		{
			name: "annotation set to false",
			annotations: map[string]string{
				AnnotationGenerateAttestation: "false",
			},
			want: false,
		},
		{
			name: "annotation not set",
			annotations: map[string]string{
				"other-annotation": "value",
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ShouldGenerateAttestation(tt.annotations)
			if got != tt.want {
				t.Errorf("ShouldGenerateAttestation() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestShouldTrackProvenance(t *testing.T) {
	tests := []struct {
		name        string
		annotations map[string]string
		want        bool
	}{
		{
			name:        "nil annotations",
			annotations: nil,
			want:        false,
		},
		{
			name:        "empty annotations",
			annotations: map[string]string{},
			want:        false,
		},
		{
			name: "annotation set to true",
			annotations: map[string]string{
				AnnotationTrackProvenance: "true",
			},
			want: true,
		},
		{
			name: "annotation set to false",
			annotations: map[string]string{
				AnnotationTrackProvenance: "false",
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ShouldTrackProvenance(tt.annotations)
			if got != tt.want {
				t.Errorf("ShouldTrackProvenance() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGenerateInvocationID(t *testing.T) {
	namespace := "test-namespace"
	name := "test-name"
	timestamp := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)

	id1 := GenerateInvocationID(namespace, name, timestamp)
	id2 := GenerateInvocationID(namespace, name, timestamp)

	if id1 != id2 {
		t.Errorf("GenerateInvocationID() should be deterministic, got %s and %s", id1, id2)
	}

	if len(id1) != 32 {
		t.Errorf("GenerateInvocationID() should return 32 char hex string, got %d chars", len(id1))
	}

	differentTime := timestamp.Add(time.Second)
	id3 := GenerateInvocationID(namespace, name, differentTime)
	if id1 == id3 {
		t.Errorf("GenerateInvocationID() should produce different IDs for different times")
	}
}

func TestSourceInfoFromGitSpec(t *testing.T) {
	url := "https://github.com/example/repo"
	ref := "main"
	commitSHA := "abc123"
	path := "subdir"

	info := SourceInfoFromGitSpec(url, ref, commitSHA, path)

	if info.Type != "Git" {
		t.Errorf("expected type 'Git', got '%s'", info.Type)
	}

	if info.Git == nil {
		t.Fatal("expected Git to be set")
	}

	if info.Git.URL != url {
		t.Errorf("expected URL '%s', got '%s'", url, info.Git.URL)
	}

	if info.Git.Ref != ref {
		t.Errorf("expected Ref '%s', got '%s'", ref, info.Git.Ref)
	}

	if info.Git.CommitSHA != commitSHA {
		t.Errorf("expected CommitSHA '%s', got '%s'", commitSHA, info.Git.CommitSHA)
	}

	if info.Git.Path != path {
		t.Errorf("expected Path '%s', got '%s'", path, info.Git.Path)
	}
}

func TestSourceInfoFromS3Spec(t *testing.T) {
	bucket := "my-bucket"
	key := "path/to/package.tar.zst"
	region := "us-east-1"
	versionID := "v123"

	info := SourceInfoFromS3Spec(bucket, key, region, versionID)

	if info.Type != "S3" {
		t.Errorf("expected type 'S3', got '%s'", info.Type)
	}

	if info.S3 == nil {
		t.Fatal("expected S3 to be set")
	}

	if info.S3.Bucket != bucket {
		t.Errorf("expected Bucket '%s', got '%s'", bucket, info.S3.Bucket)
	}

	if info.S3.Key != key {
		t.Errorf("expected Key '%s', got '%s'", key, info.S3.Key)
	}

	if info.S3.Region != region {
		t.Errorf("expected Region '%s', got '%s'", region, info.S3.Region)
	}

	if info.S3.VersionID != versionID {
		t.Errorf("expected VersionID '%s', got '%s'", versionID, info.S3.VersionID)
	}
}

func TestSourceInfoFromOCISpec(t *testing.T) {
	registry := "ghcr.io"
	repository := "myorg/packages"
	tag := "v1.0.0"
	digest := "sha256:abc123"

	info := SourceInfoFromOCISpec(registry, repository, tag, digest)

	if info.Type != "OCI" {
		t.Errorf("expected type 'OCI', got '%s'", info.Type)
	}

	if info.OCI == nil {
		t.Fatal("expected OCI to be set")
	}

	if info.OCI.Registry != registry {
		t.Errorf("expected Registry '%s', got '%s'", registry, info.OCI.Registry)
	}

	if info.OCI.Repository != repository {
		t.Errorf("expected Repository '%s', got '%s'", repository, info.OCI.Repository)
	}

	if info.OCI.Tag != tag {
		t.Errorf("expected Tag '%s', got '%s'", tag, info.OCI.Tag)
	}

	if info.OCI.Digest != digest {
		t.Errorf("expected Digest '%s', got '%s'", digest, info.OCI.Digest)
	}
}

func TestDestinationInfoFromS3Spec(t *testing.T) {
	bucket := "dest-bucket"
	key := "packages/package.tar.zst"
	region := "us-west-2"
	versionID := "v456"

	info := DestinationInfoFromS3Spec(bucket, key, region, versionID)

	if info.Type != "S3" {
		t.Errorf("expected type 'S3', got '%s'", info.Type)
	}

	if info.S3 == nil {
		t.Fatal("expected S3 to be set")
	}

	if info.S3.Bucket != bucket {
		t.Errorf("expected Bucket '%s', got '%s'", bucket, info.S3.Bucket)
	}
}

func TestDestinationInfoFromOCISpec(t *testing.T) {
	registry := "docker.io"
	repository := "myuser/packages"
	tag := "latest"
	digest := "sha256:def456"

	info := DestinationInfoFromOCISpec(registry, repository, tag, digest)

	if info.Type != "OCI" {
		t.Errorf("expected type 'OCI', got '%s'", info.Type)
	}

	if info.OCI == nil {
		t.Fatal("expected OCI to be set")
	}

	if info.OCI.Digest != digest {
		t.Errorf("expected Digest '%s', got '%s'", digest, info.OCI.Digest)
	}
}

func TestDeployTargetInfoFromSpec(t *testing.T) {
	targetType := "RemoteCluster"
	namespace := "production"
	clusterName := "prod-cluster"
	endpoint := "https://k8s.example.com"

	info := DeployTargetInfoFromSpec(targetType, namespace, clusterName, endpoint)

	if info.Type != targetType {
		t.Errorf("expected Type '%s', got '%s'", targetType, info.Type)
	}

	if info.Namespace != namespace {
		t.Errorf("expected Namespace '%s', got '%s'", namespace, info.Namespace)
	}

	if info.ClusterName != clusterName {
		t.Errorf("expected ClusterName '%s', got '%s'", clusterName, info.ClusterName)
	}

	if info.ClusterEndpoint != endpoint {
		t.Errorf("expected ClusterEndpoint '%s', got '%s'", endpoint, info.ClusterEndpoint)
	}
}

func TestAddAttestationAnnotations(t *testing.T) {
	obj := &metav1.ObjectMeta{
		Name:      "test-obj",
		Namespace: "default",
	}

	reference := "ghcr.io/org/attestations:abc123"
	digest := "sha256:fedcba"

	AddAttestationAnnotations(obj, reference, digest)

	annotations := obj.GetAnnotations()
	if annotations == nil {
		t.Fatal("expected annotations to be set")
	}

	if annotations[AnnotationAttestationReference] != reference {
		t.Errorf("expected reference '%s', got '%s'", reference, annotations[AnnotationAttestationReference])
	}

	if annotations[AnnotationAttestationDigest] != digest {
		t.Errorf("expected digest '%s', got '%s'", digest, annotations[AnnotationAttestationDigest])
	}
}

func TestAddAttestationAnnotationsExisting(t *testing.T) {
	obj := &metav1.ObjectMeta{
		Name:      "test-obj",
		Namespace: "default",
		Annotations: map[string]string{
			"existing-annotation": "existing-value",
		},
	}

	reference := "ghcr.io/org/attestations:xyz789"
	digest := "sha256:123abc"

	AddAttestationAnnotations(obj, reference, digest)

	annotations := obj.GetAnnotations()
	if annotations["existing-annotation"] != "existing-value" {
		t.Error("existing annotation should be preserved")
	}

	if annotations[AnnotationAttestationReference] != reference {
		t.Errorf("expected reference '%s', got '%s'", reference, annotations[AnnotationAttestationReference])
	}
}

func TestGetAttestationReference(t *testing.T) {
	tests := []struct {
		name        string
		annotations map[string]string
		want        string
	}{
		{
			name:        "nil annotations",
			annotations: nil,
			want:        "",
		},
		{
			name:        "empty annotations",
			annotations: map[string]string{},
			want:        "",
		},
		{
			name: "reference present",
			annotations: map[string]string{
				AnnotationAttestationReference: "ghcr.io/org/att:123",
			},
			want: "ghcr.io/org/att:123",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetAttestationReference(tt.annotations)
			if got != tt.want {
				t.Errorf("GetAttestationReference() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetAttestationDigest(t *testing.T) {
	tests := []struct {
		name        string
		annotations map[string]string
		want        string
	}{
		{
			name:        "nil annotations",
			annotations: nil,
			want:        "",
		},
		{
			name: "digest present",
			annotations: map[string]string{
				AnnotationAttestationDigest: "sha256:abc",
			},
			want: "sha256:abc",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetAttestationDigest(tt.annotations)
			if got != tt.want {
				t.Errorf("GetAttestationDigest() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestValidateAttestationBundle(t *testing.T) {
	tests := []struct {
		name    string
		bundle  *AttestationBundle
		wantErr bool
	}{
		{
			name:    "nil bundle",
			bundle:  nil,
			wantErr: true,
		},
		{
			name: "valid bundle",
			bundle: &AttestationBundle{
				Statement: Statement{
					Type:          "https://in-toto.io/Statement/v1",
					PredicateType: PredicateTypeSLSAProvenance,
					Subject: []Subject{
						{Name: "test", Digest: map[string]string{"sha256": "abc"}},
					},
					Predicate: SLSAProvenance{},
				},
			},
			wantErr: false,
		},
		{
			name: "invalid statement type",
			bundle: &AttestationBundle{
				Statement: Statement{
					Type:          "invalid",
					PredicateType: PredicateTypeSLSAProvenance,
					Subject: []Subject{
						{Name: "test", Digest: map[string]string{"sha256": "abc"}},
					},
					Predicate: SLSAProvenance{},
				},
			},
			wantErr: true,
		},
		{
			name: "no subjects",
			bundle: &AttestationBundle{
				Statement: Statement{
					Type:          "https://in-toto.io/Statement/v1",
					PredicateType: PredicateTypeSLSAProvenance,
					Subject:       []Subject{},
					Predicate:     SLSAProvenance{},
				},
			},
			wantErr: true,
		},
		{
			name: "empty predicate type",
			bundle: &AttestationBundle{
				Statement: Statement{
					Type:          "https://in-toto.io/Statement/v1",
					PredicateType: "",
					Subject: []Subject{
						{Name: "test", Digest: map[string]string{"sha256": "abc"}},
					},
					Predicate: SLSAProvenance{},
				},
			},
			wantErr: true,
		},
		{
			name: "nil predicate",
			bundle: &AttestationBundle{
				Statement: Statement{
					Type:          "https://in-toto.io/Statement/v1",
					PredicateType: PredicateTypeSLSAProvenance,
					Subject: []Subject{
						{Name: "test", Digest: map[string]string{"sha256": "abc"}},
					},
					Predicate: nil,
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateAttestationBundle(tt.bundle)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateAttestationBundle() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestSerializeDeserializeAttestation(t *testing.T) {
	bundle := &AttestationBundle{
		Statement: Statement{
			Type:          "https://in-toto.io/Statement/v1",
			PredicateType: PredicateTypeSLSAProvenance,
			Subject: []Subject{
				{
					Name: "test-artifact",
					Digest: map[string]string{
						"sha256": "abc123",
					},
				},
			},
			Predicate: map[string]interface{}{
				"buildType": "test",
			},
		},
	}

	data, err := SerializeAttestation(bundle)
	if err != nil {
		t.Fatalf("SerializeAttestation() error = %v", err)
	}

	if len(data) == 0 {
		t.Fatal("SerializeAttestation() returned empty data")
	}

	deserialized, err := DeserializeAttestation(data)
	if err != nil {
		t.Fatalf("DeserializeAttestation() error = %v", err)
	}

	if deserialized.Statement.Type != bundle.Statement.Type {
		t.Errorf("Type mismatch after deserialization")
	}

	if deserialized.Statement.PredicateType != bundle.Statement.PredicateType {
		t.Errorf("PredicateType mismatch after deserialization")
	}

	if len(deserialized.Statement.Subject) != len(bundle.Statement.Subject) {
		t.Errorf("Subject count mismatch after deserialization")
	}
}

func TestDeserializeAttestationInvalid(t *testing.T) {
	invalidJSON := []byte("{invalid json")

	_, err := DeserializeAttestation(invalidJSON)
	if err == nil {
		t.Error("DeserializeAttestation() should fail on invalid JSON")
	}
}

func TestComputeAttestationDigest(t *testing.T) {
	bundle := &AttestationBundle{
		Statement: Statement{
			Type:          "https://in-toto.io/Statement/v1",
			PredicateType: PredicateTypeSLSAProvenance,
			Subject: []Subject{
				{
					Name: "test",
					Digest: map[string]string{
						"sha256": "abc",
					},
				},
			},
			Predicate: SLSAProvenance{},
		},
	}

	digest1, err := ComputeAttestationDigest(bundle)
	if err != nil {
		t.Fatalf("ComputeAttestationDigest() error = %v", err)
	}

	if len(digest1) != 64 {
		t.Errorf("expected 64 char hex digest, got %d chars", len(digest1))
	}

	digest2, err := ComputeAttestationDigest(bundle)
	if err != nil {
		t.Fatalf("ComputeAttestationDigest() error = %v", err)
	}

	if digest1 != digest2 {
		t.Errorf("ComputeAttestationDigest() should be deterministic")
	}
}
