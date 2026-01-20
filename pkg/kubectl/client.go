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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"

	udsv1alpha3 "github.com/kylegalloway/forge/pkg/apis/uds/v1alpha3"
	zarfv1alpha3 "github.com/kylegalloway/forge/pkg/apis/zarf/v1alpha3"
	"github.com/kylegalloway/forge/pkg/constants"
)

// Client wraps Kubernetes clients for kubectl-forge operations
type Client struct {
	kubeClient kubernetes.Interface
	restConfig *rest.Config
	scheme     *runtime.Scheme
}

// JobInfo contains information about a Forge job for listing
type JobInfo struct {
	Namespace string
	Name      string
	Type      string // "zarf" or "uds"
	Action    string
	Phase     string
	Age       string
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
		kubeClient: kubeClient,
		restConfig: restConfig,
		scheme:     forgeScheme,
	}, nil
}

// GetNamespace returns the namespace from config flags or default
func GetNamespace(configFlags *genericclioptions.ConfigFlags) string {
	namespace := "default"
	if configFlags.Namespace != nil && *configFlags.Namespace != "" {
		namespace = *configFlags.Namespace
	}
	return namespace
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
func (c *Client) FindArtifactPVC(_ context.Context, job *batchv1.Job) (string, error) {
	// Get the job's pod template volumes
	for _, volume := range job.Spec.Template.Spec.Volumes {
		if volume.PersistentVolumeClaim != nil {
			// Check if this looks like an artifact PVC (contains "artifact" in name)
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
			RestartPolicy: corev1.RestartPolicyNever,
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
					Image:   "busybox:latest",
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

	// Make stdin a TTY
	t := &TTY{
		In:  streams.In,
		Out: streams.Out,
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
			RestartPolicy: corev1.RestartPolicyNever,
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

// ListJobs lists Forge jobs (ZarfPackageJobs and UDSBundleJobs)
func (c *Client) ListJobs(ctx context.Context, namespace, jobType string) ([]JobInfo, error) {
	var jobs []JobInfo

	// This is a simplified version - in reality, you'd use a dynamic client
	// to query the CRDs directly. For now, we'll list Kubernetes Jobs
	// and filter by Forge labels.

	listOptions := metav1.ListOptions{
		LabelSelector: "app=forge",
	}

	jobList, err := c.kubeClient.BatchV1().Jobs(namespace).List(ctx, listOptions)
	if err != nil {
		return nil, err
	}

	for _, job := range jobList.Items {
		resourceType := job.Labels["resource-type"]
		action := job.Labels[constants.LabelAction]

		// Skip if type filter doesn't match
		if jobType != "all" {
			if jobType == "zarf" && resourceType != "zarfpackagejob" {
				continue
			}
			if jobType == "uds" && resourceType != "udsbundlejob" {
				continue
			}
		}

		phase := "Unknown"
		switch {
		case job.Status.Succeeded > 0:
			phase = constants.PhaseCompleted
		case job.Status.Failed > 0:
			phase = constants.PhaseFailed
		case job.Status.Active > 0:
			phase = constants.PhaseRunning
		}

		age := time.Since(job.CreationTimestamp.Time).Round(time.Second).String()

		typeStr := "zarf"
		if resourceType == "udsbundlejob" {
			typeStr = "uds"
		}

		jobs = append(jobs, JobInfo{
			Namespace: job.Namespace,
			Name:      job.Name,
			Type:      typeStr,
			Action:    action,
			Phase:     phase,
			Age:       age,
		})
	}

	return jobs, nil
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

// TTY handles terminal sizing for interactive exec
type TTY struct {
	In  io.Reader
	Out io.Writer
}

// GetSize returns the current terminal size
func (t *TTY) GetSize() *remotecommand.TerminalSize {
	return &remotecommand.TerminalSize{
		Width:  80,
		Height: 24,
	}
}

// MonitorSize returns a channel that sends terminal size updates
func (t *TTY) MonitorSize(initial *remotecommand.TerminalSize) remotecommand.TerminalSizeQueue {
	// Simple implementation - just return initial size
	return &fixedSizeQueue{size: initial}
}

// Safe executes a function with terminal setup
func (t *TTY) Safe(fn func() error) error {
	return fn()
}

type fixedSizeQueue struct {
	size *remotecommand.TerminalSize
}

// Next returns the terminal size
func (s *fixedSizeQueue) Next() *remotecommand.TerminalSize {
	return s.size
}

// boolPtr returns a pointer to a bool value
func boolPtr(b bool) *bool {
	return &b
}

// int64Ptr returns a pointer to an int64 value
func int64Ptr(i int64) *int64 {
	return &i
}
