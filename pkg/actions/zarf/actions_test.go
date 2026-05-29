package zarf

import (
	"context"
	"strings"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/dynamic/fake"
	kubefake "k8s.io/client-go/kubernetes/fake"

	actionscommon "github.com/kylegalloway/forge/pkg/actions/common"
	apiscommon "github.com/kylegalloway/forge/pkg/apis/common"
	zarfv1alpha3 "github.com/kylegalloway/forge/pkg/apis/zarf/v1alpha3"
	"github.com/kylegalloway/forge/pkg/constants"
	testhelpers "github.com/kylegalloway/forge/pkg/controller/testing"
	"github.com/kylegalloway/forge/pkg/telemetry"
)

func TestNewBuildHandler(t *testing.T) {
	kubeClient := kubefake.NewClientset()
	metrics := testhelpers.MustNewMetrics()
	tracer := telemetry.NewTracer()

	handler := NewBuildHandler(kubeClient, metrics, tracer)
	if handler == nil {
		t.Fatal("NewBuildHandler returned nil")
	}
	if handler.kubeClient == nil {
		t.Error("kubeClient not set")
	}
	if handler.metrics == nil {
		t.Error("metrics not set")
	}
	if handler.tracer == nil {
		t.Error("tracer not set")
	}
}

func TestNewPublishHandler(t *testing.T) {
	kubeClient := kubefake.NewClientset()
	metrics := testhelpers.MustNewMetrics()
	tracer := telemetry.NewTracer()

	handler := NewPublishHandler(kubeClient, metrics, tracer)
	if handler == nil {
		t.Fatal("NewPublishHandler returned nil")
	}
	if handler.kubeClient == nil {
		t.Error("kubeClient not set")
	}
}

func TestNewDeployHandler(t *testing.T) {
	kubeClient := kubefake.NewClientset()
	dynamicClient := fake.NewSimpleDynamicClient(runtime.NewScheme())
	metrics := testhelpers.MustNewMetrics()
	tracer := telemetry.NewTracer()

	handler := NewDeployHandler(kubeClient, dynamicClient, metrics, tracer)
	if handler == nil {
		t.Fatal("NewDeployHandler returned nil")
	}
	if handler.kubeClient == nil {
		t.Error("kubeClient not set")
	}
	if handler.dynamicClient == nil {
		t.Error("dynamicClient not set")
	}
}

func TestBuildHandlerExecute(t *testing.T) {
	kubeClient := kubefake.NewClientset()
	handler := NewBuildHandler(kubeClient, testhelpers.MustNewMetrics(), telemetry.NewTracer())

	tests := []struct {
		name    string
		pkg     *zarfv1alpha3.ZarfPackageJob
		wantErr bool
	}{
		{
			name: "build with git source",
			pkg: &zarfv1alpha3.ZarfPackageJob{
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
			},
			wantErr: false,
		},
		{
			name: "build without source",
			pkg: &zarfv1alpha3.ZarfPackageJob{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-build",
					Namespace: "default",
				},
				Spec: zarfv1alpha3.ZarfPackageJobSpec{
					ServiceAccountName: "test-sa",
					Action:             zarfv1alpha3.ActionBuild,
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := handler.Execute(context.Background(), tt.pkg, actionscommon.ExecuteOptions{})
			if (err != nil) != tt.wantErr {
				t.Errorf("BuildHandler.Execute() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestPublishHandlerExecute(t *testing.T) {
	kubeClient := kubefake.NewClientset()
	handler := NewPublishHandler(kubeClient, testhelpers.MustNewMetrics(), telemetry.NewTracer())

	tests := []struct {
		name    string
		pkg     *zarfv1alpha3.ZarfPackageJob
		wantErr bool
	}{
		{
			name: "publish without config",
			pkg: &zarfv1alpha3.ZarfPackageJob{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-publish",
					Namespace: "default",
				},
				Spec: zarfv1alpha3.ZarfPackageJobSpec{
					ServiceAccountName: "test-sa",
					Action:             zarfv1alpha3.ActionPublish,
					Source: zarfv1alpha3.PackageSource{
						Type: zarfv1alpha3.SourceTypeGit,
						Git: &zarfv1alpha3.GitSource{
							URL: "https://github.com/test/repo",
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "publish with OCI destination",
			pkg: &zarfv1alpha3.ZarfPackageJob{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-publish",
					Namespace: "default",
				},
				Spec: zarfv1alpha3.ZarfPackageJobSpec{
					ServiceAccountName: "test-sa",
					Action:             zarfv1alpha3.ActionPublish,
					Source: zarfv1alpha3.PackageSource{
						Type: zarfv1alpha3.SourceTypeGit,
						Git: &zarfv1alpha3.GitSource{
							URL: "https://github.com/test/repo",
						},
					},
					Publish: &zarfv1alpha3.PublishConfig{
						Destination: zarfv1alpha3.PublishDestination{
							Type: zarfv1alpha3.DestinationTypeOCI,
							OCI: &zarfv1alpha3.OCIDestination{
								Registry:   "ghcr.io",
								Repository: "test/repo",
								Tag:        "v1.0.0",
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
			_, err := handler.Execute(context.Background(), tt.pkg, actionscommon.ExecuteOptions{ArtifactPath: "/workspace/test.tar.zst"})
			if (err != nil) != tt.wantErr {
				t.Errorf("PublishHandler.Execute() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestDeployHandlerExecute(t *testing.T) {
	kubeClient := kubefake.NewClientset()
	dynamicClient := fake.NewSimpleDynamicClient(runtime.NewScheme())
	handler := NewDeployHandler(kubeClient, dynamicClient, testhelpers.MustNewMetrics(), telemetry.NewTracer())

	tests := []struct {
		name    string
		pkg     *zarfv1alpha3.ZarfPackageJob
		wantErr bool
	}{
		{
			name: "deploy without config",
			pkg: &zarfv1alpha3.ZarfPackageJob{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-deploy",
					Namespace: "default",
				},
				Spec: zarfv1alpha3.ZarfPackageJobSpec{
					ServiceAccountName: "test-sa",
					Action:             zarfv1alpha3.ActionDeploy,
					Source: zarfv1alpha3.PackageSource{
						Type: zarfv1alpha3.SourceTypeOCI,
						OCI: &zarfv1alpha3.OCISource{
							Reference: "ghcr.io/test/package:v1.0.0",
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "deploy to in-cluster",
			pkg: &zarfv1alpha3.ZarfPackageJob{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-deploy",
					Namespace: "default",
				},
				Spec: zarfv1alpha3.ZarfPackageJobSpec{
					ServiceAccountName: "test-sa",
					Action:             zarfv1alpha3.ActionDeploy,
					Source: zarfv1alpha3.PackageSource{
						Type: zarfv1alpha3.SourceTypeOCI,
						OCI: &zarfv1alpha3.OCISource{
							Reference: "ghcr.io/test/package:v1.0.0",
						},
					},
					Deploy: &zarfv1alpha3.DeployConfig{
						Target:    zarfv1alpha3.DeployTargetInCluster,
						Namespace: "default",
					},
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := handler.Execute(context.Background(), tt.pkg, actionscommon.ExecuteOptions{ArtifactPath: "/workspace/test.tar.zst"})
			if (err != nil) != tt.wantErr {
				t.Errorf("DeployHandler.Execute() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestDeployHandlerExecute_ExternalCluster(t *testing.T) {
	client := kubefake.NewClientset()
	dynamicClient := fake.NewSimpleDynamicClient(runtime.NewScheme())
	handler := NewDeployHandler(client, dynamicClient, testhelpers.MustNewMetrics(), telemetry.NewTracer())

	pkg := &zarfv1alpha3.ZarfPackageJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-deploy-external",
			Namespace: "default",
		},
		Spec: zarfv1alpha3.ZarfPackageJobSpec{
			ServiceAccountName: "test-sa",
			Action:             zarfv1alpha3.ActionDeploy,
			Source: zarfv1alpha3.PackageSource{
				Type: zarfv1alpha3.SourceTypeLocal,
			},
			Deploy: &zarfv1alpha3.DeployConfig{
				Target: zarfv1alpha3.DeployTargetExternalCluster,
				ExternalCluster: &apiscommon.ExternalClusterConfig{
					SecretRef: apiscommon.SecretReference{
						Name: "external-kubeconfig",
					},
				},
			},
		},
	}

	result, err := handler.Execute(context.Background(), pkg, actionscommon.ExecuteOptions{ArtifactPath: "/workspace/test.tar.zst"})
	if err != nil {
		t.Errorf("DeployHandler.Execute() with external cluster failed: %v", err)
	}

	if result == nil {
		t.Fatal("Expected non-nil result")
	}

	// Verify the job was created
	jobs, err := client.BatchV1().Jobs("default").List(context.Background(), metav1.ListOptions{})
	if err != nil {
		t.Fatalf("Failed to list jobs: %v", err)
	}

	if len(jobs.Items) != 1 {
		t.Errorf("Expected 1 job, got %d", len(jobs.Items))
	}

	// Verify kubeconfig volume was added
	job := jobs.Items[0]
	foundVolume := false
	for _, vol := range job.Spec.Template.Spec.Volumes {
		if vol.Name == "kubeconfig" {
			foundVolume = true
			if vol.Secret == nil || vol.Secret.SecretName != "external-kubeconfig" { // pragma: allowlist secret
				t.Error("Kubeconfig volume not configured correctly")
			}
			break
		}
	}
	if !foundVolume {
		t.Error("Kubeconfig volume not found in job spec")
	}

	// Verify kubeconfig volume mount was added
	foundMount := false
	for _, mount := range job.Spec.Template.Spec.Containers[0].VolumeMounts {
		if mount.Name == "kubeconfig" {
			foundMount = true
			if mount.MountPath != constants.VolumeMountPathKubeconfig || !mount.ReadOnly {
				t.Errorf("Kubeconfig volume mount not configured correctly: got %s, want %s", mount.MountPath, constants.VolumeMountPathKubeconfig)
			}
			break
		}
	}
	if !foundMount {
		t.Error("Kubeconfig volume mount not found in container spec")
	}
}

func TestDeployHandlerExecute_MissingDeployConfig(t *testing.T) {
	client := kubefake.NewClientset()
	dynamicClient := fake.NewSimpleDynamicClient(runtime.NewScheme())
	handler := NewDeployHandler(client, dynamicClient, testhelpers.MustNewMetrics(), telemetry.NewTracer())

	pkg := &zarfv1alpha3.ZarfPackageJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-deploy-no-config",
			Namespace: "default",
		},
		Spec: zarfv1alpha3.ZarfPackageJobSpec{
			ServiceAccountName: "test-sa",
			Action:             zarfv1alpha3.ActionDeploy,
			Source: zarfv1alpha3.PackageSource{
				Type: zarfv1alpha3.SourceTypeLocal,
			},
			// Deploy is nil
		},
	}

	_, err := handler.Execute(context.Background(), pkg, actionscommon.ExecuteOptions{ArtifactPath: "/workspace/test.tar.zst"})
	if err == nil {
		t.Error("Expected error for missing deploy config")
	}
}

func TestPublishHandlerExecute_MissingPublishConfig(t *testing.T) {
	client := kubefake.NewClientset()
	handler := NewPublishHandler(client, testhelpers.MustNewMetrics(), telemetry.NewTracer())

	pkg := &zarfv1alpha3.ZarfPackageJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-publish-no-config",
			Namespace: "default",
		},
		Spec: zarfv1alpha3.ZarfPackageJobSpec{
			ServiceAccountName: "test-sa",
			Action:             zarfv1alpha3.ActionPublish,
			Source: zarfv1alpha3.PackageSource{
				Type: zarfv1alpha3.SourceTypeLocal,
			},
			// Publish is nil
		},
	}

	_, err := handler.Execute(context.Background(), pkg, actionscommon.ExecuteOptions{ArtifactPath: "/workspace/test.tar.zst"})
	if err == nil {
		t.Error("Expected error for missing publish config")
	}
}

func TestBuildHandlerExecute_LocalSource(t *testing.T) {
	kubeClient := kubefake.NewClientset()
	handler := NewBuildHandler(kubeClient, testhelpers.MustNewMetrics(), telemetry.NewTracer())

	pkg := &zarfv1alpha3.ZarfPackageJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-local-build",
			Namespace: "default",
		},
		Spec: zarfv1alpha3.ZarfPackageJobSpec{
			ServiceAccountName: "test-sa",
			Action:             zarfv1alpha3.ActionBuild,
			Source: zarfv1alpha3.PackageSource{
				Type: zarfv1alpha3.SourceTypeLocal,
				Local: &zarfv1alpha3.LocalSource{
					Path:    "/tmp/package",
					DevMode: true,
				},
			},
		},
	}

	result, err := handler.Execute(context.Background(), pkg, actionscommon.ExecuteOptions{})
	if err != nil {
		t.Errorf("BuildHandler.Execute() with local source failed: %v", err)
	}
	if result == nil {
		t.Fatal("Expected non-nil result")
	}

	// Verify the job was created without init containers (local source)
	jobs, err := kubeClient.BatchV1().Jobs("default").List(context.Background(), metav1.ListOptions{})
	if err != nil {
		t.Fatalf("Failed to list jobs: %v", err)
	}

	if len(jobs.Items) != 1 {
		t.Errorf("Expected 1 job, got %d", len(jobs.Items))
	}

	job := jobs.Items[0]
	if len(job.Spec.Template.Spec.InitContainers) != 0 {
		t.Errorf("Expected 0 init containers for local source, got %d", len(job.Spec.Template.Spec.InitContainers))
	}
}

func TestPublishHandlerExecute_LocalSource(t *testing.T) {
	kubeClient := kubefake.NewClientset()
	handler := NewPublishHandler(kubeClient, testhelpers.MustNewMetrics(), telemetry.NewTracer())

	pkg := &zarfv1alpha3.ZarfPackageJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-local-publish",
			Namespace: "default",
		},
		Spec: zarfv1alpha3.ZarfPackageJobSpec{
			ServiceAccountName: "test-sa",
			Action:             zarfv1alpha3.ActionPublish,
			Source: zarfv1alpha3.PackageSource{
				Type: zarfv1alpha3.SourceTypeLocal,
				Local: &zarfv1alpha3.LocalSource{
					Path:    "/tmp/package.tar.zst",
					DevMode: true,
				},
			},
			Publish: &zarfv1alpha3.PublishConfig{
				Destination: zarfv1alpha3.PublishDestination{
					Type: zarfv1alpha3.DestinationTypeOCI,
					OCI: &zarfv1alpha3.OCIDestination{
						Registry:   "ghcr.io",
						Repository: "test/package",
						Tag:        "v1.0.0",
					},
				},
			},
		},
	}

	result, err := handler.Execute(context.Background(), pkg, actionscommon.ExecuteOptions{ArtifactPath: "/workspace/test.tar.zst"})
	if err != nil {
		t.Errorf("PublishHandler.Execute() with local source failed: %v", err)
	}
	if result == nil {
		t.Fatal("Expected non-nil result")
	}

	// Verify no init containers for local source
	jobs, err := kubeClient.BatchV1().Jobs("default").List(context.Background(), metav1.ListOptions{})
	if err != nil {
		t.Fatalf("Failed to list jobs: %v", err)
	}

	if len(jobs.Items) != 1 {
		t.Errorf("Expected 1 job, got %d", len(jobs.Items))
	}

	job := jobs.Items[0]
	if len(job.Spec.Template.Spec.InitContainers) != 0 {
		t.Errorf("Expected 0 init containers for local source, got %d", len(job.Spec.Template.Spec.InitContainers))
	}
}

func TestDeployHandlerExecute_LocalSource(t *testing.T) {
	kubeClient := kubefake.NewClientset()
	dynamicClient := fake.NewSimpleDynamicClient(runtime.NewScheme())
	handler := NewDeployHandler(kubeClient, dynamicClient, testhelpers.MustNewMetrics(), telemetry.NewTracer())

	pkg := &zarfv1alpha3.ZarfPackageJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-local-deploy",
			Namespace: "default",
		},
		Spec: zarfv1alpha3.ZarfPackageJobSpec{
			ServiceAccountName: "test-sa",
			Action:             zarfv1alpha3.ActionDeploy,
			Source: zarfv1alpha3.PackageSource{
				Type: zarfv1alpha3.SourceTypeLocal,
				Local: &zarfv1alpha3.LocalSource{
					Path:    "/tmp/package.tar.zst",
					DevMode: true,
				},
			},
			Deploy: &zarfv1alpha3.DeployConfig{
				Target:    zarfv1alpha3.DeployTargetInCluster,
				Namespace: "default",
			},
		},
	}

	result, err := handler.Execute(context.Background(), pkg, actionscommon.ExecuteOptions{ArtifactPath: "/workspace/test.tar.zst"})
	if err != nil {
		t.Errorf("DeployHandler.Execute() with local source failed: %v", err)
	}
	if result == nil {
		t.Fatal("Expected non-nil result")
	}

	// Verify init containers: only kubeconfig-init for in-cluster deploy (no source init containers for local source)
	jobs, err := kubeClient.BatchV1().Jobs("default").List(context.Background(), metav1.ListOptions{})
	if err != nil {
		t.Fatalf("Failed to list jobs: %v", err)
	}

	if len(jobs.Items) != 1 {
		t.Errorf("Expected 1 job, got %d", len(jobs.Items))
	}

	job := jobs.Items[0]
	if len(job.Spec.Template.Spec.InitContainers) != 1 {
		t.Errorf("Expected 1 init container (kubeconfig-init) for local source in-cluster deploy, got %d", len(job.Spec.Template.Spec.InitContainers))
	}
	if len(job.Spec.Template.Spec.InitContainers) > 0 && job.Spec.Template.Spec.InitContainers[0].Name != "kubeconfig-init" {
		t.Errorf("Expected init container named 'kubeconfig-init', got %q", job.Spec.Template.Spec.InitContainers[0].Name)
	}
}

func TestDeployHandlerExecute_WithComponentsAndVariables(t *testing.T) {
	kubeClient := kubefake.NewClientset()
	dynamicClient := fake.NewSimpleDynamicClient(runtime.NewScheme())
	handler := NewDeployHandler(kubeClient, dynamicClient, testhelpers.MustNewMetrics(), telemetry.NewTracer())

	pkg := &zarfv1alpha3.ZarfPackageJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-deploy-advanced",
			Namespace: "default",
		},
		Spec: zarfv1alpha3.ZarfPackageJobSpec{
			ServiceAccountName: "test-sa",
			Action:             zarfv1alpha3.ActionDeploy,
			Source: zarfv1alpha3.PackageSource{
				Type: zarfv1alpha3.SourceTypeLocal,
				Local: &zarfv1alpha3.LocalSource{
					Path:    "/tmp/package.tar.zst",
					DevMode: true,
				},
			},
			Deploy: &zarfv1alpha3.DeployConfig{
				Target:     zarfv1alpha3.DeployTargetInCluster,
				Namespace:  "test-namespace",
				Components: []string{"component1", "component2"},
				Variables: map[string]string{
					"IMAGE_TAG": "v1.2.3",
					"REPLICAS":  "3",
				},
			},
		},
	}

	result, err := handler.Execute(context.Background(), pkg, actionscommon.ExecuteOptions{ArtifactPath: "/workspace/test.tar.zst"})
	if err != nil {
		t.Errorf("DeployHandler.Execute() with components and variables failed: %v", err)
	}
	if result == nil {
		t.Fatal("Expected non-nil result")
	}

	// Verify the job was created
	jobs, err := kubeClient.BatchV1().Jobs("default").List(context.Background(), metav1.ListOptions{})
	if err != nil {
		t.Fatalf("Failed to list jobs: %v", err)
	}

	if len(jobs.Items) != 1 {
		t.Errorf("Expected 1 job, got %d", len(jobs.Items))
	}

	// Verify environment variables include namespace
	job := jobs.Items[0]
	foundNamespaceEnv := false
	for _, env := range job.Spec.Template.Spec.Containers[0].Env {
		if env.Name == "ZARF_NAMESPACE" && env.Value == "test-namespace" {
			foundNamespaceEnv = true
			break
		}
	}
	if !foundNamespaceEnv {
		t.Error("Expected ZARF_NAMESPACE environment variable not found")
	}
}

func TestDeployHandlerExecute_ExternalClusterWithContext(t *testing.T) {
	kubeClient := kubefake.NewClientset()
	dynamicClient := fake.NewSimpleDynamicClient(runtime.NewScheme())
	handler := NewDeployHandler(kubeClient, dynamicClient, testhelpers.MustNewMetrics(), telemetry.NewTracer())

	pkg := &zarfv1alpha3.ZarfPackageJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-deploy-external-context",
			Namespace: "default",
		},
		Spec: zarfv1alpha3.ZarfPackageJobSpec{
			ServiceAccountName: "test-sa",
			Action:             zarfv1alpha3.ActionDeploy,
			Source: zarfv1alpha3.PackageSource{
				Type: zarfv1alpha3.SourceTypeLocal,
				Local: &zarfv1alpha3.LocalSource{
					Path:    "/tmp/package.tar.zst",
					DevMode: true,
				},
			},
			Deploy: &zarfv1alpha3.DeployConfig{
				Target: zarfv1alpha3.DeployTargetExternalCluster,
				ExternalCluster: &apiscommon.ExternalClusterConfig{
					SecretRef: apiscommon.SecretReference{ // pragma: allowlist secret
						Name: "external-kubeconfig",
					},
					Context: "production-cluster",
				},
			},
		},
	}

	result, err := handler.Execute(context.Background(), pkg, actionscommon.ExecuteOptions{ArtifactPath: "/workspace/test.tar.zst"})
	if err != nil {
		t.Errorf("DeployHandler.Execute() with external cluster context failed: %v", err)
	}
	if result == nil {
		t.Fatal("Expected non-nil result")
	}

	// Verify the job was created
	jobs, err := kubeClient.BatchV1().Jobs("default").List(context.Background(), metav1.ListOptions{})
	if err != nil {
		t.Fatalf("Failed to list jobs: %v", err)
	}

	if len(jobs.Items) != 1 {
		t.Errorf("Expected 1 job, got %d", len(jobs.Items))
	}
}

func TestBuildHandlerBuildZarfCommand(t *testing.T) {
	handler := &BuildHandler{}

	tests := []struct {
		name            string
		pkg             *zarfv1alpha3.ZarfPackageJob
		artifactPVCName string
		wantContains    []string
		wantNotContains []string
	}{
		{
			name: "basic build without variables",
			pkg: &zarfv1alpha3.ZarfPackageJob{
				Spec: zarfv1alpha3.ZarfPackageJobSpec{
					Source: zarfv1alpha3.PackageSource{
						Type: zarfv1alpha3.SourceTypeGit,
						Git: &zarfv1alpha3.GitSource{
							URL: "https://github.com/test/repo",
						},
					},
				},
			},
			artifactPVCName: "",
			wantContains:    []string{"zarf package create", "--confirm", "--output-directory"},
			wantNotContains: []string{"--set"},
		},
		{
			name: "build with single variable",
			pkg: &zarfv1alpha3.ZarfPackageJob{
				Spec: zarfv1alpha3.ZarfPackageJobSpec{
					Source: zarfv1alpha3.PackageSource{
						Type: zarfv1alpha3.SourceTypeGit,
						Git: &zarfv1alpha3.GitSource{
							URL: "https://github.com/test/repo",
						},
					},
					Build: &zarfv1alpha3.BuildConfig{
						Variables: map[string]string{
							"IMAGE_TAG": "v1.2.3",
						},
					},
				},
			},
			artifactPVCName: "",
			wantContains:    []string{"zarf package create", "--set IMAGE_TAG=v1.2.3"},
		},
		{
			name: "build with multiple variables",
			pkg: &zarfv1alpha3.ZarfPackageJob{
				Spec: zarfv1alpha3.ZarfPackageJobSpec{
					Source: zarfv1alpha3.PackageSource{
						Type: zarfv1alpha3.SourceTypeGit,
						Git: &zarfv1alpha3.GitSource{
							URL: "https://github.com/test/repo",
						},
					},
					Build: &zarfv1alpha3.BuildConfig{
						Variables: map[string]string{
							"IMAGE_TAG": "v1.2.3",
							"REGISTRY":  "ghcr.io/myorg",
						},
					},
				},
			},
			artifactPVCName: "",
			wantContains:    []string{"zarf package create", "--set IMAGE_TAG=v1.2.3", "--set REGISTRY=ghcr.io/myorg"},
		},
		{
			name: "build with PVC and variables",
			pkg: &zarfv1alpha3.ZarfPackageJob{
				Spec: zarfv1alpha3.ZarfPackageJobSpec{
					Source: zarfv1alpha3.PackageSource{
						Type: zarfv1alpha3.SourceTypeGit,
						Git: &zarfv1alpha3.GitSource{
							URL: "https://github.com/test/repo",
						},
					},
					Build: &zarfv1alpha3.BuildConfig{
						Variables: map[string]string{
							"VERSION": "2.0.0",
						},
					},
				},
			},
			artifactPVCName: "my-pvc",
			wantContains:    []string{"zarf package create", constants.VolumeMountPathArtifacts, "--set VERSION=2.0.0"},
		},
		{
			name: "build with empty variables map",
			pkg: &zarfv1alpha3.ZarfPackageJob{
				Spec: zarfv1alpha3.ZarfPackageJobSpec{
					Source: zarfv1alpha3.PackageSource{
						Type: zarfv1alpha3.SourceTypeGit,
						Git: &zarfv1alpha3.GitSource{
							URL: "https://github.com/test/repo",
						},
					},
					Build: &zarfv1alpha3.BuildConfig{
						Variables: map[string]string{},
					},
				},
			},
			artifactPVCName: "",
			wantContains:    []string{"zarf package create"},
			wantNotContains: []string{"--set"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd, err := handler.buildZarfCommand(tt.pkg, tt.artifactPVCName)
			if err != nil {
				t.Fatalf("buildZarfCommand() unexpected error: %v", err)
			}

			for _, want := range tt.wantContains {
				if !containsSubstring(cmd, want) {
					t.Errorf("buildZarfCommand() cmd = %q, want to contain %q", cmd, want)
				}
			}

			for _, notWant := range tt.wantNotContains {
				if containsSubstring(cmd, notWant) {
					t.Errorf("buildZarfCommand() cmd = %q, should not contain %q", cmd, notWant)
				}
			}
		})
	}
}

func containsSubstring(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && findSubstring(s, substr)))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestBuildHandlerExecute_WithExtraMounts(t *testing.T) {
	kubeClient := kubefake.NewClientset()
	handler := NewBuildHandler(kubeClient, testhelpers.MustNewMetrics(), telemetry.NewTracer())

	pkg := &zarfv1alpha3.ZarfPackageJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-build-extra-mounts",
			Namespace: "default",
		},
		Spec: zarfv1alpha3.ZarfPackageJobSpec{
			ServiceAccountName: "test-sa",
			Action:             zarfv1alpha3.ActionBuild,
			Source: zarfv1alpha3.PackageSource{
				Type: zarfv1alpha3.SourceTypeGit,
				Git: &zarfv1alpha3.GitSource{
					URL: "https://github.com/test/repo",
				},
			},
			ExtraMounts: []apiscommon.ExtraMount{
				{
					ConfigMapRef: &apiscommon.LocalObjectReference{Name: "my-configmap"},
					MountPath:    "/etc/config",
				},
			},
			Build: &zarfv1alpha3.BuildConfig{
				ExtraMounts: []apiscommon.ExtraMount{
					{
						SecretRef: &apiscommon.LocalObjectReference{Name: "my-secret"},
						MountPath: "/etc/secret",
					},
				},
			},
		},
	}

	result, err := handler.Execute(context.Background(), pkg, actionscommon.ExecuteOptions{})
	if err != nil {
		t.Fatalf("BuildHandler.Execute() with extra mounts failed: %v", err)
	}
	if result == nil {
		t.Fatal("Expected non-nil result")
	}

	// Verify the job was created
	jobs, err := kubeClient.BatchV1().Jobs("default").List(context.Background(), metav1.ListOptions{})
	if err != nil {
		t.Fatalf("Failed to list jobs: %v", err)
	}
	if len(jobs.Items) != 1 {
		t.Fatalf("Expected 1 job, got %d", len(jobs.Items))
	}

	job := jobs.Items[0]

	// Verify extra mount volumes are present
	foundExtraMount0 := false
	foundExtraMount1 := false
	for _, vol := range job.Spec.Template.Spec.Volumes {
		if vol.Name == "extra-mount-0" {
			foundExtraMount0 = true
			if vol.ConfigMap == nil || vol.ConfigMap.Name != "my-configmap" {
				t.Error("extra-mount-0 volume not configured as expected ConfigMap")
			}
		}
		if vol.Name == "extra-mount-1" {
			foundExtraMount1 = true
			if vol.Secret == nil || vol.Secret.SecretName != "my-secret" { // pragma: allowlist secret
				t.Error("extra-mount-1 volume not configured as expected Secret")
			}
		}
	}
	if !foundExtraMount0 {
		t.Error("extra-mount-0 volume not found in job spec")
	}
	if !foundExtraMount1 {
		t.Error("extra-mount-1 volume not found in job spec")
	}

	// Verify extra mount volume mounts are on the container
	foundMount0 := false
	foundMount1 := false
	for _, mount := range job.Spec.Template.Spec.Containers[0].VolumeMounts {
		if mount.Name == "extra-mount-0" {
			foundMount0 = true
			if mount.MountPath != "/etc/config" {
				t.Errorf("extra-mount-0 mount path = %q, want %q", mount.MountPath, "/etc/config")
			}
		}
		if mount.Name == "extra-mount-1" {
			foundMount1 = true
			if mount.MountPath != "/etc/secret" {
				t.Errorf("extra-mount-1 mount path = %q, want %q", mount.MountPath, "/etc/secret")
			}
		}
	}
	if !foundMount0 {
		t.Error("extra-mount-0 volume mount not found on container")
	}
	if !foundMount1 {
		t.Error("extra-mount-1 volume mount not found on container")
	}
}

func TestBuildHandlerExecute_WithExtraMountsConflict(t *testing.T) {
	kubeClient := kubefake.NewClientset()
	handler := NewBuildHandler(kubeClient, testhelpers.MustNewMetrics(), telemetry.NewTracer())

	pkg := &zarfv1alpha3.ZarfPackageJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-build-extra-mounts-conflict",
			Namespace: "default",
		},
		Spec: zarfv1alpha3.ZarfPackageJobSpec{
			ServiceAccountName: "test-sa",
			Action:             zarfv1alpha3.ActionBuild,
			Source: zarfv1alpha3.PackageSource{
				Type: zarfv1alpha3.SourceTypeGit,
				Git: &zarfv1alpha3.GitSource{
					URL: "https://github.com/test/repo",
				},
			},
			ExtraMounts: []apiscommon.ExtraMount{
				{
					ConfigMapRef: &apiscommon.LocalObjectReference{Name: "spec-configmap"},
					MountPath:    "/etc/custom",
				},
			},
			Build: &zarfv1alpha3.BuildConfig{
				ExtraMounts: []apiscommon.ExtraMount{
					{
						SecretRef: &apiscommon.LocalObjectReference{Name: "build-secret"},
						MountPath: "/etc/custom",
					},
				},
			},
		},
	}

	_, err := handler.Execute(context.Background(), pkg, actionscommon.ExecuteOptions{})
	if err == nil {
		t.Fatal("Expected error for conflicting extraMounts, got nil")
	}
	if !strings.Contains(err.Error(), "extraMounts") {
		t.Errorf("Expected error to contain 'extraMounts', got: %v", err)
	}
}
