package webhook

import (
	"context"
	"testing"

	zarfv1alpha1 "github.com/kylegalloway/forge/pkg/apis/zarf/v1alpha1"
	"github.com/kylegalloway/forge/pkg/constants"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestNewZarfPackageJobValidator(t *testing.T) {
	kubeClient := fake.NewSimpleClientset()
	validator := NewZarfPackageJobValidator(kubeClient)
	if validator == nil {
		t.Fatal("NewZarfPackageJobValidator returned nil")
	}
	if validator.kubeClient == nil {
		t.Error("kubeClient not set")
	}
}

func TestValidateZarfPackageJob_ValidBuild(t *testing.T) {
	kubeClient := fake.NewSimpleClientset()
	validator := NewZarfPackageJobValidator(kubeClient)

	// Create ServiceAccount with permissions
	sa := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-sa",
			Namespace: "default",
			Annotations: map[string]string{
				constants.AnnotationAllowedActions:     "Build,Publish",
				constants.AnnotationAllowedSourceRepos: "https://github.com/test/*",
			},
		},
	}
	_, err := kubeClient.CoreV1().ServiceAccounts("default").Create(context.Background(), sa, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("Failed to create ServiceAccount: %v", err)
	}

	pkg := &zarfv1alpha1.ZarfPackageJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-build",
			Namespace: "default",
		},
		Spec: zarfv1alpha1.ZarfPackageJobSpec{
			ServiceAccountName: "test-sa",
			Action:             zarfv1alpha1.ActionBuild,
			Source: zarfv1alpha1.PackageSource{
				Type: zarfv1alpha1.SourceTypeGit,
				Git: &zarfv1alpha1.GitSource{
					URL: "https://github.com/test/repo",
					Ref: "main",
				},
			},
		},
	}

	err = validator.ValidateZarfPackageJob(context.Background(), pkg)
	if err != nil {
		t.Errorf("ValidateZarfPackageJob() failed for valid package: %v", err)
	}
}

func TestValidateZarfPackageJob_MissingServiceAccount(t *testing.T) {
	kubeClient := fake.NewSimpleClientset()
	validator := NewZarfPackageJobValidator(kubeClient)

	pkg := &zarfv1alpha1.ZarfPackageJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-build",
			Namespace: "default",
		},
		Spec: zarfv1alpha1.ZarfPackageJobSpec{
			ServiceAccountName: "nonexistent-sa",
			Action:             zarfv1alpha1.ActionBuild,
			Source: zarfv1alpha1.PackageSource{
				Type: zarfv1alpha1.SourceTypeGit,
				Git: &zarfv1alpha1.GitSource{
					URL: "https://github.com/test/repo",
				},
			},
		},
	}

	err := validator.ValidateZarfPackageJob(context.Background(), pkg)
	if err == nil {
		t.Error("Expected error for missing ServiceAccount, got nil")
	}
}

func TestValidateAction(t *testing.T) {
	validator := &ZarfPackageJobValidator{}

	tests := []struct {
		name          string
		annotations   map[string]string
		action        zarfv1alpha1.Action
		wantErr       bool
		errorContains string
	}{
		{
			name: "action allowed",
			annotations: map[string]string{
				constants.AnnotationAllowedActions: "Build,Publish",
			},
			action:  zarfv1alpha1.ActionBuild,
			wantErr: false,
		},
		{
			name: "action not allowed",
			annotations: map[string]string{
				constants.AnnotationAllowedActions: "Build",
			},
			action:        zarfv1alpha1.ActionPublish,
			wantErr:       true,
			errorContains: "not allowed",
		},
		{
			name:          "missing annotation",
			annotations:   map[string]string{},
			action:        zarfv1alpha1.ActionBuild,
			wantErr:       true,
			errorContains: "no allowed-actions annotation",
		},
		{
			name: "compound action allowed",
			annotations: map[string]string{
				constants.AnnotationAllowedActions: "Build,Publish,Deploy,BuildPublish",
			},
			action:  zarfv1alpha1.ActionBuildPublish,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sa := &corev1.ServiceAccount{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "test-sa",
					Namespace:   "default",
					Annotations: tt.annotations,
				},
			}

			err := validator.validateAction(sa, tt.action)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateAction() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr && tt.errorContains != "" && err != nil {
				if !contains(err.Error(), tt.errorContains) {
					t.Errorf("Error should contain %q, got %q", tt.errorContains, err.Error())
				}
			}
		})
	}
}

func TestValidateSource(t *testing.T) {
	validator := &ZarfPackageJobValidator{}

	tests := []struct {
		name          string
		annotations   map[string]string
		source        *zarfv1alpha1.PackageSource
		wantErr       bool
		errorContains string
	}{
		{
			name: "git source allowed",
			annotations: map[string]string{
				constants.AnnotationAllowedSourceRepos: "https://github.com/myorg/*",
			},
			source: &zarfv1alpha1.PackageSource{
				Type: zarfv1alpha1.SourceTypeGit,
				Git: &zarfv1alpha1.GitSource{
					URL: "https://github.com/myorg/repo",
				},
			},
			wantErr: false,
		},
		{
			name: "git source not allowed",
			annotations: map[string]string{
				constants.AnnotationAllowedSourceRepos: "https://github.com/myorg/*",
			},
			source: &zarfv1alpha1.PackageSource{
				Type: zarfv1alpha1.SourceTypeGit,
				Git: &zarfv1alpha1.GitSource{
					URL: "https://github.com/otherorg/repo",
				},
			},
			wantErr:       true,
			errorContains: "not allowed",
		},
		{
			name: "OCI source allowed",
			annotations: map[string]string{
				constants.AnnotationAllowedSourceRegistries: "ghcr.io/myorg/*",
			},
			source: &zarfv1alpha1.PackageSource{
				Type: zarfv1alpha1.SourceTypeOCI,
				OCI: &zarfv1alpha1.OCISource{
					Image: "ghcr.io/myorg/package:v1.0.0",
				},
			},
			wantErr: false,
		},
		{
			name: "S3 source allowed",
			annotations: map[string]string{
				constants.AnnotationAllowedSourceBuckets: "my-bucket,other-bucket",
			},
			source: &zarfv1alpha1.PackageSource{
				Type: zarfv1alpha1.SourceTypeS3,
				S3: &zarfv1alpha1.S3Source{
					Bucket: "my-bucket",
					Key:    "packages/test.tar.zst",
					Region: "us-east-1",
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sa := &corev1.ServiceAccount{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "test-sa",
					Namespace:   "default",
					Annotations: tt.annotations,
				},
			}

			err := validator.validateSource(sa, tt.source)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateSource() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr && tt.errorContains != "" && err != nil {
				if !contains(err.Error(), tt.errorContains) {
					t.Errorf("Error should contain %q, got %q", tt.errorContains, err.Error())
				}
			}
		})
	}
}

func TestValidatePublish(t *testing.T) {
	validator := &ZarfPackageJobValidator{}

	tests := []struct {
		name          string
		annotations   map[string]string
		publish       *zarfv1alpha1.PublishConfig
		wantErr       bool
		errorContains string
	}{
		{
			name: "OCI publish allowed",
			annotations: map[string]string{
				constants.AnnotationAllowedPublishRegistries: "ghcr.io/myorg/*",
			},
			publish: &zarfv1alpha1.PublishConfig{
				Destination: zarfv1alpha1.PublishDestination{
					Type: zarfv1alpha1.DestinationTypeOCI,
					OCI: &zarfv1alpha1.OCIDestination{
						Registry:   "ghcr.io",
						Repository: "myorg/packages",
						Tag:        "v1.0.0",
					},
				},
			},
			wantErr: false,
		},
		{
			name: "OCI publish not allowed",
			annotations: map[string]string{
				constants.AnnotationAllowedPublishRegistries: "ghcr.io/myorg/*",
			},
			publish: &zarfv1alpha1.PublishConfig{
				Destination: zarfv1alpha1.PublishDestination{
					Type: zarfv1alpha1.DestinationTypeOCI,
					OCI: &zarfv1alpha1.OCIDestination{
						Registry:   "docker.io",
						Repository: "otherorg/packages",
					},
				},
			},
			wantErr:       true,
			errorContains: "not allowed",
		},
		{
			name: "S3 publish allowed",
			annotations: map[string]string{
				constants.AnnotationAllowedPublishBuckets: "publish-bucket,backup-bucket",
			},
			publish: &zarfv1alpha1.PublishConfig{
				Destination: zarfv1alpha1.PublishDestination{
					Type: zarfv1alpha1.DestinationTypeS3,
					S3: &zarfv1alpha1.S3Destination{
						Bucket:    "publish-bucket",
						KeyPrefix: "prod/",
						Region:    "us-east-1",
					},
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sa := &corev1.ServiceAccount{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "test-sa",
					Namespace:   "default",
					Annotations: tt.annotations,
				},
			}

			err := validator.validatePublish(sa, tt.publish)
			if (err != nil) != tt.wantErr {
				t.Errorf("validatePublish() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr && tt.errorContains != "" && err != nil {
				if !contains(err.Error(), tt.errorContains) {
					t.Errorf("Error should contain %q, got %q", tt.errorContains, err.Error())
				}
			}
		})
	}
}

func TestValidateDeploy(t *testing.T) {
	validator := &ZarfPackageJobValidator{}

	tests := []struct {
		name          string
		annotations   map[string]string
		deploy        *zarfv1alpha1.DeployConfig
		wantErr       bool
		errorContains string
	}{
		{
			name: "in-cluster deploy allowed",
			annotations: map[string]string{
				constants.AnnotationAllowedDeployTargets: "InCluster",
			},
			deploy: &zarfv1alpha1.DeployConfig{
				Target:    zarfv1alpha1.DeployTargetInCluster,
				Namespace: "default",
			},
			wantErr: false,
		},
		{
			name: "deploy target not allowed",
			annotations: map[string]string{
				constants.AnnotationAllowedDeployTargets: "InCluster",
			},
			deploy: &zarfv1alpha1.DeployConfig{
				Target: zarfv1alpha1.DeployTargetExternalCluster,
			},
			wantErr:       true,
			errorContains: "not allowed",
		},
		{
			name:        "missing annotation",
			annotations: map[string]string{},
			deploy: &zarfv1alpha1.DeployConfig{
				Target: zarfv1alpha1.DeployTargetInCluster,
			},
			wantErr:       true,
			errorContains: "no allowed-deploy-targets annotation",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sa := &corev1.ServiceAccount{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "test-sa",
					Namespace:   "default",
					Annotations: tt.annotations,
				},
			}

			err := validator.validateDeploy(sa, tt.deploy)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateDeploy() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr && tt.errorContains != "" && err != nil {
				if !contains(err.Error(), tt.errorContains) {
					t.Errorf("Error should contain %q, got %q", tt.errorContains, err.Error())
				}
			}
		})
	}
}

func TestGetAnnotation(t *testing.T) {
	sa := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-sa",
			Namespace: "default",
			Annotations: map[string]string{
				"key1": "value1",
				"key2": "value2",
			},
		},
	}

	tests := []struct {
		name     string
		key      string
		expected string
	}{
		{"existing key", "key1", "value1"},
		{"another key", "key2", "value2"},
		{"missing key", "key3", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getAnnotation(sa, tt.key)
			if result != tt.expected {
				t.Errorf("getAnnotation() = %q, want %q", result, tt.expected)
			}
		})
	}

	// Test with nil annotations
	saNil := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-sa",
			Namespace: "default",
		},
	}
	result := getAnnotation(saNil, "key1")
	if result != "" {
		t.Errorf("getAnnotation() with nil annotations = %q, want empty string", result)
	}
}

func TestMatchesGlob(t *testing.T) {
	tests := []struct {
		name     string
		value    string
		patterns string
		want     bool
	}{
		{"exact match", "https://github.com/test/repo", "https://github.com/test/repo", true},
		{"wildcard match", "https://github.com/test/repo", "https://github.com/test/*", true},
		{"no match", "https://github.com/other/repo", "https://github.com/test/*", false},
		{"empty pattern", "anything", "", false},
		{"complex wildcard", "ghcr.io/myorg/prod/package", "ghcr.io/myorg/*/package", true},
		{"asterisk matches all", "anything", "*", true},
		{"multi-segment URL match", "https://github.com/defenseunicorns/zarf", "https://github.com/*", true},
		{"multi-segment URL with path", "https://github.com/defenseunicorns/zarf/examples/dos-games", "https://github.com/*", true},
		{"multi-segment registry match", "ghcr.io/myorg/team/package", "ghcr.io/*", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := matchesGlob(tt.value, tt.patterns)
			if result != tt.want {
				t.Errorf("matchesGlob(%q, %q) = %v, want %v", tt.value, tt.patterns, result, tt.want)
			}
		})
	}
}

// Helper function to check if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) &&
		(s[:len(substr)] == substr || s[len(s)-len(substr):] == substr ||
			len(s) > len(substr) && len(substr) > 0 && hasSubstring(s, substr)))
}

func hasSubstring(str, substr string) bool {
	for index := 0; index <= len(str)-len(substr); index++ {
		if str[index:index+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestValidatePublish_LocalDestination(t *testing.T) {
	sa := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-sa",
			Namespace: "default",
		},
	}

	client := fake.NewSimpleClientset(sa)
	validator := NewZarfPackageJobValidator(client)

	publish := &zarfv1alpha1.PublishConfig{
		Destination: zarfv1alpha1.PublishDestination{
			Type: zarfv1alpha1.DestinationTypeLocal,
			Local: &zarfv1alpha1.LocalDestination{
				Path:    "/tmp/output",
				DevMode: true,
			},
		},
	}

	err := validator.validatePublish(sa, publish)
	if err != nil {
		t.Errorf("validatePublish() with local destination failed: %v", err)
	}
}

func TestValidatePublish_UnknownDestination(t *testing.T) {
	sa := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-sa",
			Namespace: "default",
		},
	}

	client := fake.NewSimpleClientset(sa)
	validator := NewZarfPackageJobValidator(client)

	publish := &zarfv1alpha1.PublishConfig{
		Destination: zarfv1alpha1.PublishDestination{
			Type: "UnknownType",
		},
	}

	err := validator.validatePublish(sa, publish)
	if err == nil {
		t.Error("validatePublish() with unknown destination should fail")
	}
	if !contains(err.Error(), "unknown publish destination type") {
		t.Errorf("Expected error about unknown type, got: %v", err)
	}
}

func TestValidateSource_UnknownType(t *testing.T) {
	sa := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-sa",
			Namespace: "default",
		},
	}

	client := fake.NewSimpleClientset(sa)
	validator := NewZarfPackageJobValidator(client)

	source := &zarfv1alpha1.PackageSource{
		Type: "UnknownSourceType",
	}

	err := validator.validateSource(sa, source)
	if err == nil {
		t.Error("validateSource() with unknown type should fail")
	}
	if !contains(err.Error(), "unknown source type") {
		t.Errorf("Expected error about unknown source type, got: %v", err)
	}
}

func TestMatchesGlob_InvalidPattern(t *testing.T) {
	// Test with an invalid glob pattern (malformed bracket expression)
	result := matchesGlob("test-value", "[invalid")
	// Invalid patterns should not match
	if result {
		t.Error("matchesGlob() with invalid pattern should return false")
	}
}
