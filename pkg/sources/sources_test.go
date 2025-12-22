package sources

import (
	"testing"

	zarfv1alpha1 "github.com/kylegalloway/forge/pkg/apis/zarf/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestNew(t *testing.T) {
	tests := []struct {
		name     string
		pkg      *zarfv1alpha1.ZarfPackageJob
		wantType string
		wantErr  bool
	}{
		{
			name: "git source",
			pkg: &zarfv1alpha1.ZarfPackageJob{
				Spec: zarfv1alpha1.ZarfPackageJobSpec{
					Source: zarfv1alpha1.PackageSource{
						Type: zarfv1alpha1.SourceTypeGit,
					},
				},
			},
			wantType: "*sources.GitSource",
			wantErr:  false,
		},
		{
			name: "s3 source",
			pkg: &zarfv1alpha1.ZarfPackageJob{
				Spec: zarfv1alpha1.ZarfPackageJobSpec{
					Source: zarfv1alpha1.PackageSource{
						Type: zarfv1alpha1.SourceTypeS3,
					},
				},
			},
			wantType: "*sources.S3Source",
			wantErr:  false,
		},
		{
			name: "oci source",
			pkg: &zarfv1alpha1.ZarfPackageJob{
				Spec: zarfv1alpha1.ZarfPackageJobSpec{
					Source: zarfv1alpha1.PackageSource{
						Type: zarfv1alpha1.SourceTypeOCI,
					},
				},
			},
			wantType: "*sources.OCISource",
			wantErr:  false,
		},
		{
			name: "local source",
			pkg: &zarfv1alpha1.ZarfPackageJob{
				Spec: zarfv1alpha1.ZarfPackageJobSpec{
					Source: zarfv1alpha1.PackageSource{
						Type: zarfv1alpha1.SourceTypeLocal,
					},
				},
			},
			wantType: "*sources.LocalSource",
			wantErr:  false,
		},
		{
			name: "unsupported source",
			pkg: &zarfv1alpha1.ZarfPackageJob{
				Spec: zarfv1alpha1.ZarfPackageJobSpec{
					Source: zarfv1alpha1.PackageSource{
						Type: "UnsupportedType",
					},
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			source, err := New(tt.pkg)
			if (err != nil) != tt.wantErr {
				t.Errorf("New() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && source == nil {
				t.Error("New() returned nil source")
			}
		})
	}
}

func TestGitSourceGetInitContainer(t *testing.T) {
	tests := []struct {
		name    string
		pkg     *zarfv1alpha1.ZarfPackageJob
		wantErr bool
	}{
		{
			name: "basic git clone",
			pkg: &zarfv1alpha1.ZarfPackageJob{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pkg",
					Namespace: "default",
				},
				Spec: zarfv1alpha1.ZarfPackageJobSpec{
					Source: zarfv1alpha1.PackageSource{
						Type: zarfv1alpha1.SourceTypeGit,
						Git: &zarfv1alpha1.GitSource{
							URL: "https://github.com/test/repo",
							Ref: "main",
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "git clone with path",
			pkg: &zarfv1alpha1.ZarfPackageJob{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pkg",
					Namespace: "default",
				},
				Spec: zarfv1alpha1.ZarfPackageJobSpec{
					Source: zarfv1alpha1.PackageSource{
						Type: zarfv1alpha1.SourceTypeGit,
						Git: &zarfv1alpha1.GitSource{
							URL:  "https://github.com/test/repo",
							Ref:  "main",
							Path: "examples/zarf-package",
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "git clone with credentials",
			pkg: &zarfv1alpha1.ZarfPackageJob{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pkg",
					Namespace: "default",
				},
				Spec: zarfv1alpha1.ZarfPackageJobSpec{
					Source: zarfv1alpha1.PackageSource{
						Type: zarfv1alpha1.SourceTypeGit,
						Git: &zarfv1alpha1.GitSource{
							URL: "https://github.com/test/private-repo",
							Ref: "main",
							CredentialsSecretRef: &zarfv1alpha1.SecretReference{
								Name: "git-creds",
							},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "git clone with credentials disabled",
			pkg: &zarfv1alpha1.ZarfPackageJob{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pkg",
					Namespace: "default",
				},
				Spec: zarfv1alpha1.ZarfPackageJobSpec{
					Source: zarfv1alpha1.PackageSource{
						Type: zarfv1alpha1.SourceTypeGit,
						Git: &zarfv1alpha1.GitSource{
							URL: "https://github.com/test/public-repo",
							Ref: "main",
							CredentialsSecretRef: &zarfv1alpha1.SecretReference{
								Name: "git-creds",
							},
							DisableCloneCredentials: true,
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "missing git config",
			pkg: &zarfv1alpha1.ZarfPackageJob{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pkg",
					Namespace: "default",
				},
				Spec: zarfv1alpha1.ZarfPackageJobSpec{
					Source: zarfv1alpha1.PackageSource{
						Type: zarfv1alpha1.SourceTypeGit,
					},
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			source := &GitSource{}
			container, err := source.GetInitContainer(tt.pkg)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetInitContainer() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if container == nil {
					t.Error("GetInitContainer() returned nil container")
					return
				}
				if container.Name != "git-clone" {
					t.Errorf("Expected container name 'git-clone', got %s", container.Name)
				}
				if len(container.Args) == 0 {
					t.Error("Container has no args")
				}

				// Verify DisableCloneCredentials behavior
				if tt.pkg.Spec.Source.Git != nil && tt.pkg.Spec.Source.Git.DisableCloneCredentials {
					// Should contain GIT_ASKPASS=''
					found := false
					for _, arg := range container.Args {
						if containsString(arg, "GIT_ASKPASS=''") {
							found = true
							break
						}
					}
					if !found {
						t.Error("DisableCloneCredentials is true but GIT_ASKPASS='' not found in command")
					}

					// Should NOT mount git-creds volume
					for _, vm := range container.VolumeMounts {
						if vm.Name == "git-creds" {
							t.Error("DisableCloneCredentials is true but git-creds volume is mounted")
						}
					}
				}

				// Verify credentials are mounted when enabled  # pragma: allowlist secret
				if tt.pkg.Spec.Source.Git != nil &&
					tt.pkg.Spec.Source.Git.CredentialsSecretRef != nil && // pragma: allowlist secret
					!tt.pkg.Spec.Source.Git.DisableCloneCredentials {
					// Should mount git-creds volume
					found := false
					for _, vm := range container.VolumeMounts {
						if vm.Name == "git-creds" { // pragma: allowlist secret
							found = true
							break
						}
					}
					if !found {
						t.Error("Credentials provided but git-creds volume not mounted")
					}
				}
			}
		})
	}
}

// Helper function to check if a string contains a substring
func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) &&
		(s[:len(substr)] == substr || s[len(s)-len(substr):] == substr ||
			findSubstring(s, substr)))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestS3SourceGetInitContainer(t *testing.T) {
	tests := []struct {
		name    string
		pkg     *zarfv1alpha1.ZarfPackageJob
		wantErr bool
	}{
		{
			name: "basic s3 download",
			pkg: &zarfv1alpha1.ZarfPackageJob{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pkg",
					Namespace: "default",
				},
				Spec: zarfv1alpha1.ZarfPackageJobSpec{
					Source: zarfv1alpha1.PackageSource{
						Type: zarfv1alpha1.SourceTypeS3,
						S3: &zarfv1alpha1.S3Source{
							Bucket: "my-bucket",
							Key:    "packages/test.tar.zst",
							Region: "us-east-1",
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "s3 with credentials",
			pkg: &zarfv1alpha1.ZarfPackageJob{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pkg",
					Namespace: "default",
				},
				Spec: zarfv1alpha1.ZarfPackageJobSpec{
					Source: zarfv1alpha1.PackageSource{
						Type: zarfv1alpha1.SourceTypeS3,
						S3: &zarfv1alpha1.S3Source{
							Bucket: "my-bucket",
							Key:    "packages/test.tar.zst",
							Region: "us-east-1",
							CredentialsSecretRef: &zarfv1alpha1.SecretReference{
								Name: "aws-creds",
							},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "missing s3 config",
			pkg: &zarfv1alpha1.ZarfPackageJob{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pkg",
					Namespace: "default",
				},
				Spec: zarfv1alpha1.ZarfPackageJobSpec{
					Source: zarfv1alpha1.PackageSource{
						Type: zarfv1alpha1.SourceTypeS3,
					},
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			source := &S3Source{}
			container, err := source.GetInitContainer(tt.pkg)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetInitContainer() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if container == nil {
					t.Error("GetInitContainer() returned nil container")
					return
				}
				if container.Name != "s3-download" {
					t.Errorf("Expected container name 's3-download', got %s", container.Name)
				}
			}
		})
	}
}

func TestOCISourceGetInitContainer(t *testing.T) {
	tests := []struct {
		name    string
		pkg     *zarfv1alpha1.ZarfPackageJob
		wantErr bool
	}{
		{
			name: "basic oci pull",
			pkg: &zarfv1alpha1.ZarfPackageJob{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pkg",
					Namespace: "default",
				},
				Spec: zarfv1alpha1.ZarfPackageJobSpec{
					Source: zarfv1alpha1.PackageSource{
						Type: zarfv1alpha1.SourceTypeOCI,
						OCI: &zarfv1alpha1.OCISource{
							Image: "ghcr.io/test/package:v1.0.0",
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "oci with credentials",
			pkg: &zarfv1alpha1.ZarfPackageJob{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pkg",
					Namespace: "default",
				},
				Spec: zarfv1alpha1.ZarfPackageJobSpec{
					Source: zarfv1alpha1.PackageSource{
						Type: zarfv1alpha1.SourceTypeOCI,
						OCI: &zarfv1alpha1.OCISource{
							Image: "ghcr.io/test/package:v1.0.0",
							CredentialsSecretRef: &zarfv1alpha1.SecretReference{
								Name: "registry-creds",
							},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "missing oci config",
			pkg: &zarfv1alpha1.ZarfPackageJob{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pkg",
					Namespace: "default",
				},
				Spec: zarfv1alpha1.ZarfPackageJobSpec{
					Source: zarfv1alpha1.PackageSource{
						Type: zarfv1alpha1.SourceTypeOCI,
					},
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			source := &OCISource{}
			container, err := source.GetInitContainer(tt.pkg)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetInitContainer() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if container == nil {
					t.Error("GetInitContainer() returned nil container")
					return
				}
				if container.Name != "oci-pull" {
					t.Errorf("Expected container name 'oci-pull', got %s", container.Name)
				}
			}
		})
	}
}

func TestLocalSourceGetInitContainer(t *testing.T) {
	source := &LocalSource{}
	pkg := &zarfv1alpha1.ZarfPackageJob{
		Spec: zarfv1alpha1.ZarfPackageJobSpec{
			Source: zarfv1alpha1.PackageSource{
				Type: zarfv1alpha1.SourceTypeLocal,
			},
		},
	}

	// LocalSource returns nil container and nil error - it's mounted directly
	container, err := source.GetInitContainer(pkg)
	if err != nil {
		t.Errorf("GetInitContainer() unexpected error = %v", err)
	}
	if container != nil {
		t.Error("GetInitContainer() should return nil for local sources")
	}
}

func TestPtr(t *testing.T) {
	// Test the ptr helper function
	intVal := 42
	intPtr := ptr(intVal)
	if intPtr == nil {
		t.Fatal("ptr() returned nil")
	}
	if *intPtr != intVal {
		t.Errorf("Expected *ptr = %d, got %d", intVal, *intPtr)
	}

	strVal := "test"
	strPtr := ptr(strVal)
	if strPtr == nil {
		t.Fatal("ptr() returned nil for string")
	}
	if *strPtr != strVal {
		t.Errorf("Expected *ptr = %s, got %s", strVal, *strPtr)
	}
}
