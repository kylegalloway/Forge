// Package main implements the kubectl-forge CLI retry command
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/kylegalloway/forge/pkg/constants"
	"github.com/kylegalloway/forge/pkg/kubectl"
	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/cli-runtime/pkg/genericclioptions"
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
		Use:   "retry [RESOURCE_NAME]",
		Short: "Retry a failed Forge resource",
		Long: `Retry a failed Forge resource by triggering a new execution.

This command works by deleting the underlying Kubernetes Job for the failed
operation, which causes the controller to create a new one based on the CRD
specification.

Use --all-failed to retry all failed resources in the namespace.`,
		Example: `  # Retry a specific failed resource
  kubectl forge retry my-package

  # Retry all failed resources in the namespace
  kubectl forge retry --all-failed

  # Retry without confirmation
  kubectl forge retry my-package --force`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				o.JobName = args[0]
			}
			if o.JobName == "" && !o.AllFailed {
				return fmt.Errorf("either RESOURCE_NAME or --all-failed is required")
			}
			return o.Run(cmd.Context())
		},
	}

	cmd.Flags().BoolVar(&o.AllFailed, "all-failed", false, "Retry all failed resources in the namespace")
	cmd.Flags().BoolVarP(&o.Force, "force", "f", false, "Skip confirmation prompt")

	return cmd
}

// Run executes the retry command
func (o *RetryOptions) Run(ctx context.Context) error {
	client, err := kubectl.NewClientFromFlags(o.configFlags)
	if err != nil {
		return fmt.Errorf("failed to create Forge client: %w", err)
	}

	namespace := kubectl.GetNamespace(o.configFlags)

	if o.AllFailed {
		return o.retryAllFailed(ctx, client, namespace)
	}

	return o.retrySingleResource(ctx, client, namespace)
}

func (o *RetryOptions) retrySingleResource(ctx context.Context, client *kubectl.Client, namespace string) error {
	// Resolve CRD resource
	resource, err := client.GetForgeResource(ctx, namespace, o.JobName)
	if err != nil {
		return fmt.Errorf("failed to find resource %s: %w", o.JobName, err)
	}

	// Check if resource is in a failed state
	if resource.Phase == constants.PhaseCompleted {
		return fmt.Errorf("resource %s has already completed, nothing to retry", o.JobName)
	}
	if resource.Phase == constants.PhaseRunning {
		return fmt.Errorf("resource %s is still running, cannot retry", o.JobName)
	}

	// Find the failed operation's batch Job name
	var failedJobName string
	for _, op := range resource.Operations {
		if op.Phase == constants.PhaseFailed && op.JobName != "" {
			failedJobName = op.JobName
			break
		}
	}

	if failedJobName == "" {
		// If no specific failed op found, try to get the active job
		activeJob, activeErr := client.GetActiveJob(ctx, resource, "")
		if activeErr != nil {
			return fmt.Errorf("no failed batch Job found for resource %s (it may have been cleaned up)", o.JobName)
		}
		failedJobName = activeJob.Name
	}

	// Confirm unless --force
	if !o.Force {
		//nolint:errcheck // Writing to stdout in CLI context
		fmt.Fprintf(o.IOStreams.Out, "This will retry resource %s/%s by deleting batch Job %s and triggering reconciliation.\n",
			namespace, o.JobName, failedJobName)
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

	// Delete the batch Job to trigger controller retry
	propagationPolicy := metav1.DeletePropagationBackground
	err = client.DeleteJob(ctx, namespace, failedJobName, &propagationPolicy)
	if err != nil {
		return fmt.Errorf("failed to delete batch Job for retry: %w", err)
	}

	//nolint:errcheck // Writing to stdout in CLI context
	fmt.Fprintf(o.IOStreams.Out, "Batch Job %s deleted. The controller will create a new job based on the CRD specification.\n", failedJobName)

	// Trigger reconciliation by updating the CRD annotation
	err = o.triggerReconciliation(ctx, client, resource)
	if err != nil {
		//nolint:errcheck // Writing to stderr in CLI context
		fmt.Fprintf(o.IOStreams.ErrOut, "Warning: Could not trigger immediate reconciliation: %v\n", err)
		//nolint:errcheck // Writing to stdout in CLI context
		fmt.Fprintln(o.IOStreams.Out, "The controller will pick up the change on its next reconciliation cycle.")
	}

	return nil
}

func (o *RetryOptions) retryAllFailed(ctx context.Context, client *kubectl.Client, namespace string) error {
	resources, err := client.ListForgeResources(ctx, namespace, "all")
	if err != nil {
		return fmt.Errorf("failed to list resources: %w", err)
	}

	var failedResources []kubectl.ForgeResource
	for _, r := range resources {
		if r.Phase == constants.PhaseFailed {
			failedResources = append(failedResources, r)
		}
	}

	if len(failedResources) == 0 {
		//nolint:errcheck // Writing to stdout in CLI context
		fmt.Fprintln(o.IOStreams.Out, "No failed resources found.")
		return nil
	}

	// Confirm unless --force
	if !o.Force {
		//nolint:errcheck // Writing to stdout in CLI context
		fmt.Fprintf(o.IOStreams.Out, "Found %d failed resource(s):\n", len(failedResources))
		for _, r := range failedResources {
			typeStr := "zarf"
			if r.ResourceType == "UDSBundleJob" {
				typeStr = "uds"
			}
			//nolint:errcheck // Writing to stdout in CLI context
			fmt.Fprintf(o.IOStreams.Out, "  - %s (%s)\n", r.Name, typeStr)
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

	// Retry each failed resource
	var retried, failed int
	for i := range failedResources {
		r := &failedResources[i]

		// Find the failed batch Job
		var jobName string
		for _, op := range r.Operations {
			if op.Phase == constants.PhaseFailed && op.JobName != "" {
				jobName = op.JobName
				break
			}
		}
		if jobName == "" {
			//nolint:errcheck // Writing to stderr in CLI context
			fmt.Fprintf(o.IOStreams.ErrOut, "Skipped %s: no failed batch Job found\n", r.Name)
			failed++
			continue
		}

		propagationPolicy := metav1.DeletePropagationBackground
		err = client.DeleteJob(ctx, r.Namespace, jobName, &propagationPolicy)
		if err != nil {
			//nolint:errcheck // Writing to stderr in CLI context
			fmt.Fprintf(o.IOStreams.ErrOut, "Failed to retry %s: %v\n", r.Name, err)
			failed++
			continue
		}

		// Try to trigger reconciliation
		//nolint:errcheck // Best effort reconciliation trigger
		_ = o.triggerReconciliation(ctx, client, r)

		//nolint:errcheck // Writing to stdout in CLI context
		fmt.Fprintf(o.IOStreams.Out, "Retried: %s\n", r.Name)
		retried++
	}

	//nolint:errcheck // Writing to stdout in CLI context
	fmt.Fprintf(o.IOStreams.Out, "\nRetried %d resource(s)", retried)
	if failed > 0 {
		//nolint:errcheck // Writing to stdout in CLI context
		fmt.Fprintf(o.IOStreams.Out, ", %d failed", failed)
	}
	//nolint:errcheck // Writing to stdout in CLI context
	fmt.Fprintln(o.IOStreams.Out)

	return nil
}

func (o *RetryOptions) triggerReconciliation(ctx context.Context, client *kubectl.Client, resource *kubectl.ForgeResource) error {
	var gvr = constants.ZarfPackageJobGVR
	if resource.ResourceType == "UDSBundleJob" {
		gvr = constants.UDSBundleJobGVR
	}

	// Get the CRD
	dynamicClient := client.DynamicClient()
	obj, err := dynamicClient.Resource(gvr).Namespace(resource.Namespace).Get(ctx, resource.Name, metav1.GetOptions{})
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
	_, err = dynamicClient.Resource(gvr).Namespace(resource.Namespace).Update(ctx, obj, metav1.UpdateOptions{})
	return err
}
