package uds

import (
	"context"
	"strings"
	"testing"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/kylegalloway/forge/pkg/actions"
	udsv1alpha2 "github.com/kylegalloway/forge/pkg/apis/uds/v1alpha2"
	"github.com/kylegalloway/forge/pkg/telemetry"
)

// mustNewMetrics creates metrics instance for testing
func mustNewMetrics() *telemetry.Metrics {
	m, err := telemetry.NewMetrics()
	if err != nil {
		panic(err)
	}
	return m
}

// TestNewCreateHandler tests CreateHandler initialization
func TestNewCreateHandler(t *testing.T) {
	kubeClient := fake.NewClientset()
	metrics := mustNewMetrics()
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
	kubeClient := fake.NewClientset()
	metrics := mustNewMetrics()
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
	kubeClient := fake.NewClientset()
	metrics := mustNewMetrics()
	tracer := telemetry.NewTracer()

	handler := NewDeployHandler(kubeClient, metrics, tracer)
	if handler == nil {
		t.Fatal("NewDeployHandler returned nil")
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

// TestCreateHandlerExecute tests CreateHandler.Execute
func TestCreateHandlerExecute(t *testing.T) {
	kubeClient := fake.NewClientset()
	handler := NewCreateHandler(kubeClient, mustNewMetrics(), telemetry.NewTracer())

	tests := []struct {
		name    string
		bundle  *udsv1alpha2.UDSPackageJob
		wantErr bool
	}{
		{
			name: "create with git source",
			bundle: &udsv1alpha2.UDSPackageJob{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-create",
					Namespace: "default",
				},
				Spec: udsv1alpha2.UDSPackageJobSpec{
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
			},
			wantErr: false,
		},
		{
			name: "create with oci source",
			bundle: &udsv1alpha2.UDSPackageJob{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-create-oci",
					Namespace: "default",
				},
				Spec: udsv1alpha2.UDSPackageJobSpec{
					ServiceAccountName: "test-sa",
					Action:             udsv1alpha2.ActionCreate,
					Source: udsv1alpha2.PackageSource{
						Type: udsv1alpha2.SourceTypeOCI,
						OCI: &udsv1alpha2.OCISource{
							Reference: "ghcr.io/test/bundles:v1.0.0",
						},
					},
				},
			},
			wantErr: false, // OCI source now implemented
		},
		{
			name: "create with local source",
			bundle: &udsv1alpha2.UDSPackageJob{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-create-local",
					Namespace: "default",
				},
				Spec: udsv1alpha2.UDSPackageJobSpec{
					ServiceAccountName: "test-sa",
					Action:             udsv1alpha2.ActionCreate,
					Source: udsv1alpha2.PackageSource{
						Type: udsv1alpha2.SourceTypeLocal,
						Local: &udsv1alpha2.LocalSource{
							Path: "/workspace/bundle",
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "create without source type",
			bundle: &udsv1alpha2.UDSPackageJob{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-create-no-source",
					Namespace: "default",
				},
				Spec: udsv1alpha2.UDSPackageJobSpec{
					ServiceAccountName: "test-sa",
					Action:             udsv1alpha2.ActionCreate,
					Source:             udsv1alpha2.PackageSource{},
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := handler.Execute(context.Background(), tt.bundle)
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
	kubeClient := fake.NewClientset()
	handler := NewPublishHandler(kubeClient, mustNewMetrics(), telemetry.NewTracer())

	tests := []struct {
		name    string
		bundle  *udsv1alpha2.UDSPackageJob
		wantErr bool
	}{
		{
			name: "publish to oci",
			bundle: &udsv1alpha2.UDSPackageJob{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-publish",
					Namespace: "default",
				},
				Spec: udsv1alpha2.UDSPackageJobSpec{
					ServiceAccountName: "test-sa",
					Action:             udsv1alpha2.ActionPublish,
					Source: udsv1alpha2.PackageSource{
						Type: udsv1alpha2.SourceTypeLocal,
						Local: &udsv1alpha2.LocalSource{
							Path: "/workspace/bundle",
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
			},
			wantErr: false,
		},
		{
			name: "publish to s3",
			bundle: &udsv1alpha2.UDSPackageJob{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-publish-s3",
					Namespace: "default",
				},
				Spec: udsv1alpha2.UDSPackageJobSpec{
					ServiceAccountName: "test-sa",
					Action:             udsv1alpha2.ActionPublish,
					Source: udsv1alpha2.PackageSource{
						Type: udsv1alpha2.SourceTypeLocal,
						Local: &udsv1alpha2.LocalSource{
							Path: "/workspace/bundle",
						},
					},
					Publish: &udsv1alpha2.PublishConfig{
						Destination: udsv1alpha2.PublishDestination{
							Type: udsv1alpha2.DestinationTypeS3,
							S3: &udsv1alpha2.S3Destination{
								Bucket: "my-bundles",
								Key:    "uds/bundle.tar.zst",
								Region: "us-east-1",
							},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "publish without destination",
			bundle: &udsv1alpha2.UDSPackageJob{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-publish-no-dest",
					Namespace: "default",
				},
				Spec: udsv1alpha2.UDSPackageJobSpec{
					ServiceAccountName: "test-sa",
					Action:             udsv1alpha2.ActionPublish,
					Source: udsv1alpha2.PackageSource{
						Type: udsv1alpha2.SourceTypeLocal,
						Local: &udsv1alpha2.LocalSource{
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
			result, err := handler.Execute(context.Background(), tt.bundle)
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
	kubeClient := fake.NewClientset()
	handler := NewDeployHandler(kubeClient, mustNewMetrics(), telemetry.NewTracer())

	tests := []struct {
		name    string
		bundle  *udsv1alpha2.UDSPackageJob
		wantErr bool
	}{
		{
			name: "deploy to in-cluster",
			bundle: &udsv1alpha2.UDSPackageJob{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-deploy",
					Namespace: "default",
				},
				Spec: udsv1alpha2.UDSPackageJobSpec{
					ServiceAccountName: "test-sa",
					Action:             udsv1alpha2.ActionDeploy,
					Source: udsv1alpha2.PackageSource{
						Type: udsv1alpha2.SourceTypeOCI,
						OCI: &udsv1alpha2.OCISource{
							Reference: "ghcr.io/test/bundles:v1.0.0",
						},
					},
					Deploy: &udsv1alpha2.DeployConfig{
						Target: udsv1alpha2.DeployTargetInCluster,
					},
				},
			},
			wantErr: false,
		},
		{
			name: "deploy with specific packages",
			bundle: &udsv1alpha2.UDSPackageJob{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-deploy-packages",
					Namespace: "default",
				},
				Spec: udsv1alpha2.UDSPackageJobSpec{
					ServiceAccountName: "test-sa",
					Action:             udsv1alpha2.ActionDeploy,
					Source: udsv1alpha2.PackageSource{
						Type: udsv1alpha2.SourceTypeOCI,
						OCI: &udsv1alpha2.OCISource{
							Reference: "ghcr.io/test/bundles:v1.0.0",
						},
					},
					Deploy: &udsv1alpha2.DeployConfig{
						Target:     udsv1alpha2.DeployTargetInCluster,
						Components: []string{"package1", "package2"},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "deploy without deploy spec",
			bundle: &udsv1alpha2.UDSPackageJob{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-deploy-no-spec",
					Namespace: "default",
				},
				Spec: udsv1alpha2.UDSPackageJobSpec{
					ServiceAccountName: "test-sa",
					Action:             udsv1alpha2.ActionDeploy,
					Source: udsv1alpha2.PackageSource{
						Type: udsv1alpha2.SourceTypeOCI,
						OCI: &udsv1alpha2.OCISource{
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
			result, err := handler.Execute(context.Background(), tt.bundle)
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
		bundle     *udsv1alpha2.UDSPackageJob
		wantCmd    []string
		wantDir    string
		wantErr    bool
		checkFlags []string // flags that should be present
	}{
		{
			name: "git source",
			bundle: &udsv1alpha2.UDSPackageJob{
				Spec: udsv1alpha2.UDSPackageJobSpec{
					Source: udsv1alpha2.PackageSource{
						Type: udsv1alpha2.SourceTypeGit,
						Git: &udsv1alpha2.GitSource{
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
			bundle: &udsv1alpha2.UDSPackageJob{
				Spec: udsv1alpha2.UDSPackageJobSpec{
					Source: udsv1alpha2.PackageSource{
						Type: udsv1alpha2.SourceTypeLocal,
						Local: &udsv1alpha2.LocalSource{
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
			bundle: &udsv1alpha2.UDSPackageJob{
				Spec: udsv1alpha2.UDSPackageJobSpec{
					Source: udsv1alpha2.PackageSource{
						Type: udsv1alpha2.SourceTypeOCI,
						OCI: &udsv1alpha2.OCISource{
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
			cmd, workingDir, err := handler.buildUDSCommand(tt.bundle)
			if (err != nil) != tt.wantErr {
				t.Errorf("buildUDSCommand() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
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
	bundle := &udsv1alpha2.UDSPackageJob{
		Spec: udsv1alpha2.UDSPackageJobSpec{},
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
	handler := &DeployHandler{}

	tests := []struct {
		name   string
		bundle *udsv1alpha2.UDSPackageJob
		job    *batchv1.Job
		want   int // number of volumes expected
	}{
		{
			name: "no kubeconfig",
			bundle: &udsv1alpha2.UDSPackageJob{
				Spec: udsv1alpha2.UDSPackageJobSpec{
					Deploy: &udsv1alpha2.DeployConfig{},
				},
			},
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
			name: "with kubeconfig",
			bundle: &udsv1alpha2.UDSPackageJob{
				Spec: udsv1alpha2.UDSPackageJobSpec{
					Deploy: &udsv1alpha2.DeployConfig{
						Kubeconfig: &udsv1alpha2.KubeconfigReference{
							SecretRef: corev1.SecretReference{
								Name: "my-kubeconfig",
							},
							Key: "config",
						},
					},
				},
			},
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
			handler.addKubeconfigVolume(tt.bundle, tt.job)
			if len(tt.job.Spec.Template.Spec.Volumes) != tt.want {
				t.Errorf("addKubeconfigVolume() volumes = %v, want %v", len(tt.job.Spec.Template.Spec.Volumes), tt.want)
			}
			if tt.want > 0 && len(tt.job.Spec.Template.Spec.Containers[0].VolumeMounts) != 1 {
				t.Errorf("addKubeconfigVolume() volume mounts = %v, want 1", len(tt.job.Spec.Template.Spec.Containers[0].VolumeMounts))
			}
		})
	}
}

func TestAddCredentialVolumes(t *testing.T) {
	handler := &PublishHandler{}

	job := &batchv1.Job{
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
	}

	bundle := &udsv1alpha2.UDSPackageJob{
		Spec: udsv1alpha2.UDSPackageJobSpec{
			Publish: &udsv1alpha2.PublishConfig{
				Destination: udsv1alpha2.PublishDestination{
					Type: udsv1alpha2.DestinationTypeOCI,
					OCI: &udsv1alpha2.OCIDestination{
						CredentialsSecretRef: &corev1.SecretReference{
							Name: "oci-creds",
						},
					},
				},
			},
		},
	}

	handler.addCredentialVolumes(bundle, job)
	if len(job.Spec.Template.Spec.Volumes) != 1 {
		t.Errorf("addCredentialVolumes() volumes = %v, want 1", len(job.Spec.Template.Spec.Volumes))
	}
	if len(job.Spec.Template.Spec.Containers[0].VolumeMounts) != 1 {
		t.Errorf("addCredentialVolumes() volume mounts = %v, want 1", len(job.Spec.Template.Spec.Containers[0].VolumeMounts))
	}

	// Test S3 credentials
	job2 := &batchv1.Job{
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
	}

	bundle2 := &udsv1alpha2.UDSPackageJob{
		Spec: udsv1alpha2.UDSPackageJobSpec{
			Publish: &udsv1alpha2.PublishConfig{
				Destination: udsv1alpha2.PublishDestination{
					Type: udsv1alpha2.DestinationTypeS3,
					S3: &udsv1alpha2.S3Destination{
						CredentialsSecretRef: &corev1.SecretReference{
							Name: "s3-creds",
						},
					},
				},
			},
		},
	}

	handler.addCredentialVolumes(bundle2, job2)
	// S3 credentials are not added as volumes by this function
	if len(job2.Spec.Template.Spec.Volumes) != 0 {
		t.Errorf("addCredentialVolumes() S3 volumes = %v, want 0 (S3 uses env vars, not volumes)", len(job2.Spec.Template.Spec.Volumes))
	}
}

// TestBuildInitContainers tests init container generation
func TestBuildInitContainers(t *testing.T) {
	handler := &CreateHandler{
		kubeClient: fake.NewClientset(),
		metrics:    mustNewMetrics(),
		tracer:     telemetry.NewTracer(),
	}

	tests := []struct {
		name                    string
		bundle                  *udsv1alpha2.UDSPackageJob
		wantContainerCount      int
		wantGitAskpassInCmd     bool
		wantGitCredsVolumeMount bool
		wantErr                 bool
	}{
		{
			name: "git source without credentials",
			bundle: &udsv1alpha2.UDSPackageJob{
				Spec: udsv1alpha2.UDSPackageJobSpec{
					Source: udsv1alpha2.PackageSource{
						Type: udsv1alpha2.SourceTypeGit,
						Git: &udsv1alpha2.GitSource{
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
			bundle: &udsv1alpha2.UDSPackageJob{
				Spec: udsv1alpha2.UDSPackageJobSpec{
					Source: udsv1alpha2.PackageSource{
						Type: udsv1alpha2.SourceTypeGit,
						Git: &udsv1alpha2.GitSource{
							URL: "https://github.com/test/private-repo",
							Ref: "main",
							CredentialsSecretRef: &corev1.SecretReference{
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
			bundle: &udsv1alpha2.UDSPackageJob{
				Spec: udsv1alpha2.UDSPackageJobSpec{
					Source: udsv1alpha2.PackageSource{
						Type: udsv1alpha2.SourceTypeGit,
						Git: &udsv1alpha2.GitSource{
							URL: "https://github.com/test/public-repo",
							Ref: "main",
							CredentialsSecretRef: &corev1.SecretReference{
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
			bundle: &udsv1alpha2.UDSPackageJob{
				Spec: udsv1alpha2.UDSPackageJobSpec{
					Source: udsv1alpha2.PackageSource{
						Type: udsv1alpha2.SourceTypeOCI,
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
		kubeClient: fake.NewClientset(),
		metrics:    mustNewMetrics(),
		tracer:     telemetry.NewTracer(),
	}

	tests := []struct {
		name               string
		bundle             *udsv1alpha2.UDSPackageJob
		wantVolumeCount    int
		wantGitCredsVolume bool
	}{
		{
			name: "git source without credentials",
			bundle: &udsv1alpha2.UDSPackageJob{
				Spec: udsv1alpha2.UDSPackageJobSpec{
					Source: udsv1alpha2.PackageSource{
						Type: udsv1alpha2.SourceTypeGit,
						Git: &udsv1alpha2.GitSource{
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
			bundle: &udsv1alpha2.UDSPackageJob{
				Spec: udsv1alpha2.UDSPackageJobSpec{
					Source: udsv1alpha2.PackageSource{
						Type: udsv1alpha2.SourceTypeGit,
						Git: &udsv1alpha2.GitSource{
							URL: "https://github.com/test/private-repo",
							Ref: "main",
							CredentialsSecretRef: &corev1.SecretReference{
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
			bundle: &udsv1alpha2.UDSPackageJob{
				Spec: udsv1alpha2.UDSPackageJobSpec{
					Source: udsv1alpha2.PackageSource{
						Type: udsv1alpha2.SourceTypeGit,
						Git: &udsv1alpha2.GitSource{
							URL: "https://github.com/test/public-repo",
							Ref: "main",
							CredentialsSecretRef: &corev1.SecretReference{
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
			bundle: &udsv1alpha2.UDSPackageJob{
				Spec: udsv1alpha2.UDSPackageJobSpec{
					Source: udsv1alpha2.PackageSource{
						Type: udsv1alpha2.SourceTypeOCI,
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
