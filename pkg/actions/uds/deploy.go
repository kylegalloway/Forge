package uds

import (
	"context"
	"fmt"
	"strings"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"

	"github.com/kylegalloway/forge/pkg/actions"
	udsv1alpha2 "github.com/kylegalloway/forge/pkg/apis/uds/v1alpha2"
	"github.com/kylegalloway/forge/pkg/constants"
	"github.com/kylegalloway/forge/pkg/resources"
	"github.com/kylegalloway/forge/pkg/telemetry"
)

// DeployHandler handles Deploy actions for UDSBundleJob resources
type DeployHandler struct {
	kubeClient    kubernetes.Interface
	dynamicClient dynamic.Interface
	metrics       *telemetry.Metrics
	tracer        *telemetry.Tracer
}

// NewDeployHandler creates a new DeployHandler
func NewDeployHandler(kubeClient kubernetes.Interface, dynamicClient dynamic.Interface, metrics *telemetry.Metrics, tracer *telemetry.Tracer) *DeployHandler {
	return &DeployHandler{
		kubeClient:    kubeClient,
		dynamicClient: dynamicClient,
		metrics:       metrics,
		tracer:        tracer,
	}
}

// Execute performs a Deploy action for the given UDSBundleJob
//
// The artifactPath and artifactPVCName parameters enable multi-action job support (CreateDeploy, etc.)
// When artifactPVCName is set, the PVC is mounted and artifactPath specifies where to find the bundle.
// When empty, assumes standalone deploy with bundle source in workspace or from spec.
func (handler *DeployHandler) Execute(ctx context.Context, bundle *udsv1alpha2.UDSBundleJob, artifactPath, artifactPVCName string) (*actions.ActionResult, error) {

	klog.InfoS("Executing UDS Bundle Deploy action", "name", bundle.Name, "namespace", bundle.Namespace, "artifactPath", artifactPath, "artifactPVC", artifactPVCName)

	// Record deploy started
	handler.metrics.RecordBundleDeployStarted(ctx, bundle.Namespace, bundle.Name)

	// Validate deploy configuration
	if bundle.Spec.Deploy == nil || bundle.Spec.Deploy.Target == "" {
		handler.metrics.RecordBundleDeployFailed(ctx, bundle.Namespace, bundle.Name)
		return nil, fmt.Errorf("deploy target is required for Deploy action")
	}

	// Validate and handle resource adoption configuration
	if err := handler.validateAdoptionConfig(ctx, bundle); err != nil {
		handler.metrics.RecordBundleDeployFailed(ctx, bundle.Namespace, bundle.Name)
		return nil, fmt.Errorf("adoption configuration validation failed: %w", err)
	}

	// Create Kubernetes Job to deploy the bundle
	job, err := handler.createDeployJob(ctx, bundle, artifactPath, artifactPVCName)
	if err != nil {
		handler.metrics.RecordBundleDeployFailed(ctx, bundle.Namespace, bundle.Name)
		return nil, fmt.Errorf("failed to create deploy job: %w", err)
	}

	// Record Job creation
	handler.metrics.RecordBundleJobCreated(ctx, bundle.Namespace, bundle.Name, "deploy")

	klog.InfoS("Bundle deploy job created", "name", bundle.Name, "job", job.Name)

	result := &actions.ActionResult{
		JobName:   job.Name,
		Phase:     "Running",
		Message:   fmt.Sprintf("Bundle deploy job %s created", job.Name),
		StartTime: metav1.Now(),
		Completed: false,
	}

	return result, nil
}

// createDeployJob creates a Kubernetes Job to deploy a UDS bundle
func (handler *DeployHandler) createDeployJob(ctx context.Context, bundle *udsv1alpha2.UDSBundleJob, artifactPath, artifactPVCName string) (*batchv1.Job, error) {
	jobName := fmt.Sprintf("%s-deploy", bundle.Name)

	// Build UDS CLI deploy command
	udsCmd := handler.buildDeployCommand(bundle, artifactPath)

	// Build env vars
	envVars := handler.buildEnvVars(bundle)

	// Determine timeout and retry policy - use Deploy config if specified
	timeoutStr := ""
	var retryPolicy *udsv1alpha2.RetryPolicy
	if bundle.Spec.Deploy != nil {
		timeoutStr = bundle.Spec.Deploy.Timeout
		retryPolicy = bundle.Spec.Deploy.Retry
	}
	activeDeadlineSeconds := actions.ParseTimeoutWithDefault(timeoutStr, constants.DefaultDeployTimeout)

	// Build Job using JobBuilder
	builder := actions.NewJobBuilder(jobName, bundle.Namespace).
		WithKubeClient(handler.kubeClient).
		WithOwnerReference(bundle, udsv1alpha2.SchemeGroupVersion.WithKind("UDSBundleJob")).
		WithLabels(map[string]string{
			"app":                  "forge",
			"resource-type":        "udsbundlejob",
			constants.LabelPackage: bundle.Name,
			constants.LabelAction:  "deploy",
		}).
		WithContainerImage(constants.UDSCLIImage).
		WithContainerName(constants.ContainerNameUDSDeploy).
		WithCommand([]string{"/bin/sh", "-c"}).
		WithArgs([]string{udsCmd}).
		WithWorkingDir(constants.VolumeMountPathWorkspace).
		WithResources(handler.getResources(bundle)).
		WithUDSRetryPolicy(retryPolicy).
		WithActiveDeadlineSeconds(activeDeadlineSeconds).
		WithTTLSecondsAfterFinished(3600).
		WithWorkspaceVolume().
		WithArtifactPVC(artifactPVCName)

	// Add env vars
	for _, envVar := range envVars {
		builder.WithEnvVar(envVar.Name, envVar.Value)
	}

	// Build the job spec
	job := builder.Build()

	// Set ServiceAccount and SecurityContexts
	job.Spec.Template.Spec.ServiceAccountName = bundle.Spec.ServiceAccountName
	job.Spec.Template.Spec.SecurityContext = actions.NonRootPodSecurityContextWithUID(constants.DefaultUDSUID)
	if len(job.Spec.Template.Spec.Containers) > 0 {
		job.Spec.Template.Spec.Containers[0].SecurityContext = actions.NonRootSecurityContextWithUID(constants.DefaultUDSUID)
	}

	// Add kubeconfig volume for external cluster deployment
	if bundle.Spec.Deploy.Target == udsv1alpha2.DeployTargetExternalCluster {
		handler.addKubeconfigVolume(bundle, job)
	}

	// Check if job already exists
	existingJob, err := handler.kubeClient.BatchV1().Jobs(bundle.Namespace).Get(ctx, jobName, metav1.GetOptions{})
	if err == nil {
		klog.V(2).InfoS("Job already exists, reusing", "name", bundle.Name, "job", jobName)
		return existingJob, nil
	}

	// Create the job
	createdJob, err := handler.kubeClient.BatchV1().Jobs(bundle.Namespace).Create(ctx, job, metav1.CreateOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to create job: %w", err)
	}

	return createdJob, nil
}

// buildDeployCommand builds the UDS CLI deploy command
func (handler *DeployHandler) buildDeployCommand(bundle *udsv1alpha2.UDSBundleJob, artifactPath string) string {
	deploy := bundle.Spec.Deploy

	// Determine bundle path - use artifactPath if provided (multi-action workflow),
	// otherwise search workspace for bundle (standalone deploy)
	var bundlePath string
	if artifactPath != "" {
		bundlePath = artifactPath
	} else {
		bundlePath = constants.VolumeMountPathWorkspace + "/uds-bundle-*.tar.zst"
	}

	// Base command
	cmd := "uds deploy " + bundlePath + " --confirm"

	// Add namespace if specified
	if deploy.Namespace != "" {
		cmd += fmt.Sprintf(" --namespace %s", deploy.Namespace)
	}

	// Add component selection if specified
	if len(deploy.Components) > 0 {
		components := strings.Join(deploy.Components, ",")
		cmd += fmt.Sprintf(" --components %s", components)
	}

	// Add variables if specified
	for key, value := range deploy.Variables {
		cmd += fmt.Sprintf(" --set %s=%s", key, value)
	}

	// Add kubeconfig for external cluster
	if deploy.Target == udsv1alpha2.DeployTargetExternalCluster {
		cmd = "export KUBECONFIG=/etc/kubeconfig/kubeconfig && " + cmd
	}

	return cmd
}

// buildEnvVars builds environment variables for the deploy job
func (handler *DeployHandler) buildEnvVars(bundle *udsv1alpha2.UDSBundleJob) []corev1.EnvVar {
	envVars := []corev1.EnvVar{}

	// Add timeout as env var for UDS CLI
	if bundle.Spec.Deploy.Timeout != "" {
		envVars = append(envVars, corev1.EnvVar{
			Name:  "UDS_TIMEOUT",
			Value: bundle.Spec.Deploy.Timeout,
		})
	}

	return envVars
}

// addKubeconfigVolume adds kubeconfig volume and mount for external cluster deployment
func (handler *DeployHandler) addKubeconfigVolume(bundle *udsv1alpha2.UDSBundleJob, job *batchv1.Job) {
	if bundle.Spec.Deploy.Kubeconfig == nil || bundle.Spec.Deploy.Kubeconfig.SecretRef.Name == "" { // pragma: allowlist secret
		return
	}

	// Ensure the job has at least one container before accessing Containers[0]
	if len(job.Spec.Template.Spec.Containers) == 0 {
		klog.ErrorS(nil, "Job has no containers, cannot add kubeconfig volume", "job", job.Name)
		return
	}

	kubeconfigKey := "kubeconfig"
	if bundle.Spec.Deploy.Kubeconfig.Key != "" {
		kubeconfigKey = bundle.Spec.Deploy.Kubeconfig.Key
	}

	// Add kubeconfig volume
	job.Spec.Template.Spec.Volumes = append(job.Spec.Template.Spec.Volumes, corev1.Volume{
		Name: "kubeconfig",
		VolumeSource: corev1.VolumeSource{
			Secret: &corev1.SecretVolumeSource{ // pragma: allowlist secret
				SecretName: bundle.Spec.Deploy.Kubeconfig.SecretRef.Name, // pragma: allowlist secret
				Items: []corev1.KeyToPath{
					{
						Key:  kubeconfigKey,
						Path: "kubeconfig",
					},
				},
			},
		},
	})

	// Add volume mount to container
	job.Spec.Template.Spec.Containers[0].VolumeMounts = append(
		job.Spec.Template.Spec.Containers[0].VolumeMounts,
		corev1.VolumeMount{
			Name:      "kubeconfig",
			MountPath: "/etc/kubeconfig",
			ReadOnly:  true,
		},
	)
}

// getResources returns resource requirements for the deploy job
func (handler *DeployHandler) getResources(bundle *udsv1alpha2.UDSBundleJob) corev1.ResourceRequirements {
	if bundle.Spec.Resources != nil {
		return *bundle.Spec.Resources
	}

	// Default resources for deploy jobs
	// Standardized with Zarf Deploy (both deploy packages to clusters)
	return actions.DeployResourceRequirements()
}

// validateAdoptionConfig validates resource adoption configuration
func (handler *DeployHandler) validateAdoptionConfig(ctx context.Context, bundle *udsv1alpha2.UDSBundleJob) error {
	// If no adoption policy specified, nothing to validate
	if bundle.Spec.Deploy.AdoptionPolicy == nil {
		return nil
	}

	adoptionPolicy := *bundle.Spec.Deploy.AdoptionPolicy

	klog.V(4).InfoS("Validating adoption configuration", "policy", adoptionPolicy, "bundle", bundle.Name)

	// If policy is "Adopt", ResourceSelector must be provided
	if adoptionPolicy == udsv1alpha2.AdoptionPolicyAdopt {
		if bundle.Spec.Deploy.ResourceSelector == nil {
			return fmt.Errorf("resourceSelector is required when adoptionPolicy is 'Adopt'")
		}

		// Validate that at least one selector criterion is provided
		selector := bundle.Spec.Deploy.ResourceSelector
		if len(selector.MatchLabels) == 0 && len(selector.MatchNames) == 0 {
			return fmt.Errorf("resourceSelector must specify at least one of matchLabels or matchNames")
		}

		// Pre-deployment validation: Check for conflicting owners
		namespaces := selector.Namespaces
		if len(namespaces) == 0 {
			// Default to bundle namespace if not specified
			namespaces = []string{bundle.Spec.Deploy.Namespace}
		}

		// Discover existing resources to validate ownership
		discoverer := resources.NewDiscoverer(handler.dynamicClient)
		discovered, err := discoverer.DiscoverUDSResources(ctx, selector, namespaces)
		if err != nil {
			return fmt.Errorf("failed to discover resources: %w", err)
		}

		// Validate no conflicting owners
		validateOwnership := true
		if selector.ValidateOwnership != nil {
			validateOwnership = *selector.ValidateOwnership
		}

		if validateOwnership && len(discovered) > 0 {
			adopter := resources.NewAdopter(handler.dynamicClient)
			if err := adopter.ValidateNoConflictingOwners(discovered); err != nil {
				return fmt.Errorf("pre-deployment ownership validation failed: %w", err)
			}
		}

		klog.InfoS("Resource adoption validation passed", "bundle", bundle.Name, "resourcesFound", len(discovered))
	}

	return nil
}
