package uds

import (
	"context"
	"strings"
	"testing"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/dynamic/fake"
	kubefake "k8s.io/client-go/kubernetes/fake"

	"github.com/kylegalloway/forge/pkg/actions"
	"github.com/kylegalloway/forge/pkg/apis/common"
	udsv1alpha3 "github.com/kylegalloway/forge/pkg/apis/uds/v1alpha3"
	testhelpers "github.com/kylegalloway/forge/pkg/controller/testing"
	"github.com/kylegalloway/forge/pkg/destinations"
	"github.com/kylegalloway/forge/pkg/telemetry"
)

// TestNewCreateHandler tests CreateHandler initialization
func TestNewCreateHandler(t *testing.T) {
	kubeClient := kubefake.NewClientset()
	metrics := testhelpers.MustNewMetrics()
	tracer := telemetry.NewTracer()

	handler := NewCreateHandler(kubeClient, metrics, tracer)
	if handler == nil {
		t.Fatal("NewCreateHandler returned nil")
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

// TestNewPublishHandler tests PublishHandler initialization
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
	if handler.metrics == nil {
		t.Error("metrics not set")
	}
	if handler.tracer == nil {
		t.Error("tracer not set")
	}
}

// TestNewDeployHandler tests DeployHandler initialization
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
	if handler.metrics == nil {
		t.Error("metrics not set")
	}
	if handler.tracer == nil {
		t.Error("tracer not set")
	}
}

// TestCreateHandlerExecute tests CreateHandler.Execute
func TestCreateHandlerExecute(t *testing.T) {
	kubeClient := kubefake.NewClientset()
	handler := NewCreateHandler(kubeClient, testhelpers.MustNewMetrics(), telemetry.NewTracer())

	tests := []struct {
		name    string
		bundle  *udsv1alpha3.UDSBundleJob
		wantErr bool
	}{
		{
			name: "create with git source",
			bundle: &udsv1alpha3.UDSBundleJob{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-create",
					Namespace: "default",
				},
				Spec: udsv1alpha3.UDSBundleJobSpec{
					ServiceAccountName: "test-sa",
					Action:             udsv1alpha3.ActionCreate,
					Source: udsv1alpha3.PackageSource{
						Type: udsv1alpha3.SourceTypeGit,
						Git: &udsv1alpha3.GitSource{
							URL: "https://github.com/test/repo",
							Ref: "main",
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "create with oci source",
			bundle: &udsv1alpha3.UDSBundleJob{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-create-oci",
					Namespace: "default",
				},
				Spec: udsv1alpha3.UDSBundleJobSpec{
					ServiceAccountName: "test-sa",
					Action:             udsv1alpha3.ActionCreate,
					Source: udsv1alpha3.PackageSource{
						Type: udsv1alpha3.SourceTypeOCI,
						OCI: &udsv1alpha3.OCISource{
							Reference: "ghcr.io/test/bundles:v1.0.0",
						},
					},
				},
			},
			wantErr: false, // OCI source now implemented
		},
		{
			name: "create with local source",
			bundle: &udsv1alpha3.UDSBundleJob{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-create-local",
					Namespace: "default",
				},
				Spec: udsv1alpha3.UDSBundleJobSpec{
					ServiceAccountName: "test-sa",
					Action:             udsv1alpha3.ActionCreate,
					Source: udsv1alpha3.PackageSource{
						Type: udsv1alpha3.SourceTypeLocal,
						Local: &udsv1alpha3.LocalSource{
							Path: "/workspace/bundle",
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "create without source type",
			bundle: &udsv1alpha3.UDSBundleJob{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-create-no-source",
					Namespace: "default",
				},
				Spec: udsv1alpha3.UDSBundleJobSpec{
					ServiceAccountName: "test-sa",
					Action:             udsv1alpha3.ActionCreate,
					Source:             udsv1alpha3.PackageSource{},
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test without PVC (standalone create)
			result, err := handler.Execute(context.Background(), tt.bundle, "")
			if (err != nil) != tt.wantErr {
				t.Errorf("Execute() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if result == nil {
					t.Error("Execute() returned nil result")
					return
				}
				if result.JobName == "" {
					t.Error("Execute() result has empty JobName")
				}
				if result.Phase != "Running" {
					t.Errorf("Execute() result.Phase = %v, want Running", result.Phase)
				}
				if result.StartTime.IsZero() {
					t.Error("Execute() result.StartTime is zero")
				}
			}
		})
	}
}

// TestPublishHandlerExecute tests PublishHandler.Execute
func TestPublishHandlerExecute(t *testing.T) {
	kubeClient := kubefake.NewClientset()
	handler := NewPublishHandler(kubeClient, testhelpers.MustNewMetrics(), telemetry.NewTracer())

	tests := []struct {
		name    string
		bundle  *udsv1alpha3.UDSBundleJob
		wantErr bool
	}{
		{
			name: "publish to oci",
			bundle: &udsv1alpha3.UDSBundleJob{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-publish",
					Namespace: "default",
				},
				Spec: udsv1alpha3.UDSBundleJobSpec{
					ServiceAccountName: "test-sa",
					Action:             udsv1alpha3.ActionPublish,
					Source: udsv1alpha3.PackageSource{
						Type: udsv1alpha3.SourceTypeLocal,
						Local: &udsv1alpha3.LocalSource{
							Path: "/workspace/bundle",
						},
					},
					Publish: &udsv1alpha3.PublishConfig{
						Destination: udsv1alpha3.PublishDestination{
							Type: udsv1alpha3.DestinationTypeOCI,
							OCI: &udsv1alpha3.OCIDestination{
								Registry:   "ghcr.io",
								Repository: "test/bundles",
								Tag:        "v1.0.0",
							},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "publish to s3",
			bundle: &udsv1alpha3.UDSBundleJob{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-publish-s3",
					Namespace: "default",
				},
				Spec: udsv1alpha3.UDSBundleJobSpec{
					ServiceAccountName: "test-sa",
					Action:             udsv1alpha3.ActionPublish,
					Source: udsv1alpha3.PackageSource{
						Type: udsv1alpha3.SourceTypeLocal,
						Local: &udsv1alpha3.LocalSource{
							Path: "/workspace/bundle",
						},
					},
					Publish: &udsv1alpha3.PublishConfig{
						Destination: udsv1alpha3.PublishDestination{
							Type: udsv1alpha3.DestinationTypeS3,
							S3: &udsv1alpha3.S3Destination{
								Bucket:    "my-bundles",
								KeyPrefix: "uds/bundle.tar.zst",
								Region:    "us-east-1",
							},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "publish without destination",
			bundle: &udsv1alpha3.UDSBundleJob{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-publish-no-dest",
					Namespace: "default",
				},
				Spec: udsv1alpha3.UDSBundleJobSpec{
					ServiceAccountName: "test-sa",
					Action:             udsv1alpha3.ActionPublish,
					Source: udsv1alpha3.PackageSource{
						Type: udsv1alpha3.SourceTypeLocal,
						Local: &udsv1alpha3.LocalSource{
							Path: "/workspace/bundle",
						},
					},
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test without PVC (standalone publish)
			result, err := handler.Execute(context.Background(), tt.bundle, "", "")
			if (err != nil) != tt.wantErr {
				t.Errorf("Execute() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if result == nil {
					t.Error("Execute() returned nil result")
					return
				}
				if result.JobName == "" {
					t.Error("Execute() result has empty JobName")
				}
			}
		})
	}
}

// TestDeployHandlerExecute tests DeployHandler.Execute
func TestDeployHandlerExecute(t *testing.T) {
	kubeClient := kubefake.NewClientset()
	dynamicClient := fake.NewSimpleDynamicClient(runtime.NewScheme())
	handler := NewDeployHandler(kubeClient, dynamicClient, testhelpers.MustNewMetrics(), telemetry.NewTracer())

	tests := []struct {
		name    string
		bundle  *udsv1alpha3.UDSBundleJob
		wantErr bool
	}{
		{
			name: "deploy to in-cluster",
			bundle: &udsv1alpha3.UDSBundleJob{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-deploy",
					Namespace: "default",
				},
				Spec: udsv1alpha3.UDSBundleJobSpec{
					ServiceAccountName: "test-sa",
					Action:             udsv1alpha3.ActionDeploy,
					Source: udsv1alpha3.PackageSource{
						Type: udsv1alpha3.SourceTypeOCI,
						OCI: &udsv1alpha3.OCISource{
							Reference: "ghcr.io/test/bundles:v1.0.0",
						},
					},
					Deploy: &udsv1alpha3.DeployConfig{
						Target: udsv1alpha3.DeployTargetInCluster,
					},
				},
			},
			wantErr: false,
		},
		{
			name: "deploy with specific packages",
			bundle: &udsv1alpha3.UDSBundleJob{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-deploy-packages",
					Namespace: "default",
				},
				Spec: udsv1alpha3.UDSBundleJobSpec{
					ServiceAccountName: "test-sa",
					Action:             udsv1alpha3.ActionDeploy,
					Source: udsv1alpha3.PackageSource{
						Type: udsv1alpha3.SourceTypeOCI,
						OCI: &udsv1alpha3.OCISource{
							Reference: "ghcr.io/test/bundles:v1.0.0",
						},
					},
					Deploy: &udsv1alpha3.DeployConfig{
						Target:     udsv1alpha3.DeployTargetInCluster,
						Components: []string{"package1", "package2"},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "deploy without deploy spec",
			bundle: &udsv1alpha3.UDSBundleJob{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-deploy-no-spec",
					Namespace: "default",
				},
				Spec: udsv1alpha3.UDSBundleJobSpec{
					ServiceAccountName: "test-sa",
					Action:             udsv1alpha3.ActionDeploy,
					Source: udsv1alpha3.PackageSource{
						Type: udsv1alpha3.SourceTypeOCI,
						OCI: &udsv1alpha3.OCISource{
							Reference: "ghcr.io/test/bundles:v1.0.0",
						},
					},
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test without PVC (standalone deploy)
			result, err := handler.Execute(context.Background(), tt.bundle, "", "")
			if (err != nil) != tt.wantErr {
				t.Errorf("Execute() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if result == nil {
					t.Error("Execute() returned nil result")
					return
				}
				if result.JobName == "" {
					t.Error("Execute() result has empty JobName")
				}
			}
		})
	}
}

// TestCreateHandlerBuildUDSCommand tests command building for various source types
func TestCreateHandlerBuildUDSCommand(t *testing.T) {
	handler := &CreateHandler{}

	tests := []struct {
		name       string
		bundle     *udsv1alpha3.UDSBundleJob
		wantCmd    []string
		wantDir    string
		wantErr    bool
		checkFlags []string // flags that should be present
	}{
		{
			name: "git source",
			bundle: &udsv1alpha3.UDSBundleJob{
				Spec: udsv1alpha3.UDSBundleJobSpec{
					Source: udsv1alpha3.PackageSource{
						Type: udsv1alpha3.SourceTypeGit,
						Git: &udsv1alpha3.GitSource{
							Path: "bundles/my-bundle",
						},
					},
				},
			},
			wantDir:    "/workspace",
			wantErr:    false,
			checkFlags: []string{"create"},
		},
		{
			name: "local source",
			bundle: &udsv1alpha3.UDSBundleJob{
				Spec: udsv1alpha3.UDSBundleJobSpec{
					Source: udsv1alpha3.PackageSource{
						Type: udsv1alpha3.SourceTypeLocal,
						Local: &udsv1alpha3.LocalSource{
							Path: "/custom/path",
						},
					},
				},
			},
			wantDir:    "/workspace",
			wantErr:    false,
			checkFlags: []string{"create"},
		},
		{
			name: "oci source",
			bundle: &udsv1alpha3.UDSBundleJob{
				Spec: udsv1alpha3.UDSBundleJobSpec{
					Source: udsv1alpha3.PackageSource{
						Type: udsv1alpha3.SourceTypeOCI,
						OCI: &udsv1alpha3.OCISource{
							Reference: "ghcr.io/test/bundle:v1.0.0",
						},
					},
				},
			},
			wantDir:    "/workspace",
			wantErr:    false,
			checkFlags: []string{"create"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test without PVC (standalone create)
			cmd, workingDir := handler.buildUDSCommand(tt.bundle, "")
			if workingDir != tt.wantDir {
				t.Errorf("buildUDSCommand() workingDir = %v, want %v", workingDir, tt.wantDir)
			}
			if cmd == "" {
				t.Error("buildUDSCommand() returned empty command")
				return
			}
			// Check for expected flags
			for _, flag := range tt.checkFlags {
				if !strings.Contains(cmd, flag) {
					t.Errorf("buildUDSCommand() command %v missing flag %v", cmd, flag)
				}
			}
		})
	}
}

// TestGetResources tests resource requirements extraction
func TestGetResources(t *testing.T) {
	createHandler := &CreateHandler{}
	publishHandler := &PublishHandler{}
	deployHandler := &DeployHandler{}

	// Test with nil resources
	bundle := &udsv1alpha3.UDSBundleJob{
		Spec: udsv1alpha3.UDSBundleJobSpec{},
	}

	createRes := createHandler.getResources(bundle)
	if createRes.Requests.Cpu().String() != "500m" {
		t.Errorf("default CPU request = %v, want 500m", createRes.Requests.Cpu())
	}

	publishRes := publishHandler.getResources(bundle)
	if publishRes.Requests.Memory().String() != "512Mi" {
		t.Errorf("default memory request = %v, want 512Mi", publishRes.Requests.Memory())
	}

	deployRes := deployHandler.getResources(bundle)
	if deployRes.Limits.Cpu().String() != "2" {
		t.Errorf("default CPU limit = %v, want 2", deployRes.Limits.Cpu())
	}

	// Test with custom resources
	customResources := &corev1.ResourceRequirements{
		Requests: corev1.ResourceList{
			corev1.ResourceCPU:    actions.MustParseQuantity("100m"),
			corev1.ResourceMemory: actions.MustParseQuantity("256Mi"),
		},
		Limits: corev1.ResourceList{
			corev1.ResourceCPU:    actions.MustParseQuantity("500m"),
			corev1.ResourceMemory: actions.MustParseQuantity("1Gi"),
		},
	}
	bundle.Spec.Resources = customResources

	createResCustom := createHandler.getResources(bundle)
	if createResCustom.Requests.Cpu().String() != "100m" {
		t.Errorf("custom CPU request = %v, want 100m", createResCustom.Requests.Cpu())
	}

	publishResCustom := publishHandler.getResources(bundle)
	if publishResCustom.Limits.Memory().String() != "1Gi" {
		t.Errorf("custom memory limit = %v, want 1Gi", publishResCustom.Limits.Memory())
	}

	deployResCustom := deployHandler.getResources(bundle)
	if deployResCustom.Limits.Cpu().String() != "500m" {
		t.Errorf("custom CPU limit = %v, want 500m", deployResCustom.Limits.Cpu())
	}
}

func TestAddKubeconfigVolume(t *testing.T) {
	tests := []struct {
		name       string
		secretName string
		secretKey  string
		job        *batchv1.Job
		want       int // number of volumes expected
	}{
		{
			name:       "no kubeconfig",
			secretName: "",
			secretKey:  "",
			job: &batchv1.Job{
				Spec: batchv1.JobSpec{
					Template: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							Volumes: []corev1.Volume{},
							Containers: []corev1.Container{
								{VolumeMounts: []corev1.VolumeMount{}},
							},
						},
					},
				},
			},
			want: 0,
		},
		{
			name:       "with kubeconfig",
			secretName: "my-kubeconfig",
			secretKey:  "config",
			job: &batchv1.Job{
				Spec: batchv1.JobSpec{
					Template: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							Volumes: []corev1.Volume{},
							Containers: []corev1.Container{
								{VolumeMounts: []corev1.VolumeMount{}},
							},
						},
					},
				},
			},
			want: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actions.AddKubeconfigVolume(tt.job, tt.secretName, tt.secretKey)
			if len(tt.job.Spec.Template.Spec.Volumes) != tt.want {
				t.Errorf("AddKubeconfigVolume() volumes = %v, want %v", len(tt.job.Spec.Template.Spec.Volumes), tt.want)
			}
			if tt.want > 0 && len(tt.job.Spec.Template.Spec.Containers[0].VolumeMounts) != 1 {
				t.Errorf("AddKubeconfigVolume() volume mounts = %v, want 1", len(tt.job.Spec.Template.Spec.Containers[0].VolumeMounts))
			}
		})
	}
}

func TestGetUDSJobConfiguration(t *testing.T) {
	// Test OCI credentials
	bundleOCI := &udsv1alpha3.UDSBundleJob{
		Spec: udsv1alpha3.UDSBundleJobSpec{
			Publish: &udsv1alpha3.PublishConfig{
				Destination: udsv1alpha3.PublishDestination{
					Type: udsv1alpha3.DestinationTypeOCI,
					OCI: &udsv1alpha3.OCIDestination{
						Registry:   "registry.example.com",
						Repository: "test/bundle",
						Tag:        "v1.0.0",
						CredentialRef: &common.SecretReference{
							Name: "oci-creds",
						},
					},
				},
			},
		},
	}

	config, err := destinations.GetUDSJobConfiguration(bundleOCI)
	if err != nil {
		t.Fatalf("GetUDSJobConfiguration() error = %v", err)
	}
	if len(config.Volumes) != 1 {
		t.Errorf("GetUDSJobConfiguration() OCI volumes = %v, want 1", len(config.Volumes))
	}
	if len(config.VolumeMounts) != 1 {
		t.Errorf("GetUDSJobConfiguration() OCI volume mounts = %v, want 1", len(config.VolumeMounts))
	}

	// Test S3 credentials
	bundleS3 := &udsv1alpha3.UDSBundleJob{
		Spec: udsv1alpha3.UDSBundleJobSpec{
			Publish: &udsv1alpha3.PublishConfig{
				Destination: udsv1alpha3.PublishDestination{
					Type: udsv1alpha3.DestinationTypeS3,
					S3: &udsv1alpha3.S3Destination{
						Bucket:    "my-bucket",
						KeyPrefix: "bundles/",
						Region:    "us-west-2",
						CredentialRef: &common.AWSCredentialRef{
							Name: "s3-creds",
						},
					},
				},
			},
		},
	}

	configS3, err := destinations.GetUDSJobConfiguration(bundleS3)
	if err != nil {
		t.Fatalf("GetUDSJobConfiguration() S3 error = %v", err)
	}
	// S3 credentials are added as env vars, not volumes
	if len(configS3.Volumes) != 0 {
		t.Errorf("GetUDSJobConfiguration() S3 volumes = %v, want 0 (S3 uses env vars)", len(configS3.Volumes))
	}
	// Should have AWS_REGION and 2 credential env vars
	if len(configS3.Env) < 3 {
		t.Errorf("GetUDSJobConfiguration() S3 env vars = %v, want >= 3", len(configS3.Env))
	}
}

// TestBuildInitContainers tests init container generation
func TestBuildInitContainers(t *testing.T) {
	handler := &CreateHandler{
		kubeClient: kubefake.NewClientset(),
		metrics:    testhelpers.MustNewMetrics(),
		tracer:     telemetry.NewTracer(),
	}

	tests := []struct {
		name                    string
		bundle                  *udsv1alpha3.UDSBundleJob
		wantContainerCount      int
		wantGitAskpassInCmd     bool
		wantGitCredsVolumeMount bool
		wantErr                 bool
	}{
		{
			name: "git source without credentials",
			bundle: &udsv1alpha3.UDSBundleJob{
				Spec: udsv1alpha3.UDSBundleJobSpec{
					Source: udsv1alpha3.PackageSource{
						Type: udsv1alpha3.SourceTypeGit,
						Git: &udsv1alpha3.GitSource{
							URL: "https://github.com/test/repo",
							Ref: "main",
						},
					},
				},
			},
			wantContainerCount:      1,
			wantGitAskpassInCmd:     false,
			wantGitCredsVolumeMount: false,
		},
		{
			name: "git source with credentials",
			bundle: &udsv1alpha3.UDSBundleJob{
				Spec: udsv1alpha3.UDSBundleJobSpec{
					Source: udsv1alpha3.PackageSource{
						Type: udsv1alpha3.SourceTypeGit,
						Git: &udsv1alpha3.GitSource{
							URL: "https://github.com/test/private-repo",
							Ref: "main",
							CredentialRef: &common.SecretReference{
								Name: "git-creds",
							},
						},
					},
				},
			},
			wantContainerCount:      1,
			wantGitAskpassInCmd:     false,
			wantGitCredsVolumeMount: true,
		},
		{
			name: "git source with credentials disabled",
			bundle: &udsv1alpha3.UDSBundleJob{
				Spec: udsv1alpha3.UDSBundleJobSpec{
					Source: udsv1alpha3.PackageSource{
						Type: udsv1alpha3.SourceTypeGit,
						Git: &udsv1alpha3.GitSource{
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
			wantContainerCount:      1,
			wantGitAskpassInCmd:     true,
			wantGitCredsVolumeMount: false,
		},
		{
			name: "oci source",
			bundle: &udsv1alpha3.UDSBundleJob{
				Spec: udsv1alpha3.UDSBundleJobSpec{
					Source: udsv1alpha3.PackageSource{
						Type: udsv1alpha3.SourceTypeOCI,
					},
				},
			},
			wantContainerCount: 0,
			wantErr:            true, // OCI source not yet implemented
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			containers, err := handler.buildInitContainers(tt.bundle)
			if (err != nil) != tt.wantErr {
				t.Fatalf("buildInitContainers() error = %v, wantErr %v", err, tt.wantErr)
			}

			if tt.wantErr {
				return
			}

			if len(containers) != tt.wantContainerCount {
				t.Errorf("buildInitContainers() container count = %d, want %d", len(containers), tt.wantContainerCount)
				return
			}

			if tt.wantContainerCount > 0 {
				container := containers[0]

				// Check for GIT_ASKPASS in command
				hasGitAskpass := false
				for _, arg := range container.Args {
					if strings.Contains(arg, "GIT_ASKPASS=''") {
						hasGitAskpass = true
						break
					}
				}
				if hasGitAskpass != tt.wantGitAskpassInCmd {
					t.Errorf("GIT_ASKPASS in command = %v, want %v", hasGitAskpass, tt.wantGitAskpassInCmd)
				}

				// Check for git-creds volume mount
				hasGitCredsMount := false
				for _, vm := range container.VolumeMounts {
					if vm.Name == "git-creds" {
						hasGitCredsMount = true
						break
					}
				}
				if hasGitCredsMount != tt.wantGitCredsVolumeMount {
					t.Errorf("git-creds volume mount = %v, want %v", hasGitCredsMount, tt.wantGitCredsVolumeMount)
				}
			}
		})
	}
}

// TestBuildVolumes tests volume generation
func TestBuildVolumes(t *testing.T) {
	handler := &CreateHandler{
		kubeClient: kubefake.NewClientset(),
		metrics:    testhelpers.MustNewMetrics(),
		tracer:     telemetry.NewTracer(),
	}

	tests := []struct {
		name               string
		bundle             *udsv1alpha3.UDSBundleJob
		wantVolumeCount    int
		wantGitCredsVolume bool
	}{
		{
			name: "git source without credentials",
			bundle: &udsv1alpha3.UDSBundleJob{
				Spec: udsv1alpha3.UDSBundleJobSpec{
					Source: udsv1alpha3.PackageSource{
						Type: udsv1alpha3.SourceTypeGit,
						Git: &udsv1alpha3.GitSource{
							URL: "https://github.com/test/repo",
							Ref: "main",
						},
					},
				},
			},
			wantVolumeCount:    2, // workspace + output
			wantGitCredsVolume: false,
		},
		{
			name: "git source with credentials",
			bundle: &udsv1alpha3.UDSBundleJob{
				Spec: udsv1alpha3.UDSBundleJobSpec{
					Source: udsv1alpha3.PackageSource{
						Type: udsv1alpha3.SourceTypeGit,
						Git: &udsv1alpha3.GitSource{
							URL: "https://github.com/test/private-repo",
							Ref: "main",
							CredentialRef: &common.SecretReference{
								Name: "git-creds",
							},
						},
					},
				},
			},
			wantVolumeCount:    3, // workspace + output + git-creds
			wantGitCredsVolume: true,
		},
		{
			name: "git source with credentials disabled",
			bundle: &udsv1alpha3.UDSBundleJob{
				Spec: udsv1alpha3.UDSBundleJobSpec{
					Source: udsv1alpha3.PackageSource{
						Type: udsv1alpha3.SourceTypeGit,
						Git: &udsv1alpha3.GitSource{
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
			wantVolumeCount:    2, // workspace + output (no git-creds)
			wantGitCredsVolume: false,
		},
		{
			name: "oci source",
			bundle: &udsv1alpha3.UDSBundleJob{
				Spec: udsv1alpha3.UDSBundleJobSpec{
					Source: udsv1alpha3.PackageSource{
						Type: udsv1alpha3.SourceTypeOCI,
					},
				},
			},
			wantVolumeCount:    2, // workspace + output
			wantGitCredsVolume: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			volumes := handler.buildVolumes(tt.bundle)

			if len(volumes) != tt.wantVolumeCount {
				t.Errorf("buildVolumes() volume count = %d, want %d", len(volumes), tt.wantVolumeCount)
			}

			hasGitCredsVolume := false
			for _, vol := range volumes {
				if vol.Name == "git-creds" {
					hasGitCredsVolume = true
					break
				}
			}
			if hasGitCredsVolume != tt.wantGitCredsVolume {
				t.Errorf("git-creds volume present = %v, want %v", hasGitCredsVolume, tt.wantGitCredsVolume)
			}
		})
	}
}

func TestDeployHandlerExecute_ExternalClusterWithContext(t *testing.T) {
	kubeClient := kubefake.NewClientset()
	dynamicClient := fake.NewSimpleDynamicClient(runtime.NewScheme())
	handler := NewDeployHandler(kubeClient, dynamicClient, testhelpers.MustNewMetrics(), telemetry.NewTracer())

	bundle := &udsv1alpha3.UDSBundleJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-deploy-external-context",
			Namespace: "default",
		},
		Spec: udsv1alpha3.UDSBundleJobSpec{
			ServiceAccountName: "test-sa",
			Action:             udsv1alpha3.ActionDeploy,
			Source: udsv1alpha3.PackageSource{
				Type: udsv1alpha3.SourceTypeLocal,
				Local: &udsv1alpha3.LocalSource{
					Path:    "/tmp/bundle.tar.zst",
					DevMode: true,
				},
			},
			Deploy: &udsv1alpha3.DeployConfig{
				Target: udsv1alpha3.DeployTargetExternalCluster,
				ExternalCluster: &common.ExternalClusterConfig{
					SecretRef: common.SecretReference{ // pragma: allowlist secret
						Name: "external-kubeconfig",
					},
					Context: "production-cluster",
				},
			},
		},
	}

	result, err := handler.Execute(context.Background(), bundle, "", "")
	if err != nil {
		t.Fatalf("Execute() unexpected error = %v", err)
	}
	if result == nil {
		t.Fatal("Execute() returned nil result")
	}

	// Verify the job was created
	job, err := kubeClient.BatchV1().Jobs("default").Get(context.Background(), result.JobName, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Failed to get job: %v", err)
	}

	// Verify the command includes the context flag
	container := job.Spec.Template.Spec.Containers[0]
	cmdArgs := container.Args[0]
	if !strings.Contains(cmdArgs, "--kubeconfig-context production-cluster") {
		t.Errorf("Expected command to contain '--kubeconfig-context production-cluster', got: %s", cmdArgs)
	}

	// Verify kubeconfig is exported
	if !strings.Contains(cmdArgs, "export KUBECONFIG=") {
		t.Errorf("Expected command to export KUBECONFIG, got: %s", cmdArgs)
	}
}

func TestCreateHandlerBuildUDSCommandWithVariables(t *testing.T) {
	handler := &CreateHandler{}

	tests := []struct {
		name            string
		bundle          *udsv1alpha3.UDSBundleJob
		artifactPVCName string
		wantContains    []string
		wantNotContains []string
	}{
		{
			name: "create without variables",
			bundle: &udsv1alpha3.UDSBundleJob{
				Spec: udsv1alpha3.UDSBundleJobSpec{
					Source: udsv1alpha3.PackageSource{
						Type: udsv1alpha3.SourceTypeGit,
						Git: &udsv1alpha3.GitSource{
							URL: "https://github.com/test/repo",
						},
					},
				},
			},
			artifactPVCName: "",
			wantContains:    []string{"uds create", "--confirm", "--output-directory"},
			wantNotContains: []string{"--set"},
		},
		{
			name: "create with single variable",
			bundle: &udsv1alpha3.UDSBundleJob{
				Spec: udsv1alpha3.UDSBundleJobSpec{
					Source: udsv1alpha3.PackageSource{
						Type: udsv1alpha3.SourceTypeGit,
						Git: &udsv1alpha3.GitSource{
							URL: "https://github.com/test/repo",
						},
					},
					Create: &udsv1alpha3.CreateConfig{
						Variables: map[string]string{
							"BUNDLE_VERSION": "2.0.0",
						},
					},
				},
			},
			artifactPVCName: "",
			wantContains:    []string{"uds create", "--set BUNDLE_VERSION=2.0.0"},
		},
		{
			name: "create with multiple variables",
			bundle: &udsv1alpha3.UDSBundleJob{
				Spec: udsv1alpha3.UDSBundleJobSpec{
					Source: udsv1alpha3.PackageSource{
						Type: udsv1alpha3.SourceTypeGit,
						Git: &udsv1alpha3.GitSource{
							URL: "https://github.com/test/repo",
						},
					},
					Create: &udsv1alpha3.CreateConfig{
						Variables: map[string]string{
							"BUNDLE_VERSION": "2.0.0",
							"ENVIRONMENT":    "production",
						},
					},
				},
			},
			artifactPVCName: "",
			wantContains:    []string{"uds create", "--set BUNDLE_VERSION=2.0.0", "--set ENVIRONMENT=production"},
		},
		{
			name: "create with PVC and variables",
			bundle: &udsv1alpha3.UDSBundleJob{
				Spec: udsv1alpha3.UDSBundleJobSpec{
					Source: udsv1alpha3.PackageSource{
						Type: udsv1alpha3.SourceTypeGit,
						Git: &udsv1alpha3.GitSource{
							URL: "https://github.com/test/repo",
						},
					},
					Create: &udsv1alpha3.CreateConfig{
						Variables: map[string]string{
							"VERSION": "3.0.0",
						},
					},
				},
			},
			artifactPVCName: "my-pvc",
			wantContains:    []string{"uds create", "/artifacts", "--set VERSION=3.0.0"},
		},
		{
			name: "create with empty variables map",
			bundle: &udsv1alpha3.UDSBundleJob{
				Spec: udsv1alpha3.UDSBundleJobSpec{
					Source: udsv1alpha3.PackageSource{
						Type: udsv1alpha3.SourceTypeGit,
						Git: &udsv1alpha3.GitSource{
							URL: "https://github.com/test/repo",
						},
					},
					Create: &udsv1alpha3.CreateConfig{
						Variables: map[string]string{},
					},
				},
			},
			artifactPVCName: "",
			wantContains:    []string{"uds create"},
			wantNotContains: []string{"--set"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd, workingDir := handler.buildUDSCommand(tt.bundle, tt.artifactPVCName)

			if workingDir != "/workspace" {
				t.Errorf("buildUDSCommand() workingDir = %v, want /workspace", workingDir)
			}

			for _, want := range tt.wantContains {
				if !strings.Contains(cmd, want) {
					t.Errorf("buildUDSCommand() cmd = %q, want to contain %q", cmd, want)
				}
			}

			for _, notWant := range tt.wantNotContains {
				if strings.Contains(cmd, notWant) {
					t.Errorf("buildUDSCommand() cmd = %q, should not contain %q", cmd, notWant)
				}
			}
		})
	}
}
