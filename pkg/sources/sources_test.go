package sources

import (
	"testing"

	"github.com/kylegalloway/forge/pkg/actions"
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
			name: "git source",
			pkg: &zarfv1alpha3.ZarfPackageJob{
				Spec: zarfv1alpha3.ZarfPackageJobSpec{
					Source: zarfv1alpha3.PackageSource{
						Type: zarfv1alpha3.SourceTypeGit,
					},
				},
			},
			wantType: "*sources.GitSource",
			wantErr:  false,
		},
		{
			name: "s3 source",
			pkg: &zarfv1alpha3.ZarfPackageJob{
				Spec: zarfv1alpha3.ZarfPackageJobSpec{
					Source: zarfv1alpha3.PackageSource{
						Type: zarfv1alpha3.SourceTypeS3,
					},
				},
			},
			wantType: "*sources.S3Source",
			wantErr:  false,
		},
		{
			name: "oci source",
			pkg: &zarfv1alpha3.ZarfPackageJob{
				Spec: zarfv1alpha3.ZarfPackageJobSpec{
					Source: zarfv1alpha3.PackageSource{
						Type: zarfv1alpha3.SourceTypeOCI,
					},
				},
			},
			wantType: "*sources.OCISource",
			wantErr:  false,
		},
		{
			name: "local source",
			pkg: &zarfv1alpha3.ZarfPackageJob{
				Spec: zarfv1alpha3.ZarfPackageJobSpec{
					Source: zarfv1alpha3.PackageSource{
						Type: zarfv1alpha3.SourceTypeLocal,
					},
				},
			},
			wantType: "*sources.LocalSource",
			wantErr:  false,
		},
		{
			name: "unsupported source",
			pkg: &zarfv1alpha3.ZarfPackageJob{
				Spec: zarfv1alpha3.ZarfPackageJobSpec{
					Source: zarfv1alpha3.PackageSource{
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
		pkg     *zarfv1alpha3.ZarfPackageJob
		wantErr bool
	}{
		{
			name: "basic git clone",
			pkg: &zarfv1alpha3.ZarfPackageJob{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pkg",
					Namespace: "default",
				},
				Spec: zarfv1alpha3.ZarfPackageJobSpec{
					Source: zarfv1alpha3.PackageSource{
						Type: zarfv1alpha3.SourceTypeGit,
						Git: &zarfv1alpha3.GitSource{
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
			pkg: &zarfv1alpha3.ZarfPackageJob{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pkg",
					Namespace: "default",
				},
				Spec: zarfv1alpha3.ZarfPackageJobSpec{
					Source: zarfv1alpha3.PackageSource{
						Type: zarfv1alpha3.SourceTypeGit,
						Git: &zarfv1alpha3.GitSource{
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
			pkg: &zarfv1alpha3.ZarfPackageJob{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pkg",
					Namespace: "default",
				},
				Spec: zarfv1alpha3.ZarfPackageJobSpec{
					Source: zarfv1alpha3.PackageSource{
						Type: zarfv1alpha3.SourceTypeGit,
						Git: &zarfv1alpha3.GitSource{
							URL: "https://github.com/test/private-repo",
							Ref: "main",
							CredentialRef: &common.SecretReference{
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
			pkg: &zarfv1alpha3.ZarfPackageJob{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pkg",
					Namespace: "default",
				},
				Spec: zarfv1alpha3.ZarfPackageJobSpec{
					Source: zarfv1alpha3.PackageSource{
						Type: zarfv1alpha3.SourceTypeGit,
						Git: &zarfv1alpha3.GitSource{
							URL: "https://github.com/test/public-repo",
							Ref: "main",
							CredentialRef: &common.SecretReference{
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
			pkg: &zarfv1alpha3.ZarfPackageJob{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pkg",
					Namespace: "default",
				},
				Spec: zarfv1alpha3.ZarfPackageJobSpec{
					Source: zarfv1alpha3.PackageSource{
						Type: zarfv1alpha3.SourceTypeGit,
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
					tt.pkg.Spec.Source.Git.CredentialRef != nil && // pragma: allowlist secret
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
		pkg     *zarfv1alpha3.ZarfPackageJob
		wantErr bool
	}{
		{
			name: "basic s3 download",
			pkg: &zarfv1alpha3.ZarfPackageJob{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pkg",
					Namespace: "default",
				},
				Spec: zarfv1alpha3.ZarfPackageJobSpec{
					Source: zarfv1alpha3.PackageSource{
						Type: zarfv1alpha3.SourceTypeS3,
						S3: &zarfv1alpha3.S3Source{
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
			pkg: &zarfv1alpha3.ZarfPackageJob{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pkg",
					Namespace: "default",
				},
				Spec: zarfv1alpha3.ZarfPackageJobSpec{
					Source: zarfv1alpha3.PackageSource{
						Type: zarfv1alpha3.SourceTypeS3,
						S3: &zarfv1alpha3.S3Source{
							Bucket: "my-bucket",
							Key:    "packages/test.tar.zst",
							Region: "us-east-1",
							CredentialRef: &common.AWSCredentialRef{
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
			pkg: &zarfv1alpha3.ZarfPackageJob{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pkg",
					Namespace: "default",
				},
				Spec: zarfv1alpha3.ZarfPackageJobSpec{
					Source: zarfv1alpha3.PackageSource{
						Type: zarfv1alpha3.SourceTypeS3,
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
		pkg     *zarfv1alpha3.ZarfPackageJob
		wantErr bool
	}{
		{
			name: "basic oci pull",
			pkg: &zarfv1alpha3.ZarfPackageJob{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pkg",
					Namespace: "default",
				},
				Spec: zarfv1alpha3.ZarfPackageJobSpec{
					Source: zarfv1alpha3.PackageSource{
						Type: zarfv1alpha3.SourceTypeOCI,
						OCI: &zarfv1alpha3.OCISource{
							Reference: "ghcr.io/test/package:v1.0.0",
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "oci with credentials",
			pkg: &zarfv1alpha3.ZarfPackageJob{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pkg",
					Namespace: "default",
				},
				Spec: zarfv1alpha3.ZarfPackageJobSpec{
					Source: zarfv1alpha3.PackageSource{
						Type: zarfv1alpha3.SourceTypeOCI,
						OCI: &zarfv1alpha3.OCISource{
							Reference: "ghcr.io/test/package:v1.0.0",
							CredentialRef: &common.SecretReference{
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
			pkg: &zarfv1alpha3.ZarfPackageJob{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pkg",
					Namespace: "default",
				},
				Spec: zarfv1alpha3.ZarfPackageJobSpec{
					Source: zarfv1alpha3.PackageSource{
						Type: zarfv1alpha3.SourceTypeOCI,
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
	pkg := &zarfv1alpha3.ZarfPackageJob{
		Spec: zarfv1alpha3.ZarfPackageJobSpec{
			Source: zarfv1alpha3.PackageSource{
				Type: zarfv1alpha3.SourceTypeLocal,
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

func TestGetGitCredentialVolume(t *testing.T) {
	tests := []struct {
		name                    string
		credRef                 *common.SecretReference
		disableCloneCredentials bool
		wantNil                 bool
		wantSecretName          string
	}{
		{
			name:                    "nil credential ref",
			credRef:                 nil,
			disableCloneCredentials: false,
			wantNil:                 true,
		},
		{
			name: "empty secret name",
			credRef: &common.SecretReference{
				Name: "",
			},
			disableCloneCredentials: false,
			wantNil:                 true,
		},
		{
			name: "credentials disabled",
			credRef: &common.SecretReference{
				Name: "git-creds",
			},
			disableCloneCredentials: true,
			wantNil:                 true,
		},
		{
			name: "valid credentials",
			credRef: &common.SecretReference{
				Name: "my-git-secret",
			},
			disableCloneCredentials: false,
			wantNil:                 false,
			wantSecretName:          "my-git-secret",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			vol := GetGitCredentialVolume(tt.credRef, tt.disableCloneCredentials)
			if tt.wantNil {
				if vol != nil {
					t.Errorf("GetGitCredentialVolume() = %v, want nil", vol)
				}
				return
			}
			if vol == nil {
				t.Fatal("GetGitCredentialVolume() returned nil, want non-nil")
			}
			if vol.Name != "git-creds" {
				t.Errorf("Volume name = %s, want git-creds", vol.Name)
			}
			if vol.Secret == nil { // pragma: allowlist secret
				t.Fatal("Volume secret source is nil")
			}
			if vol.Secret.SecretName != tt.wantSecretName { // pragma: allowlist secret
				t.Errorf("Secret name = %s, want %s", vol.Secret.SecretName, tt.wantSecretName)
			}
		})
	}
}

func TestPtr(t *testing.T) {
	// Test the ptr helper function
	intVal := 42
	intPtr := actions.Ptr(intVal)
	if intPtr == nil {
		t.Fatal("actions.Ptr() returned nil")
	}
	if *intPtr != intVal {
		t.Errorf("Expected *ptr = %d, got %d", intVal, *intPtr)
	}

	strVal := "test"
	strPtr := actions.Ptr(strVal)
	if strPtr == nil {
		t.Fatal("actions.Ptr() returned nil for string")
	}
	if *strPtr != strVal {
		t.Errorf("Expected *ptr = %s, got %s", strVal, *strPtr)
	}
}

func TestExtractGitHost(t *testing.T) {
	tests := []struct {
		name     string
		gitURL   string
		wantHost string
	}{
		{
			name:     "HTTPS GitHub URL",
			gitURL:   "https://github.com/user/repo.git",
			wantHost: "github.com",
		},
		{
			name:     "HTTPS GitLab URL",
			gitURL:   "https://gitlab.com/user/repo.git",
			wantHost: "gitlab.com",
		},
		{
			name:     "HTTPS private GitLab instance",
			gitURL:   "https://gitlab.mycompany.com/team/project.git",
			wantHost: "gitlab.mycompany.com",
		},
		{
			name:     "HTTPS Bitbucket URL",
			gitURL:   "https://bitbucket.org/user/repo.git",
			wantHost: "bitbucket.org",
		},
		{
			name:     "HTTPS with port",
			gitURL:   "https://git.internal.io:8443/repo.git",
			wantHost: "git.internal.io:8443",
		},
		{
			name:     "SSH GitHub URL",
			gitURL:   "git@github.com:user/repo.git",
			wantHost: "github.com",
		},
		{
			name:     "SSH GitLab URL",
			gitURL:   "git@gitlab.com:user/repo.git",
			wantHost: "gitlab.com",
		},
		{
			name:     "SSH private instance",
			gitURL:   "git@git.mycompany.io:team/project.git",
			wantHost: "git.mycompany.io",
		},
		{
			name:     "HTTPS without .git suffix",
			gitURL:   "https://github.com/user/repo",
			wantHost: "github.com",
		},
		{
			name:     "empty URL falls back to github.com",
			gitURL:   "",
			wantHost: "github.com",
		},
		{
			name:     "malformed URL falls back to github.com",
			gitURL:   "not-a-valid-url",
			wantHost: "github.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractGitHost(tt.gitURL)
			if got != tt.wantHost {
				t.Errorf("extractGitHost(%q) = %q, want %q", tt.gitURL, got, tt.wantHost)
			}
		})
	}
}
