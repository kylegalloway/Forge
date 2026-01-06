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
		Use:   "download JOB_NAME",
		Short: "Download artifacts from a completed Forge job",
		Long: `Download artifacts from a completed Forge job's artifact PVC.

This command finds the artifact PVC associated with a job, creates a temporary
pod to mount the PVC, and downloads the artifacts to your local machine.

The job can be a ZarfPackageJob or UDSBundleJob. The command will automatically
detect the job type and locate the appropriate artifact PVC.`,
		Example: `  # Download artifacts from a job to current directory
  kubectl forge download my-package-build

  # Download to a specific directory
  kubectl forge download my-package-build --output-dir ./artifacts

  # Download all artifacts (including intermediate build files)
  kubectl forge download my-package-build --all`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			o.JobName = args[0]
			return o.Run(cmd.Context())
		},
	}

	cmd.Flags().StringVarP(&o.OutputDir, "output-dir", "o", ".", "Directory to download artifacts to")
	cmd.Flags().BoolVarP(&o.AllArtifacts, "all", "a", false, "Download all artifacts including intermediate files")
	cmd.Flags().DurationVar(&o.Timeout, "timeout", 5*time.Minute, "Timeout for download operation")

	return cmd
}

// Run executes the download command
func (o *DownloadOptions) Run(ctx context.Context) error {
	fmt.Fprintf(o.IOStreams.Out, "Downloading artifacts from job: %s\n", o.JobName)

	// Get Kubernetes client
	client, err := kubectl.NewClientFromFlags(o.configFlags)
	if err != nil {
		return fmt.Errorf("failed to create Kubernetes client: %w", err)
	}

	// Get namespace (use default from context if not specified)
	namespace := kubectl.GetNamespace(o.configFlags)

	// Find the job
	job, err := client.FindJob(ctx, namespace, o.JobName)
	if err != nil {
		return fmt.Errorf("failed to find job: %w", err)
	}

	fmt.Fprintf(o.IOStreams.Out, "Found job: %s/%s\n", job.Namespace, job.Name)

	// Find the artifact PVC
	pvcName, err := client.FindArtifactPVC(ctx, job)
	if err != nil {
		return fmt.Errorf("failed to find artifact PVC: %w", err)
	}

	if pvcName == "" {
		return fmt.Errorf("no artifact PVC found for job %s (job may not have completed or may not produce artifacts)", o.JobName)
	}

	fmt.Fprintf(o.IOStreams.Out, "Found artifact PVC: %s\n", pvcName)

	// Create output directory
	if err := os.MkdirAll(o.OutputDir, 0o755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	absOutputDir, err := filepath.Abs(o.OutputDir)
	if err != nil {
		return fmt.Errorf("failed to get absolute path: %w", err)
	}

	// Download artifacts
	fmt.Fprintf(o.IOStreams.Out, "Downloading artifacts to: %s\n", absOutputDir)

	downloadCtx, cancel := context.WithTimeout(ctx, o.Timeout)
	defer cancel()

	files, err := client.DownloadFromPVC(downloadCtx, namespace, pvcName, absOutputDir, o.AllArtifacts)
	if err != nil {
		return fmt.Errorf("failed to download artifacts: %w", err)
	}

	// Print summary
	fmt.Fprintf(o.IOStreams.Out, "\n✅ Successfully downloaded %d file(s):\n", len(files))
	for _, file := range files {
		fmt.Fprintf(o.IOStreams.Out, "  - %s\n", file)
	}

	return nil
}
