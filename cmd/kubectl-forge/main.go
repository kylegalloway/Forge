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
		Short: "kubectl plugin for Forge - debugging and diagnostics for Zarf/UDS resources",
		Long: `kubectl-forge provides developer-friendly commands for debugging and managing
Forge resources (ZarfPackageJobs and UDSBundleJobs).

Forge is a Kubernetes operator that orchestrates Zarf package and UDS bundle builds,
publications, and deployments. This plugin queries CRD status directly to surface
operation phases, retry counts, artifact locations, and messages. All commands
accept CRD resource names (not batch Job names).`,
		Version: version,
		Example: `  # Check Forge system status
  kubectl forge status

  # List all resources in current namespace
  kubectl forge list

  # List with per-operation status summary
  kubectl forge list --wide

  # Diagnose problems with a resource
  kubectl forge diagnose my-package

  # Get detailed resource information
  kubectl forge get job my-package

  # Get info for a specific operation
  kubectl forge get job my-package --action build

  # Get logs from a resource
  kubectl forge get logs my-package --follow

  # Get logs for a specific operation
  kubectl forge get logs my-package --action build

  # Get pods for a resource
  kubectl forge get pods my-package

  # Get events for a resource
  kubectl forge get events my-package

  # Download artifacts from a completed resource
  kubectl forge download my-package

  # Debug a failed resource (exec into the pod)
  kubectl forge debug my-package --failed

  # Get controller logs (for operators)
  kubectl forge logs controller

  # Cancel a running resource
  kubectl forge cancel my-package

  # Retry a failed resource
  kubectl forge retry my-package`,
		SilenceUsage: true,
	}

	// Add Kubernetes config flags (--kubeconfig, --context, --namespace, etc.)
	configFlags.AddFlags(rootCmd.PersistentFlags())

	// Add subcommands
	rootCmd.AddCommand(NewDownloadCommand(configFlags))
	rootCmd.AddCommand(NewDebugCommand(configFlags))
	rootCmd.AddCommand(NewListCommand(configFlags))
	rootCmd.AddCommand(NewGetCommand(configFlags))
	rootCmd.AddCommand(NewDiagnoseCommand(configFlags))
	rootCmd.AddCommand(NewStatusCommand(configFlags))
	rootCmd.AddCommand(NewLogsCommand(configFlags))
	rootCmd.AddCommand(NewCancelCommand(configFlags))
	rootCmd.AddCommand(NewRetryCommand(configFlags))

	return rootCmd
}
