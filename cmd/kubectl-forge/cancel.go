// Package main implements the kubectl-forge CLI cancel command
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/kylegalloway/forge/pkg/kubectl"
	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/cli-runtime/pkg/genericclioptions"
)

// CancelOptions holds options for the cancel command
type CancelOptions struct {
	configFlags *genericclioptions.ConfigFlags
	IOStreams   genericclioptions.IOStreams

	JobName   string
	DeletePVC bool
	Force     bool
}

// NewCancelCommand creates the cancel command
func NewCancelCommand(configFlags *genericclioptions.ConfigFlags) *cobra.Command {
	o := &CancelOptions{
		configFlags: configFlags,
		IOStreams:   genericclioptions.IOStreams{In: os.Stdin, Out: os.Stdout, ErrOut: os.Stderr},
	}

	cmd := &cobra.Command{
		Use:   "cancel RESOURCE_NAME",
		Short: "Cancel a running or pending Forge resource",
		Long: `Cancel a Forge resource by deleting the underlying Kubernetes Job(s).

This stops any running pods and prevents the job from completing.
Use --delete-pvc to also remove the artifact PVC (data will be lost).`,
		Example: `  # Cancel a running resource
  kubectl forge cancel my-package

  # Cancel and delete the artifact PVC
  kubectl forge cancel my-package --delete-pvc

  # Force delete without confirmation
  kubectl forge cancel my-package --force`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			o.JobName = args[0]
			return o.Run(cmd.Context())
		},
	}

	cmd.Flags().BoolVar(&o.DeletePVC, "delete-pvc", false, "Also delete the artifact PVC (data will be lost)")
	cmd.Flags().BoolVarP(&o.Force, "force", "f", false, "Skip confirmation prompt")

	return cmd
}

// Run executes the cancel command
func (o *CancelOptions) Run(ctx context.Context) error {
	client, err := kubectl.NewClientFromFlags(o.configFlags)
	if err != nil {
		return fmt.Errorf("failed to create Forge client: %w", err)
	}

	namespace := kubectl.GetNamespace(o.configFlags)

	// Resolve CRD resource
	resource, err := client.GetForgeResource(ctx, namespace, o.JobName)
	if err != nil {
		return fmt.Errorf("failed to find resource %s: %w", o.JobName, err)
	}

	// Collect all active batch Job names from operations
	var jobNames []string
	for _, op := range resource.Operations {
		if op.JobName != "" {
			jobNames = append(jobNames, op.JobName)
		}
	}

	if len(jobNames) == 0 {
		return fmt.Errorf("no batch Jobs found for resource %s", o.JobName)
	}

	// Find artifact PVC if we need to delete it
	var pvcName string
	if o.DeletePVC {
		// Try to find PVC from the first batch Job
		job, jobErr := client.FindJob(ctx, namespace, jobNames[0])
		if jobErr == nil {
			pvcName, _ = client.FindArtifactPVC(ctx, job) //nolint:errcheck // Best-effort PVC lookup
		}
		if pvcName == "" {
			//nolint:errcheck // Writing to stderr in CLI context
			fmt.Fprintf(o.IOStreams.ErrOut, "Warning: could not find artifact PVC for resource %s\n", o.JobName)
		}
	}

	// Confirm unless --force
	if !o.Force {
		//nolint:errcheck // Writing to stdout in CLI context
		fmt.Fprintf(o.IOStreams.Out, "This will cancel resource %s/%s by deleting %d batch Job(s)\n",
			namespace, o.JobName, len(jobNames))
		if pvcName != "" {
			//nolint:errcheck // Writing to stdout in CLI context
			fmt.Fprintf(o.IOStreams.Out, "This will also delete PVC %s (all artifact data will be lost)\n", pvcName)
		}
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

	// Delete all batch Jobs for this resource
	propagationPolicy := metav1.DeletePropagationForeground
	for _, jobName := range jobNames {
		err = client.DeleteJob(ctx, namespace, jobName, &propagationPolicy)
		if err != nil {
			//nolint:errcheck // Writing to stderr in CLI context
			fmt.Fprintf(o.IOStreams.ErrOut, "Warning: failed to delete batch Job %s: %v\n", jobName, err)
			continue
		}
		//nolint:errcheck // Writing to stdout in CLI context
		fmt.Fprintf(o.IOStreams.Out, "Batch Job %s deleted\n", jobName)
	}

	// Delete PVC if requested
	if pvcName != "" {
		err = client.DeletePVC(ctx, namespace, pvcName)
		if err != nil {
			return fmt.Errorf("failed to delete PVC %s: %w", pvcName, err)
		}
		//nolint:errcheck // Writing to stdout in CLI context
		fmt.Fprintf(o.IOStreams.Out, "PVC %s deleted\n", pvcName)
	}

	return nil
}
