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

func mustNewMetrics() *telemetry.Metrics {
	m, err := telemetry.NewMetrics()
	if err != nil {
		panic(err)
	}
	return m
}
