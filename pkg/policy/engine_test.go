package policy

import (
	"context"
	"testing"

	udsv1alpha2 "github.com/kylegalloway/forge/pkg/apis/uds/v1alpha2"
	zarfv1alpha1 "github.com/kylegalloway/forge/pkg/apis/zarf/v1alpha1"
	"github.com/kylegalloway/forge/pkg/constants"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestValidate_MissingServiceAccount(t *testing.T) {
	client := fake.NewClientset()
	engine := NewEngine(client)

	pkg := &zarfv1alpha1.ZarfPackageJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pkg",
			Namespace: "default",
		},
		Spec: zarfv1alpha1.ZarfPackageJobSpec{
			ServiceAccountName: "",
			Action:             zarfv1alpha1.ActionBuild,
		},
	}

	err := engine.Validate(context.Background(), pkg)
	if err == nil {
		t.Fatal("expected error for missing serviceAccountName, got nil")
	}
	if err.Error() != "serviceAccountName is required" {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestValidate_ServiceAccountNotFound(t *testing.T) {
	client := fake.NewClientset()
	engine := NewEngine(client)

	pkg := &zarfv1alpha1.ZarfPackageJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pkg",
			Namespace: "default",
		},
		Spec: zarfv1alpha1.ZarfPackageJobSpec{
			ServiceAccountName: "nonexistent-sa",
			Action:             zarfv1alpha1.ActionBuild,
		},
	}

	err := engine.Validate(context.Background(), pkg)
	if err == nil {
		t.Fatal("expected error for nonexistent service account, got nil")
	}
}

func TestValidate_ActionNotAllowed(t *testing.T) {
	sa := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-sa",
			Namespace: "default",
			Annotations: map[string]string{
				constants.AnnotationAllowedActions: "Build,Publish",
			},
		},
	}

	client := fake.NewClientset(sa)
	engine := NewEngine(client)

	pkg := &zarfv1alpha1.ZarfPackageJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pkg",
			Namespace: "default",
		},
		Spec: zarfv1alpha1.ZarfPackageJobSpec{
			ServiceAccountName: "test-sa",
			Action:             zarfv1alpha1.ActionDeploy,
			Source: zarfv1alpha1.PackageSource{
				Type: zarfv1alpha1.SourceTypeGit,
				Git: &zarfv1alpha1.GitSource{
					URL: "https://github.com/test/repo",
					Ref: "main",
				},
			},
		},
	}

	err := engine.Validate(context.Background(), pkg)
	if err == nil {
		t.Fatal("expected error for disallowed action, got nil")
	}
	if !contains(err.Error(), "action Deploy is not allowed") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestValidate_GitSourceAllowed(t *testing.T) {
	sa := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-sa",
			Namespace: "default",
			Annotations: map[string]string{
				constants.AnnotationAllowedActions:     "Build",
				constants.AnnotationAllowedSourceRepos: "https://github.com/myorg/*",
			},
		},
	}

	client := fake.NewClientset(sa)
	engine := NewEngine(client)

	pkg := &zarfv1alpha1.ZarfPackageJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pkg",
			Namespace: "default",
		},
		Spec: zarfv1alpha1.ZarfPackageJobSpec{
			ServiceAccountName: "test-sa",
			Action:             zarfv1alpha1.ActionBuild,
			Source: zarfv1alpha1.PackageSource{
				Type: zarfv1alpha1.SourceTypeGit,
				Git: &zarfv1alpha1.GitSource{
					URL: "https://github.com/myorg/myrepo",
					Ref: "main",
				},
			},
		},
	}

	err := engine.Validate(context.Background(), pkg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidate_GitSourceNotAllowed(t *testing.T) {
	sa := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-sa",
			Namespace: "default",
			Annotations: map[string]string{
				constants.AnnotationAllowedActions:     "Build",
				constants.AnnotationAllowedSourceRepos: "https://github.com/myorg/*",
			},
		},
	}

	client := fake.NewClientset(sa)
	engine := NewEngine(client)

	pkg := &zarfv1alpha1.ZarfPackageJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pkg",
			Namespace: "default",
		},
		Spec: zarfv1alpha1.ZarfPackageJobSpec{
			ServiceAccountName: "test-sa",
			Action:             zarfv1alpha1.ActionBuild,
			Source: zarfv1alpha1.PackageSource{
				Type: zarfv1alpha1.SourceTypeGit,
				Git: &zarfv1alpha1.GitSource{
					URL: "https://github.com/other/repo",
					Ref: "main",
				},
			},
		},
	}

	err := engine.Validate(context.Background(), pkg)
	if err == nil {
		t.Fatal("expected error for disallowed git repo, got nil")
	}
}

func TestValidate_WildcardAllowsAll(t *testing.T) {
	sa := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-sa",
			Namespace: "default",
			Annotations: map[string]string{
				constants.AnnotationAllowedActions:     "*",
				constants.AnnotationAllowedSourceRepos: "*",
			},
		},
	}

	client := fake.NewClientset(sa)
	engine := NewEngine(client)

	pkg := &zarfv1alpha1.ZarfPackageJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pkg",
			Namespace: "default",
		},
		Spec: zarfv1alpha1.ZarfPackageJobSpec{
			ServiceAccountName: "test-sa",
			Action:             zarfv1alpha1.ActionBuildPublishDeploy,
			Source: zarfv1alpha1.PackageSource{
				Type: zarfv1alpha1.SourceTypeGit,
				Git: &zarfv1alpha1.GitSource{
					URL: "https://github.com/any/repo",
					Ref: "main",
				},
			},
		},
	}

	err := engine.Validate(context.Background(), pkg)
	if err != nil {
		t.Fatalf("unexpected error with wildcard permissions: %v", err)
	}
}

func TestValidate_S3Source(t *testing.T) {
	sa := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-sa",
			Namespace: "default",
			Annotations: map[string]string{
				constants.AnnotationAllowedActions:       "Build",
				constants.AnnotationAllowedSourceBuckets: "my-bucket-*",
			},
		},
	}

	client := fake.NewClientset(sa)
	engine := NewEngine(client)

	pkg := &zarfv1alpha1.ZarfPackageJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pkg",
			Namespace: "default",
		},
		Spec: zarfv1alpha1.ZarfPackageJobSpec{
			ServiceAccountName: "test-sa",
			Action:             zarfv1alpha1.ActionBuild,
			Source: zarfv1alpha1.PackageSource{
				Type: zarfv1alpha1.SourceTypeS3,
				S3: &zarfv1alpha1.S3Source{
					Bucket: "my-bucket-prod",
					Key:    "package.tar.zst",
					Region: "us-west-2",
				},
			},
		},
	}

	err := engine.Validate(context.Background(), pkg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidate_LocalSourceDeniedByDefault(t *testing.T) {
	sa := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-sa",
			Namespace: "default",
			Annotations: map[string]string{
				constants.AnnotationAllowedActions: "Build",
			},
		},
	}

	client := fake.NewClientset(sa)
	engine := NewEngine(client)

	pkg := &zarfv1alpha1.ZarfPackageJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pkg",
			Namespace: "default",
		},
		Spec: zarfv1alpha1.ZarfPackageJobSpec{
			ServiceAccountName: "test-sa",
			Action:             zarfv1alpha1.ActionBuild,
			Source: zarfv1alpha1.PackageSource{
				Type: zarfv1alpha1.SourceTypeLocal,
				Local: &zarfv1alpha1.LocalSource{
					Path:    "/tmp/package",
					DevMode: true,
				},
			},
		},
	}

	err := engine.Validate(context.Background(), pkg)
	if err == nil {
		t.Fatal("expected error for local source without permission, got nil")
	}
}

func TestValidate_LocalSourceAllowed(t *testing.T) {
	sa := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-sa",
			Namespace: "default",
			Annotations: map[string]string{
				constants.AnnotationAllowedActions:    "Build",
				constants.AnnotationAllowLocalSources: "true",
			},
		},
	}

	client := fake.NewClientset(sa)
	engine := NewEngine(client)

	pkg := &zarfv1alpha1.ZarfPackageJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pkg",
			Namespace: "default",
		},
		Spec: zarfv1alpha1.ZarfPackageJobSpec{
			ServiceAccountName: "test-sa",
			Action:             zarfv1alpha1.ActionBuild,
			Source: zarfv1alpha1.PackageSource{
				Type: zarfv1alpha1.SourceTypeLocal,
				Local: &zarfv1alpha1.LocalSource{
					Path:    "/tmp/package",
					DevMode: true,
				},
			},
		},
	}

	err := engine.Validate(context.Background(), pkg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParseList(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "empty string",
			input:    "",
			expected: nil,
		},
		{
			name:     "single item",
			input:    "item1",
			expected: []string{"item1"},
		},
		{
			name:     "multiple items",
			input:    "item1,item2,item3",
			expected: []string{"item1", "item2", "item3"},
		},
		{
			name:     "items with spaces",
			input:    "item1, item2 , item3",
			expected: []string{"item1", "item2", "item3"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseList(tt.input)
			if len(result) != len(tt.expected) {
				t.Errorf("expected %d items, got %d", len(tt.expected), len(result))
				return
			}
			for index := range result {
				if result[index] != tt.expected[index] {
					t.Errorf("item %d: expected %s, got %s", index, tt.expected[index], result[index])
				}
			}
		})
	}
}

func TestMatchAny(t *testing.T) {
	tests := []struct {
		name     string
		patterns []string
		value    string
		expected bool
	}{
		{
			name:     "exact match",
			patterns: []string{"github.com/myorg/repo"},
			value:    "github.com/myorg/repo",
			expected: true,
		},
		{
			name:     "prefix wildcard match",
			patterns: []string{"https://github.com/myorg/*"},
			value:    "https://github.com/myorg/repo",
			expected: true,
		},
		{
			name:     "no match",
			patterns: []string{"https://github.com/myorg/*"},
			value:    "https://github.com/other/repo",
			expected: false,
		},
		{
			name:     "multiple patterns",
			patterns: []string{"https://github.com/myorg/*", "github.com/platform/*"},
			value:    "github.com/platform/tools",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := matchAny(tt.patterns, tt.value)
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestValidate_PublishDestinationS3(t *testing.T) {
	sa := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-sa",
			Namespace: "default",
			Annotations: map[string]string{
				constants.AnnotationAllowedActions:        "Publish",
				constants.AnnotationAllowedSourceRepos:    "*",
				constants.AnnotationAllowedPublishBuckets: "my-bucket-*",
			},
		},
	}

	client := fake.NewClientset(sa)
	engine := NewEngine(client)

	pkg := &zarfv1alpha1.ZarfPackageJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pkg",
			Namespace: "default",
		},
		Spec: zarfv1alpha1.ZarfPackageJobSpec{
			ServiceAccountName: "test-sa",
			Action:             zarfv1alpha1.ActionPublish,
			Source: zarfv1alpha1.PackageSource{
				Type: zarfv1alpha1.SourceTypeGit,
				Git: &zarfv1alpha1.GitSource{
					URL: "https://github.com/test/repo",
					Ref: "main",
				},
			},
			Publish: &zarfv1alpha1.PublishConfig{
				Destination: zarfv1alpha1.PublishDestination{
					Type: zarfv1alpha1.DestinationTypeS3,
					S3: &zarfv1alpha1.S3Destination{
						Bucket:    "my-bucket-prod",
						KeyPrefix: "packages/",
						Region:    "us-west-2",
					},
				},
			},
		},
	}

	err := engine.Validate(context.Background(), pkg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidate_PublishDestinationS3NotAllowed(t *testing.T) {
	sa := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-sa",
			Namespace: "default",
			Annotations: map[string]string{
				constants.AnnotationAllowedActions:        "Publish",
				constants.AnnotationAllowedSourceRepos:    "*",
				constants.AnnotationAllowedPublishBuckets: "my-bucket-*",
			},
		},
	}

	client := fake.NewClientset(sa)
	engine := NewEngine(client)

	pkg := &zarfv1alpha1.ZarfPackageJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pkg",
			Namespace: "default",
		},
		Spec: zarfv1alpha1.ZarfPackageJobSpec{
			ServiceAccountName: "test-sa",
			Action:             zarfv1alpha1.ActionPublish,
			Source: zarfv1alpha1.PackageSource{
				Type: zarfv1alpha1.SourceTypeGit,
				Git: &zarfv1alpha1.GitSource{
					URL: "https://github.com/test/repo",
					Ref: "main",
				},
			},
			Publish: &zarfv1alpha1.PublishConfig{
				Destination: zarfv1alpha1.PublishDestination{
					Type: zarfv1alpha1.DestinationTypeS3,
					S3: &zarfv1alpha1.S3Destination{
						Bucket: "other-bucket",
						Region: "us-west-2",
					},
				},
			},
		},
	}

	err := engine.Validate(context.Background(), pkg)
	if err == nil {
		t.Fatal("expected error for disallowed S3 bucket, got nil")
	}
	if !contains(err.Error(), "is not allowed") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestValidate_PublishDestinationOCI(t *testing.T) {
	sa := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-sa",
			Namespace: "default",
			Annotations: map[string]string{
				constants.AnnotationAllowedActions:           "Publish",
				constants.AnnotationAllowedSourceRepos:       "*",
				constants.AnnotationAllowedPublishRegistries: "ghcr.io/*",
			},
		},
	}

	client := fake.NewClientset(sa)
	engine := NewEngine(client)

	pkg := &zarfv1alpha1.ZarfPackageJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pkg",
			Namespace: "default",
		},
		Spec: zarfv1alpha1.ZarfPackageJobSpec{
			ServiceAccountName: "test-sa",
			Action:             zarfv1alpha1.ActionPublish,
			Source: zarfv1alpha1.PackageSource{
				Type: zarfv1alpha1.SourceTypeGit,
				Git: &zarfv1alpha1.GitSource{
					URL: "https://github.com/test/repo",
					Ref: "main",
				},
			},
			Publish: &zarfv1alpha1.PublishConfig{
				Destination: zarfv1alpha1.PublishDestination{
					Type: zarfv1alpha1.DestinationTypeOCI,
					OCI: &zarfv1alpha1.OCIDestination{
						Registry:   "ghcr.io",
						Repository: "test/packages",
						Tag:        "v1.0.0",
					},
				},
			},
		},
	}

	err := engine.Validate(context.Background(), pkg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidate_PublishDestinationOCINotAllowed(t *testing.T) {
	sa := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-sa",
			Namespace: "default",
			Annotations: map[string]string{
				constants.AnnotationAllowedActions:           "Publish",
				constants.AnnotationAllowedSourceRepos:       "*",
				constants.AnnotationAllowedPublishRegistries: "ghcr.io/*",
			},
		},
	}

	client := fake.NewClientset(sa)
	engine := NewEngine(client)

	pkg := &zarfv1alpha1.ZarfPackageJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pkg",
			Namespace: "default",
		},
		Spec: zarfv1alpha1.ZarfPackageJobSpec{
			ServiceAccountName: "test-sa",
			Action:             zarfv1alpha1.ActionPublish,
			Source: zarfv1alpha1.PackageSource{
				Type: zarfv1alpha1.SourceTypeGit,
				Git: &zarfv1alpha1.GitSource{
					URL: "https://github.com/test/repo",
					Ref: "main",
				},
			},
			Publish: &zarfv1alpha1.PublishConfig{
				Destination: zarfv1alpha1.PublishDestination{
					Type: zarfv1alpha1.DestinationTypeOCI,
					OCI: &zarfv1alpha1.OCIDestination{
						Registry:   "docker.io",
						Repository: "test/packages",
						Tag:        "v1.0.0",
					},
				},
			},
		},
	}

	err := engine.Validate(context.Background(), pkg)
	if err == nil {
		t.Fatal("expected error for disallowed OCI registry, got nil")
	}
	if !contains(err.Error(), "is not allowed") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestValidate_PublishDestinationOCIWithRepositoryPattern(t *testing.T) {
	sa := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-sa",
			Namespace: "default",
			Annotations: map[string]string{
				constants.AnnotationAllowedActions:           "Publish",
				constants.AnnotationAllowedSourceRepos:       "*",
				constants.AnnotationAllowedPublishRegistries: "ghcr.io/myorg/*",
			},
		},
	}

	client := fake.NewClientset(sa)
	engine := NewEngine(client)

	// Test allowed repository
	pkg := &zarfv1alpha1.ZarfPackageJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pkg",
			Namespace: "default",
		},
		Spec: zarfv1alpha1.ZarfPackageJobSpec{
			ServiceAccountName: "test-sa",
			Action:             zarfv1alpha1.ActionPublish,
			Source: zarfv1alpha1.PackageSource{
				Type: zarfv1alpha1.SourceTypeGit,
				Git: &zarfv1alpha1.GitSource{
					URL: "https://github.com/test/repo",
					Ref: "main",
				},
			},
			Publish: &zarfv1alpha1.PublishConfig{
				Destination: zarfv1alpha1.PublishDestination{
					Type: zarfv1alpha1.DestinationTypeOCI,
					OCI: &zarfv1alpha1.OCIDestination{
						Registry:   "ghcr.io",
						Repository: "myorg/packages",
						Tag:        "v1.0.0",
					},
				},
			},
		},
	}

	err := engine.Validate(context.Background(), pkg)
	if err != nil {
		t.Fatalf("unexpected error for allowed repository: %v", err)
	}

	// Test disallowed repository (different org)
	pkg2 := &zarfv1alpha1.ZarfPackageJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pkg-2",
			Namespace: "default",
		},
		Spec: zarfv1alpha1.ZarfPackageJobSpec{
			ServiceAccountName: "test-sa",
			Action:             zarfv1alpha1.ActionPublish,
			Source: zarfv1alpha1.PackageSource{
				Type: zarfv1alpha1.SourceTypeGit,
				Git: &zarfv1alpha1.GitSource{
					URL: "https://github.com/test/repo",
					Ref: "main",
				},
			},
			Publish: &zarfv1alpha1.PublishConfig{
				Destination: zarfv1alpha1.PublishDestination{
					Type: zarfv1alpha1.DestinationTypeOCI,
					OCI: &zarfv1alpha1.OCIDestination{
						Registry:   "ghcr.io",
						Repository: "otherorg/packages",
						Tag:        "v1.0.0",
					},
				},
			},
		},
	}

	err = engine.Validate(context.Background(), pkg2)
	if err == nil {
		t.Fatal("expected error for disallowed repository, got nil")
	}
	if !contains(err.Error(), "is not allowed") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestValidate_PublishDestinationLocal(t *testing.T) {
	sa := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-sa",
			Namespace: "default",
			Annotations: map[string]string{
				constants.AnnotationAllowedActions:     "Publish",
				constants.AnnotationAllowedSourceRepos: "*",
				constants.AnnotationAllowLocalSources:  "true",
			},
		},
	}

	client := fake.NewClientset(sa)
	engine := NewEngine(client)

	pkg := &zarfv1alpha1.ZarfPackageJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pkg",
			Namespace: "default",
		},
		Spec: zarfv1alpha1.ZarfPackageJobSpec{
			ServiceAccountName: "test-sa",
			Action:             zarfv1alpha1.ActionPublish,
			Source: zarfv1alpha1.PackageSource{
				Type: zarfv1alpha1.SourceTypeGit,
				Git: &zarfv1alpha1.GitSource{
					URL: "https://github.com/test/repo",
					Ref: "main",
				},
			},
			Publish: &zarfv1alpha1.PublishConfig{
				Destination: zarfv1alpha1.PublishDestination{
					Type: zarfv1alpha1.DestinationTypeLocal,
					Local: &zarfv1alpha1.LocalDestination{
						Path:    "/tmp/output",
						DevMode: true,
					},
				},
			},
		},
	}

	err := engine.Validate(context.Background(), pkg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidate_PublishDestinationLocalNotAllowed(t *testing.T) {
	sa := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-sa",
			Namespace: "default",
			Annotations: map[string]string{
				constants.AnnotationAllowedActions:     "Publish",
				constants.AnnotationAllowedSourceRepos: "*",
			},
		},
	}

	client := fake.NewClientset(sa)
	engine := NewEngine(client)

	pkg := &zarfv1alpha1.ZarfPackageJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pkg",
			Namespace: "default",
		},
		Spec: zarfv1alpha1.ZarfPackageJobSpec{
			ServiceAccountName: "test-sa",
			Action:             zarfv1alpha1.ActionPublish,
			Source: zarfv1alpha1.PackageSource{
				Type: zarfv1alpha1.SourceTypeGit,
				Git: &zarfv1alpha1.GitSource{
					URL: "https://github.com/test/repo",
					Ref: "main",
				},
			},
			Publish: &zarfv1alpha1.PublishConfig{
				Destination: zarfv1alpha1.PublishDestination{
					Type: zarfv1alpha1.DestinationTypeLocal,
					Local: &zarfv1alpha1.LocalDestination{
						Path: "/tmp/output",
					},
				},
			},
		},
	}

	err := engine.Validate(context.Background(), pkg)
	if err == nil {
		t.Fatal("expected error for disallowed local destination, got nil")
	}
	if !contains(err.Error(), "not allowed") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestValidate_OCISource(t *testing.T) {
	sa := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-sa",
			Namespace: "default",
			Annotations: map[string]string{
				constants.AnnotationAllowedActions:          "Deploy",
				constants.AnnotationAllowedSourceRegistries: "ghcr.io/*",
				constants.AnnotationAllowedDeployTargets:    "InCluster",
			},
		},
	}

	client := fake.NewClientset(sa)
	engine := NewEngine(client)

	pkg := &zarfv1alpha1.ZarfPackageJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pkg",
			Namespace: "default",
		},
		Spec: zarfv1alpha1.ZarfPackageJobSpec{
			ServiceAccountName: "test-sa",
			Action:             zarfv1alpha1.ActionDeploy,
			Source: zarfv1alpha1.PackageSource{
				Type: zarfv1alpha1.SourceTypeOCI,
				OCI: &zarfv1alpha1.OCISource{
					Image: "ghcr.io/test/package:v1.0.0",
				},
			},
			Deploy: &zarfv1alpha1.DeployConfig{
				Target:    zarfv1alpha1.DeployTargetInCluster,
				Namespace: "default",
			},
		},
	}

	err := engine.Validate(context.Background(), pkg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidate_OCISourceNotAllowed(t *testing.T) {
	sa := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-sa",
			Namespace: "default",
			Annotations: map[string]string{
				constants.AnnotationAllowedActions:          "Deploy",
				constants.AnnotationAllowedSourceRegistries: "ghcr.io/*",
				constants.AnnotationAllowedDeployTargets:    "InCluster",
			},
		},
	}

	client := fake.NewClientset(sa)
	engine := NewEngine(client)

	pkg := &zarfv1alpha1.ZarfPackageJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pkg",
			Namespace: "default",
		},
		Spec: zarfv1alpha1.ZarfPackageJobSpec{
			ServiceAccountName: "test-sa",
			Action:             zarfv1alpha1.ActionDeploy,
			Source: zarfv1alpha1.PackageSource{
				Type: zarfv1alpha1.SourceTypeOCI,
				OCI: &zarfv1alpha1.OCISource{
					Image: "docker.io/test/package:v1.0.0",
				},
			},
			Deploy: &zarfv1alpha1.DeployConfig{
				Target:    zarfv1alpha1.DeployTargetInCluster,
				Namespace: "default",
			},
		},
	}

	err := engine.Validate(context.Background(), pkg)
	if err == nil {
		t.Fatal("expected error for disallowed OCI registry, got nil")
	}
	if !contains(err.Error(), "is not allowed") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestValidate_DeployTargetAllowed(t *testing.T) {
	sa := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-sa",
			Namespace: "default",
			Annotations: map[string]string{
				constants.AnnotationAllowedActions:       "Deploy",
				constants.AnnotationAllowedSourceRepos:   "*",
				constants.AnnotationAllowedDeployTargets: "InCluster",
			},
		},
	}

	client := fake.NewClientset(sa)
	engine := NewEngine(client)

	pkg := &zarfv1alpha1.ZarfPackageJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pkg",
			Namespace: "default",
		},
		Spec: zarfv1alpha1.ZarfPackageJobSpec{
			ServiceAccountName: "test-sa",
			Action:             zarfv1alpha1.ActionDeploy,
			Source: zarfv1alpha1.PackageSource{
				Type: zarfv1alpha1.SourceTypeGit,
				Git: &zarfv1alpha1.GitSource{
					URL: "https://github.com/test/repo",
					Ref: "main",
				},
			},
			Deploy: &zarfv1alpha1.DeployConfig{
				Target:    zarfv1alpha1.DeployTargetInCluster,
				Namespace: "default",
			},
		},
	}

	err := engine.Validate(context.Background(), pkg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidate_DeployTargetNotAllowed(t *testing.T) {
	sa := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-sa",
			Namespace: "default",
			Annotations: map[string]string{
				constants.AnnotationAllowedActions:       "Deploy",
				constants.AnnotationAllowedSourceRepos:   "*",
				constants.AnnotationAllowedDeployTargets: "InCluster",
			},
		},
	}

	client := fake.NewClientset(sa)
	engine := NewEngine(client)

	pkg := &zarfv1alpha1.ZarfPackageJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pkg",
			Namespace: "default",
		},
		Spec: zarfv1alpha1.ZarfPackageJobSpec{
			ServiceAccountName: "test-sa",
			Action:             zarfv1alpha1.ActionDeploy,
			Source: zarfv1alpha1.PackageSource{
				Type: zarfv1alpha1.SourceTypeGit,
				Git: &zarfv1alpha1.GitSource{
					URL: "https://github.com/test/repo",
					Ref: "main",
				},
			},
			Deploy: &zarfv1alpha1.DeployConfig{
				Target: zarfv1alpha1.DeployTargetExternalCluster,
				ExternalCluster: &zarfv1alpha1.ExternalClusterConfig{
					KubeconfigSecretRef: zarfv1alpha1.SecretReference{ // pragma: allowlist secret
						Name: "external-kubeconfig",
					},
				},
			},
		},
	}

	err := engine.Validate(context.Background(), pkg)
	if err == nil {
		t.Fatal("expected error for disallowed deploy target, got nil")
	}
	if !contains(err.Error(), "not allowed") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestMatchAny_InvalidPattern(t *testing.T) {
	// Test the error handling path in matchAny for invalid glob patterns
	patterns := []string{"[invalid"}
	value := "test-value"

	// This should not panic, just return false for the invalid pattern
	result := matchAny(patterns, value)
	if result {
		t.Error("matchAny should return false for invalid patterns")
	}
}

func contains(str, substr string) bool {
	return len(str) >= len(substr) && (str == substr || len(str) > len(substr) && containsRec(str, substr))
}

func containsRec(str, substr string) bool {
	for index := 0; index <= len(str)-len(substr); index++ {
		if str[index:index+len(substr)] == substr {
			return true
		}
	}
	return false
}

// UDS Bundle Validation Tests

func TestValidateUDSBundle_MissingServiceAccount(t *testing.T) {
	client := fake.NewClientset()
	engine := NewEngine(client)

	bundle := &udsv1alpha2.UDSBundleJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-bundle",
			Namespace: "default",
		},
		Spec: udsv1alpha2.UDSBundleJobSpec{
			ServiceAccountName: "",
			Action:             udsv1alpha2.ActionCreate,
		},
	}

	err := engine.ValidateUDSBundle(context.Background(), bundle)
	if err == nil {
		t.Fatal("expected error for missing serviceAccountName, got nil")
	}
	if err.Error() != "serviceAccountName is required" {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestValidateUDSBundle_ServiceAccountNotFound(t *testing.T) {
	client := fake.NewClientset()
	engine := NewEngine(client)

	bundle := &udsv1alpha2.UDSBundleJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-bundle",
			Namespace: "default",
		},
		Spec: udsv1alpha2.UDSBundleJobSpec{
			ServiceAccountName: "nonexistent-sa",
			Action:             udsv1alpha2.ActionCreate,
		},
	}

	err := engine.ValidateUDSBundle(context.Background(), bundle)
	if err == nil {
		t.Fatal("expected error for nonexistent service account, got nil")
	}
}

func TestValidateUDSBundle_ActionNotAllowed(t *testing.T) {
	sa := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-sa",
			Namespace: "default",
			Annotations: map[string]string{
				constants.AnnotationAllowedActions: "Create,Publish",
			},
		},
	}

	client := fake.NewClientset(sa)
	engine := NewEngine(client)

	bundle := &udsv1alpha2.UDSBundleJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-bundle",
			Namespace: "default",
		},
		Spec: udsv1alpha2.UDSBundleJobSpec{
			ServiceAccountName: "test-sa",
			Action:             udsv1alpha2.ActionDeploy,
			Source: udsv1alpha2.PackageSource{
				Type: udsv1alpha2.SourceTypeGit,
				Git: &udsv1alpha2.GitSource{
					URL: "https://github.com/test/repo",
					Ref: "main",
				},
			},
		},
	}

	err := engine.ValidateUDSBundle(context.Background(), bundle)
	if err == nil {
		t.Fatal("expected error for disallowed action, got nil")
	}
	if !contains(err.Error(), "action Deploy is not allowed") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestValidateUDSBundle_GitSourceAllowed(t *testing.T) {
	sa := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-sa",
			Namespace: "default",
			Annotations: map[string]string{
				constants.AnnotationAllowedActions:     "Create",
				constants.AnnotationAllowedSourceRepos: "https://github.com/test/*",
			},
		},
	}

	client := fake.NewClientset(sa)
	engine := NewEngine(client)

	bundle := &udsv1alpha2.UDSBundleJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-bundle",
			Namespace: "default",
		},
		Spec: udsv1alpha2.UDSBundleJobSpec{
			ServiceAccountName: "test-sa",
			Action:             udsv1alpha2.ActionCreate,
			Source: udsv1alpha2.PackageSource{
				Type: udsv1alpha2.SourceTypeGit,
				Git: &udsv1alpha2.GitSource{
					URL: "https://github.com/test/repo",
					Ref: "main",
				},
			},
		},
	}

	err := engine.ValidateUDSBundle(context.Background(), bundle)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateUDSBundle_GitSourceNotAllowed(t *testing.T) {
	sa := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-sa",
			Namespace: "default",
			Annotations: map[string]string{
				constants.AnnotationAllowedActions:     "Create",
				constants.AnnotationAllowedSourceRepos: "https://github.com/allowed/*",
			},
		},
	}

	client := fake.NewClientset(sa)
	engine := NewEngine(client)

	bundle := &udsv1alpha2.UDSBundleJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-bundle",
			Namespace: "default",
		},
		Spec: udsv1alpha2.UDSBundleJobSpec{
			ServiceAccountName: "test-sa",
			Action:             udsv1alpha2.ActionCreate,
			Source: udsv1alpha2.PackageSource{
				Type: udsv1alpha2.SourceTypeGit,
				Git: &udsv1alpha2.GitSource{
					URL: "https://github.com/forbidden/repo",
					Ref: "main",
				},
			},
		},
	}

	err := engine.ValidateUDSBundle(context.Background(), bundle)
	if err == nil {
		t.Fatal("expected error for disallowed git source, got nil")
	}
	if !contains(err.Error(), "is not allowed") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestValidateUDSBundle_S3SourceAllowed(t *testing.T) {
	sa := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-sa",
			Namespace: "default",
			Annotations: map[string]string{
				constants.AnnotationAllowedActions:       "Deploy",
				constants.AnnotationAllowedSourceBuckets: "allowed-bucket",
				constants.AnnotationAllowedDeployTargets: "InCluster",
			},
		},
	}

	client := fake.NewClientset(sa)
	engine := NewEngine(client)

	bundle := &udsv1alpha2.UDSBundleJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-bundle",
			Namespace: "default",
		},
		Spec: udsv1alpha2.UDSBundleJobSpec{
			ServiceAccountName: "test-sa",
			Action:             udsv1alpha2.ActionDeploy,
			Source: udsv1alpha2.PackageSource{
				Type: udsv1alpha2.SourceTypeS3,
				S3: &udsv1alpha2.S3Source{
					Bucket: "allowed-bucket",
					Key:    "bundles/test.tar.zst",
					Region: "us-east-1",
				},
			},
			Deploy: &udsv1alpha2.DeployConfig{
				Target:    udsv1alpha2.DeployTargetInCluster,
				Namespace: "default",
			},
		},
	}

	err := engine.ValidateUDSBundle(context.Background(), bundle)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateUDSBundle_OCISourceAllowed(t *testing.T) {
	sa := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-sa",
			Namespace: "default",
			Annotations: map[string]string{
				constants.AnnotationAllowedActions:          "Deploy",
				constants.AnnotationAllowedSourceRegistries: "ghcr.io/*",
				constants.AnnotationAllowedDeployTargets:    "InCluster",
			},
		},
	}

	client := fake.NewClientset(sa)
	engine := NewEngine(client)

	bundle := &udsv1alpha2.UDSBundleJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-bundle",
			Namespace: "default",
		},
		Spec: udsv1alpha2.UDSBundleJobSpec{
			ServiceAccountName: "test-sa",
			Action:             udsv1alpha2.ActionDeploy,
			Source: udsv1alpha2.PackageSource{
				Type: udsv1alpha2.SourceTypeOCI,
				OCI: &udsv1alpha2.OCISource{
					Reference: "ghcr.io/test/bundle:v1.0.0",
				},
			},
			Deploy: &udsv1alpha2.DeployConfig{
				Target:    udsv1alpha2.DeployTargetInCluster,
				Namespace: "default",
			},
		},
	}

	err := engine.ValidateUDSBundle(context.Background(), bundle)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateUDSBundle_PublishDestinationOCIAllowed(t *testing.T) {
	sa := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-sa",
			Namespace: "default",
			Annotations: map[string]string{
				constants.AnnotationAllowedActions:           "Publish",
				constants.AnnotationAllowedSourceRepos:       "*",
				constants.AnnotationAllowedPublishRegistries: "ghcr.io/*",
			},
		},
	}

	client := fake.NewClientset(sa)
	engine := NewEngine(client)

	bundle := &udsv1alpha2.UDSBundleJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-bundle",
			Namespace: "default",
		},
		Spec: udsv1alpha2.UDSBundleJobSpec{
			ServiceAccountName: "test-sa",
			Action:             udsv1alpha2.ActionPublish,
			Source: udsv1alpha2.PackageSource{
				Type: udsv1alpha2.SourceTypeGit,
				Git: &udsv1alpha2.GitSource{
					URL: "https://github.com/test/repo",
					Ref: "main",
				},
			},
			Publish: &udsv1alpha2.PublishConfig{
				Destination: udsv1alpha2.PublishDestination{
					Type: udsv1alpha2.DestinationTypeOCI,
					OCI: &udsv1alpha2.OCIDestination{
						Registry:   "ghcr.io",
						Repository: "test/bundles",
						Tag:        "v1.0.0",
					},
				},
			},
		},
	}

	err := engine.ValidateUDSBundle(context.Background(), bundle)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateUDSBundle_PublishDestinationS3Allowed(t *testing.T) {
	sa := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-sa",
			Namespace: "default",
			Annotations: map[string]string{
				constants.AnnotationAllowedActions:        "Publish",
				constants.AnnotationAllowedSourceRepos:    "*",
				constants.AnnotationAllowedPublishBuckets: "publish-bucket",
			},
		},
	}

	client := fake.NewClientset(sa)
	engine := NewEngine(client)

	bundle := &udsv1alpha2.UDSBundleJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-bundle",
			Namespace: "default",
		},
		Spec: udsv1alpha2.UDSBundleJobSpec{
			ServiceAccountName: "test-sa",
			Action:             udsv1alpha2.ActionPublish,
			Source: udsv1alpha2.PackageSource{
				Type: udsv1alpha2.SourceTypeGit,
				Git: &udsv1alpha2.GitSource{
					URL: "https://github.com/test/repo",
					Ref: "main",
				},
			},
			Publish: &udsv1alpha2.PublishConfig{
				Destination: udsv1alpha2.PublishDestination{
					Type: udsv1alpha2.DestinationTypeS3,
					S3: &udsv1alpha2.S3Destination{
						Bucket: "publish-bucket",
						Key:    "bundles/",
						Region: "us-east-1",
					},
				},
			},
		},
	}

	err := engine.ValidateUDSBundle(context.Background(), bundle)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateUDSBundle_DeployTargetAllowed(t *testing.T) {
	sa := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-sa",
			Namespace: "default",
			Annotations: map[string]string{
				constants.AnnotationAllowedActions:       "Deploy",
				constants.AnnotationAllowedSourceRepos:   "*",
				constants.AnnotationAllowedDeployTargets: "InCluster",
			},
		},
	}

	client := fake.NewClientset(sa)
	engine := NewEngine(client)

	bundle := &udsv1alpha2.UDSBundleJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-bundle",
			Namespace: "default",
		},
		Spec: udsv1alpha2.UDSBundleJobSpec{
			ServiceAccountName: "test-sa",
			Action:             udsv1alpha2.ActionDeploy,
			Source: udsv1alpha2.PackageSource{
				Type: udsv1alpha2.SourceTypeGit,
				Git: &udsv1alpha2.GitSource{
					URL: "https://github.com/test/repo",
					Ref: "main",
				},
			},
			Deploy: &udsv1alpha2.DeployConfig{
				Target:    udsv1alpha2.DeployTargetInCluster,
				Namespace: "default",
			},
		},
	}

	err := engine.ValidateUDSBundle(context.Background(), bundle)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateUDSBundle_DeployTargetNotAllowed(t *testing.T) {
	sa := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-sa",
			Namespace: "default",
			Annotations: map[string]string{
				constants.AnnotationAllowedActions:       "Deploy",
				constants.AnnotationAllowedSourceRepos:   "*",
				constants.AnnotationAllowedDeployTargets: "InCluster",
			},
		},
	}

	client := fake.NewClientset(sa)
	engine := NewEngine(client)

	bundle := &udsv1alpha2.UDSBundleJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-bundle",
			Namespace: "default",
		},
		Spec: udsv1alpha2.UDSBundleJobSpec{
			ServiceAccountName: "test-sa",
			Action:             udsv1alpha2.ActionDeploy,
			Source: udsv1alpha2.PackageSource{
				Type: udsv1alpha2.SourceTypeGit,
				Git: &udsv1alpha2.GitSource{
					URL: "https://github.com/test/repo",
					Ref: "main",
				},
			},
			Deploy: &udsv1alpha2.DeployConfig{
				Target: udsv1alpha2.DeployTargetExternalCluster,
			},
		},
	}

	err := engine.ValidateUDSBundle(context.Background(), bundle)
	if err == nil {
		t.Fatal("expected error for disallowed deploy target, got nil")
	}
	if !contains(err.Error(), "not allowed") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestValidateUDSBundle_CompleteWorkflow(t *testing.T) {
	sa := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-sa",
			Namespace: "default",
			Annotations: map[string]string{
				constants.AnnotationAllowedActions:           "CreatePublishDeploy",
				constants.AnnotationAllowedSourceRepos:       "https://github.com/test/*",
				constants.AnnotationAllowedPublishRegistries: "ghcr.io/test/*",
				constants.AnnotationAllowedDeployTargets:     "InCluster",
			},
		},
	}

	client := fake.NewClientset(sa)
	engine := NewEngine(client)

	bundle := &udsv1alpha2.UDSBundleJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-bundle",
			Namespace: "default",
		},
		Spec: udsv1alpha2.UDSBundleJobSpec{
			ServiceAccountName: "test-sa",
			Action:             udsv1alpha2.ActionCreatePublishDeploy,
			Source: udsv1alpha2.PackageSource{
				Type: udsv1alpha2.SourceTypeGit,
				Git: &udsv1alpha2.GitSource{
					URL: "https://github.com/test/repo",
					Ref: "main",
				},
			},
			Publish: &udsv1alpha2.PublishConfig{
				Destination: udsv1alpha2.PublishDestination{
					Type: udsv1alpha2.DestinationTypeOCI,
					OCI: &udsv1alpha2.OCIDestination{
						Registry:   "ghcr.io",
						Repository: "test/bundles",
						Tag:        "v1.0.0",
					},
				},
			},
			Deploy: &udsv1alpha2.DeployConfig{
				Target:    udsv1alpha2.DeployTargetInCluster,
				Namespace: "default",
			},
		},
	}

	err := engine.ValidateUDSBundle(context.Background(), bundle)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateUDSBundle_LocalSourceAllowed(t *testing.T) {
	sa := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-sa",
			Namespace: "default",
			Annotations: map[string]string{
				constants.AnnotationAllowedActions:    "Create",
				constants.AnnotationAllowLocalSources: "true",
			},
		},
	}

	client := fake.NewClientset(sa)
	engine := NewEngine(client)

	bundle := &udsv1alpha2.UDSBundleJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-bundle",
			Namespace: "default",
		},
		Spec: udsv1alpha2.UDSBundleJobSpec{
			ServiceAccountName: "test-sa",
			Action:             udsv1alpha2.ActionCreate,
			Source: udsv1alpha2.PackageSource{
				Type: udsv1alpha2.SourceTypeLocal,
				Local: &udsv1alpha2.LocalSource{
					Path: "/tmp/bundle",
				},
			},
		},
	}

	err := engine.ValidateUDSBundle(context.Background(), bundle)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateUDSBundle_LocalSourceNotAllowed(t *testing.T) {
	sa := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-sa",
			Namespace: "default",
			Annotations: map[string]string{
				constants.AnnotationAllowedActions: "Create",
			},
		},
	}

	client := fake.NewClientset(sa)
	engine := NewEngine(client)

	bundle := &udsv1alpha2.UDSBundleJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-bundle",
			Namespace: "default",
		},
		Spec: udsv1alpha2.UDSBundleJobSpec{
			ServiceAccountName: "test-sa",
			Action:             udsv1alpha2.ActionCreate,
			Source: udsv1alpha2.PackageSource{
				Type: udsv1alpha2.SourceTypeLocal,
				Local: &udsv1alpha2.LocalSource{
					Path: "/tmp/bundle",
				},
			},
		},
	}

	err := engine.ValidateUDSBundle(context.Background(), bundle)
	if err == nil {
		t.Fatal("expected error for disallowed local source, got nil")
	}
	if !contains(err.Error(), "not allowed") {
		t.Errorf("unexpected error message: %v", err)
	}
}
