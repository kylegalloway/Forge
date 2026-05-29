package actions

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	kubefake "k8s.io/client-go/kubernetes/fake"

	"github.com/kylegalloway/forge/pkg/constants"
)

// TestBuildActionJob_Basic verifies that BuildActionJob creates a Job with the
// expected name, namespace, container image, and command.
func TestBuildActionJob_Basic(t *testing.T) {
	kubeClient := kubefake.NewClientset()

	owner := &metav1.ObjectMeta{
		Name:      "test-resource",
		Namespace: "default",
		UID:       "test-uid",
	}

	params := JobParams{
		JobName:       "test-job",
		Namespace:     "default",
		CLIImage:      "test-image:latest",
		ContainerUID:  1000,
		ContainerName: "test-container",
		Args:          []string{"echo hello"},
		Labels: map[string]string{
			"app": "test",
		},
		OwnerRef: owner,
		OwnerGVK: schema.GroupVersionKind{
			Group:   "test.io",
			Version: "v1",
			Kind:    "TestKind",
		},
		Resources:   BuildResourceRequirements(),
		Timeout:     3600,
		VolumeSizes: nil,
		DebugMode:   false,
		KubeClient:  kubeClient,
	}

	job, err := BuildActionJob(context.Background(), params)
	if err != nil {
		t.Fatalf("BuildActionJob() error = %v", err)
	}
	if job == nil {
		t.Fatal("BuildActionJob() returned nil job")
	}
	if job.Name != "test-job" {
		t.Errorf("job.Name = %q, want %q", job.Name, "test-job")
	}
	if job.Namespace != "default" {
		t.Errorf("job.Namespace = %q, want %q", job.Namespace, "default")
	}

	containers := job.Spec.Template.Spec.Containers
	if len(containers) != 1 {
		t.Fatalf("expected 1 container, got %d", len(containers))
	}
	if containers[0].Image != "test-image:latest" {
		t.Errorf("container.Image = %q, want %q", containers[0].Image, "test-image:latest")
	}
	if containers[0].Name != "test-container" {
		t.Errorf("container.Name = %q, want %q", containers[0].Name, "test-container")
	}
}

// TestBuildActionJob_IdempotentCreateOrGet verifies that calling BuildActionJob twice
// for the same job name returns the existing job without error.
func TestBuildActionJob_IdempotentCreateOrGet(t *testing.T) {
	kubeClient := kubefake.NewClientset()

	params := minimalJobParams(kubeClient, "idempotent-job")

	job1, err := BuildActionJob(context.Background(), params)
	if err != nil {
		t.Fatalf("first call error = %v", err)
	}

	job2, err := BuildActionJob(context.Background(), params)
	if err != nil {
		t.Fatalf("second call error = %v", err)
	}

	if job1.Name != job2.Name {
		t.Errorf("job names differ: %q vs %q", job1.Name, job2.Name)
	}
}

// TestBuildActionJob_ArtifactPVC verifies that an artifact PVC is added as a volume
// when ArtifactPVCName is set.
func TestBuildActionJob_ArtifactPVC(t *testing.T) {
	kubeClient := kubefake.NewClientset()
	params := minimalJobParams(kubeClient, "pvc-job")
	params.ArtifactPVCName = "my-artifacts-pvc"

	job, err := BuildActionJob(context.Background(), params)
	if err != nil {
		t.Fatalf("BuildActionJob() error = %v", err)
	}

	foundVolume := false
	for _, vol := range job.Spec.Template.Spec.Volumes {
		if vol.Name == "artifacts" && vol.PersistentVolumeClaim != nil {
			if vol.PersistentVolumeClaim.ClaimName == "my-artifacts-pvc" {
				foundVolume = true
			}
		}
	}
	if !foundVolume {
		t.Error("artifacts PVC volume not found in job spec")
	}
}

// TestBuildActionJob_InClusterKubeconfig verifies that the kubeconfig init container
// and volume are added when UseInClusterKubeconfig is true.
func TestBuildActionJob_InClusterKubeconfig(t *testing.T) {
	kubeClient := kubefake.NewClientset()
	params := minimalJobParams(kubeClient, "incluster-job")
	params.UseInClusterKubeconfig = true
	params.ServiceAccountName = "deploy-sa"

	job, err := BuildActionJob(context.Background(), params)
	if err != nil {
		t.Fatalf("BuildActionJob() error = %v", err)
	}

	foundKubeconfigVol := false
	for _, vol := range job.Spec.Template.Spec.Volumes {
		if vol.Name == constants.VolumeNameKubeconfig {
			foundKubeconfigVol = true
		}
	}
	if !foundKubeconfigVol {
		t.Error("kubeconfig volume not found in job spec")
	}

	foundKubeconfigInit := false
	for _, c := range job.Spec.Template.Spec.InitContainers {
		if c.Name == "kubeconfig-init" {
			foundKubeconfigInit = true
		}
	}
	if !foundKubeconfigInit {
		t.Error("kubeconfig-init init container not found")
	}

	foundKubeconfigEnv := false
	for _, env := range job.Spec.Template.Spec.Containers[0].Env {
		if env.Name == "KUBECONFIG" {
			foundKubeconfigEnv = true
		}
	}
	if !foundKubeconfigEnv {
		t.Error("KUBECONFIG env var not found in main container")
	}
}

// TestBuildActionJob_ExternalKubeconfig verifies that an external kubeconfig secret
// volume is added when KubeconfigSecretName is set.
func TestBuildActionJob_ExternalKubeconfig(t *testing.T) {
	kubeClient := kubefake.NewClientset()
	params := minimalJobParams(kubeClient, "external-job")
	params.KubeconfigSecretName = "external-kubeconfig" // pragma: allowlist secret
	params.KubeconfigKey = "config"

	job, err := BuildActionJob(context.Background(), params)
	if err != nil {
		t.Fatalf("BuildActionJob() error = %v", err)
	}

	foundKubeconfigVol := false
	for _, vol := range job.Spec.Template.Spec.Volumes {
		if vol.Name == constants.VolumeNameKubeconfig {
			if vol.Secret != nil && vol.Secret.SecretName == "external-kubeconfig" { // pragma: allowlist secret
				foundKubeconfigVol = true
			}
		}
	}
	if !foundKubeconfigVol {
		t.Error("external kubeconfig volume not found with correct secret name")
	}
}

// TestBuildActionJob_SourceCredentials verifies that source credential volumes
// are added to the job when provided.
func TestBuildActionJob_SourceCredentials(t *testing.T) {
	kubeClient := kubefake.NewClientset()
	params := minimalJobParams(kubeClient, "cred-job")
	params.SourceOCICredSecret = "oci-creds" // pragma: allowlist secret
	params.SourceS3CredVol = &corev1.Volume{
		Name: "s3-creds",
		VolumeSource: corev1.VolumeSource{
			Secret: &corev1.SecretVolumeSource{SecretName: "s3-secret"}, // pragma: allowlist secret
		},
	}

	job, err := BuildActionJob(context.Background(), params)
	if err != nil {
		t.Fatalf("BuildActionJob() error = %v", err)
	}

	foundOCI := false
	foundS3 := false
	for _, vol := range job.Spec.Template.Spec.Volumes {
		if vol.Name == "docker-config" {
			foundOCI = true
		}
		if vol.Name == "s3-creds" {
			foundS3 = true
		}
	}
	if !foundOCI {
		t.Error("OCI docker-config volume not found")
	}
	if !foundS3 {
		t.Error("S3 credentials volume not found")
	}
}

// TestBuildActionJob_EnvVars verifies that extra env vars are passed through.
func TestBuildActionJob_EnvVars(t *testing.T) {
	kubeClient := kubefake.NewClientset()
	params := minimalJobParams(kubeClient, "env-job")
	params.EnvVars = []corev1.EnvVar{
		{Name: "MY_VAR", Value: "my-value"},
	}

	job, err := BuildActionJob(context.Background(), params)
	if err != nil {
		t.Fatalf("BuildActionJob() error = %v", err)
	}

	found := false
	for _, env := range job.Spec.Template.Spec.Containers[0].Env {
		if env.Name == "MY_VAR" && env.Value == "my-value" {
			found = true
		}
	}
	if !found {
		t.Error("MY_VAR env var not found in container env")
	}
}

// TestBuildActionJob_BackoffLimit verifies that MaxRetries is translated correctly.
func TestBuildActionJob_BackoffLimit(t *testing.T) {
	kubeClient := kubefake.NewClientset()

	t.Run("nil MaxRetries gives BackoffLimit 0", func(t *testing.T) {
		params := minimalJobParams(kubeClient, "retry-nil-job")
		params.MaxRetries = nil

		job, err := BuildActionJob(context.Background(), params)
		if err != nil {
			t.Fatalf("BuildActionJob() error = %v", err)
		}
		if job.Spec.BackoffLimit == nil || *job.Spec.BackoffLimit != 0 {
			t.Errorf("BackoffLimit = %v, want 0", job.Spec.BackoffLimit)
		}
	})

	t.Run("MaxRetries=3 gives BackoffLimit 3", func(t *testing.T) {
		params := minimalJobParams(kubeClient, "retry-3-job")
		retries := int32(3)
		params.MaxRetries = &retries

		job, err := BuildActionJob(context.Background(), params)
		if err != nil {
			t.Fatalf("BuildActionJob() error = %v", err)
		}
		if job.Spec.BackoffLimit == nil || *job.Spec.BackoffLimit != 3 {
			t.Errorf("BackoffLimit = %v, want 3", job.Spec.BackoffLimit)
		}
	})
}

// TestActionResultFromJob verifies the helper returns expected fields.
func TestActionResultFromJob(t *testing.T) {
	kubeClient := kubefake.NewClientset()
	params := minimalJobParams(kubeClient, "result-job")
	job, err := BuildActionJob(context.Background(), params)
	if err != nil {
		t.Fatalf("BuildActionJob() error = %v", err)
	}

	result := ActionResultFromJob(job, "test message")
	if result == nil {
		t.Fatal("ActionResultFromJob() returned nil")
	}
	if result.JobName != "result-job" {
		t.Errorf("result.JobName = %q, want %q", result.JobName, "result-job")
	}
	if result.Phase != "Running" {
		t.Errorf("result.Phase = %q, want Running", result.Phase)
	}
	if result.Message != "test message" {
		t.Errorf("result.Message = %q, want %q", result.Message, "test message")
	}
	if result.Completed {
		t.Error("result.Completed should be false")
	}
	if result.StartTime.IsZero() {
		t.Error("result.StartTime should not be zero")
	}
}

// minimalJobParams returns a JobParams with the bare minimum set to create a Job.
func minimalJobParams(kubeClient *kubefake.Clientset, jobName string) JobParams {
	owner := &metav1.ObjectMeta{
		Name:      "owner",
		Namespace: "default",
		UID:       "owner-uid",
	}
	return JobParams{
		JobName:       jobName,
		Namespace:     "default",
		CLIImage:      "test-image:latest",
		ContainerUID:  1000,
		ContainerName: "test-container",
		Args:          []string{"echo hello"},
		Labels:        map[string]string{"app": "test"},
		OwnerRef:      owner,
		OwnerGVK: schema.GroupVersionKind{
			Group:   "test.io",
			Version: "v1",
			Kind:    "TestKind",
		},
		Resources:  BuildResourceRequirements(),
		Timeout:    3600,
		KubeClient: kubeClient,
	}
}
