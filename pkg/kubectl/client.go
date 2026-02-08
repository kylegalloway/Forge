// Package kubectl provides Kubernetes client utilities for kubectl-forge operations
package kubectl

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"

	"golang.org/x/term"

	udsv1alpha3 "github.com/kylegalloway/forge/pkg/apis/uds/v1alpha3"
	zarfv1alpha3 "github.com/kylegalloway/forge/pkg/apis/zarf/v1alpha3"
	"github.com/kylegalloway/forge/pkg/constants"
)

// Client wraps Kubernetes clients for kubectl-forge operations
type Client struct {
	kubeClient    kubernetes.Interface
	dynamicClient dynamic.Interface
	restConfig    *rest.Config
	scheme        *runtime.Scheme
}

// ForgeResource is a unified representation of a ZarfPackageJob or UDSBundleJob CRD
type ForgeResource struct {
	Name           string                     `json:"name" yaml:"name"`
	Namespace      string                     `json:"namespace" yaml:"namespace"`
	ResourceType   string                     `json:"resourceType" yaml:"resourceType"` // "ZarfPackageJob" or "UDSBundleJob"
	Action         string                     `json:"action" yaml:"action"`
	Phase          string                     `json:"phase" yaml:"phase"`
	Message        string                     `json:"message,omitempty" yaml:"message,omitempty"`
	CreatedAt      time.Time                  `json:"createdAt" yaml:"createdAt"`
	LastUpdateTime *time.Time                 `json:"lastUpdateTime,omitempty" yaml:"lastUpdateTime,omitempty"`
	Operations     []OperationInfo            `json:"operations,omitempty" yaml:"operations,omitempty"`
	Labels         map[string]string          `json:"labels,omitempty" yaml:"labels,omitempty"`
	Annotations    map[string]string          `json:"annotations,omitempty" yaml:"annotations,omitempty"`
	Raw            *unstructured.Unstructured `json:"-" yaml:"-"`
}

// OperationInfo contains status details for a single operation (build/create/publish/deploy)
type OperationInfo struct {
	Name              string     `json:"name" yaml:"name"`
	Phase             string     `json:"phase,omitempty" yaml:"phase,omitempty"`
	Message           string     `json:"message,omitempty" yaml:"message,omitempty"`
	ArtifactLocation  string     `json:"artifactLocation,omitempty" yaml:"artifactLocation,omitempty"`
	JobName           string     `json:"jobName,omitempty" yaml:"jobName,omitempty"`
	RetryCount        int32      `json:"retryCount,omitempty" yaml:"retryCount,omitempty"`
	LastFailureReason string     `json:"lastFailureReason,omitempty" yaml:"lastFailureReason,omitempty"`
	StartTime         *time.Time `json:"startTime,omitempty" yaml:"startTime,omitempty"`
	CompletionTime    *time.Time `json:"completionTime,omitempty" yaml:"completionTime,omitempty"`
	NextRetryTime     *time.Time `json:"nextRetryTime,omitempty" yaml:"nextRetryTime,omitempty"`
}

// NewClientFromFlags creates a new Client from CLI flags
func NewClientFromFlags(configFlags *genericclioptions.ConfigFlags) (*Client, error) {
	restConfig, err := configFlags.ToRESTConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to create REST config: %w", err)
	}

	kubeClient, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create Kubernetes client: %w", err)
	}

	dynamicClient, err := dynamic.NewForConfig(restConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create dynamic client: %w", err)
	}

	// Register Forge CRDs with scheme
	forgeScheme := runtime.NewScheme()
	if err := scheme.AddToScheme(forgeScheme); err != nil {
		return nil, err
	}
	if err := zarfv1alpha3.AddToScheme(forgeScheme); err != nil {
		return nil, err
	}
	if err := udsv1alpha3.AddToScheme(forgeScheme); err != nil {
		return nil, err
	}

	return &Client{
		kubeClient:    kubeClient,
		dynamicClient: dynamicClient,
		restConfig:    restConfig,
		scheme:        forgeScheme,
	}, nil
}

// DynamicClient returns the dynamic client for direct CRD operations
func (c *Client) DynamicClient() dynamic.Interface {
	return c.dynamicClient
}

// GetNamespace returns the namespace from config flags or default
func GetNamespace(configFlags *genericclioptions.ConfigFlags) string {
	namespace := "default"
	if configFlags.Namespace != nil && *configFlags.Namespace != "" {
		namespace = *configFlags.Namespace
	}
	return namespace
}

// GetForgeResource fetches a ForgeResource by name, trying both ZarfPackageJob and UDSBundleJob GVRs.
// If neither CRD is found, it falls back to finding a batch Job with that name and resolving
// back to the CRD via the forge.dev/package label.
func (c *Client) GetForgeResource(ctx context.Context, namespace, name string) (*ForgeResource, error) {
	// Try ZarfPackageJob first
	obj, err := c.dynamicClient.Resource(constants.ZarfPackageJobGVR).Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
	if err == nil {
		return c.unstructuredToForgeResource(obj, "ZarfPackageJob")
	}

	// Try UDSBundleJob
	obj, err = c.dynamicClient.Resource(constants.UDSBundleJobGVR).Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
	if err == nil {
		return c.unstructuredToForgeResource(obj, "UDSBundleJob")
	}
	if !errors.IsNotFound(err) {
		return nil, fmt.Errorf("failed to get UDSBundleJob %s: %w", name, err)
	}

	// Fallback: try finding a batch Job by that name and resolve back to CRD
	job, jobErr := c.kubeClient.BatchV1().Jobs(namespace).Get(ctx, name, metav1.GetOptions{})
	if jobErr == nil {
		pkgName := job.Labels[constants.LabelPackage]
		if pkgName != "" {
			// Try to resolve the CRD by the package label
			res, resolveErr := c.GetForgeResource(ctx, namespace, pkgName)
			if resolveErr == nil {
				return res, nil
			}
		}
	}

	return nil, fmt.Errorf("resource %q not found as ZarfPackageJob, UDSBundleJob, or batch Job in namespace %s", name, namespace)
}

// ListForgeResources lists ForgeResources of the given type ("zarf", "uds", or "all")
func (c *Client) ListForgeResources(ctx context.Context, namespace, resourceType string) ([]ForgeResource, error) {
	var resources []ForgeResource

	if resourceType == "all" || resourceType == "zarf" {
		list, err := c.dynamicClient.Resource(constants.ZarfPackageJobGVR).Namespace(namespace).List(ctx, metav1.ListOptions{})
		if err != nil && !errors.IsNotFound(err) {
			return nil, fmt.Errorf("failed to list ZarfPackageJobs: %w", err)
		}
		if list != nil {
			for i := range list.Items {
				r, err := c.unstructuredToForgeResource(&list.Items[i], "ZarfPackageJob")
				if err != nil {
					continue
				}
				resources = append(resources, *r)
			}
		}
	}

	if resourceType == "all" || resourceType == "uds" {
		list, err := c.dynamicClient.Resource(constants.UDSBundleJobGVR).Namespace(namespace).List(ctx, metav1.ListOptions{})
		if err != nil && !errors.IsNotFound(err) {
			return nil, fmt.Errorf("failed to list UDSBundleJobs: %w", err)
		}
		if list != nil {
			for i := range list.Items {
				r, err := c.unstructuredToForgeResource(&list.Items[i], "UDSBundleJob")
				if err != nil {
					continue
				}
				resources = append(resources, *r)
			}
		}
	}

	return resources, nil
}

// ResolveJobForAction returns the batch Job name for a specific operation within a ForgeResource.
// action should be lowercase: "build", "create", "publish", "deploy"
func ResolveJobForAction(resource *ForgeResource, action string) (string, error) {
	for _, op := range resource.Operations {
		if strings.EqualFold(op.Name, action) {
			if op.JobName == "" {
				return "", fmt.Errorf("operation %q has no associated batch Job (may not have started or was cleaned up)", action)
			}
			return op.JobName, nil
		}
	}
	return "", fmt.Errorf("operation %q not found in resource %s (available: %s)", action, resource.Name, availableOps(resource))
}

// GetActiveJob finds the most relevant batch Job for a ForgeResource.
// Priority: active (Running) > failed > completed, most recent first.
// If action is non-empty, it filters to that specific operation.
func (c *Client) GetActiveJob(ctx context.Context, resource *ForgeResource, action string) (*batchv1.Job, error) {
	if action != "" {
		jobName, err := ResolveJobForAction(resource, action)
		if err != nil {
			return nil, err
		}
		return c.FindJob(ctx, resource.Namespace, jobName)
	}

	// Find the most relevant operation's job
	var bestOp *OperationInfo
	for i := range resource.Operations {
		op := &resource.Operations[i]
		if op.JobName == "" {
			continue
		}
		if bestOp == nil {
			bestOp = op
			continue
		}
		// Prefer active > failed > completed
		if opPriority(op.Phase) > opPriority(bestOp.Phase) {
			bestOp = op
		} else if opPriority(op.Phase) == opPriority(bestOp.Phase) && op.StartTime != nil && bestOp.StartTime != nil && op.StartTime.After(*bestOp.StartTime) {
			bestOp = op
		}
	}

	if bestOp == nil || bestOp.JobName == "" {
		return nil, fmt.Errorf("no batch Jobs found for resource %s", resource.Name)
	}

	return c.FindJob(ctx, resource.Namespace, bestOp.JobName)
}

// FindJob finds a Kubernetes Job by name
func (c *Client) FindJob(ctx context.Context, namespace, jobName string) (*batchv1.Job, error) {
	job, err := c.kubeClient.BatchV1().Jobs(namespace).Get(ctx, jobName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	return job, nil
}

// FindArtifactPVC finds the artifact PVC for a job
// Returns PVC name or empty string if not found
func (c *Client) FindArtifactPVC(ctx context.Context, job *batchv1.Job) (string, error) {
	// First, try to find PVC by label (preferred method)
	pvcList, err := c.kubeClient.CoreV1().PersistentVolumeClaims(job.Namespace).List(ctx, metav1.ListOptions{
		LabelSelector: "forge.dev/artifact-storage=true",
	})
	if err == nil && len(pvcList.Items) > 0 {
		// Find PVC that matches a volume in this job
		jobPVCs := make(map[string]bool)
		for _, volume := range job.Spec.Template.Spec.Volumes {
			if volume.PersistentVolumeClaim != nil {
				jobPVCs[volume.PersistentVolumeClaim.ClaimName] = true
			}
		}
		for _, pvc := range pvcList.Items {
			if jobPVCs[pvc.Name] {
				return pvc.Name, nil
			}
		}
	}

	// Fallback: check job's volumes for PVC with "artifact" in name
	for _, volume := range job.Spec.Template.Spec.Volumes {
		if volume.PersistentVolumeClaim != nil {
			if strings.Contains(volume.PersistentVolumeClaim.ClaimName, "artifact") {
				return volume.PersistentVolumeClaim.ClaimName, nil
			}
		}
	}

	return "", nil
}

// FindJobPods finds pods associated with a job
func (c *Client) FindJobPods(ctx context.Context, job *batchv1.Job, failedOnly bool) ([]*corev1.Pod, error) {
	labelSelector := fmt.Sprintf("job-name=%s", job.Name)
	podList, err := c.kubeClient.CoreV1().Pods(job.Namespace).List(ctx, metav1.ListOptions{
		LabelSelector: labelSelector,
	})
	if err != nil {
		return nil, err
	}

	var pods []*corev1.Pod
	for i := range podList.Items {
		pod := &podList.Items[i]
		if failedOnly && pod.Status.Phase != corev1.PodFailed {
			continue
		}
		pods = append(pods, pod)
	}

	return pods, nil
}

// DownloadFromPVC downloads artifacts from a PVC to local directory
func (c *Client) DownloadFromPVC(ctx context.Context, namespace, pvcName, outputDir string, allFiles bool) ([]string, error) {
	// Create a temporary pod to access the PVC
	podName := fmt.Sprintf("forge-download-%d", time.Now().Unix())
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      podName,
			Namespace: namespace,
			Labels: map[string]string{
				"app":  "forge-download",
				"temp": "true",
			},
		},
		Spec: corev1.PodSpec{
			RestartPolicy:                corev1.RestartPolicyNever,
			AutomountServiceAccountToken: boolPtr(false),
			SecurityContext: &corev1.PodSecurityContext{
				RunAsNonRoot: boolPtr(true),
				RunAsUser:    int64Ptr(65534),
				RunAsGroup:   int64Ptr(65534),
				FSGroup:      int64Ptr(65534),
				SeccompProfile: &corev1.SeccompProfile{
					Type: corev1.SeccompProfileTypeRuntimeDefault,
				},
			},
			Containers: []corev1.Container{
				{
					Name:    "download",
					Image:   "busybox:1.36",
					Command: []string{"/bin/sh", "-c", "sleep 3600"},
					SecurityContext: &corev1.SecurityContext{
						AllowPrivilegeEscalation: boolPtr(false),
						ReadOnlyRootFilesystem:   boolPtr(true),
						Capabilities: &corev1.Capabilities{
							Drop: []corev1.Capability{"ALL"},
						},
					},
					VolumeMounts: []corev1.VolumeMount{
						{
							Name:      "artifacts",
							MountPath: "/artifacts",
							ReadOnly:  true,
						},
					},
				},
			},
			Volumes: []corev1.Volume{
				{
					Name: "artifacts",
					VolumeSource: corev1.VolumeSource{
						PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
							ClaimName: pvcName,
							ReadOnly:  true,
						},
					},
				},
			},
		},
	}

	// Create the pod
	pod, err := c.kubeClient.CoreV1().Pods(namespace).Create(ctx, pod, metav1.CreateOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to create download pod: %w", err)
	}

	// Ensure cleanup
	defer func() {
		deleteCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		//nolint:errcheck,gosec // Best-effort cleanup in defer
		c.kubeClient.CoreV1().Pods(namespace).Delete(deleteCtx, podName, metav1.DeleteOptions{})
	}()

	// Wait for pod to be running
	if waitErr := c.waitForPodRunning(ctx, pod, 2*time.Minute); waitErr != nil {
		return nil, fmt.Errorf("download pod failed to start: %w", waitErr)
	}

	// List files in the PVC
	files, err := c.listFilesInPod(ctx, pod, "/artifacts")
	if err != nil {
		return nil, fmt.Errorf("failed to list files: %w", err)
	}

	if len(files) == 0 {
		return nil, fmt.Errorf("no files found in artifact PVC")
	}

	// Filter files if not downloading all
	if !allFiles {
		files = filterArtifacts(files)
	}

	// Download each file
	var downloaded []string
	for _, file := range files {
		remotePath := filepath.Join("/artifacts", file)
		localPath := filepath.Join(outputDir, filepath.Base(file))

		if err := c.copyFromPod(ctx, pod, "download", remotePath, localPath); err != nil {
			return downloaded, fmt.Errorf("failed to copy %s: %w", file, err)
		}

		downloaded = append(downloaded, filepath.Base(file))
	}

	return downloaded, nil
}

// filterArtifacts returns only final artifact files (not intermediate build files)
func filterArtifacts(files []string) []string {
	var artifacts []string
	for _, file := range files {
		// Include tar.zst, tar.gz, sbom files, exclude build cache
		if strings.HasSuffix(file, ".tar.zst") ||
			strings.HasSuffix(file, ".tar.gz") ||
			strings.HasSuffix(file, ".tar") ||
			strings.Contains(file, "sbom") {
			artifacts = append(artifacts, file)
		}
	}
	return artifacts
}

// listFilesInPod lists files in a directory within a pod
func (c *Client) listFilesInPod(ctx context.Context, pod *corev1.Pod, path string) ([]string, error) {
	cmd := []string{"find", path, "-type", "f"}
	stdout, err := c.execInPod(ctx, pod, "download", cmd)
	if err != nil {
		return nil, err
	}

	lines := strings.Split(strings.TrimSpace(stdout), "\n")
	var files []string
	for _, line := range lines {
		if line != "" {
			// Remove the base path
			relPath := strings.TrimPrefix(line, path+"/")
			files = append(files, relPath)
		}
	}

	return files, nil
}

// copyFromPod copies a file from pod to local filesystem
func (c *Client) copyFromPod(ctx context.Context, pod *corev1.Pod, container, remotePath, localPath string) error {
	cmd := []string{"cat", remotePath}
	stdout, err := c.execInPod(ctx, pod, container, cmd)
	if err != nil {
		return err
	}

	// Create parent directory
	if err := os.MkdirAll(filepath.Dir(localPath), 0o750); err != nil {
		return err
	}

	return os.WriteFile(localPath, []byte(stdout), 0o600)
}

// execInPod executes a command in a pod and returns stdout
func (c *Client) execInPod(ctx context.Context, pod *corev1.Pod, container string, cmd []string) (string, error) {
	req := c.kubeClient.CoreV1().RESTClient().Post().
		Resource("pods").
		Name(pod.Name).
		Namespace(pod.Namespace).
		SubResource("exec").
		VersionedParams(&corev1.PodExecOptions{
			Container: container,
			Command:   cmd,
			Stdin:     false,
			Stdout:    true,
			Stderr:    true,
			TTY:       false,
		}, runtime.NewParameterCodec(c.scheme))

	exec, err := remotecommand.NewSPDYExecutor(c.restConfig, "POST", req.URL())
	if err != nil {
		return "", err
	}

	var stdout, stderr strings.Builder
	err = exec.StreamWithContext(ctx, remotecommand.StreamOptions{
		Stdout: &stdout,
		Stderr: &stderr,
	})
	if err != nil {
		return "", fmt.Errorf("exec failed: %w (stderr: %s)", err, stderr.String())
	}

	return stdout.String(), nil
}

// waitForPodRunning waits for a pod to be running
func (c *Client) waitForPodRunning(ctx context.Context, pod *corev1.Pod, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("timeout waiting for pod to start")
		case <-ticker.C:
			p, err := c.kubeClient.CoreV1().Pods(pod.Namespace).Get(ctx, pod.Name, metav1.GetOptions{})
			if err != nil {
				return err
			}

			if p.Status.Phase == corev1.PodRunning {
				return nil
			}

			if p.Status.Phase == corev1.PodFailed || p.Status.Phase == corev1.PodSucceeded {
				return fmt.Errorf("pod entered terminal state: %s", p.Status.Phase)
			}
		}
	}
}

// ExecIntoPod provides an interactive shell into a pod
func (c *Client) ExecIntoPod(ctx context.Context, pod *corev1.Pod, container, shell string, streams genericclioptions.IOStreams) error {
	req := c.kubeClient.CoreV1().RESTClient().Post().
		Resource("pods").
		Name(pod.Name).
		Namespace(pod.Namespace).
		SubResource("exec").
		VersionedParams(&corev1.PodExecOptions{
			Container: container,
			Command:   []string{shell},
			Stdin:     true,
			Stdout:    true,
			Stderr:    true,
			TTY:       true,
		}, runtime.NewParameterCodec(c.scheme))

	exec, err := remotecommand.NewSPDYExecutor(c.restConfig, "POST", req.URL())
	if err != nil {
		return err
	}

	// Make stdin a TTY with terminal size detection
	t := &TTY{
		In:  streams.In,
		Out: streams.Out,
		FD:  int(os.Stdout.Fd()),
	}

	sizeQueue := t.MonitorSize(t.GetSize())

	return t.Safe(func() error {
		return exec.StreamWithContext(ctx, remotecommand.StreamOptions{
			Stdin:             streams.In,
			Stdout:            streams.Out,
			Stderr:            streams.ErrOut,
			Tty:               true,
			TerminalSizeQueue: sizeQueue,
		})
	})
}

// CreateDebugPod creates a debug pod based on an existing pod
func (c *Client) CreateDebugPod(ctx context.Context, originalPod *corev1.Pod, debugImage string) (*corev1.Pod, error) {
	debugPodName := fmt.Sprintf("%s-debug-%d", originalPod.Name, time.Now().Unix())

	// Copy volumes from original pod (workspace, artifacts, etc.)
	volumes := []corev1.Volume{}
	volumeMounts := []corev1.VolumeMount{}

	for _, vol := range originalPod.Spec.Volumes {
		// Only copy persistent volumes and configmaps, not secrets
		if vol.PersistentVolumeClaim != nil || vol.ConfigMap != nil {
			volumes = append(volumes, vol)
			volumeMounts = append(volumeMounts, corev1.VolumeMount{
				Name:      vol.Name,
				MountPath: "/" + vol.Name,
			})
		}
	}

	// Add /tmp emptyDir for writable temp space (needed with readOnlyRootFilesystem)
	volumes = append(volumes, corev1.Volume{
		Name: "tmp",
		VolumeSource: corev1.VolumeSource{
			EmptyDir: &corev1.EmptyDirVolumeSource{},
		},
	})
	volumeMounts = append(volumeMounts, corev1.VolumeMount{
		Name:      "tmp",
		MountPath: "/tmp",
	})

	debugPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      debugPodName,
			Namespace: originalPod.Namespace,
			Labels: map[string]string{
				"app":          "forge-debug",
				"original-pod": originalPod.Name,
			},
		},
		Spec: corev1.PodSpec{
			RestartPolicy:                corev1.RestartPolicyNever,
			AutomountServiceAccountToken: boolPtr(false),
			SecurityContext: &corev1.PodSecurityContext{
				RunAsNonRoot: boolPtr(true),
				RunAsUser:    int64Ptr(65534),
				RunAsGroup:   int64Ptr(65534),
				FSGroup:      int64Ptr(65534),
				SeccompProfile: &corev1.SeccompProfile{
					Type: corev1.SeccompProfileTypeRuntimeDefault,
				},
			},
			Containers: []corev1.Container{
				{
					Name:         "debug",
					Image:        debugImage,
					Command:      []string{"/bin/sh", "-c", "sleep 3600"},
					VolumeMounts: volumeMounts,
					SecurityContext: &corev1.SecurityContext{
						AllowPrivilegeEscalation: boolPtr(false),
						ReadOnlyRootFilesystem:   boolPtr(true),
						Capabilities: &corev1.Capabilities{
							Drop: []corev1.Capability{"ALL"},
						},
					},
				},
			},
			Volumes: volumes,
		},
	}

	pod, err := c.kubeClient.CoreV1().Pods(originalPod.Namespace).Create(ctx, debugPod, metav1.CreateOptions{})
	if err != nil {
		return nil, err
	}

	// Wait for pod to be running
	if err := c.waitForPodRunning(ctx, pod, 2*time.Minute); err != nil {
		// Cleanup on failure
		//nolint:errcheck,gosec // Best-effort cleanup on error path
		c.kubeClient.CoreV1().Pods(pod.Namespace).Delete(context.Background(), pod.Name, metav1.DeleteOptions{})
		return nil, err
	}

	return pod, nil
}

// DeletePod deletes a pod
func (c *Client) DeletePod(ctx context.Context, pod *corev1.Pod) error {
	return c.kubeClient.CoreV1().Pods(pod.Namespace).Delete(ctx, pod.Name, metav1.DeleteOptions{})
}

// DeleteJob deletes a Kubernetes Job
func (c *Client) DeleteJob(ctx context.Context, namespace, name string, propagationPolicy *metav1.DeletionPropagation) error {
	opts := metav1.DeleteOptions{}
	if propagationPolicy != nil {
		opts.PropagationPolicy = propagationPolicy
	}
	return c.kubeClient.BatchV1().Jobs(namespace).Delete(ctx, name, opts)
}

// DeletePVC deletes a PersistentVolumeClaim
func (c *Client) DeletePVC(ctx context.Context, namespace, name string) error {
	return c.kubeClient.CoreV1().PersistentVolumeClaims(namespace).Delete(ctx, name, metav1.DeleteOptions{})
}

// LogOptions configures log retrieval from pods
type LogOptions struct {
	Follow        bool
	Previous      bool
	Timestamps    bool
	TailLines     int64
	SinceSeconds  int64
	Container     string
	AllContainers bool
}

// GetPodLogs retrieves logs from a pod and writes them to the output writer
func (c *Client) GetPodLogs(ctx context.Context, pod *corev1.Pod, opts *LogOptions, output io.Writer) error {
	containers := []string{}

	switch {
	case opts.AllContainers:
		for _, container := range pod.Spec.InitContainers {
			containers = append(containers, container.Name)
		}
		for _, container := range pod.Spec.Containers {
			containers = append(containers, container.Name)
		}
	case opts.Container != "":
		containers = []string{opts.Container}
	case len(pod.Spec.Containers) > 0:
		containers = []string{pod.Spec.Containers[0].Name}
	}

	for i, container := range containers {
		if opts.AllContainers && len(containers) > 1 {
			//nolint:errcheck // Writing to output in CLI context
			fmt.Fprintf(output, "==> Container: %s <==\n", container)
		}

		logOpts := &corev1.PodLogOptions{
			Container:  container,
			Follow:     opts.Follow,
			Previous:   opts.Previous,
			Timestamps: opts.Timestamps,
		}

		if opts.TailLines > 0 {
			logOpts.TailLines = &opts.TailLines
		}
		if opts.SinceSeconds > 0 {
			logOpts.SinceSeconds = &opts.SinceSeconds
		}

		req := c.kubeClient.CoreV1().Pods(pod.Namespace).GetLogs(pod.Name, logOpts)
		stream, err := req.Stream(ctx)
		if err != nil {
			return fmt.Errorf("failed to get log stream for container %s: %w", container, err)
		}

		_, err = io.Copy(output, stream)
		closeErr := stream.Close()
		if err != nil && err != io.EOF {
			return fmt.Errorf("failed to read logs from container %s: %w", container, err)
		}
		if closeErr != nil {
			return fmt.Errorf("failed to close log stream: %w", closeErr)
		}

		if i < len(containers)-1 {
			//nolint:errcheck // Writing to output in CLI context
			fmt.Fprintln(output)
		}
	}

	return nil
}

// GetJobEvents retrieves events for a job and its associated pods
func (c *Client) GetJobEvents(ctx context.Context, namespace, jobName string, allEvents bool) ([]corev1.Event, error) {
	// Get events for the job itself
	jobFieldSelector := fmt.Sprintf("involvedObject.name=%s,involvedObject.kind=Job", jobName)

	jobEvents, err := c.kubeClient.CoreV1().Events(namespace).List(ctx, metav1.ListOptions{
		FieldSelector: jobFieldSelector,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list job events: %w", err)
	}

	events := jobEvents.Items

	// Get events for pods of this job
	pods, err := c.kubeClient.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: fmt.Sprintf("job-name=%s", jobName),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list job pods: %w", err)
	}

	for _, pod := range pods.Items {
		podFieldSelector := fmt.Sprintf("involvedObject.name=%s,involvedObject.kind=Pod", pod.Name)
		podEvents, err := c.kubeClient.CoreV1().Events(namespace).List(ctx, metav1.ListOptions{
			FieldSelector: podFieldSelector,
		})
		if err != nil {
			continue // Skip pod events on error
		}
		events = append(events, podEvents.Items...)
	}

	// Filter to warnings only if not showing all
	if !allEvents {
		filtered := make([]corev1.Event, 0)
		for _, e := range events {
			if e.Type == corev1.EventTypeWarning {
				filtered = append(filtered, e)
			}
		}
		events = filtered
	}

	return events, nil
}

// unstructuredToForgeResource converts an unstructured CRD object to a ForgeResource
func (c *Client) unstructuredToForgeResource(obj *unstructured.Unstructured, resourceType string) (*ForgeResource, error) {
	r := &ForgeResource{
		Name:         obj.GetName(),
		Namespace:    obj.GetNamespace(),
		ResourceType: resourceType,
		CreatedAt:    obj.GetCreationTimestamp().Time,
		Labels:       obj.GetLabels(),
		Annotations:  obj.GetAnnotations(),
		Raw:          obj,
	}

	// Extract spec.action
	//nolint:errcheck // Best-effort field extraction from unstructured object
	action, _, _ := unstructured.NestedString(obj.Object, "spec", "action")
	r.Action = action

	// Extract top-level status
	//nolint:errcheck // Best-effort field extraction from unstructured object
	phase, _, _ := unstructured.NestedString(obj.Object, "status", "phase")
	r.Phase = phase

	//nolint:errcheck // Best-effort field extraction from unstructured object
	message, _, _ := unstructured.NestedString(obj.Object, "status", "message")
	r.Message = message

	//nolint:errcheck // Best-effort field extraction from unstructured object
	lastUpdate, _, _ := unstructured.NestedString(obj.Object, "status", "lastUpdateTime")
	if lastUpdate != "" {
		if t, err := time.Parse(time.RFC3339, lastUpdate); err == nil {
			r.LastUpdateTime = &t
		}
	}

	// Extract operation statuses
	var opFields []string
	switch resourceType {
	case "ZarfPackageJob":
		opFields = []string{
			constants.StatusFieldBuild,
			constants.StatusFieldPublish,
			constants.StatusFieldDeploy,
		}
	case "UDSBundleJob":
		opFields = []string{
			constants.StatusFieldCreate,
			constants.StatusFieldPublish,
			constants.StatusFieldDeploy,
		}
	}

	for _, field := range opFields {
		//nolint:errcheck // Best-effort field extraction from unstructured object
		opMap, exists, _ := unstructured.NestedMap(obj.Object, "status", field)
		if !exists || opMap == nil {
			continue
		}

		opName := strings.TrimSuffix(field, "Status")
		op := OperationInfo{Name: opName}

		if v, ok := opMap["phase"].(string); ok {
			op.Phase = v
		}
		if v, ok := opMap["message"].(string); ok {
			op.Message = v
		}
		if v, ok := opMap["artifactLocation"].(string); ok {
			op.ArtifactLocation = v
		}
		if v, ok := opMap["jobName"].(string); ok {
			op.JobName = v
		}
		if v, ok := opMap["lastFailureReason"].(string); ok {
			op.LastFailureReason = v
		}
		if v, ok := opMap["retryCount"].(int64); ok {
			//nolint:gosec // G115: retry count is bounded by CRD validation
			op.RetryCount = int32(v)
		}
		if v, ok := opMap["retryCount"].(float64); ok {
			//nolint:gosec // G115: retry count is bounded by CRD validation
			op.RetryCount = int32(v)
		}

		if v, ok := opMap["startTime"].(string); ok {
			if t, err := time.Parse(time.RFC3339, v); err == nil {
				op.StartTime = &t
			}
		}
		if v, ok := opMap["completionTime"].(string); ok {
			if t, err := time.Parse(time.RFC3339, v); err == nil {
				op.CompletionTime = &t
			}
		}
		if v, ok := opMap["nextRetryTime"].(string); ok {
			if t, err := time.Parse(time.RFC3339, v); err == nil {
				op.NextRetryTime = &t
			}
		}

		r.Operations = append(r.Operations, op)
	}

	return r, nil
}

// opPriority returns a priority value for sorting operations by relevance
func opPriority(phase string) int {
	switch phase {
	case constants.PhaseRunning:
		return 4
	case constants.PhaseRetrying:
		return 3
	case constants.PhaseFailed:
		return 2
	case constants.PhasePending, constants.PhaseQueued:
		return 1
	case constants.PhaseCompleted:
		return 0
	default:
		return -1
	}
}

// availableOps returns a comma-separated list of operation names in a ForgeResource
func availableOps(resource *ForgeResource) string {
	var ops []string
	for _, op := range resource.Operations {
		ops = append(ops, op.Name)
	}
	if len(ops) == 0 {
		return "none"
	}
	return strings.Join(ops, ", ")
}

// TTY handles terminal sizing for interactive exec
type TTY struct {
	In  io.Reader
	Out io.Writer
	FD  int // File descriptor for terminal size detection
}

// GetSize returns the current terminal size
func (t *TTY) GetSize() *remotecommand.TerminalSize {
	// Try to get actual terminal size
	if t.FD > 0 {
		width, height, err := term.GetSize(t.FD)
		if err == nil && width > 0 && height > 0 {
			//nolint:gosec // G115: terminal dimensions are bounded by reasonable screen sizes
			return &remotecommand.TerminalSize{
				Width:  uint16(width),
				Height: uint16(height),
			}
		}
	}
	// Fallback to default size
	return &remotecommand.TerminalSize{
		Width:  80,
		Height: 24,
	}
}

// MonitorSize returns a channel that sends terminal size updates
func (t *TTY) MonitorSize(initial *remotecommand.TerminalSize) remotecommand.TerminalSizeQueue {
	return &dynamicSizeQueue{tty: t, lastSize: initial}
}

// Safe executes a function with terminal setup
func (t *TTY) Safe(fn func() error) error {
	return fn()
}

type dynamicSizeQueue struct {
	tty      *TTY
	lastSize *remotecommand.TerminalSize
}

// Next returns the terminal size, blocking until size changes or context is done
func (s *dynamicSizeQueue) Next() *remotecommand.TerminalSize {
	// Get current size
	size := s.tty.GetSize()

	// If size changed, return new size
	if s.lastSize == nil || size.Width != s.lastSize.Width || size.Height != s.lastSize.Height {
		s.lastSize = size
		return size
	}

	// Return nil to indicate no change (caller should handle this)
	return nil
}

// boolPtr returns a pointer to a bool value
func boolPtr(b bool) *bool {
	return &b
}

// int64Ptr returns a pointer to an int64 value
func int64Ptr(i int64) *int64 {
	return &i
}
