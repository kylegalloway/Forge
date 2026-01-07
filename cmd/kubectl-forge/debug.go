// Package main implements the kubectl-forge CLI debug command
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/kylegalloway/forge/pkg/kubectl"
	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"
)

// DebugOptions holds options for the debug command
type DebugOptions struct {
	configFlags *genericclioptions.ConfigFlags
	IOStreams   genericclioptions.IOStreams

	JobName       string
	FailedOnly    bool
	Container     string
	Shell         string
	PreservePod   bool
	DebugImage    string
	CopyWorkspace bool
}

// NewDebugCommand creates the debug subcommand
func NewDebugCommand(configFlags *genericclioptions.ConfigFlags) *cobra.Command {
	o := &DebugOptions{
		configFlags: configFlags,
		IOStreams:   genericclioptions.IOStreams{In: os.Stdin, Out: os.Stdout, ErrOut: os.Stderr},
	}

	cmd := &cobra.Command{
		Use:   "debug JOB_NAME",
		Short: "Debug a failed or running Forge job",
		Long: `Debug a Forge job by exec'ing into the job's pod or creating a debug pod.

This command helps you debug failed jobs by:
1. Finding the failed pod(s) for a job
2. Exec'ing into the pod with an interactive shell
3. Optionally creating a new debug pod with the same volumes

If the pod has already been deleted, you can use --preserve-pod to prevent
automatic cleanup and keep pods around for debugging.`,
		Example: `  # Debug a failed job (exec into the pod)
  kubectl forge debug my-package-build --failed

  # Debug using a specific container (for multi-container pods)
  kubectl forge debug my-package-build --container zarf-build

  # Use bash instead of default sh
  kubectl forge debug my-package-build --shell /bin/bash

  # Create a new debug pod with access to the workspace
  kubectl forge debug my-package-build --copy-workspace

  # Use a custom debug image
  kubectl forge debug my-package-build --debug-image ubuntu:22.04`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			o.JobName = args[0]
			return o.Run(cmd.Context())
		},
	}

	cmd.Flags().BoolVar(&o.FailedOnly, "failed", true, "Only debug failed pods")
	cmd.Flags().StringVarP(&o.Container, "container", "c", "", "Container name (for multi-container pods)")
	cmd.Flags().StringVar(&o.Shell, "shell", "/bin/sh", "Shell to use for exec")
	cmd.Flags().BoolVar(&o.PreservePod, "preserve-pod", false, "Keep the debug pod after exit")
	cmd.Flags().StringVar(&o.DebugImage, "debug-image", "busybox:latest", "Image to use for debug pod")
	cmd.Flags().BoolVar(&o.CopyWorkspace, "copy-workspace", false, "Create debug pod with workspace volume mounted")

	return cmd
}

// Run executes the debug command
func (o *DebugOptions) Run(ctx context.Context) error {
	//nolint:errcheck // Writing to stdout in CLI context
	fmt.Fprintf(o.IOStreams.Out, "Debugging job: %s\n", o.JobName)

	// Get Kubernetes client
	client, err := kubectl.NewClientFromFlags(o.configFlags)
	if err != nil {
		return fmt.Errorf("failed to create Kubernetes client: %w", err)
	}

	// Get namespace
	namespace := kubectl.GetNamespace(o.configFlags)

	// Find the job
	job, err := client.FindJob(ctx, namespace, o.JobName)
	if err != nil {
		return fmt.Errorf("failed to find job: %w", err)
	}

	//nolint:errcheck // Writing to stdout in CLI context
	fmt.Fprintf(o.IOStreams.Out, "Found job: %s/%s\n", job.Namespace, job.Name)

	// Find pods for the job
	pods, err := client.FindJobPods(ctx, job, o.FailedOnly)
	if err != nil {
		return fmt.Errorf("failed to find job pods: %w", err)
	}

	if len(pods) == 0 {
		if o.FailedOnly {
			return fmt.Errorf("no failed pods found for job %s (pod may have been deleted or job succeeded)", o.JobName)
		}
		return fmt.Errorf("no pods found for job %s", o.JobName)
	}

	// Use the first pod found (typically there's only one per job)
	pod := pods[0]
	//nolint:errcheck // Writing to stdout in CLI context
	fmt.Fprintf(o.IOStreams.Out, "Found pod: %s (status: %s)\n", pod.Name, pod.Status.Phase)

	// If pod is failed and copyWorkspace is requested, create debug pod
	if o.CopyWorkspace {
		//nolint:errcheck // Writing to stdout in CLI context
		fmt.Fprintf(o.IOStreams.Out, "Creating debug pod with workspace access...\n")
		debugPod, debugErr := client.CreateDebugPod(ctx, pod, o.DebugImage)
		if debugErr != nil {
			return fmt.Errorf("failed to create debug pod: %w", debugErr)
		}

		//nolint:errcheck // Writing to stdout in CLI context
		fmt.Fprintf(o.IOStreams.Out, "Debug pod created: %s\n", debugPod.Name)
		pod = debugPod

		if !o.PreservePod {
			defer func() {
				//nolint:errcheck // Writing to stdout in CLI context
				fmt.Fprintf(o.IOStreams.Out, "\nCleaning up debug pod...\n")
				if deleteErr := client.DeletePod(context.Background(), pod); deleteErr != nil {
					//nolint:errcheck // Writing to stderr in CLI context
					fmt.Fprintf(o.IOStreams.ErrOut, "Warning: failed to delete debug pod: %v\n", deleteErr)
				}
			}()
		}
	}

	// Determine container name
	containerName := o.Container
	if containerName == "" {
		// Use first container
		if len(pod.Spec.Containers) > 0 {
			containerName = pod.Spec.Containers[0].Name
		} else {
			return fmt.Errorf("pod has no containers")
		}
	}

	//nolint:errcheck // Writing to stdout in CLI context
	fmt.Fprintf(o.IOStreams.Out, "Exec'ing into container: %s\n", containerName)
	//nolint:errcheck // Writing to stdout in CLI context
	fmt.Fprintf(o.IOStreams.Out, "Shell: %s\n\n", o.Shell)
	//nolint:errcheck // Writing to stdout in CLI context
	fmt.Fprintf(o.IOStreams.Out, "---\n")

	// Exec into the pod
	err = client.ExecIntoPod(ctx, pod, containerName, o.Shell, o.IOStreams)
	if err != nil {
		return fmt.Errorf("failed to exec into pod: %w", err)
	}

	return nil
}
