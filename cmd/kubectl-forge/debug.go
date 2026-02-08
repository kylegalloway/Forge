// Package main implements the kubectl-forge CLI debug command
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/kylegalloway/forge/pkg/kubectl"
	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/cli-runtime/pkg/genericclioptions"
)

// DebugOptions holds options for the debug command
type DebugOptions struct {
	configFlags *genericclioptions.ConfigFlags
	IOStreams   genericclioptions.IOStreams

	JobName       string
	Action        string
	FailedOnly    bool
	Container     string
	Shell         string
	PreservePod   bool
	DebugImage    string
	CopyWorkspace bool
	AllPods       bool
	PodIndex      int
}

// NewDebugCommand creates the debug subcommand
func NewDebugCommand(configFlags *genericclioptions.ConfigFlags) *cobra.Command {
	o := &DebugOptions{
		configFlags: configFlags,
		IOStreams:   genericclioptions.IOStreams{In: os.Stdin, Out: os.Stdout, ErrOut: os.Stderr},
	}

	cmd := &cobra.Command{
		Use:   "debug RESOURCE_NAME",
		Short: "Debug a failed or running Forge resource",
		Long: `Debug a Forge resource by exec'ing into the batch Job's pod or creating a debug pod.

This command helps you debug failed resources by:
1. Finding the failed pod(s) for a resource's batch Job
2. Exec'ing into the pod with an interactive shell
3. Optionally creating a new debug pod with the same volumes

Use --action to target a specific operation (build, create, publish, deploy).
If the pod has already been deleted, you can use --preserve-pod to prevent
automatic cleanup and keep pods around for debugging.`,
		Example: `  # Debug a failed resource (exec into the pod)
  kubectl forge debug my-package --failed

  # Debug a specific operation
  kubectl forge debug my-package --action build

  # Debug using a specific container (for multi-container pods)
  kubectl forge debug my-package --container zarf-build

  # Use bash instead of default sh
  kubectl forge debug my-package --shell /bin/bash

  # Create a new debug pod with access to the workspace
  kubectl forge debug my-package --copy-workspace

  # Use a custom debug image
  kubectl forge debug my-package --debug-image ubuntu:22.04`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			o.JobName = args[0]
			return o.Run(cmd.Context())
		},
	}

	cmd.Flags().StringVarP(&o.Action, "action", "a", "", "Debug a specific operation (build, create, publish, deploy)")
	cmd.Flags().BoolVar(&o.FailedOnly, "failed", true, "Only debug failed pods")
	cmd.Flags().StringVarP(&o.Container, "container", "c", "", "Container name (for multi-container pods)")
	cmd.Flags().StringVar(&o.Shell, "shell", "/bin/sh", "Shell to use for exec")
	cmd.Flags().BoolVar(&o.PreservePod, "preserve-pod", false, "Keep the debug pod after exit")
	cmd.Flags().StringVar(&o.DebugImage, "debug-image", "busybox:1.36", "Image to use for debug pod")
	cmd.Flags().BoolVar(&o.CopyWorkspace, "copy-workspace", false, "Create debug pod with workspace volume mounted")
	cmd.Flags().BoolVar(&o.AllPods, "all-pods", false, "Debug all pods in sequence (for multi-pod jobs)")
	cmd.Flags().IntVar(&o.PodIndex, "pod-index", 0, "Index of pod to debug (0-based, when multiple pods exist)")

	return cmd
}

// Run executes the debug command
func (o *DebugOptions) Run(ctx context.Context) error {
	//nolint:errcheck // Writing to stdout in CLI context
	fmt.Fprintf(o.IOStreams.Out, "Debugging resource: %s\n", o.JobName)

	client, err := kubectl.NewClientFromFlags(o.configFlags)
	if err != nil {
		return fmt.Errorf("failed to create Kubernetes client: %w", err)
	}

	namespace := kubectl.GetNamespace(o.configFlags)

	// Resolve CRD resource
	resource, err := client.GetForgeResource(ctx, namespace, o.JobName)
	if err != nil {
		return fmt.Errorf("failed to find resource %s: %w", o.JobName, err)
	}

	// Resolve to batch Job via CRD status
	job, err := client.GetActiveJob(ctx, resource, o.Action)
	if err != nil {
		return fmt.Errorf("failed to find batch Job for resource %s: %w", o.JobName, err)
	}

	//nolint:errcheck // Writing to stdout in CLI context
	fmt.Fprintf(o.IOStreams.Out, "Found batch Job: %s/%s\n", job.Namespace, job.Name)

	// Find pods for the job
	pods, err := client.FindJobPods(ctx, job, o.FailedOnly)
	if err != nil {
		return fmt.Errorf("failed to find job pods: %w", err)
	}

	if len(pods) == 0 {
		if o.FailedOnly {
			return fmt.Errorf("no failed pods found for resource %s (pod may have been deleted or job succeeded)", o.JobName)
		}
		return fmt.Errorf("no pods found for resource %s", o.JobName)
	}

	// Show available pods if there are multiple
	if len(pods) > 1 {
		//nolint:errcheck // Writing to stdout in CLI context
		fmt.Fprintf(o.IOStreams.Out, "Found %d pods:\n", len(pods))
		for i, p := range pods {
			//nolint:errcheck // Writing to stdout in CLI context
			fmt.Fprintf(o.IOStreams.Out, "  [%d] %s (status: %s)\n", i, p.Name, p.Status.Phase)
		}
	}

	// If --all-pods, debug each pod in sequence
	if o.AllPods && len(pods) > 1 {
		for i, pod := range pods {
			//nolint:errcheck // Writing to stdout in CLI context
			fmt.Fprintf(o.IOStreams.Out, "\n=== Debugging pod %d/%d: %s ===\n", i+1, len(pods), pod.Name)
			if err := o.debugPod(ctx, client, pod); err != nil {
				//nolint:errcheck // Writing to stderr in CLI context
				fmt.Fprintf(o.IOStreams.ErrOut, "Warning: failed to debug pod %s: %v\n", pod.Name, err)
			}
			if i < len(pods)-1 {
				//nolint:errcheck // Writing to stdout in CLI context
				fmt.Fprintf(o.IOStreams.Out, "\nPress Enter to continue to next pod (or Ctrl+C to exit)...")
				//nolint:errcheck,gosec // Best effort read from user input
				fmt.Fscanln(o.IOStreams.In)
			}
		}
		return nil
	}

	// Select single pod based on index
	if o.PodIndex >= len(pods) {
		return fmt.Errorf("pod index %d out of range (only %d pods available)", o.PodIndex, len(pods))
	}
	pod := pods[o.PodIndex]

	return o.debugPod(ctx, client, pod)
}

func (o *DebugOptions) debugPod(ctx context.Context, client *kubectl.Client, pod *corev1.Pod) error {
	// If copyWorkspace is requested, create debug pod
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
	err := client.ExecIntoPod(ctx, pod, containerName, o.Shell, o.IOStreams)
	if err != nil {
		return fmt.Errorf("failed to exec into pod: %w", err)
	}

	return nil
}
