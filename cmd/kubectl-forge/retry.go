// Package main implements the kubectl-forge CLI retry command
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/kylegalloway/forge/pkg/kubectl"
	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/dynamic"
)

// RetryOptions holds options for the retry command
type RetryOptions struct {
	configFlags *genericclioptions.ConfigFlags
	IOStreams   genericclioptions.IOStreams

	JobName   string
	AllFailed bool
	Force     bool
}

// NewRetryCommand creates the retry command
func NewRetryCommand(configFlags *genericclioptions.ConfigFlags) *cobra.Command {
	o := &RetryOptions{
		configFlags: configFlags,
		IOStreams:   genericclioptions.IOStreams{In: os.Stdin, Out: os.Stdout, ErrOut: os.Stderr},
	}

	cmd := &cobra.Command{
		Use:   "retry [JOB_NAME]",
		Short: "Retry a failed Forge job",
		Long: `Retry a failed Forge job by triggering a new execution.

This command works by deleting the underlying Kubernetes Job, which causes
the controller to create a new one based on the CRD specification.

Use --all-failed to retry all failed jobs in the namespace.`,
		Example: `  # Retry a specific failed job
  kubectl forge retry my-package-build

  # Retry all failed jobs in the namespace
  kubectl forge retry --all-failed

  # Retry without confirmation
  kubectl forge retry my-package-build --force`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				o.JobName = args[0]
			}
			if o.JobName == "" && !o.AllFailed {
				return fmt.Errorf("either JOB_NAME or --all-failed is required")
			}
			return o.Run(cmd.Context())
		},
	}

	cmd.Flags().BoolVar(&o.AllFailed, "all-failed", false, "Retry all failed jobs in the namespace")
	cmd.Flags().BoolVarP(&o.Force, "force", "f", false, "Skip confirmation prompt")

	return cmd
}

// Run executes the retry command
func (o *RetryOptions) Run(ctx context.Context) error {
	restConfig, err := o.configFlags.ToRESTConfig()
	if err != nil {
		return fmt.Errorf("failed to create REST config: %w", err)
	}

	dynamicClient, err := dynamic.NewForConfig(restConfig)
	if err != nil {
		return fmt.Errorf("failed to create dynamic client: %w", err)
	}

	forgeClient, err := kubectl.NewClientFromFlags(o.configFlags)
	if err != nil {
		return fmt.Errorf("failed to create Forge client: %w", err)
	}

	namespace := kubectl.GetNamespace(o.configFlags)

	if o.AllFailed {
		return o.retryAllFailed(ctx, forgeClient, dynamicClient, namespace)
	}

	return o.retrySingleJob(ctx, forgeClient, dynamicClient, namespace)
}

func (o *RetryOptions) retrySingleJob(ctx context.Context, forgeClient *kubectl.Client, dynamicClient dynamic.Interface, namespace string) error {
	// Find the job to get its type
	job, err := forgeClient.FindJob(ctx, namespace, o.JobName)
	if err != nil {
		return fmt.Errorf("failed to find job %s: %w", o.JobName, err)
	}

	// Check if job is in a failed state
	if job.Status.Succeeded > 0 {
		return fmt.Errorf("job %s has already succeeded, nothing to retry", o.JobName)
	}
	if job.Status.Active > 0 {
		return fmt.Errorf("job %s is still running, cannot retry", o.JobName)
	}

	// Confirm unless --force
	if !o.Force {
		//nolint:errcheck // Writing to stdout in CLI context
		fmt.Fprintf(o.IOStreams.Out, "This will retry job %s/%s by deleting and recreating the underlying Kubernetes Job.\n", namespace, o.JobName)
		//nolint:errcheck // Writing to stdout in CLI context
		fmt.Fprintf(o.IOStreams.Out, "Continue? [y/N]: ")

		var response string
		//nolint:errcheck,gosec // Best effort read from user input
		fmt.Fscanln(o.IOStreams.In, &response)
		if response != "y" && response != "Y" && response != "yes" && response != "Yes" {
			//nolint:errcheck // Writing to stdout in CLI context
			fmt.Fprintln(o.IOStreams.Out, "Canceled.")
			return nil
		}
	}

	// Delete the Kubernetes Job to trigger controller retry
	propagationPolicy := metav1.DeletePropagationBackground
	err = forgeClient.DeleteJob(ctx, namespace, o.JobName, &propagationPolicy)
	if err != nil {
		return fmt.Errorf("failed to delete job for retry: %w", err)
	}

	//nolint:errcheck // Writing to stdout in CLI context
	fmt.Fprintf(o.IOStreams.Out, "Job %s deleted. The controller will create a new job based on the CRD specification.\n", o.JobName)

	// Trigger reconciliation by updating the CRD annotation
	err = o.triggerReconciliation(ctx, dynamicClient, namespace, job.Labels["resource-type"], o.JobName)
	if err != nil {
		//nolint:errcheck // Writing to stderr in CLI context
		fmt.Fprintf(o.IOStreams.ErrOut, "Warning: Could not trigger immediate reconciliation: %v\n", err)
		//nolint:errcheck // Writing to stdout in CLI context
		fmt.Fprintln(o.IOStreams.Out, "The controller will pick up the change on its next reconciliation cycle.")
	}

	return nil
}

func (o *RetryOptions) retryAllFailed(ctx context.Context, forgeClient *kubectl.Client, dynamicClient dynamic.Interface, namespace string) error {
	jobs, err := forgeClient.ListJobs(ctx, namespace, "all")
	if err != nil {
		return fmt.Errorf("failed to list jobs: %w", err)
	}

	var failedJobs []kubectl.JobInfo
	for _, job := range jobs {
		if job.Phase == "Failed" {
			failedJobs = append(failedJobs, job)
		}
	}

	if len(failedJobs) == 0 {
		//nolint:errcheck // Writing to stdout in CLI context
		fmt.Fprintln(o.IOStreams.Out, "No failed jobs found.")
		return nil
	}

	// Confirm unless --force
	if !o.Force {
		//nolint:errcheck // Writing to stdout in CLI context
		fmt.Fprintf(o.IOStreams.Out, "Found %d failed job(s):\n", len(failedJobs))
		for _, job := range failedJobs {
			//nolint:errcheck // Writing to stdout in CLI context
			fmt.Fprintf(o.IOStreams.Out, "  - %s (%s)\n", job.Name, job.Type)
		}
		//nolint:errcheck // Writing to stdout in CLI context
		fmt.Fprintf(o.IOStreams.Out, "\nRetry all? [y/N]: ")

		var response string
		//nolint:errcheck,gosec // Best effort read from user input
		fmt.Fscanln(o.IOStreams.In, &response)
		if response != "y" && response != "Y" && response != "yes" && response != "Yes" {
			//nolint:errcheck // Writing to stdout in CLI context
			fmt.Fprintln(o.IOStreams.Out, "Canceled.")
			return nil
		}
	}

	// Retry each failed job
	var retried, failed int
	for _, jobInfo := range failedJobs {
		propagationPolicy := metav1.DeletePropagationBackground
		err = forgeClient.DeleteJob(ctx, namespace, jobInfo.Name, &propagationPolicy)
		if err != nil {
			//nolint:errcheck // Writing to stderr in CLI context
			fmt.Fprintf(o.IOStreams.ErrOut, "Failed to retry %s: %v\n", jobInfo.Name, err)
			failed++
			continue
		}

		// Try to trigger reconciliation
		resourceType := "zarfpackagejob"
		if jobInfo.Type == "uds" {
			resourceType = "udsbundlejob"
		}
		//nolint:errcheck // Best effort reconciliation trigger
		_ = o.triggerReconciliation(ctx, dynamicClient, namespace, resourceType, jobInfo.Name)

		//nolint:errcheck // Writing to stdout in CLI context
		fmt.Fprintf(o.IOStreams.Out, "Retried: %s\n", jobInfo.Name)
		retried++
	}

	//nolint:errcheck // Writing to stdout in CLI context
	fmt.Fprintf(o.IOStreams.Out, "\nRetried %d job(s)", retried)
	if failed > 0 {
		//nolint:errcheck // Writing to stdout in CLI context
		fmt.Fprintf(o.IOStreams.Out, ", %d failed", failed)
	}
	//nolint:errcheck // Writing to stdout in CLI context
	fmt.Fprintln(o.IOStreams.Out)

	return nil
}

func (o *RetryOptions) triggerReconciliation(ctx context.Context, dynamicClient dynamic.Interface, namespace, resourceType, name string) error {
	var gvr schema.GroupVersionResource

	switch resourceType {
	case "zarfpackagejob":
		gvr = schema.GroupVersionResource{
			Group:    "forge.dev",
			Version:  "v1alpha3",
			Resource: "zarfpackagejobs",
		}
	case "udsbundlejob":
		gvr = schema.GroupVersionResource{
			Group:    "forge.dev",
			Version:  "v1alpha3",
			Resource: "udsbundlejobs",
		}
	default:
		return fmt.Errorf("unknown resource type: %s", resourceType)
	}

	// Get the CRD
	obj, err := dynamicClient.Resource(gvr).Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return err
	}

	// Add/update an annotation to trigger reconciliation
	annotations := obj.GetAnnotations()
	if annotations == nil {
		annotations = make(map[string]string)
	}
	annotations["forge.dev/retry-requested"] = metav1.Now().Format(metav1.RFC3339Micro)
	obj.SetAnnotations(annotations)

	// Update the CRD
	_, err = dynamicClient.Resource(gvr).Namespace(namespace).Update(ctx, obj, metav1.UpdateOptions{})
	return err
}
