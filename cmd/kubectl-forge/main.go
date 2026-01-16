package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"
)

var (
	version = "dev"
)

func main() {
	rootCmd := NewRootCommand()
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

// NewRootCommand creates the root kubectl-forge command
func NewRootCommand() *cobra.Command {
	configFlags := genericclioptions.NewConfigFlags(true)

	rootCmd := &cobra.Command{
		Use:   "kubectl-forge",
		Short: "kubectl plugin for Forge - Kubernetes job orchestrator for Zarf and UDS",
		Long: `kubectl-forge provides developer-friendly commands for working with Forge jobs.

Forge is a Kubernetes operator that orchestrates Zarf package and UDS bundle builds,
publications, and deployments. This plugin simplifies common developer workflows like
downloading artifacts and debugging failed jobs.`,
		Version: version,
		Example: `  # List all jobs in current namespace
  kubectl forge list

  # Get detailed job information
  kubectl forge get job my-package-build

  # Get logs from a job
  kubectl forge get logs my-package-build --follow

  # Get pods for a job
  kubectl forge get pods my-package-build

  # Get events for a job
  kubectl forge get events my-package-build

  # Download artifacts from a completed job
  kubectl forge download my-package-build

  # Debug a failed job (exec into the pod)
  kubectl forge debug my-package-build --failed`,
		SilenceUsage: true,
	}

	// Add Kubernetes config flags (--kubeconfig, --context, --namespace, etc.)
	configFlags.AddFlags(rootCmd.PersistentFlags())

	// Add subcommands
	rootCmd.AddCommand(NewDownloadCommand(configFlags))
	rootCmd.AddCommand(NewDebugCommand(configFlags))
	rootCmd.AddCommand(NewListCommand(configFlags))
	rootCmd.AddCommand(NewGetCommand(configFlags))

	return rootCmd
}
