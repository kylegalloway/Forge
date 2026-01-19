package zarf

import (
	"context"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/dynamic/fake"
	kubefake "k8s.io/client-go/kubernetes/fake"

	"github.com/kylegalloway/forge/pkg/apis/common"
	zarfv1alpha3 "github.com/kylegalloway/forge/pkg/apis/zarf/v1alpha3"
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
			_, err := handler.Execute(context.Background(), tt.pkg, "")
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
			_, err := handler.Execute(context.Background(), tt.pkg, "/workspace/test.tar.zst", "")
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
			_, err := handler.Execute(context.Background(), tt.pkg, "/workspace/test.tar.zst", "")
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
				ExternalCluster: &common.ExternalClusterConfig{
					SecretRef: common.SecretReference{
						Name: "external-kubeconfig",
					},
				},
			},
		},
	}

	result, err := handler.Execute(context.Background(), pkg, "/workspace/test.tar.zst", "")
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
			if mount.MountPath != "/kubeconfig" || !mount.ReadOnly {
				t.Error("Kubeconfig volume mount not configured correctly")
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

	_, err := handler.Execute(context.Background(), pkg, "/workspace/test.tar.zst", "")
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

	_, err := handler.Execute(context.Background(), pkg, "/workspace/test.tar.zst", "")
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

	result, err := handler.Execute(context.Background(), pkg, "")
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

	result, err := handler.Execute(context.Background(), pkg, "/workspace/test.tar.zst", "")
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

	result, err := handler.Execute(context.Background(), pkg, "/workspace/test.tar.zst", "")
	if err != nil {
		t.Errorf("DeployHandler.Execute() with local source failed: %v", err)
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

	result, err := handler.Execute(context.Background(), pkg, "/workspace/test.tar.zst", "")
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
				ExternalCluster: &common.ExternalClusterConfig{
					SecretRef: common.SecretReference{ // pragma: allowlist secret
						Name: "external-kubeconfig",
					},
					Context: "production-cluster",
				},
			},
		},
	}

	result, err := handler.Execute(context.Background(), pkg, "/workspace/test.tar.zst", "")
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
