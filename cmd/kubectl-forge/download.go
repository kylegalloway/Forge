// Package main implements the kubectl-forge CLI download command
package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/kylegalloway/forge/pkg/kubectl"
	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"
)

// DownloadOptions holds options for the download command
type DownloadOptions struct {
	configFlags *genericclioptions.ConfigFlags
	IOStreams   genericclioptions.IOStreams

	JobName      string
	Action       string
	OutputDir    string
	AllArtifacts bool
	Timeout      time.Duration
}

// NewDownloadCommand creates the download subcommand
func NewDownloadCommand(configFlags *genericclioptions.ConfigFlags) *cobra.Command {
	o := &DownloadOptions{
		configFlags: configFlags,
		IOStreams:   genericclioptions.IOStreams{In: os.Stdin, Out: os.Stdout, ErrOut: os.Stderr},
	}

	cmd := &cobra.Command{
		Use:   "download RESOURCE_NAME",
		Short: "Download artifacts from a completed Forge resource",
		Long: `Download artifacts from a completed Forge resource's artifact PVC.

This command finds the artifact PVC associated with a resource's batch Job,
creates a temporary pod to mount the PVC, and downloads the artifacts to your
local machine.

If the CRD status includes an artifact location (e.g., OCI registry or S3),
it will be displayed for reference.

Use --action to target a specific operation when the resource has multiple
operations (build, create, publish, deploy).`,
		Example: `  # Download artifacts from a resource to current directory
  kubectl forge download my-package

  # Download artifacts for a specific operation
  kubectl forge download my-package --action build

  # Download to a specific directory
  kubectl forge download my-package --output-dir ./artifacts

  # Download all artifacts (including intermediate build files)
  kubectl forge download my-package --all`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			o.JobName = args[0]
			return o.Run(cmd.Context())
		},
	}

	cmd.Flags().StringVarP(&o.Action, "action", "a", "", "Download artifacts for a specific operation (build, create, publish, deploy)")
	cmd.Flags().StringVarP(&o.OutputDir, "output-dir", "o", ".", "Directory to download artifacts to")
	cmd.Flags().BoolVar(&o.AllArtifacts, "all", false, "Download all artifacts including intermediate files")
	cmd.Flags().DurationVar(&o.Timeout, "timeout", 5*time.Minute, "Timeout for download operation")

	return cmd
}

// Run executes the download command
func (o *DownloadOptions) Run(ctx context.Context) error {
	//nolint:errcheck // Writing to stdout in CLI context
	fmt.Fprintf(o.IOStreams.Out, "Downloading artifacts from resource: %s\n", o.JobName)

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

	// Check for remote artifact locations in CRD status
	for _, op := range resource.Operations {
		if op.ArtifactLocation != "" {
			//nolint:errcheck // Writing to stdout in CLI context
			fmt.Fprintf(o.IOStreams.Out, "Note: %s artifacts are at: %s\n", op.Name, op.ArtifactLocation)
		}
	}

	// Resolve to batch Job via CRD status
	job, err := client.GetActiveJob(ctx, resource, o.Action)
	if err != nil {
		return fmt.Errorf("failed to find batch Job for resource %s: %w", o.JobName, err)
	}

	//nolint:errcheck // Writing to stdout in CLI context
	fmt.Fprintf(o.IOStreams.Out, "Found batch Job: %s/%s\n", job.Namespace, job.Name)

	// Find the artifact PVC
	pvcName, err := client.FindArtifactPVC(ctx, job)
	if err != nil {
		return fmt.Errorf("failed to find artifact PVC: %w", err)
	}

	if pvcName == "" {
		return fmt.Errorf("no artifact PVC found for resource %s (job may not have completed or may not produce artifacts)", o.JobName)
	}

	//nolint:errcheck // Writing to stdout in CLI context
	fmt.Fprintf(o.IOStreams.Out, "Found artifact PVC: %s\n", pvcName)

	// Create output directory
	if err = os.MkdirAll(o.OutputDir, 0o750); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	absOutputDir, err := filepath.Abs(o.OutputDir)
	if err != nil {
		return fmt.Errorf("failed to get absolute path: %w", err)
	}

	// Download artifacts
	//nolint:errcheck // Writing to stdout in CLI context
	fmt.Fprintf(o.IOStreams.Out, "Downloading artifacts to: %s\n", absOutputDir)

	downloadCtx, cancel := context.WithTimeout(ctx, o.Timeout)
	defer cancel()

	files, err := client.DownloadFromPVC(downloadCtx, namespace, pvcName, absOutputDir, o.AllArtifacts)
	if err != nil {
		return fmt.Errorf("failed to download artifacts: %w", err)
	}

	// Print summary
	//nolint:errcheck // Writing to stdout in CLI context
	fmt.Fprintf(o.IOStreams.Out, "\nSuccessfully downloaded %d file(s):\n", len(files))
	for _, file := range files {
		//nolint:errcheck // Writing to stdout in CLI context
		fmt.Fprintf(o.IOStreams.Out, "  - %s\n", file)
	}

	return nil
}
