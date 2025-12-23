package uds

import (
	"context"
	"fmt"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"

	udsv1alpha1 "github.com/kylegalloway/forge/pkg/apis/uds/v1alpha1"
	"github.com/kylegalloway/forge/pkg/constants"
	"github.com/kylegalloway/forge/pkg/telemetry"
	"github.com/kylegalloway/forge/pkg/util"
)

// CreateHandler handles Create actions for UDSBundleJob resources
type CreateHandler struct {
	kubeClient kubernetes.Interface
	metrics    *telemetry.Metrics
	tracer     *telemetry.Tracer
}

// NewCreateHandler creates a new CreateHandler
func NewCreateHandler(kubeClient kubernetes.Interface, metrics *telemetry.Metrics, tracer *telemetry.Tracer) *CreateHandler {
	return &CreateHandler{
		kubeClient: kubeClient,
		metrics:    metrics,
		tracer:     tracer,
	}
}

// Execute performs a Create action for the given UDSBundleJob
func (handler *CreateHandler) Execute(ctx context.Context, bundle *udsv1alpha1.UDSBundleJob) (*ActionResult, error) {

	klog.InfoS("Executing UDS Bundle Create action", "name", bundle.Name, "namespace", bundle.Namespace)

	// Record create started
	handler.metrics.RecordBundleCreateStarted(ctx, bundle.Namespace, bundle.Name)

	// Validate source is provided
	if bundle.Spec.Source.Type == "" {
		handler.metrics.RecordBundleCreateFailed(ctx, bundle.Namespace, bundle.Name)
		return nil, fmt.Errorf("source type is required for Create action")
	}

	// Create Kubernetes Job to create the bundle
	job, err := handler.createBundleJob(ctx, bundle)
	if err != nil {
		handler.metrics.RecordBundleCreateFailed(ctx, bundle.Namespace, bundle.Name)
		return nil, fmt.Errorf("failed to create bundle job: %w", err)
	}

	// Record Job creation
	handler.metrics.RecordBundleJobCreated(ctx, bundle.Namespace, bundle.Name, "create")

	klog.InfoS("Bundle create job created", "name", bundle.Name, "job", job.Name)

	result := &ActionResult{
		JobName:   job.Name,
		Phase:     "Running",
		Message:   fmt.Sprintf("Bundle create job %s created", job.Name),
		StartTime: metav1.Now(),
		Completed: false,
	}

	return result, nil
}

// createBundleJob creates a Kubernetes Job to create a UDS bundle
func (handler *CreateHandler) createBundleJob(ctx context.Context, bundle *udsv1alpha1.UDSBundleJob) (*batchv1.Job, error) {
	jobName := fmt.Sprintf("%s-create", bundle.Name)
	namespace := bundle.Namespace

	// Build UDS CLI command
	udsCmd, workingDir, err := handler.buildUDSCommand(bundle)
	if err != nil {
		return nil, err
	}

	// Build init containers for source retrieval
	initContainers := handler.buildInitContainers(bundle)

	// Job configuration
	backoffLimit := int32(0)             // Don't retry failed creates
	activeDeadlineSeconds := int64(7200) // 2 hour timeout (bundles can be large)

	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      jobName,
			Namespace: namespace,
			Labels: map[string]string{
				"app":                  "forge-uds",
				constants.LabelPackage: bundle.Name,
				constants.LabelAction:  "create",
			},
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(bundle, udsv1alpha1.SchemeGroupVersion.WithKind("UDSBundleJob")),
			},
		},
		Spec: batchv1.JobSpec{
			BackoffLimit:            &backoffLimit,
			ActiveDeadlineSeconds:   &activeDeadlineSeconds,
			TTLSecondsAfterFinished: util.Ptr(int32(3600)), // Clean up after 1 hour
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app":                  "forge-uds",
						constants.LabelPackage: bundle.Name,
						constants.LabelAction:  "create",
					},
				},
				Spec: corev1.PodSpec{
					RestartPolicy:      corev1.RestartPolicyNever,
					ServiceAccountName: bundle.Spec.ServiceAccountName,
					InitContainers:     initContainers,
					Containers: []corev1.Container{
						{
							Name:       "uds-create",
							Image:      constants.UDSCLIImage,
							Command:    []string{"/bin/sh", "-c"},
							Args:       []string{udsCmd},
							WorkingDir: workingDir,
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "workspace",
									MountPath: "/workspace",
								},
								{
									Name:      "output",
									MountPath: "/output",
								},
							},
							SecurityContext: &corev1.SecurityContext{
								RunAsNonRoot:             util.Ptr(true),
								RunAsUser:                util.Ptr(int64(constants.DefaultUDSUID)),
								AllowPrivilegeEscalation: util.Ptr(false),
								Capabilities: &corev1.Capabilities{
									Drop: []corev1.Capability{"ALL"},
								},
								SeccompProfile: &corev1.SeccompProfile{
									Type: corev1.SeccompProfileTypeRuntimeDefault,
								},
							},
							Resources: handler.getResources(bundle),
						},
					},
					Volumes: handler.buildVolumes(bundle),
					SecurityContext: &corev1.PodSecurityContext{
						RunAsNonRoot: util.Ptr(true),
						RunAsUser:    util.Ptr(int64(constants.DefaultUDSUID)),
						FSGroup:      util.Ptr(int64(constants.DefaultUDSUID)),
						SeccompProfile: &corev1.SeccompProfile{
							Type: corev1.SeccompProfileTypeRuntimeDefault,
						},
					},
				},
			},
		},
	}

	// Check if job already exists
	existingJob, err := handler.kubeClient.BatchV1().Jobs(namespace).Get(ctx, jobName, metav1.GetOptions{})
	if err == nil {
		// Job already exists, return it
		klog.V(2).InfoS("Job already exists, reusing", "name", bundle.Name, "job", jobName)
		return existingJob, nil
	}

	// Create the job
	createdJob, err := handler.kubeClient.BatchV1().Jobs(namespace).Create(ctx, job, metav1.CreateOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to create job: %w", err)
	}

	return createdJob, nil
}

// buildUDSCommand builds the UDS CLI command for bundle creation
func (handler *CreateHandler) buildUDSCommand(_ *udsv1alpha1.UDSBundleJob) (string, string, error) {
	workingDir := "/workspace"

	// UDS bundle create command
	// Assumes uds-bundle.yaml is in the workspace root
	cmd := "uds create . --confirm --output-directory /output"

	return cmd, workingDir, nil
}

// buildInitContainers creates init containers for source retrieval
func (handler *CreateHandler) buildInitContainers(bundle *udsv1alpha1.UDSBundleJob) []corev1.Container {
	// UDS bundles currently use a simplified inline Git source handler.
	// Full integration with pkg/sources handlers is planned for future versions.

	// For Git sources, clone the repository directly
	if bundle.Spec.Source.Type == udsv1alpha1.BundleSourceTypeGit && bundle.Spec.Source.Git != nil {
		gitSource := bundle.Spec.Source.Git

		// Construct git clone command (with or without credentials)
		var cloneCmd string
		if gitSource.DisableCloneCredentials {
			cloneCmd = fmt.Sprintf("GIT_ASKPASS='' git clone --depth 1 --branch %s %s /workspace", gitSource.Ref, gitSource.URL)
		} else {
			cloneCmd = fmt.Sprintf("git clone --depth 1 --branch %s %s /workspace", gitSource.Ref, gitSource.URL)
		}

		if gitSource.Path != "" && gitSource.Path != "." {
			cloneCmd = fmt.Sprintf("%s && cd /workspace && mv %s/* . && rm -rf %s", cloneCmd, gitSource.Path, gitSource.Path)
		}

		cloneCmd = fmt.Sprintf("%s && cd /workspace && ls -la", cloneCmd)

		container := &corev1.Container{
			Name:    "fetch-source",
			Image:   "alpine/git:latest",
			Command: []string{"/bin/sh", "-c"},
			Args:    []string{cloneCmd},
			VolumeMounts: []corev1.VolumeMount{
				{
					Name:      "workspace",
					MountPath: "/workspace",
				},
			},
			SecurityContext: &corev1.SecurityContext{
				RunAsNonRoot:             util.Ptr(true),
				RunAsUser:                util.Ptr(int64(65532)),
				AllowPrivilegeEscalation: util.Ptr(false),
				Capabilities: &corev1.Capabilities{
					Drop: []corev1.Capability{"ALL"},
				},
			},
		}

		// Handle credentials if provided and not disabled  # pragma: allowlist secret
		if gitSource.CredentialsSecretRef != nil && !gitSource.DisableCloneCredentials { // pragma: allowlist secret
			// Mount secret to /etc/git-secret  # pragma: allowlist secret
			container.VolumeMounts = append(container.VolumeMounts, corev1.VolumeMount{
				Name:      "git-creds",
				MountPath: "/etc/git-secret",
				ReadOnly:  true,
			})

			// Setup command to configure credentials
			// #nosec G101 - This is a shell script template, not a hardcoded credential  # pragma: allowlist secret
			setupCmd := `
if [ -f /etc/git-secret/ssh-key ]; then  # pragma: allowlist secret
  mkdir -p ~/.ssh
  cp /etc/git-secret/ssh-key ~/.ssh/id_rsa  # pragma: allowlist secret
  chmod 600 ~/.ssh/id_rsa
  echo "StrictHostKeyChecking no" >> ~/.ssh/config
elif [ -f /etc/git-secret/token ]; then  # pragma: allowlist secret
  git config --global credential.helper store
  token=$(cat /etc/git-secret/token)  # pragma: allowlist secret
  echo "https://oauth2:${token}@github.com" > ~/.git-credentials
  echo "https://oauth2:${token}@gitlab.com" >> ~/.git-credentials
fi
`
			// Prepend setup to clone command
			cloneCmd = fmt.Sprintf("%s && %s", setupCmd, cloneCmd)
			container.Args = []string{cloneCmd}
		}

		return []corev1.Container{*container}
	}

	// For other source types (S3, OCI), return empty for now
	// These will be implemented as part of full source handler integration
	return nil
}

// buildVolumes creates volumes for the create job
func (handler *CreateHandler) buildVolumes(bundle *udsv1alpha1.UDSBundleJob) []corev1.Volume {
	volumes := []corev1.Volume{
		{
			Name: "workspace",
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		},
		{
			Name: "output",
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		},
	}

	// Add git credentials volume if needed  # pragma: allowlist secret
	if bundle.Spec.Source.Type == udsv1alpha1.BundleSourceTypeGit &&
		bundle.Spec.Source.Git != nil &&
		bundle.Spec.Source.Git.CredentialsSecretRef != nil && // pragma: allowlist secret
		!bundle.Spec.Source.Git.DisableCloneCredentials {

		secretName := bundle.Spec.Source.Git.CredentialsSecretRef.Name // pragma: allowlist secret
		volumes = append(volumes, corev1.Volume{
			Name: "git-creds",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: secretName,
				},
			},
		})
	}

	return volumes
}

// getResources returns resource requirements for the bundle create job
func (handler *CreateHandler) getResources(bundle *udsv1alpha1.UDSBundleJob) corev1.ResourceRequirements {
	// Use user-provided resources if specified
	if bundle.Spec.Resources != nil {
		return *bundle.Spec.Resources
	}

	// Default resources for UDS bundle creation (higher than Zarf due to bundle size)
	return corev1.ResourceRequirements{
		Requests: corev1.ResourceList{
			corev1.ResourceCPU:    util.MustParseQuantity("500m"),
			corev1.ResourceMemory: util.MustParseQuantity("1Gi"),
		},
		Limits: corev1.ResourceList{
			corev1.ResourceCPU:    util.MustParseQuantity("2000m"),
			corev1.ResourceMemory: util.MustParseQuantity("4Gi"),
		},
	}
}
