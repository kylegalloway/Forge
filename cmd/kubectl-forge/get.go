// Package main implements the kubectl-forge CLI get command
package main

import (
	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"
)

// NewGetCommand creates the get parent command with subcommands for debugging
func NewGetCommand(configFlags *genericclioptions.ConfigFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get",
		Short: "Get information about Forge jobs and resources",
		Long: `Get detailed information about Forge jobs including logs, pods, events, and job details.

Supports multiple output formats: table (default), json, yaml.`,
		Example: `  # Get logs from a job
  kubectl forge get logs my-package-build

  # Get pods for a job
  kubectl forge get pods my-package-build

  # Get events for a job in JSON format
  kubectl forge get events my-package-build --output json

  # Get detailed job information
  kubectl forge get job my-package-build --output yaml`,
	}

	// Add subcommands
	cmd.AddCommand(NewGetLogsCommand(configFlags))
	cmd.AddCommand(NewGetPodsCommand(configFlags))
	cmd.AddCommand(NewGetEventsCommand(configFlags))
	cmd.AddCommand(NewGetJobCommand(configFlags))

	return cmd
}
