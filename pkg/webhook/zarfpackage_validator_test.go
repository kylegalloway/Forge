package webhook

import (
	"context"
	"testing"

	zarfv1alpha3 "github.com/kylegalloway/forge/pkg/apis/zarf/v1alpha3"
	"github.com/kylegalloway/forge/pkg/constants"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestNewZarfPackageJobValidator(t *testing.T) {
	kubeClient := fake.NewClientset()
	validator := NewZarfPackageJobValidator(kubeClient)
	if validator == nil {
		t.Fatal("NewZarfPackageJobValidator returned nil")
	}
	if validator.kubeClient == nil {
		t.Error("kubeClient not set")
	}
}

func TestValidateZarfPackageJob_ValidBuild(t *testing.T) {
	kubeClient := fake.NewClientset()
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

	pkg := &zarfv1alpha3.ZarfPackageJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-build",
			Namespace: "default",
		},
		Spec: zarfv1alpha3.ZarfPackageJobSpec{
			ServiceAccountName: "test-sa",
			Action:             zarfv1alpha3.ActionBuild,
			Source: zarfv1alpha3.PackageSource{
				Type: zarfv1alpha3.SourceTypeGit,
				Git: &zarfv1alpha3.GitSource{
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
	kubeClient := fake.NewClientset()
	validator := NewZarfPackageJobValidator(kubeClient)

	pkg := &zarfv1alpha3.ZarfPackageJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-build",
			Namespace: "default",
		},
		Spec: zarfv1alpha3.ZarfPackageJobSpec{
			ServiceAccountName: "nonexistent-sa",
			Action:             zarfv1alpha3.ActionBuild,
			Source: zarfv1alpha3.PackageSource{
				Type: zarfv1alpha3.SourceTypeGit,
				Git: &zarfv1alpha3.GitSource{
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
		action        zarfv1alpha3.Action
		wantErr       bool
		errorContains string
	}{
		{
			name: "action allowed",
			annotations: map[string]string{
				constants.AnnotationAllowedActions: "Build,Publish",
			},
			action:  zarfv1alpha3.ActionBuild,
			wantErr: false,
		},
		{
			name: "action not allowed",
			annotations: map[string]string{
				constants.AnnotationAllowedActions: "Build",
			},
			action:        zarfv1alpha3.ActionPublish,
			wantErr:       true,
			errorContains: "not allowed",
		},
		{
			name:          "missing annotation",
			annotations:   map[string]string{},
			action:        zarfv1alpha3.ActionBuild,
			wantErr:       true,
			errorContains: "no allowed-actions annotation",
		},
		{
			name: "compound action allowed",
			annotations: map[string]string{
				constants.AnnotationAllowedActions: "Build,Publish,Deploy,BuildPublish",
			},
			action:  zarfv1alpha3.ActionBuildPublish,
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
		source        *zarfv1alpha3.PackageSource
		wantErr       bool
		errorContains string
	}{
		{
			name: "git source allowed",
			annotations: map[string]string{
				constants.AnnotationAllowedSourceRepos: "https://github.com/myorg/*",
			},
			source: &zarfv1alpha3.PackageSource{
				Type: zarfv1alpha3.SourceTypeGit,
				Git: &zarfv1alpha3.GitSource{
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
			source: &zarfv1alpha3.PackageSource{
				Type: zarfv1alpha3.SourceTypeGit,
				Git: &zarfv1alpha3.GitSource{
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
			source: &zarfv1alpha3.PackageSource{
				Type: zarfv1alpha3.SourceTypeOCI,
				OCI: &zarfv1alpha3.OCISource{
					Reference: "ghcr.io/myorg/package:v1.0.0",
				},
			},
			wantErr: false,
		},
		{
			name: "S3 source allowed",
			annotations: map[string]string{
				constants.AnnotationAllowedSourceBuckets: "my-bucket,other-bucket",
			},
			source: &zarfv1alpha3.PackageSource{
				Type: zarfv1alpha3.SourceTypeS3,
				S3: &zarfv1alpha3.S3Source{
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
		publish       *zarfv1alpha3.PublishConfig
		wantErr       bool
		errorContains string
	}{
		{
			name: "OCI publish allowed",
			annotations: map[string]string{
				constants.AnnotationAllowedPublishRegistries: "ghcr.io/myorg/*",
			},
			publish: &zarfv1alpha3.PublishConfig{
				Destination: zarfv1alpha3.PublishDestination{
					Type: zarfv1alpha3.DestinationTypeOCI,
					OCI: &zarfv1alpha3.OCIDestination{
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
			publish: &zarfv1alpha3.PublishConfig{
				Destination: zarfv1alpha3.PublishDestination{
					Type: zarfv1alpha3.DestinationTypeOCI,
					OCI: &zarfv1alpha3.OCIDestination{
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
			publish: &zarfv1alpha3.PublishConfig{
				Destination: zarfv1alpha3.PublishDestination{
					Type: zarfv1alpha3.DestinationTypeS3,
					S3: &zarfv1alpha3.S3Destination{
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
		deploy        *zarfv1alpha3.DeployConfig
		wantErr       bool
		errorContains string
	}{
		{
			name: "in-cluster deploy allowed",
			annotations: map[string]string{
				constants.AnnotationAllowedDeployTargets: "InCluster",
			},
			deploy: &zarfv1alpha3.DeployConfig{
				Target:    zarfv1alpha3.DeployTargetInCluster,
				Namespace: "default",
			},
			wantErr: false,
		},
		{
			name: "deploy target not allowed",
			annotations: map[string]string{
				constants.AnnotationAllowedDeployTargets: "InCluster",
			},
			deploy: &zarfv1alpha3.DeployConfig{
				Target: zarfv1alpha3.DeployTargetExternalCluster,
			},
			wantErr:       true,
			errorContains: "not allowed",
		},
		{
			name:        "missing annotation",
			annotations: map[string]string{},
			deploy: &zarfv1alpha3.DeployConfig{
				Target: zarfv1alpha3.DeployTargetInCluster,
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
		{"multi-segment URL match", "https://github.com/stefanprodan/podinfo", "https://github.com/*", true},
		{"multi-segment URL with path", "https://github.com/stefanprodan/podinfo/charts/podinfo", "https://github.com/*", true},
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

	client := fake.NewClientset(sa)
	validator := NewZarfPackageJobValidator(client)

	publish := &zarfv1alpha3.PublishConfig{
		Destination: zarfv1alpha3.PublishDestination{
			Type: zarfv1alpha3.DestinationTypeLocal,
			Local: &zarfv1alpha3.LocalDestination{
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

	client := fake.NewClientset(sa)
	validator := NewZarfPackageJobValidator(client)

	publish := &zarfv1alpha3.PublishConfig{
		Destination: zarfv1alpha3.PublishDestination{
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

	client := fake.NewClientset(sa)
	validator := NewZarfPackageJobValidator(client)

	source := &zarfv1alpha3.PackageSource{
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

// TestValidateAction_CaseSensitivity tests that action validation is case-sensitive
func TestValidateAction_CaseSensitivity(t *testing.T) {
	validator := &ZarfPackageJobValidator{}

	tests := []struct {
		name          string
		allowedAction string
		requestAction string
		wantErr       bool
	}{
		{"exact case match", "Build", "Build", false},
		{"lowercase not allowed", "Build", "build", true},
		{"uppercase not allowed", "build", "Build", true},
		{"mixed case not allowed", "Build", "BUILD", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sa := &corev1.ServiceAccount{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-sa",
					Namespace: "default",
					Annotations: map[string]string{
						constants.AnnotationAllowedActions: tt.allowedAction,
					},
				},
			}

			var action zarfv1alpha3.Action
			switch tt.requestAction {
			case "Build":
				action = zarfv1alpha3.ActionBuild
			case "build":
				action = zarfv1alpha3.Action("build")
			case "BUILD":
				action = zarfv1alpha3.Action("BUILD")
			}

			err := validator.validateAction(sa, action)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateAction() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestValidateSource_MultiplePatterns tests glob pattern matching with multiple comma-separated patterns
func TestValidateSource_MultiplePatterns(t *testing.T) {
	validator := &ZarfPackageJobValidator{}

	tests := []struct {
		name           string
		allowedPattern string
		gitURL         string
		wantErr        bool
	}{
		{"matches first pattern", "https://github.com/org1/*,https://gitlab.com/org2/*", "https://github.com/org1/repo", false},
		{"matches second pattern", "https://github.com/org1/*,https://gitlab.com/org2/*", "https://gitlab.com/org2/repo", false},
		{"matches neither pattern", "https://github.com/org1/*,https://gitlab.com/org2/*", "https://github.com/org2/repo", true},
		{"matches with wildcard", "https://github.com/*,https://gitlab.com/*", "https://github.com/anything/repo", false},
		{"single pattern match", "https://github.com/myorg/*", "https://github.com/myorg/test", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sa := &corev1.ServiceAccount{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-sa",
					Namespace: "default",
					Annotations: map[string]string{
						constants.AnnotationAllowedSourceRepos: tt.allowedPattern,
					},
				},
			}

			source := &zarfv1alpha3.PackageSource{
				Type: zarfv1alpha3.SourceTypeGit,
				Git: &zarfv1alpha3.GitSource{
					URL: tt.gitURL,
				},
			}

			err := validator.validateSource(sa, source)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateSource() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestValidateSource_EdgeCases tests edge cases in glob pattern matching
func TestValidateSource_EdgeCases(t *testing.T) {
	validator := &ZarfPackageJobValidator{}

	tests := []struct {
		name           string
		allowedPattern string
		gitURL         string
		wantErr        bool
	}{
		{"wildcard allows all", "*", "https://github.com/any/url", false},
		{"URL with port", "https://github.com:443/org/*", "https://github.com:443/org/repo", false},
		{"empty URL", "", "https://github.com/org/repo", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sa := &corev1.ServiceAccount{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-sa",
					Namespace: "default",
					Annotations: map[string]string{
						constants.AnnotationAllowedSourceRepos: tt.allowedPattern,
					},
				},
			}

			source := &zarfv1alpha3.PackageSource{
				Type: zarfv1alpha3.SourceTypeGit,
				Git: &zarfv1alpha3.GitSource{
					URL: tt.gitURL,
				},
			}

			err := validator.validateSource(sa, source)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateSource() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestValidateAction_AllActions tests all supported Zarf actions
func TestValidateAction_AllActions(t *testing.T) {
	validator := &ZarfPackageJobValidator{}

	actions := []zarfv1alpha3.Action{
		zarfv1alpha3.ActionBuild,
		zarfv1alpha3.ActionPublish,
		zarfv1alpha3.ActionDeploy,
		zarfv1alpha3.ActionBuildPublish,
		zarfv1alpha3.ActionBuildPublishDeploy,
		zarfv1alpha3.ActionPublishDeploy,
	}

	// Test allowing all actions
	allowedActionsStr := "Build,Publish,Deploy,BuildPublish,BuildPublishDeploy,PublishDeploy"

	for _, action := range actions {
		t.Run(string(action), func(t *testing.T) {
			sa := &corev1.ServiceAccount{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-sa",
					Namespace: "default",
					Annotations: map[string]string{
						constants.AnnotationAllowedActions: allowedActionsStr,
					},
				},
			}

			err := validator.validateAction(sa, action)
			if err != nil {
				t.Errorf("validateAction() failed for action %s: %v", action, err)
			}
		})
	}
}

// TestValidateGitSource_ReferenceVariations tests various Git reference formats
func TestValidateGitSource_ReferenceVariations(t *testing.T) {
	validator := &ZarfPackageJobValidator{}

	sa := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-sa",
			Namespace: "default",
			Annotations: map[string]string{
				constants.AnnotationAllowedSourceRepos: "https://github.com/kylegalloway/*",
			},
		},
	}

	tests := []struct {
		name string
		ref  string
	}{
		{"main branch", "main"},
		{"feature branch", "feature/test"},
		{"tag reference", "v1.0.0"},
		{"commit SHA", "abc123def"}, // pragma: allowlist secret
		{"empty ref defaults", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			source := &zarfv1alpha3.PackageSource{
				Type: zarfv1alpha3.SourceTypeGit,
				Git: &zarfv1alpha3.GitSource{
					URL: "https://github.com/kylegalloway/forge",
					Ref: tt.ref,
				},
			}

			err := validator.validateSource(sa, source)
			if err != nil {
				t.Errorf("validateSource() failed for ref %q: %v", tt.ref, err)
			}
		})
	}
}

// TestValidateOCISource_ImageVariations tests various OCI image reference formats
func TestValidateOCISource_ImageVariations(t *testing.T) {
	validator := &ZarfPackageJobValidator{}

	sa := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-sa",
			Namespace: "default",
			Annotations: map[string]string{
				constants.AnnotationAllowedSourceRegistries: "ghcr.io/*,docker.io/library/*",
			},
		},
	}

	tests := []struct {
		name      string
		reference string
		wantErr   bool
	}{
		{"ghcr with tag", "ghcr.io/kylegalloway/forge:v1.0.0", false},
		{"ghcr with digest", "ghcr.io/kylegalloway/forge@sha256:abc123", false},
		{"docker library allowed", "docker.io/library/nginx:latest", false},
		{"docker non-library not allowed", "docker.io/myorg/image:latest", true},
		{"no tag", "ghcr.io/kylegalloway/forge", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			source := &zarfv1alpha3.PackageSource{
				Type: zarfv1alpha3.SourceTypeOCI,
				OCI: &zarfv1alpha3.OCISource{
					Reference: tt.reference,
				},
			}

			err := validator.validateSource(sa, source)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateSource() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestValidateDeploy_NamespaceValidation tests namespace specification for deploy
func TestValidateDeploy_NamespaceValidation(t *testing.T) {
	validator := &ZarfPackageJobValidator{}

	tests := []struct {
		name      string
		target    zarfv1alpha3.DeployTargetType
		namespace string
		wantErr   bool
	}{
		{"in-cluster with namespace", zarfv1alpha3.DeployTargetInCluster, "default", false},
		{"in-cluster empty namespace", zarfv1alpha3.DeployTargetInCluster, "", false},
		{"external cluster", zarfv1alpha3.DeployTargetExternalCluster, "default", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sa := &corev1.ServiceAccount{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-sa",
					Namespace: "default",
					Annotations: map[string]string{
						constants.AnnotationAllowedDeployTargets: "InCluster,ExternalCluster",
					},
				},
			}

			deploy := &zarfv1alpha3.DeployConfig{
				Target:    tt.target,
				Namespace: tt.namespace,
			}

			err := validator.validateDeploy(sa, deploy)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateDeploy() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestValidatePublish_AllDestinationTypes tests all publish destination types
func TestValidatePublish_AllDestinationTypes(t *testing.T) {
	validator := &ZarfPackageJobValidator{}

	tests := []struct {
		name          string
		publish       *zarfv1alpha3.PublishConfig
		annotations   map[string]string
		wantErr       bool
		errorContains string
	}{
		{
			name: "OCI destination",
			publish: &zarfv1alpha3.PublishConfig{
				Destination: zarfv1alpha3.PublishDestination{
					Type: zarfv1alpha3.DestinationTypeOCI,
					OCI: &zarfv1alpha3.OCIDestination{
						Registry:   "ghcr.io",
						Repository: "myorg/packages",
					},
				},
			},
			annotations: map[string]string{
				constants.AnnotationAllowedPublishRegistries: "ghcr.io/*",
			},
			wantErr: false,
		},
		{
			name: "S3 destination",
			publish: &zarfv1alpha3.PublishConfig{
				Destination: zarfv1alpha3.PublishDestination{
					Type: zarfv1alpha3.DestinationTypeS3,
					S3: &zarfv1alpha3.S3Destination{
						Bucket: "my-bucket",
					},
				},
			},
			annotations: map[string]string{
				constants.AnnotationAllowedPublishBuckets: "my-bucket,backup-bucket",
			},
			wantErr: false,
		},
		{
			name: "Local destination",
			publish: &zarfv1alpha3.PublishConfig{
				Destination: zarfv1alpha3.PublishDestination{
					Type: zarfv1alpha3.DestinationTypeLocal,
					Local: &zarfv1alpha3.LocalDestination{
						Path:    "/tmp/output",
						DevMode: false,
					},
				},
			},
			annotations: map[string]string{},
			wantErr:     false,
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
		})
	}
}

// TestAnnotationParsingWhitespace tests annotation parsing with various whitespace patterns
func TestAnnotationParsingWhitespace(t *testing.T) {
	validator := &ZarfPackageJobValidator{}

	tests := []struct {
		name       string
		annotation string
		action     zarfv1alpha3.Action
		wantErr    bool
	}{
		{"no spaces", "Build,Publish,Deploy", zarfv1alpha3.ActionBuild, false},
		{"spaces after comma", "Build, Publish, Deploy", zarfv1alpha3.ActionBuild, false},
		{"mixed spacing", "Build ,Publish , Deploy", zarfv1alpha3.ActionBuild, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sa := &corev1.ServiceAccount{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-sa",
					Namespace: "default",
					Annotations: map[string]string{
						constants.AnnotationAllowedActions: tt.annotation,
					},
				},
			}

			err := validator.validateAction(sa, tt.action)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateAction() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestValidateExtraArgs tests command injection prevention in extra arguments
func TestValidateExtraArgs(t *testing.T) {
	validator := &ZarfPackageJobValidator{}

	tests := []struct {
		name        string
		extraArgs   []string
		wantErr     bool
		errorSubstr string
	}{
		{
			name:      "clean arguments",
			extraArgs: []string{"--registry", "ghcr.io", "--output", "/tmp"},
			wantErr:   false,
		},
		{
			name:        "command injection attempt with semicolon",
			extraArgs:   []string{"--output", "/tmp; rm -rf /"},
			wantErr:     true,
			errorSubstr: "forbidden character",
		},
		{
			name:        "command injection attempt with pipe",
			extraArgs:   []string{"--output", "/tmp | cat /etc/passwd"},
			wantErr:     true,
			errorSubstr: "forbidden character",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spec := &zarfv1alpha3.ZarfPackageJobSpec{
				ServiceAccountName: "test-sa",
				Action:             zarfv1alpha3.ActionBuild,
				Source: zarfv1alpha3.PackageSource{
					Type: zarfv1alpha3.SourceTypeGit,
					Git: &zarfv1alpha3.GitSource{
						URL: "https://github.com/test/repo",
					},
				},
			}

			// Add extra args to Build config if present
			if len(tt.extraArgs) > 0 {
				spec.Build = &zarfv1alpha3.BuildConfig{
					ExtraArgs: tt.extraArgs,
				}
			}

			err := validator.validateExtraArgs(spec)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateExtraArgs() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr && tt.errorSubstr != "" && err != nil {
				if !contains(err.Error(), tt.errorSubstr) {
					t.Errorf("Error should contain %q, got %q", tt.errorSubstr, err.Error())
				}
			}
		})
	}
}
