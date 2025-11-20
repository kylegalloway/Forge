package policy

import (
	"context"
	"testing"

	zarfv1alpha1 "github.com/kylegalloway/forge/pkg/apis/zarf/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestValidate_MissingServiceAccount(t *testing.T) {
	client := fake.NewSimpleClientset()
	engine := NewEngine(client)

	pkg := &zarfv1alpha1.ZarfPackage{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pkg",
			Namespace: "default",
		},
		Spec: zarfv1alpha1.ZarfPackageSpec{
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
	client := fake.NewSimpleClientset()
	engine := NewEngine(client)

	pkg := &zarfv1alpha1.ZarfPackage{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pkg",
			Namespace: "default",
		},
		Spec: zarfv1alpha1.ZarfPackageSpec{
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
				AnnotationAllowedActions: "Build,Publish",
			},
		},
	}

	client := fake.NewSimpleClientset(sa)
	engine := NewEngine(client)

	pkg := &zarfv1alpha1.ZarfPackage{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pkg",
			Namespace: "default",
		},
		Spec: zarfv1alpha1.ZarfPackageSpec{
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
				AnnotationAllowedActions:     "Build",
				AnnotationAllowedSourceRepos: "https://github.com/myorg/*",
			},
		},
	}

	client := fake.NewSimpleClientset(sa)
	engine := NewEngine(client)

	pkg := &zarfv1alpha1.ZarfPackage{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pkg",
			Namespace: "default",
		},
		Spec: zarfv1alpha1.ZarfPackageSpec{
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
				AnnotationAllowedActions:     "Build",
				AnnotationAllowedSourceRepos: "https://github.com/myorg/*",
			},
		},
	}

	client := fake.NewSimpleClientset(sa)
	engine := NewEngine(client)

	pkg := &zarfv1alpha1.ZarfPackage{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pkg",
			Namespace: "default",
		},
		Spec: zarfv1alpha1.ZarfPackageSpec{
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
				AnnotationAllowedActions:     "*",
				AnnotationAllowedSourceRepos: "*",
			},
		},
	}

	client := fake.NewSimpleClientset(sa)
	engine := NewEngine(client)

	pkg := &zarfv1alpha1.ZarfPackage{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pkg",
			Namespace: "default",
		},
		Spec: zarfv1alpha1.ZarfPackageSpec{
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
				AnnotationAllowedActions:       "Build",
				AnnotationAllowedSourceBuckets: "my-bucket-*",
			},
		},
	}

	client := fake.NewSimpleClientset(sa)
	engine := NewEngine(client)

	pkg := &zarfv1alpha1.ZarfPackage{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pkg",
			Namespace: "default",
		},
		Spec: zarfv1alpha1.ZarfPackageSpec{
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
				AnnotationAllowedActions: "Build",
			},
		},
	}

	client := fake.NewSimpleClientset(sa)
	engine := NewEngine(client)

	pkg := &zarfv1alpha1.ZarfPackage{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pkg",
			Namespace: "default",
		},
		Spec: zarfv1alpha1.ZarfPackageSpec{
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
				AnnotationAllowedActions:    "Build",
				AnnotationAllowLocalSources: "true",
			},
		},
	}

	client := fake.NewSimpleClientset(sa)
	engine := NewEngine(client)

	pkg := &zarfv1alpha1.ZarfPackage{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pkg",
			Namespace: "default",
		},
		Spec: zarfv1alpha1.ZarfPackageSpec{
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
			for i := range result {
				if result[i] != tt.expected[i] {
					t.Errorf("item %d: expected %s, got %s", i, tt.expected[i], result[i])
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

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && containsRec(s, substr))
}

func containsRec(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
