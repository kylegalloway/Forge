package actions

import (
	"context"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	zarfv1alpha1 "github.com/kylegalloway/forge/pkg/apis/zarf/v1alpha1"
	"github.com/kylegalloway/forge/pkg/telemetry"
)

func TestNewBuildHandler(t *testing.T) {
	kubeClient := fake.NewSimpleClientset()
	metrics := mustNewMetrics()
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
	kubeClient := fake.NewSimpleClientset()
	metrics := mustNewMetrics()
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
	kubeClient := fake.NewSimpleClientset()
	metrics := mustNewMetrics()
	tracer := telemetry.NewTracer()

	handler := NewDeployHandler(kubeClient, metrics, tracer)
	if handler == nil {
		t.Fatal("NewDeployHandler returned nil")
	}
	if handler.kubeClient == nil {
		t.Error("kubeClient not set")
	}
}

func TestBuildHandlerExecute(t *testing.T) {
	kubeClient := fake.NewSimpleClientset()
	handler := NewBuildHandler(kubeClient, mustNewMetrics(), telemetry.NewTracer())

	tests := []struct {
		name    string
		pkg     *zarfv1alpha1.ZarfPackageJob
		wantErr bool
	}{
		{
			name: "build with git source",
			pkg: &zarfv1alpha1.ZarfPackageJob{
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
			},
			wantErr: false,
		},
		{
			name: "build without source",
			pkg: &zarfv1alpha1.ZarfPackageJob{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-build",
					Namespace: "default",
				},
				Spec: zarfv1alpha1.ZarfPackageJobSpec{
					ServiceAccountName: "test-sa",
					Action:             zarfv1alpha1.ActionBuild,
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := handler.Execute(context.Background(), tt.pkg)
			if (err != nil) != tt.wantErr {
				t.Errorf("BuildHandler.Execute() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestPublishHandlerExecute(t *testing.T) {
	kubeClient := fake.NewSimpleClientset()
	handler := NewPublishHandler(kubeClient, mustNewMetrics(), telemetry.NewTracer())

	tests := []struct {
		name    string
		pkg     *zarfv1alpha1.ZarfPackageJob
		wantErr bool
	}{
		{
			name: "publish without config",
			pkg: &zarfv1alpha1.ZarfPackageJob{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-publish",
					Namespace: "default",
				},
				Spec: zarfv1alpha1.ZarfPackageJobSpec{
					ServiceAccountName: "test-sa",
					Action:             zarfv1alpha1.ActionPublish,
					Source: zarfv1alpha1.PackageSource{
						Type: zarfv1alpha1.SourceTypeGit,
						Git: &zarfv1alpha1.GitSource{
							URL: "https://github.com/test/repo",
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "publish with OCI destination",
			pkg: &zarfv1alpha1.ZarfPackageJob{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-publish",
					Namespace: "default",
				},
				Spec: zarfv1alpha1.ZarfPackageJobSpec{
					ServiceAccountName: "test-sa",
					Action:             zarfv1alpha1.ActionPublish,
					Source: zarfv1alpha1.PackageSource{
						Type: zarfv1alpha1.SourceTypeGit,
						Git: &zarfv1alpha1.GitSource{
							URL: "https://github.com/test/repo",
						},
					},
					Publish: &zarfv1alpha1.PublishConfig{
						Destination: zarfv1alpha1.PublishDestination{
							Type: zarfv1alpha1.DestinationTypeOCI,
							OCI: &zarfv1alpha1.OCIDestination{
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
			_, err := handler.Execute(context.Background(), tt.pkg, "/workspace/test.tar.zst")
			if (err != nil) != tt.wantErr {
				t.Errorf("PublishHandler.Execute() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestDeployHandlerExecute(t *testing.T) {
	kubeClient := fake.NewSimpleClientset()
	handler := NewDeployHandler(kubeClient, mustNewMetrics(), telemetry.NewTracer())

	tests := []struct {
		name    string
		pkg     *zarfv1alpha1.ZarfPackageJob
		wantErr bool
	}{
		{
			name: "deploy without config",
			pkg: &zarfv1alpha1.ZarfPackageJob{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-deploy",
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
				},
			},
			wantErr: true,
		},
		{
			name: "deploy to in-cluster",
			pkg: &zarfv1alpha1.ZarfPackageJob{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-deploy",
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
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := handler.Execute(context.Background(), tt.pkg, "/workspace/test.tar.zst")
			if (err != nil) != tt.wantErr {
				t.Errorf("DeployHandler.Execute() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestDeployHandlerExecute_ExternalCluster(t *testing.T) {
	client := fake.NewSimpleClientset()
	handler := NewDeployHandler(client, mustNewMetrics(), telemetry.NewTracer())

	pkg := &zarfv1alpha1.ZarfPackageJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-deploy-external",
			Namespace: "default",
		},
		Spec: zarfv1alpha1.ZarfPackageJobSpec{
			ServiceAccountName: "test-sa",
			Action:             zarfv1alpha1.ActionDeploy,
			Source: zarfv1alpha1.PackageSource{
				Type: zarfv1alpha1.SourceTypeLocal,
			},
			Deploy: &zarfv1alpha1.DeployConfig{
				Target: zarfv1alpha1.DeployTargetExternalCluster,
				ExternalCluster: &zarfv1alpha1.ExternalClusterConfig{
					KubeconfigSecretRef: zarfv1alpha1.SecretReference{
						Name: "external-kubeconfig",
					},
				},
			},
		},
	}

	result, err := handler.Execute(context.Background(), pkg, "/workspace/test.tar.zst")
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
	client := fake.NewSimpleClientset()
	handler := NewDeployHandler(client, mustNewMetrics(), telemetry.NewTracer())

	pkg := &zarfv1alpha1.ZarfPackageJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-deploy-no-config",
			Namespace: "default",
		},
		Spec: zarfv1alpha1.ZarfPackageJobSpec{
			ServiceAccountName: "test-sa",
			Action:             zarfv1alpha1.ActionDeploy,
			Source: zarfv1alpha1.PackageSource{
				Type: zarfv1alpha1.SourceTypeLocal,
			},
			// Deploy is nil
		},
	}

	_, err := handler.Execute(context.Background(), pkg, "/workspace/test.tar.zst")
	if err == nil {
		t.Error("Expected error for missing deploy config")
	}
}

func TestPublishHandlerExecute_MissingPublishConfig(t *testing.T) {
	client := fake.NewSimpleClientset()
	handler := NewPublishHandler(client, mustNewMetrics(), telemetry.NewTracer())

	pkg := &zarfv1alpha1.ZarfPackageJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-publish-no-config",
			Namespace: "default",
		},
		Spec: zarfv1alpha1.ZarfPackageJobSpec{
			ServiceAccountName: "test-sa",
			Action:             zarfv1alpha1.ActionPublish,
			Source: zarfv1alpha1.PackageSource{
				Type: zarfv1alpha1.SourceTypeLocal,
			},
			// Publish is nil
		},
	}

	_, err := handler.Execute(context.Background(), pkg, "/workspace/test.tar.zst")
	if err == nil {
		t.Error("Expected error for missing publish config")
	}
}

func mustNewMetrics() *telemetry.Metrics {
	m, err := telemetry.NewMetrics()
	if err != nil {
		panic(err)
	}
	return m
}
