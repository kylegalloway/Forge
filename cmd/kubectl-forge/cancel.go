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
	"k8s.io/client-go/kubernetes"
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
		Use:   "cancel JOB_NAME",
		Short: "Cancel a running or pending Forge job",
		Long: `Cancel a Forge job by deleting the underlying Kubernetes Job.

This stops any running pods and prevents the job from completing.
Use --delete-pvc to also remove the artifact PVC (data will be lost).`,
		Example: `  # Cancel a running job
  kubectl forge cancel my-package-build

  # Cancel and delete the artifact PVC
  kubectl forge cancel my-package-build --delete-pvc

  # Force delete without confirmation
  kubectl forge cancel my-package-build --force`,
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
	restConfig, err := o.configFlags.ToRESTConfig()
	if err != nil {
		return fmt.Errorf("failed to create REST config: %w", err)
	}

	kubeClient, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return fmt.Errorf("failed to create Kubernetes client: %w", err)
	}

	forgeClient, err := kubectl.NewClientFromFlags(o.configFlags)
	if err != nil {
		return fmt.Errorf("failed to create Forge client: %w", err)
	}

	namespace := kubectl.GetNamespace(o.configFlags)

	// Find the job first
	job, err := forgeClient.FindJob(ctx, namespace, o.JobName)
	if err != nil {
		return fmt.Errorf("failed to find job %s: %w", o.JobName, err)
	}

	// Find artifact PVC if we need to delete it
	var pvcName string
	if o.DeletePVC {
		pvcName, err = forgeClient.FindArtifactPVC(ctx, job)
		if err != nil {
			//nolint:errcheck // Writing to stderr in CLI context
			fmt.Fprintf(o.IOStreams.ErrOut, "Warning: failed to find artifact PVC: %v\n", err)
		}
	}

	// Confirm unless --force
	if !o.Force {
		//nolint:errcheck // Writing to stdout in CLI context
		fmt.Fprintf(o.IOStreams.Out, "This will cancel job %s/%s\n", namespace, o.JobName)
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

	// Delete the job with propagation policy to delete pods
	propagationPolicy := metav1.DeletePropagationForeground
	err = kubeClient.BatchV1().Jobs(namespace).Delete(ctx, o.JobName, metav1.DeleteOptions{
		PropagationPolicy: &propagationPolicy,
	})
	if err != nil {
		return fmt.Errorf("failed to delete job: %w", err)
	}
	//nolint:errcheck // Writing to stdout in CLI context
	fmt.Fprintf(o.IOStreams.Out, "Job %s deleted\n", o.JobName)

	// Delete PVC if requested
	if pvcName != "" {
		err = kubeClient.CoreV1().PersistentVolumeClaims(namespace).Delete(ctx, pvcName, metav1.DeleteOptions{})
		if err != nil {
			return fmt.Errorf("failed to delete PVC %s: %w", pvcName, err)
		}
		//nolint:errcheck // Writing to stdout in CLI context
		fmt.Fprintf(o.IOStreams.Out, "PVC %s deleted\n", pvcName)
	}

	return nil
}
