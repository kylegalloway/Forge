package controller

import (
	"context"
	"fmt"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"

	scriptrunnerv1alpha1 "github.com/kylegalloway/scriptrunner/pkg/apis/scriptrunner/v1alpha1"
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

// SimpleController watches ScriptRunner resources and creates Jobs
type SimpleController struct {
	kubeclientset      kubernetes.Interface
	dynamicClient      dynamic.Interface
	namespace          string
	processedResources map[string]bool
}

// NewSimpleController creates a new simple controller
func NewSimpleController(
	kubeclientset kubernetes.Interface,
	dynamicClient dynamic.Interface,
	namespace string) *SimpleController {

	return &SimpleController{
		kubeclientset:      kubeclientset,
		dynamicClient:      dynamicClient,
		namespace:          namespace,
		processedResources: make(map[string]bool),
	}
}

// Run starts the controller
func (c *SimpleController) Run(ctx context.Context) error {
	klog.Info("Starting SimpleController")

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
			klog.Info("Stopping SimpleController")
			return nil
		default:
			watcher, err := c.dynamicClient.Resource(gvr).Namespace(c.namespace).Watch(ctx, metav1.ListOptions{})
			if err != nil {
				klog.Errorf("Error creating watcher: %v", err)
				time.Sleep(5 * time.Second)
				continue
			}

			c.watchResources(ctx, watcher)
			watcher.Stop()
		}
	}
}

// watchResources processes watch events
func (c *SimpleController) watchResources(ctx context.Context, watcher watch.Interface) {
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
					klog.Errorf("Unexpected object type: %T", event.Object)
					continue
				}

				if err := c.handleScriptRunner(ctx, unstructuredObj); err != nil {
					klog.Errorf("Error handling ScriptRunner: %v", err)
				}
			case watch.Deleted:
				klog.Info("ScriptRunner deleted")
			case watch.Error:
				klog.Errorf("Watch error: %v", event.Object)
				return
			}
		}
	}
}

// handleScriptRunner processes a ScriptRunner resource
func (c *SimpleController) handleScriptRunner(ctx context.Context, obj *unstructured.Unstructured) error {
	name := obj.GetName()
	namespace := obj.GetNamespace()
	key := fmt.Sprintf("%s/%s", namespace, name)

	// Check if already processed
	if c.processedResources[key] {
		klog.V(4).Infof("ScriptRunner %s already processed", key)
		return nil
	}

	klog.Infof("Processing ScriptRunner: %s", key)

	// Convert unstructured to ScriptRunner
	var scriptRunner scriptrunnerv1alpha1.ScriptRunner
	err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, &scriptRunner)
	if err != nil {
		return fmt.Errorf("error converting to ScriptRunner: %v", err)
	}

	// Check if Job already exists in status
	if scriptRunner.Status.JobName != "" {
		klog.Infof("Job already exists for ScriptRunner %s: %s", key, scriptRunner.Status.JobName)
		c.processedResources[key] = true
		return nil
	}

	// Create the Job
	jobName := fmt.Sprintf("%s-job-%d", scriptRunner.Name, time.Now().Unix())
	job := c.createJob(&scriptRunner, jobName)

	klog.Infof("Creating Job %s for ScriptRunner %s", jobName, key)
	_, err = c.kubeclientset.BatchV1().Jobs(namespace).Create(ctx, job, metav1.CreateOptions{})
	if err != nil {
		if errors.IsAlreadyExists(err) {
			klog.Infof("Job %s already exists", jobName)
		} else {
			return fmt.Errorf("error creating job: %v", err)
		}
	}

	// Update ScriptRunner status
	if err := c.updateStatus(ctx, obj, jobName); err != nil {
		return fmt.Errorf("error updating status: %v", err)
	}

	c.processedResources[key] = true
	klog.Infof("Successfully created Job %s for ScriptRunner %s", jobName, key)

	return nil
}

// createJob creates a Kubernetes Job from a ScriptRunner
func (c *SimpleController) createJob(scriptRunner *scriptrunnerv1alpha1.ScriptRunner, jobName string) *batchv1.Job {
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
			BackoffLimit: &backoffLimit,
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					RestartPolicy: corev1.RestartPolicyNever,
					Containers: []corev1.Container{
						{
							Name:    "script-runner",
							Image:   image,
							Command: command,
							Args:    args,
							Env:     envVars,
						},
					},
				},
			},
		},
	}

	return job
}

// updateStatus updates the status of a ScriptRunner resource
func (c *SimpleController) updateStatus(ctx context.Context, obj *unstructured.Unstructured, jobName string) error {
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
