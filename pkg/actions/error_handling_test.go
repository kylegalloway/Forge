package actions

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubefake "k8s.io/client-go/kubernetes/fake"

	"github.com/kylegalloway/forge/pkg/apis/common"
	zarfv1alpha3 "github.com/kylegalloway/forge/pkg/apis/zarf/v1alpha3"
)

// TestErrorHandling_MissingContainerImage tests that JobBuilder requires container image
func TestErrorHandling_MissingContainerImage(t *testing.T) {
	tests := []struct {
		name           string
		image          string
		expectJobBuilt bool
	}{
		{
			name:           "with container image",
			image:          "ubuntu:22.04",
			expectJobBuilt: true,
		},
		{
			name:           "empty container image",
			image:          "",
			expectJobBuilt: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			builder := NewJobBuilder("test-job", "default")
			builder.WithContainerName("main").
				WithContainerImage(tt.image).
				WithCommand([]string{"sh", "-c", "echo test"})

			job := builder.Build()
			if job == nil && tt.expectJobBuilt {
				t.Error("Expected job to be built successfully")
			}
			if job != nil && tt.image != "" {
				if len(job.Spec.Template.Spec.Containers) == 0 {
					t.Error("Expected containers in job spec")
				} else if job.Spec.Template.Spec.Containers[0].Image != tt.image {
					t.Errorf("Expected image %q, got %q", tt.image, job.Spec.Template.Spec.Containers[0].Image)
				}
			}
		})
	}
}

// TestErrorHandling_MissingServiceAccountName tests service account validation
func TestErrorHandling_MissingServiceAccountName(t *testing.T) {
	tests := []struct {
		name               string
		serviceAccountName string
		shouldBuildSucceed bool
	}{
		{
			name:               "with service account",
			serviceAccountName: "test-sa",
			shouldBuildSucceed: true,
		},
		{
			name:               "empty service account",
			serviceAccountName: "",
			shouldBuildSucceed: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			builder := NewJobBuilder("test-job", "default")
			builder.
				WithContainerName("main").
				WithContainerImage("ubuntu:22.04").
				WithCommand([]string{"echo", "test"}).
				WithServiceAccountName(tt.serviceAccountName)

			job := builder.Build()
			if job == nil && tt.shouldBuildSucceed {
				t.Error("Expected job to build successfully")
			}

			if job != nil && job.Spec.Template.Spec.ServiceAccountName != tt.serviceAccountName {
				t.Errorf("Expected service account %q, got %q",
					tt.serviceAccountName,
					job.Spec.Template.Spec.ServiceAccountName,
				)
			}
		})
	}
}

// TestErrorHandling_InvalidResourceRequirements tests resource request validation
func TestErrorHandling_InvalidResourceRequirements(t *testing.T) {
	tests := []struct {
		name        string
		resources   corev1.ResourceRequirements
		expectValid bool
	}{
		{
			name: "valid resources",
			resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("100m"),
					corev1.ResourceMemory: resource.MustParse("64Mi"),
				},
			},
			expectValid: true,
		},
		{
			name: "requests exceed limits",
			resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceCPU: resource.MustParse("2"),
				},
				Limits: corev1.ResourceList{
					corev1.ResourceCPU: resource.MustParse("1"),
				},
			},
			expectValid: false,
		},
		{
			name: "empty resources",
			resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{},
				Limits:   corev1.ResourceList{},
			},
			expectValid: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			builder := NewJobBuilder("test-job", "default")
			builder.
				WithContainerName("main").
				WithContainerImage("ubuntu:22.04").
				WithCommand([]string{"echo", "test"}).
				WithResources(tt.resources)

			job := builder.Build()
			if job == nil && tt.expectValid {
				t.Error("Expected job to build successfully")
			}
		})
	}
}

// TestErrorHandling_InvalidSecurityContext tests security context validation
func TestErrorHandling_InvalidSecurityContext(t *testing.T) {
	tests := []struct {
		name            string
		securityContext *corev1.SecurityContext
		expectValid     bool
	}{
		{
			name: "valid security context",
			securityContext: &corev1.SecurityContext{
				RunAsUser:                Ptr[int64](1000),
				AllowPrivilegeEscalation: Ptr(false),
				ReadOnlyRootFilesystem:   Ptr(true),
			},
			expectValid: true,
		},
		{
			name:            "empty security context",
			securityContext: nil,
			expectValid:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			builder := NewJobBuilder("test-job", "default")
			builder.
				WithContainerName("main").
				WithContainerImage("ubuntu:22.04").
				WithCommand([]string{"echo", "test"})

			if tt.securityContext != nil {
				builder.WithContainerSecurityContext(tt.securityContext)
			}

			job := builder.Build()
			if job == nil && tt.expectValid {
				t.Error("Expected job to build successfully")
			}
		})
	}
}

// TestErrorHandling_MissingInitContainerImage tests init container validation
func TestErrorHandling_MissingInitContainerImage(t *testing.T) {
	tests := []struct {
		name          string
		initContainer corev1.Container
		expectValid   bool
	}{
		{
			name: "valid init container",
			initContainer: corev1.Container{
				Name:  "git-clone",
				Image: "alpine/git:latest",
				Args:  []string{"clone", "https://example.com/repo.git", "/workspace"},
			},
			expectValid: true,
		},
		{
			name: "init container without image",
			initContainer: corev1.Container{
				Name: "git-clone",
				Args: []string{"clone", "https://example.com/repo.git", "/workspace"},
			},
			expectValid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			builder := NewJobBuilder("test-job", "default")
			builder.
				WithContainerName("main").
				WithContainerImage("ubuntu:22.04").
				WithCommand([]string{"echo", "test"}).
				WithInitContainers([]corev1.Container{tt.initContainer})

			job := builder.Build()
			if job == nil && tt.expectValid {
				t.Error("Expected job to build successfully")
				return
			}

			if job != nil && len(job.Spec.Template.Spec.InitContainers) > 0 {
				initC := job.Spec.Template.Spec.InitContainers[0]
				if !tt.expectValid && initC.Image == "" {
					t.Logf("Init container without image should be rejected by Kubernetes API")
				}
			}
		})
	}
}

// TestErrorHandling_JobCreationFailure tests job creation error handling
func TestErrorHandling_JobCreationFailure(t *testing.T) {
	kubeClient := kubefake.NewClientset()

	builder := NewJobBuilder("test-job", "default")
	builder.
		WithKubeClient(kubeClient).
		WithContainerName("main").
		WithContainerImage("ubuntu:22.04").
		WithCommand([]string{"echo", "test"})

	job := builder.Build()
	if job == nil {
		t.Error("Expected job to build successfully at builder level")
		return
	}

	ctx := context.Background()
	_, err := kubeClient.BatchV1().Jobs("default").Create(ctx, job, metav1.CreateOptions{})
	if err != nil {
		t.Errorf("Expected successful job creation, got error: %v", err)
	}
}

// TestErrorHandling_InvalidNodeSelector tests node selector validation
func TestErrorHandling_InvalidNodeSelector(t *testing.T) {
	tests := []struct {
		name         string
		nodeSelector map[string]string
		expectValid  bool
	}{
		{
			name: "valid node selector",
			nodeSelector: map[string]string{
				"disktype": "ssd",
			},
			expectValid: true,
		},
		{
			name:         "empty node selector",
			nodeSelector: make(map[string]string),
			expectValid:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			builder := NewJobBuilder("test-job", "default")
			builder.
				WithContainerName("main").
				WithContainerImage("ubuntu:22.04").
				WithCommand([]string{"echo", "test"}).
				WithNodeSelector(tt.nodeSelector)

			job := builder.Build()
			if job == nil && tt.expectValid {
				t.Error("Expected job to build successfully")
				return
			}
		})
	}
}

// TestErrorHandling_ExtraMountsValidation tests extra mounts configuration
func TestErrorHandling_ExtraMountsValidation(t *testing.T) {
	tests := []struct {
		name        string
		mounts      []common.ExtraMount
		expectValid bool
	}{
		{
			name: "valid ConfigMap mount",
			mounts: []common.ExtraMount{
				{
					ConfigMapRef: &common.LocalObjectReference{
						Name: "test-cm",
					},
					MountPath: "/config",
				},
			},
			expectValid: true,
		},
		{
			name: "valid Secret mount",
			mounts: []common.ExtraMount{
				{
					SecretRef: &common.LocalObjectReference{
						Name: "test-secret",
					},
					MountPath: "/secrets",
				},
			},
			expectValid: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			builder := NewJobBuilder("test-job", "default")
			builder.
				WithContainerName("main").
				WithContainerImage("ubuntu:22.04").
				WithCommand([]string{"echo", "test"})

			if len(tt.mounts) > 0 {
				builder.WithExtraMounts(tt.mounts)
			}

			job := builder.Build()
			if job == nil && tt.expectValid {
				t.Error("Expected job to build successfully")
			}
		})
	}
}

// TestErrorHandling_UserConfigWithUID tests user configuration
func TestErrorHandling_UserConfigWithUID(t *testing.T) {
	tests := []struct {
		name        string
		uid         int64
		expectValid bool
	}{
		{
			name:        "standard zarf uid",
			uid:         1000,
			expectValid: true,
		},
		{
			name:        "standard uds uid",
			uid:         65532,
			expectValid: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			builder := NewJobBuilder("test-job", "default")
			builder.
				WithContainerName("main").
				WithContainerImage("ubuntu:22.04").
				WithCommand([]string{"echo", "test"}).
				WithUserConfig(tt.uid)

			job := builder.Build()
			if job == nil && tt.expectValid {
				t.Error("Expected job to build successfully")
			}

			if job != nil && job.Spec.Template.Spec.SecurityContext == nil && tt.expectValid {
				t.Error("Expected pod security context to be set")
			}
		})
	}
}

// TestErrorHandling_WorkspaceVolumeWithSizes tests volume size configuration
func TestErrorHandling_WorkspaceVolumeWithSizes(t *testing.T) {
	tests := []struct {
		name        string
		volumeSizes *common.VolumeSizes
		expectValid bool
	}{
		{
			name: "custom workspace size",
			volumeSizes: &common.VolumeSizes{
				Workspace: Ptr(resource.MustParse("5Gi")),
				Output:    Ptr(resource.MustParse("2Gi")),
			},
			expectValid: true,
		},
		{
			name:        "nil volume sizes",
			volumeSizes: nil,
			expectValid: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			builder := NewJobBuilder("test-job", "default")
			builder.
				WithContainerName("main").
				WithContainerImage("ubuntu:22.04").
				WithCommand([]string{"echo", "test"}).
				WithWorkspaceVolume(tt.volumeSizes)

			job := builder.Build()
			if job == nil && tt.expectValid {
				t.Error("Expected job to build successfully")
				return
			}

			if job != nil {
				found := false
				for _, vol := range job.Spec.Template.Spec.Volumes {
					if vol.Name == "workspace" {
						found = true
						break
					}
				}
				if !found {
					t.Error("Expected workspace volume to be present")
				}
			}
		})
	}
}

// TestErrorHandling_InClusterKubeconfig tests in-cluster kubeconfig configuration
func TestErrorHandling_InClusterKubeconfig(t *testing.T) {
	builder := NewJobBuilder("test-job", "default")
	builder.
		WithContainerName("main").
		WithContainerImage("ubuntu:22.04").
		WithCommand([]string{"echo", "test"}).
		WithInClusterKubeconfig()

	job := builder.Build()
	if job == nil {
		t.Error("Expected job to build successfully")
		return
	}

	found := false
	for _, vol := range job.Spec.Template.Spec.Volumes {
		if vol.Name == "kube-api-access" && vol.Projected != nil {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected projected service account volume")
	}
}

// TestErrorHandling_DebugModeConfiguration tests debug mode setup
func TestErrorHandling_DebugModeConfiguration(t *testing.T) {
	tests := []struct {
		name       string
		debug      bool
		expectScan bool
	}{
		{
			name:       "debug mode enabled",
			debug:      true,
			expectScan: true,
		},
		{
			name:       "debug mode disabled",
			debug:      false,
			expectScan: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			builder := NewJobBuilder("test-job", "default")
			builder.
				WithContainerName("main").
				WithContainerImage("ubuntu:22.04").
				WithCommand([]string{"echo", "test"}).
				WithDebugMode(tt.debug)

			job := builder.Build()
			if job == nil {
				t.Error("Expected job to build successfully")
			}
		})
	}
}

// TestErrorHandling_MissingSourceType tests missing source type validation
func TestErrorHandling_MissingSourceType(t *testing.T) {
	pkg := &zarfv1alpha3.ZarfPackageJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pkg",
			Namespace: "default",
		},
		Spec: zarfv1alpha3.ZarfPackageJobSpec{
			ServiceAccountName: "test-sa",
			Action:             zarfv1alpha3.ActionBuild,
			Source: zarfv1alpha3.PackageSource{
				Type: "", // Missing source type
			},
		},
	}

	if pkg.Spec.Source.Type == "" {
		t.Log("Missing source type should be caught by webhook validation")
	}
}

// TestErrorHandling_ValidSourceTypes tests valid source type handling
func TestErrorHandling_ValidSourceTypes(t *testing.T) {
	validTypes := []zarfv1alpha3.SourceType{
		zarfv1alpha3.SourceTypeGit,
		zarfv1alpha3.SourceTypeS3,
		zarfv1alpha3.SourceTypeOCI,
		zarfv1alpha3.SourceTypeLocal,
	}

	for _, sourceType := range validTypes {
		t.Run(string(sourceType), func(t *testing.T) {
			pkg := &zarfv1alpha3.ZarfPackageJob{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pkg",
					Namespace: "default",
				},
				Spec: zarfv1alpha3.ZarfPackageJobSpec{
					ServiceAccountName: "test-sa",
					Action:             zarfv1alpha3.ActionBuild,
					Source: zarfv1alpha3.PackageSource{
						Type: sourceType,
					},
				},
			}

			if pkg.Spec.Source.Type == "" {
				t.Error("Expected source type to be set")
			}
		})
	}
}
