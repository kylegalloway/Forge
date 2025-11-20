// Package controller implements the ScriptRunner Kubernetes controller.
//
// The controller watches ScriptRunner custom resources in the cluster and
// creates corresponding Kubernetes Jobs to execute the specified scripts.
// It provides health checks, structured logging, and OpenTelemetry-based
// observability for production deployments.
package controller

import (
	"context"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"sync/atomic"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"

	"go.opentelemetry.io/otel/attribute"

	scriptrunnerv1alpha1 "github.com/kylegalloway/scriptrunner/pkg/apis/scriptrunner/v1alpha1"
	"github.com/kylegalloway/scriptrunner/pkg/telemetry"
)

const (
	// DefaultImage is the default container image for the job
	DefaultImage = "busybox:latest"

	// DefaultScript is the default script to run
	DefaultScript = `#!/bin/sh
echo "Starting script execution..."
echo "Inputs received:"
env | grep INPUT_ | sort
echo "Script completed successfully!"
`
)

// Controller watches ScriptRunner resources and creates Jobs
type Controller struct {
	kubeclientset      kubernetes.Interface
	dynamicClient      dynamic.Interface
	namespace          string
	processedResources map[string]bool
	ready              atomic.Bool // Tracks controller readiness
	metrics            *telemetry.Metrics
	tracer             *telemetry.Tracer
}

// NewController creates a new controller
func NewController(
	kubeclientset kubernetes.Interface,
	dynamicClient dynamic.Interface,
	namespace string,
	metrics *telemetry.Metrics,
	tracer *telemetry.Tracer) *Controller {

	return &Controller{
		kubeclientset:      kubeclientset,
		dynamicClient:      dynamicClient,
		namespace:          namespace,
		processedResources: make(map[string]bool),
		metrics:            metrics,
		tracer:             tracer,
	}
}

// Run starts the controller
func (c *Controller) Run(ctx context.Context) error {
	klog.Info("Starting ScriptRunner controller")

	// Mark controller as ready once we start watching
	defer c.ready.Store(false)

	// Define the GVR for ScriptRunner
	gvr := schema.GroupVersionResource{
		Group:    scriptrunnerv1alpha1.GroupName,
		Version:  scriptrunnerv1alpha1.Version,
		Resource: "scriptrunners",
	}

	// Watch ScriptRunner resources
	for {
		select {
		case <-ctx.Done():
			klog.Info("Stopping ScriptRunner controller")
			return nil
		default:
			watcher, err := c.dynamicClient.Resource(gvr).Namespace(c.namespace).Watch(ctx, metav1.ListOptions{})
			if err != nil {
				klog.ErrorS(err, "Error creating watcher")
				c.ready.Store(false)
				time.Sleep(5 * time.Second)
				continue
			}

			// Successfully created watcher, mark as ready
			c.ready.Store(true)

			c.watchResources(ctx, watcher)
			watcher.Stop()
		}
	}
}

// watchResources processes watch events
func (c *Controller) watchResources(ctx context.Context, watcher watch.Interface) {
	for {
		select {
		case <-ctx.Done():
			return
		case event, ok := <-watcher.ResultChan():
			if !ok {
				klog.Info("Watcher channel closed, will restart")
				return
			}

			switch event.Type {
			case watch.Added, watch.Modified:
				unstructuredObj, ok := event.Object.(*unstructured.Unstructured)
				if !ok {
					klog.ErrorS(fmt.Errorf("unexpected type"), "Unexpected object type", "type", fmt.Sprintf("%T", event.Object))
					continue
				}

				if err := c.handleScriptRunner(ctx, unstructuredObj); err != nil {
					klog.ErrorS(err, "Error handling ScriptRunner")
				}
			case watch.Deleted:
				klog.Info("ScriptRunner deleted")
			case watch.Error:
				klog.ErrorS(fmt.Errorf("watch error"), "Watch error", "object", event.Object)
				return
			}
		}
	}
}

// handleScriptRunner processes a ScriptRunner resource
func (c *Controller) handleScriptRunner(ctx context.Context, obj *unstructured.Unstructured) error {
	startTime := time.Now()

	name := obj.GetName()
	namespace := obj.GetNamespace()
	key := fmt.Sprintf("%s/%s", namespace, name)

	// Start tracing span
	ctx, span := c.tracer.StartReconcileSpan(ctx, namespace, name)
	defer span.End()

	defer func() {
		// Record reconcile duration
		c.metrics.RecordReconcileDuration(ctx, time.Since(startTime).Seconds())
	}()

	// Check if already processed
	if c.processedResources[key] {
		klog.V(4).Infof("ScriptRunner %s already processed", key)
		return nil
	}

	klog.InfoS("Processing ScriptRunner", "key", key)

	// Record ScriptRunner creation
	c.metrics.RecordScriptRunnerCreated(ctx, namespace)

	// Convert unstructured to ScriptRunner
	var scriptRunner scriptrunnerv1alpha1.ScriptRunner
	err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, &scriptRunner)
	if err != nil {
		c.metrics.RecordReconcileError(ctx, "conversion_error")
		telemetry.RecordError(span, err)
		return fmt.Errorf("error converting to ScriptRunner: %v", err)
	}

	// Check if Job already exists in status
	if scriptRunner.Status.JobName != "" {
		klog.InfoS("Job already exists for ScriptRunner", "key", key, "job", scriptRunner.Status.JobName)
		c.processedResources[key] = true
		return nil
	}

	// Create the Job
	jobName := fmt.Sprintf("%s-job-%d", scriptRunner.Name, time.Now().Unix())
	job := c.createJob(&scriptRunner, jobName)

	// Start job creation span
	_, jobSpan := c.tracer.StartJobCreationSpan(ctx, namespace, scriptRunner.Name, jobName)
	defer jobSpan.End()

	klog.InfoS("Creating Job for ScriptRunner", "job", jobName, "key", key)
	_, err = c.kubeclientset.BatchV1().Jobs(namespace).Create(ctx, job, metav1.CreateOptions{})
	if err != nil {
		if errors.IsAlreadyExists(err) {
			klog.InfoS("Job already exists", "job", jobName)
		} else {
			c.metrics.RecordReconcileError(ctx, "job_creation_error")
			telemetry.RecordError(jobSpan, err)
			telemetry.RecordError(span, err)
			return fmt.Errorf("error creating job: %v", err)
		}
	}

	// Record Job creation
	c.metrics.RecordJobCreated(ctx, namespace, scriptRunner.Name)
	telemetry.AddEvent(jobSpan, "job_created", attribute.String("job.name", jobName))

	// Update ScriptRunner status
	if err := c.updateStatus(ctx, obj, jobName); err != nil {
		c.metrics.RecordReconcileError(ctx, "status_update_error")
		telemetry.RecordError(span, err)
		return fmt.Errorf("error updating status: %v", err)
	}

	c.processedResources[key] = true
	telemetry.SetSuccess(span)
	telemetry.SetSuccess(jobSpan)
	klog.InfoS("Successfully created Job", "job", jobName, "key", key)

	return nil
}

// createJob creates a Kubernetes Job from a ScriptRunner
func (c *Controller) createJob(scriptRunner *scriptrunnerv1alpha1.ScriptRunner, jobName string) *batchv1.Job {
	image := scriptRunner.Spec.Image
	if image == "" {
		image = DefaultImage
	}

	// Convert inputs to environment variables
	envVars := []corev1.EnvVar{
		{
			Name:  "SCRIPTRUNNER_NAME",
			Value: scriptRunner.Name,
		},
		{
			Name:  "SCRIPTRUNNER_NAMESPACE",
			Value: scriptRunner.Namespace,
		},
	}

	for key, value := range scriptRunner.Spec.Inputs {
		envVars = append(envVars, corev1.EnvVar{
			Name:  fmt.Sprintf("INPUT_%s", key),
			Value: value,
		})
	}

	// Determine command and args based on whether using inline script or scriptRef
	var command []string
	var args []string

	if scriptRunner.Spec.ScriptRef != "" {
		// Using a pre-built script in the container
		klog.V(4).Infof("Using pre-built script: %s", scriptRunner.Spec.ScriptRef)
		command = []string{scriptRunner.Spec.ScriptRef}
		args = scriptRunner.Spec.ScriptArgs
	} else {
		// Using inline script
		script := scriptRunner.Spec.Script
		if script == "" {
			script = DefaultScript
		}
		klog.V(4).Infof("Using inline script (%d bytes)", len(script))
		command = []string{"/bin/sh", "-c"}
		args = []string{script}
	}

	backoffLimit := int32(0)
	ttlSecondsAfterFinished := int32(3600) // Clean up Jobs after 1 hour
	activeDeadlineSeconds := int64(600)    // Kill Jobs running longer than 10 minutes

	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      jobName,
			Namespace: scriptRunner.Namespace,
			Labels: map[string]string{
				"app":                        "scriptrunner",
				"scriptrunner.io/name":       scriptRunner.Name,
				"scriptrunner.io/created-by": "scriptrunner-controller",
			},
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: scriptrunnerv1alpha1.SchemeGroupVersion.String(),
					Kind:       "ScriptRunner",
					Name:       scriptRunner.Name,
					UID:        scriptRunner.UID,
				},
			},
		},
		Spec: batchv1.JobSpec{
			BackoffLimit:            &backoffLimit,
			TTLSecondsAfterFinished: &ttlSecondsAfterFinished,
			ActiveDeadlineSeconds:   &activeDeadlineSeconds,
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					RestartPolicy: corev1.RestartPolicyNever,
					SecurityContext: &corev1.PodSecurityContext{
						RunAsNonRoot: &[]bool{true}[0],
						RunAsUser:    &[]int64{1000}[0],
						FSGroup:      &[]int64{1000}[0],
						SeccompProfile: &corev1.SeccompProfile{
							Type: corev1.SeccompProfileTypeRuntimeDefault,
						},
					},
					Containers: []corev1.Container{
						{
							Name:            "scriptrunner",
							Image:           image,
							ImagePullPolicy: getImagePullPolicy(image),
							Command:         command,
							Args:            args,
							Env:             envVars,
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    mustParseQuantity("250m"),
									corev1.ResourceMemory: mustParseQuantity("256Mi"),
								},
								Limits: corev1.ResourceList{
									corev1.ResourceCPU:    mustParseQuantity("1000m"),
									corev1.ResourceMemory: mustParseQuantity("1Gi"),
								},
							},
							SecurityContext: &corev1.SecurityContext{
								AllowPrivilegeEscalation: &[]bool{false}[0],
								RunAsNonRoot:             &[]bool{true}[0],
								RunAsUser:                &[]int64{1000}[0],
								Capabilities: &corev1.Capabilities{
									Drop: []corev1.Capability{"ALL"},
								},
								ReadOnlyRootFilesystem: &[]bool{true}[0],
							},
						},
					},
				},
			},
		},
	}

	return job
}

// updateStatus updates the status of a ScriptRunner resource
func (c *Controller) updateStatus(ctx context.Context, obj *unstructured.Unstructured, jobName string) error {
	gvr := schema.GroupVersionResource{
		Group:    scriptrunnerv1alpha1.GroupName,
		Version:  scriptrunnerv1alpha1.Version,
		Resource: "scriptrunners",
	}

	// Update status
	status := map[string]interface{}{
		"jobName":        jobName,
		"phase":          "JobCreated",
		"message":        "Job created successfully",
		"lastUpdateTime": metav1.Now().Format(time.RFC3339),
	}

	unstructured.SetNestedMap(obj.Object, status, "status")

	_, err := c.dynamicClient.Resource(gvr).Namespace(obj.GetNamespace()).UpdateStatus(
		ctx,
		obj,
		metav1.UpdateOptions{},
	)

	return err
}

// mustParseQuantity parses a resource quantity or panics (for compile-time constants)
func mustParseQuantity(s string) resource.Quantity {
	q, err := resource.ParseQuantity(s)
	if err != nil {
		panic(fmt.Sprintf("invalid quantity %q: %v", s, err))
	}
	return q
}

// HealthzHandler returns an HTTP handler for the /healthz endpoint
// This endpoint indicates if the controller process is alive
func (c *Controller) HealthzHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	}
}

// ReadyzHandler returns an HTTP handler for the /readyz endpoint
// This endpoint indicates if the controller is ready to process resources
func (c *Controller) ReadyzHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if c.ready.Load() {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("ready"))
		} else {
			w.WriteHeader(http.StatusServiceUnavailable)
			w.Write([]byte("not ready"))
		}
	}
}

// getImagePullPolicy determines the appropriate ImagePullPolicy based on image tag
// - :latest or no tag → Always (mutable, always pull)
// - @sha256:... → IfNotPresent (immutable, can cache)
// - :vX.Y.Z (semver) → IfNotPresent (immutable, can cache)
// - Other tags → Always (conservative, assume mutable)
func getImagePullPolicy(image string) corev1.PullPolicy {
	// If image uses :latest tag or no tag, always pull
	if strings.HasSuffix(image, ":latest") || !strings.Contains(image, ":") {
		return corev1.PullAlways
	}

	// If image uses SHA digest, can cache (immutable)
	if strings.Contains(image, "@sha256:") {
		return corev1.PullIfNotPresent
	}

	// If image uses semver tag (v1.2.3 or 1.2.3), can cache
	// Extract tag portion
	parts := strings.Split(image, ":")
	if len(parts) > 1 {
		tag := parts[len(parts)-1]
		// Match vX.Y.Z or X.Y.Z pattern
		matched, _ := regexp.MatchString(`^v?\d+\.\d+\.\d+(-[a-zA-Z0-9.-]+)?$`, tag)
		if matched {
			return corev1.PullIfNotPresent
		}
	}

	// Default: always pull (conservative, assume tag is mutable)
	return corev1.PullAlways
}
