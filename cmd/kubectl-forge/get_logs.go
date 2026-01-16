// Package main implements the kubectl-forge CLI get logs command
package main

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/kylegalloway/forge/pkg/kubectl"
	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"
)

// GetLogsOptions holds options for the get logs command
type GetLogsOptions struct {
	configFlags *genericclioptions.ConfigFlags
	IOStreams   genericclioptions.IOStreams

	JobName       string
	Follow        bool
	Container     string
	Previous      bool
	SaveFile      string
	AllContainers bool
	Timestamps    bool
	TailLines     int64
	SinceSeconds  int64
}

// NewGetLogsCommand creates the get logs subcommand
func NewGetLogsCommand(configFlags *genericclioptions.ConfigFlags) *cobra.Command {
	o := &GetLogsOptions{
		configFlags: configFlags,
		IOStreams:   genericclioptions.IOStreams{In: os.Stdin, Out: os.Stdout, ErrOut: os.Stderr},
	}

	cmd := &cobra.Command{
		Use:   "logs JOB_NAME",
		Short: "Get logs from a Forge job's pod(s)",
		Long: `Get logs from pods associated with a Forge job.

This command finds the pod(s) running the specified job and retrieves their logs.
Supports following logs in real-time, filtering by container, and saving to file.`,
		Example: `  # Get logs from a job
  kubectl forge get logs my-package-build

  # Follow log output in real-time
  kubectl forge get logs my-package-build --follow

  # Get logs from a specific container
  kubectl forge get logs my-package-build --container zarf-build

  # Get logs from previous container instance (after restart)
  kubectl forge get logs my-package-build --previous

  # Save logs to a file (also prints to stdout)
  kubectl forge get logs my-package-build -S build.log

  # Get logs from all containers (init + regular)
  kubectl forge get logs my-package-build --all-containers

  # Include timestamps in output
  kubectl forge get logs my-package-build --timestamps

  # Get last 100 lines
  kubectl forge get logs my-package-build --tail 100

  # Get logs from last 5 minutes
  kubectl forge get logs my-package-build --since 300`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			o.JobName = args[0]
			return o.Run(cmd.Context())
		},
	}

	// Add flags
	cmd.Flags().BoolVarP(&o.Follow, "follow", "f", false, "Follow log output in real-time")
	cmd.Flags().StringVarP(&o.Container, "container", "c", "", "Specific container name")
	cmd.Flags().BoolVar(&o.Previous, "previous", false, "Print logs from previous container instance")
	cmd.Flags().StringVarP(&o.SaveFile, "save", "S", "", "Save logs to file (also prints to stdout)")
	cmd.Flags().BoolVar(&o.AllContainers, "all-containers", false, "Get logs from all containers (init + regular)")
	cmd.Flags().BoolVar(&o.Timestamps, "timestamps", false, "Include timestamps in output")
	cmd.Flags().Int64Var(&o.TailLines, "tail", -1, "Number of lines from end of logs (-1 for all)")
	cmd.Flags().Int64Var(&o.SinceSeconds, "since", 0, "Only return logs newer than relative duration in seconds")

	return cmd
}

// Run executes the get logs command
func (o *GetLogsOptions) Run(ctx context.Context) error {
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
		return fmt.Errorf("failed to find job %s: %w", o.JobName, err)
	}

	// Find pods for the job
	pods, err := client.FindJobPods(ctx, job, false)
	if err != nil {
		return fmt.Errorf("failed to find pods for job %s: %w", o.JobName, err)
	}

	if len(pods) == 0 {
		return fmt.Errorf("no pods found for job %s", o.JobName)
	}

	// Prepare output writer
	output := o.IOStreams.Out
	var file *os.File
	if o.SaveFile != "" {
		file, err = os.Create(o.SaveFile)
		if err != nil {
			return fmt.Errorf("failed to create output file %s: %w", o.SaveFile, err)
		}
		defer func() {
			//nolint:errcheck,gosec // Best-effort close in defer
			file.Close()
		}()
		// Write to both stdout and file
		output = io.MultiWriter(o.IOStreams.Out, file)
	}

	// Build log options
	logOptions := &kubectl.LogOptions{
		Follow:        o.Follow,
		Previous:      o.Previous,
		Timestamps:    o.Timestamps,
		TailLines:     o.TailLines,
		SinceSeconds:  o.SinceSeconds,
		Container:     o.Container,
		AllContainers: o.AllContainers,
	}

	// Get logs from pod(s)
	for i, pod := range pods {
		if len(pods) > 1 {
			//nolint:errcheck // Writing to output in CLI context
			fmt.Fprintf(output, "==> Pod: %s <==\n", pod.Name)
		}

		if err := client.GetPodLogs(ctx, pod, logOptions, output); err != nil {
			// Don't fail completely if one pod fails, report and continue
			//nolint:errcheck // Writing to stderr in CLI context
			fmt.Fprintf(o.IOStreams.ErrOut, "Warning: failed to get logs from pod %s: %v\n", pod.Name, err)
			continue
		}

		if len(pods) > 1 && i < len(pods)-1 {
			//nolint:errcheck // Writing to output in CLI context
			fmt.Fprintln(output)
		}
	}

	if o.SaveFile != "" {
		//nolint:errcheck // Writing to stderr in CLI context
		fmt.Fprintf(o.IOStreams.ErrOut, "Logs saved to: %s\n", o.SaveFile)
	}

	return nil
}
