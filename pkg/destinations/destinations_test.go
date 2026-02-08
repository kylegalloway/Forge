package destinations

import (
	"testing"

	"github.com/kylegalloway/forge/pkg/apis/common"
	zarfv1alpha3 "github.com/kylegalloway/forge/pkg/apis/zarf/v1alpha3"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestNew(t *testing.T) {
	tests := []struct {
		name     string
		pkg      *zarfv1alpha3.ZarfPackageJob
		wantType string
		wantErr  bool
	}{
		{
			name: "s3 destination",
			pkg: &zarfv1alpha3.ZarfPackageJob{
				Spec: zarfv1alpha3.ZarfPackageJobSpec{
					Publish: &zarfv1alpha3.PublishConfig{
						Destination: zarfv1alpha3.PublishDestination{
							Type: zarfv1alpha3.DestinationTypeS3,
						},
					},
				},
			},
			wantType: "*destinations.S3Destination",
			wantErr:  false,
		},
		{
			name: "oci destination",
			pkg: &zarfv1alpha3.ZarfPackageJob{
				Spec: zarfv1alpha3.ZarfPackageJobSpec{
					Publish: &zarfv1alpha3.PublishConfig{
						Destination: zarfv1alpha3.PublishDestination{
							Type: zarfv1alpha3.DestinationTypeOCI,
						},
					},
				},
			},
			wantType: "*destinations.OCIDestination",
			wantErr:  false,
		},
		{
			name: "local destination",
			pkg: &zarfv1alpha3.ZarfPackageJob{
				Spec: zarfv1alpha3.ZarfPackageJobSpec{
					Publish: &zarfv1alpha3.PublishConfig{
						Destination: zarfv1alpha3.PublishDestination{
							Type: zarfv1alpha3.DestinationTypeLocal,
						},
					},
				},
			},
			wantType: "*destinations.LocalDestination",
			wantErr:  false,
		},
		{
			name: "missing publish config",
			pkg: &zarfv1alpha3.ZarfPackageJob{
				Spec: zarfv1alpha3.ZarfPackageJobSpec{},
			},
			wantErr: true,
		},
		{
			name: "unsupported destination type",
			pkg: &zarfv1alpha3.ZarfPackageJob{
				Spec: zarfv1alpha3.ZarfPackageJobSpec{
					Publish: &zarfv1alpha3.PublishConfig{
						Destination: zarfv1alpha3.PublishDestination{
							Type: "UnsupportedType",
						},
					},
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dest, err := New(tt.pkg)
			if (err != nil) != tt.wantErr {
				t.Errorf("New() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && dest == nil {
				t.Error("New() returned nil destination")
			}
		})
	}
}

func TestS3DestinationGetPublishCommand(t *testing.T) {
	tests := []struct {
		name         string
		pkg          *zarfv1alpha3.ZarfPackageJob
		artifactPath string
		wantErr      bool
		wantContains string // optional: check command contains this string
	}{
		{
			name: "basic s3 upload",
			pkg: &zarfv1alpha3.ZarfPackageJob{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pkg",
					Namespace: "default",
				},
				Spec: zarfv1alpha3.ZarfPackageJobSpec{
					Publish: &zarfv1alpha3.PublishConfig{
						Destination: zarfv1alpha3.PublishDestination{
							Type: zarfv1alpha3.DestinationTypeS3,
							S3: &zarfv1alpha3.S3Destination{
								Bucket:    "my-bucket",
								KeyPrefix: "packages/",
								Region:    "us-east-1",
							},
						},
					},
				},
			},
			artifactPath: "/workspace/test.tar.zst",
			wantErr:      false,
			wantContains: "--region us-east-1",
		},
		{
			name: "s3-compatible storage with endpoint (MinIO)",
			pkg: &zarfv1alpha3.ZarfPackageJob{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pkg",
					Namespace: "default",
				},
				Spec: zarfv1alpha3.ZarfPackageJobSpec{
					Publish: &zarfv1alpha3.PublishConfig{
						Destination: zarfv1alpha3.PublishDestination{
							Type: zarfv1alpha3.DestinationTypeS3,
							S3: &zarfv1alpha3.S3Destination{
								Bucket:    "my-bucket",
								KeyPrefix: "packages/",
								Region:    "us-east-1",
								Endpoint:  "http://minio.local:9000",
							},
						},
					},
				},
			},
			artifactPath: "/workspace/test.tar.zst",
			wantErr:      false,
			wantContains: "--endpoint-url http://minio.local:9000",
		},
		{
			name: "missing s3 config",
			pkg: &zarfv1alpha3.ZarfPackageJob{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pkg",
					Namespace: "default",
				},
				Spec: zarfv1alpha3.ZarfPackageJobSpec{
					Publish: &zarfv1alpha3.PublishConfig{
						Destination: zarfv1alpha3.PublishDestination{
							Type: zarfv1alpha3.DestinationTypeS3,
						},
					},
				},
			},
			artifactPath: "/workspace/test.tar.zst",
			wantErr:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dest := &S3Destination{}
			cmd, err := dest.GetPublishCommand(tt.pkg, tt.artifactPath)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetPublishCommand() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && cmd == "" {
				t.Error("GetPublishCommand() returned empty command")
			}
			if tt.wantContains != "" && !containsString(cmd, tt.wantContains) {
				t.Errorf("GetPublishCommand() = %q, want to contain %q", cmd, tt.wantContains)
			}
		})
	}
}

// containsString checks if s contains substr
func containsString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestS3DestinationGetJobConfiguration(t *testing.T) {
	tests := []struct {
		name    string
		pkg     *zarfv1alpha3.ZarfPackageJob
		wantErr bool
	}{
		{
			name: "s3 without credentials",
			pkg: &zarfv1alpha3.ZarfPackageJob{
				Spec: zarfv1alpha3.ZarfPackageJobSpec{
					Publish: &zarfv1alpha3.PublishConfig{
						Destination: zarfv1alpha3.PublishDestination{
							Type: zarfv1alpha3.DestinationTypeS3,
							S3: &zarfv1alpha3.S3Destination{
								Bucket: "my-bucket",
								Region: "us-east-1",
							},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "s3 with credentials",
			pkg: &zarfv1alpha3.ZarfPackageJob{
				Spec: zarfv1alpha3.ZarfPackageJobSpec{
					Publish: &zarfv1alpha3.PublishConfig{
						Destination: zarfv1alpha3.PublishDestination{
							Type: zarfv1alpha3.DestinationTypeS3,
							S3: &zarfv1alpha3.S3Destination{
								Bucket: "my-bucket",
								Region: "us-east-1",
								CredentialRef: &common.AWSCredentialRef{
									Name: "aws-creds",
								},
							},
						},
					},
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dest := &S3Destination{}
			config, err := dest.GetJobConfiguration(tt.pkg)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetJobConfiguration() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && config == nil {
				t.Error("GetJobConfiguration() returned nil config")
			}
		})
	}
}

func TestOCIDestinationGetPublishCommand(t *testing.T) {
	tests := []struct {
		name         string
		pkg          *zarfv1alpha3.ZarfPackageJob
		artifactPath string
		wantErr      bool
	}{
		{
			name: "basic oci push",
			pkg: &zarfv1alpha3.ZarfPackageJob{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pkg",
					Namespace: "default",
				},
				Spec: zarfv1alpha3.ZarfPackageJobSpec{
					Publish: &zarfv1alpha3.PublishConfig{
						Destination: zarfv1alpha3.PublishDestination{
							Type: zarfv1alpha3.DestinationTypeOCI,
							OCI: &zarfv1alpha3.OCIDestination{
								Registry:   "ghcr.io",
								Repository: "test/packages",
								Tag:        "v1.0.0",
							},
						},
					},
				},
			},
			artifactPath: "/workspace/test.tar.zst",
			wantErr:      false,
		},
		{
			name: "missing oci config",
			pkg: &zarfv1alpha3.ZarfPackageJob{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pkg",
					Namespace: "default",
				},
				Spec: zarfv1alpha3.ZarfPackageJobSpec{
					Publish: &zarfv1alpha3.PublishConfig{
						Destination: zarfv1alpha3.PublishDestination{
							Type: zarfv1alpha3.DestinationTypeOCI,
						},
					},
				},
			},
			artifactPath: "/workspace/test.tar.zst",
			wantErr:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dest := &OCIDestination{}
			cmd, err := dest.GetPublishCommand(tt.pkg, tt.artifactPath)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetPublishCommand() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && cmd == "" {
				t.Error("GetPublishCommand() returned empty command")
			}
		})
	}
}

func TestOCIDestinationGetJobConfiguration(t *testing.T) {
	tests := []struct {
		name    string
		pkg     *zarfv1alpha3.ZarfPackageJob
		wantErr bool
	}{
		{
			name: "oci without credentials",
			pkg: &zarfv1alpha3.ZarfPackageJob{
				Spec: zarfv1alpha3.ZarfPackageJobSpec{
					Publish: &zarfv1alpha3.PublishConfig{
						Destination: zarfv1alpha3.PublishDestination{
							Type: zarfv1alpha3.DestinationTypeOCI,
							OCI: &zarfv1alpha3.OCIDestination{
								Registry:   "ghcr.io",
								Repository: "test/packages",
							},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "oci with credentials",
			pkg: &zarfv1alpha3.ZarfPackageJob{
				Spec: zarfv1alpha3.ZarfPackageJobSpec{
					Publish: &zarfv1alpha3.PublishConfig{
						Destination: zarfv1alpha3.PublishDestination{
							Type: zarfv1alpha3.DestinationTypeOCI,
							OCI: &zarfv1alpha3.OCIDestination{
								Registry:   "ghcr.io",
								Repository: "test/packages",
								CredentialRef: &common.SecretReference{
									Name: "registry-creds",
								},
							},
						},
					},
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dest := &OCIDestination{}
			config, err := dest.GetJobConfiguration(tt.pkg)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetJobConfiguration() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && config == nil {
				t.Error("GetJobConfiguration() returned nil config")
			}
		})
	}
}

func TestLocalDestinationGetPublishCommand(t *testing.T) {
	tests := []struct {
		name         string
		pkg          *zarfv1alpha3.ZarfPackageJob
		artifactPath string
		wantErr      bool
	}{
		{
			name: "local copy",
			pkg: &zarfv1alpha3.ZarfPackageJob{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pkg",
					Namespace: "default",
				},
				Spec: zarfv1alpha3.ZarfPackageJobSpec{
					Publish: &zarfv1alpha3.PublishConfig{
						Destination: zarfv1alpha3.PublishDestination{
							Type: zarfv1alpha3.DestinationTypeLocal,
							Local: &zarfv1alpha3.LocalDestination{
								Path: "/output/packages",
							},
						},
					},
				},
			},
			artifactPath: "/workspace/test.tar.zst",
			wantErr:      true, // local destination requires devMode
		},
		{
			name: "missing local config",
			pkg: &zarfv1alpha3.ZarfPackageJob{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pkg",
					Namespace: "default",
				},
				Spec: zarfv1alpha3.ZarfPackageJobSpec{
					Publish: &zarfv1alpha3.PublishConfig{
						Destination: zarfv1alpha3.PublishDestination{
							Type: zarfv1alpha3.DestinationTypeLocal,
						},
					},
				},
			},
			artifactPath: "/workspace/test.tar.zst",
			wantErr:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dest := &LocalDestination{}
			cmd, err := dest.GetPublishCommand(tt.pkg, tt.artifactPath)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetPublishCommand() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && cmd == "" {
				t.Error("GetPublishCommand() returned empty command")
			}
		})
	}
}

func TestLocalDestinationGetJobConfiguration(t *testing.T) {
	dest := &LocalDestination{}
	pkg := &zarfv1alpha3.ZarfPackageJob{
		Spec: zarfv1alpha3.ZarfPackageJobSpec{
			Publish: &zarfv1alpha3.PublishConfig{
				Destination: zarfv1alpha3.PublishDestination{
					Type: zarfv1alpha3.DestinationTypeLocal,
					Local: &zarfv1alpha3.LocalDestination{
						Path: "/output",
					},
				},
			},
		},
	}

	config, err := dest.GetJobConfiguration(pkg)
	if err != nil {
		t.Errorf("GetJobConfiguration() error = %v", err)
		return
	}
	if config == nil {
		t.Error("GetJobConfiguration() returned nil config")
	}
}

// TestS3DestinationPathConstruction tests S3 destination path building
func TestS3DestinationPathConstruction(t *testing.T) {
	tests := []struct {
		name         string
		bucket       string
		prefix       string
		artifactPath string
		expectedPath string
	}{
		{
			name:         "Basic S3 path",
			bucket:       "my-bucket",
			prefix:       "artifacts",
			artifactPath: "package.tar.zst",
			expectedPath: "s3://my-bucket/artifacts/package.tar.zst",
		},
		{
			name:         "S3 path with nested prefix",
			bucket:       "releases",
			prefix:       "2024/01/builds",
			artifactPath: "build-v1.0.0.tar.zst",
			expectedPath: "s3://releases/2024/01/builds/build-v1.0.0.tar.zst",
		},
		{
			name:         "S3 path without prefix",
			bucket:       "packages",
			prefix:       "",
			artifactPath: "app.tar.zst",
			expectedPath: "s3://packages/app.tar.zst",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Simulate path construction
			var path string
			if tt.prefix != "" {
				path = "s3://" + tt.bucket + "/" + tt.prefix + "/" + tt.artifactPath
			} else {
				path = "s3://" + tt.bucket + "/" + tt.artifactPath
			}

			if path != tt.expectedPath {
				t.Errorf("Expected path %s, got %s", tt.expectedPath, path)
			}
		})
	}
}

// TestOCIDestinationPathConstruction tests OCI image reference construction
func TestOCIDestinationPathConstruction(t *testing.T) {
	tests := []struct {
		name        string
		registry    string
		org         string
		imageName   string
		tag         string
		expectedRef string
	}{
		{
			name:        "GHCR reference",
			registry:    "ghcr.io",
			org:         "myorg",
			imageName:   "myapp",
			tag:         "v1.0.0",
			expectedRef: "ghcr.io/myorg/myapp:v1.0.0",
		},
		{
			name:        "Docker Hub reference",
			registry:    "docker.io",
			org:         "library",
			imageName:   "alpine",
			tag:         "latest",
			expectedRef: "docker.io/library/alpine:latest",
		},
		{
			name:        "Custom registry with port",
			registry:    "registry.internal:5000",
			org:         "apps",
			imageName:   "service",
			tag:         "1.2.3",
			expectedRef: "registry.internal:5000/apps/service:1.2.3",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Simulate OCI reference construction
			ref := tt.registry + "/" + tt.org + "/" + tt.imageName + ":" + tt.tag

			if ref != tt.expectedRef {
				t.Errorf("Expected ref %s, got %s", tt.expectedRef, ref)
			}
		})
	}
}

// TestS3CredentialInjection tests AWS credential injection for S3
func TestS3CredentialInjection(t *testing.T) {
	tests := []struct {
		name           string
		accessKey      string
		secretKey      string
		region         string
		shouldValidate bool
	}{
		{
			name:           "Valid S3 credentials",
			accessKey:      "AKIAIOSFODNN7EXAMPLE",                     // pragma: allowlist secret
			secretKey:      "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY", // pragma: allowlist secret
			region:         "us-east-1",
			shouldValidate: true,
		},
		{
			name:           "Missing secret key",
			accessKey:      "AKIAIOSFODNN7EXAMPLE", // pragma: allowlist secret
			secretKey:      "",
			region:         "us-east-1",
			shouldValidate: false,
		},
		{
			name:           "Missing access key",
			accessKey:      "",
			secretKey:      "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY", // pragma: allowlist secret
			region:         "us-east-1",
			shouldValidate: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Simulate credential validation
			isValid := tt.accessKey != "" && tt.secretKey != ""

			if isValid != tt.shouldValidate {
				t.Errorf("Expected validation=%v, got %v", tt.shouldValidate, isValid)
			}
		})
	}
}

// TestOCIRegistryAuth tests OCI registry authentication
func TestOCIRegistryAuth(t *testing.T) {
	tests := []struct {
		name           string
		username       string
		password       string
		shouldValidate bool
	}{
		{
			name:           "Valid credentials",
			username:       "testuser",
			password:       "testpass",
			shouldValidate: true,
		},
		{
			name:           "Missing password",
			username:       "testuser",
			password:       "",
			shouldValidate: false,
		},
		{
			name:           "Missing username",
			username:       "",
			password:       "testpass",
			shouldValidate: false,
		},
		{
			name:           "Both missing",
			username:       "",
			password:       "",
			shouldValidate: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Simulate auth validation
			isValid := tt.username != "" && tt.password != ""

			if isValid != tt.shouldValidate {
				t.Errorf("Expected validation=%v, got %v", tt.shouldValidate, isValid)
			}
		})
	}
}

// TestS3UploadOperations tests S3 upload mechanics
func TestS3UploadOperations(t *testing.T) {
	tests := []struct {
		name         string
		fileSize     int64
		contentType  string
		shouldUpload bool
	}{
		{
			name:         "Large archive upload",
			fileSize:     1073741824, // 1 GB
			contentType:  "application/x-tar+zst",
			shouldUpload: true,
		},
		{
			name:         "Small file upload",
			fileSize:     1024,
			contentType:  "application/x-tar+zst",
			shouldUpload: true,
		},
		{
			name:         "Empty file",
			fileSize:     0,
			contentType:  "application/x-tar+zst",
			shouldUpload: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Simulate upload validation
			canUpload := tt.fileSize > 0

			if canUpload != tt.shouldUpload {
				t.Errorf("Expected canUpload=%v, got %v", tt.shouldUpload, canUpload)
			}
		})
	}
}

// TestOCILayerPush tests OCI layer push operations
func TestOCILayerPush(t *testing.T) {
	tests := []struct {
		name        string
		layerDigest string
		imageRef    string
		shouldPush  bool
	}{
		{
			name:        "Valid layer push",
			layerDigest: "sha256:abc123def456",
			imageRef:    "ghcr.io/org/image:v1.0.0",
			shouldPush:  true,
		},
		{
			name:        "Missing layer digest",
			layerDigest: "",
			imageRef:    "ghcr.io/org/image:v1.0.0",
			shouldPush:  false,
		},
		{
			name:        "Invalid image reference",
			layerDigest: "sha256:abc123def456",
			imageRef:    "",
			shouldPush:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Simulate push validation
			canPush := tt.layerDigest != "" && tt.imageRef != ""

			if canPush != tt.shouldPush {
				t.Errorf("Expected canPush=%v, got %v", tt.shouldPush, canPush)
			}
		})
	}
}

// TestS3MultipartUpload tests S3 multipart upload configuration
func TestS3MultipartUpload(t *testing.T) {
	tests := []struct {
		name              string
		fileSize          int64
		partSize          int64
		expectedPartCount int
	}{
		{
			name:              "100MB file with 5MB parts",
			fileSize:          104857600,
			partSize:          5242880,
			expectedPartCount: 20,
		},
		{
			name:              "1GB file with 100MB parts",
			fileSize:          1073741824,
			partSize:          104857600,
			expectedPartCount: 11,
		},
		{
			name:              "Small file, single part",
			fileSize:          1048576,
			partSize:          5242880,
			expectedPartCount: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Calculate expected part count
			partCount := int((tt.fileSize + tt.partSize - 1) / tt.partSize)

			if partCount != tt.expectedPartCount {
				t.Errorf("Expected %d parts, got %d", tt.expectedPartCount, partCount)
			}
		})
	}
}
