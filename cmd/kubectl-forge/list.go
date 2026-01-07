// Package main implements the kubectl-forge CLI list command
package main

import (
	"context"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/kylegalloway/forge/pkg/kubectl"
	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"
)

// ListOptions holds options for the list command
type ListOptions struct {
	configFlags *genericclioptions.ConfigFlags
	IOStreams   genericclioptions.IOStreams

	AllNamespaces bool
	JobType       string
}

// NewListCommand creates the list subcommand
func NewListCommand(configFlags *genericclioptions.ConfigFlags) *cobra.Command {
	o := &ListOptions{
		configFlags: configFlags,
		IOStreams:   genericclioptions.IOStreams{In: os.Stdin, Out: os.Stdout, ErrOut: os.Stderr},
	}

	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List Forge jobs",
		Long: `List ZarfPackageJobs and UDSBundleJobs in the cluster.

Shows job name, type, action, phase, and age for each job.`,
		Example: `  # List jobs in current namespace
  kubectl forge list

  # List jobs in all namespaces
  kubectl forge list --all-namespaces

  # List only Zarf package jobs
  kubectl forge list --type zarf

  # List only UDS bundle jobs
  kubectl forge list --type uds`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return o.Run(cmd.Context())
		},
	}

	cmd.Flags().BoolVarP(&o.AllNamespaces, "all-namespaces", "A", false, "List jobs across all namespaces")
	cmd.Flags().StringVarP(&o.JobType, "type", "t", "all", "Filter by job type (zarf, uds, all)")

	return cmd
}

// Run executes the list command
func (o *ListOptions) Run(ctx context.Context) error {
	// Get Kubernetes client
	client, err := kubectl.NewClientFromFlags(o.configFlags)
	if err != nil {
		return fmt.Errorf("failed to create Kubernetes client: %w", err)
	}

	// Get namespace
	namespace := kubectl.GetNamespace(o.configFlags)
	if o.AllNamespaces {
		namespace = ""
	}

	// List jobs
	jobs, err := client.ListJobs(ctx, namespace, o.JobType)
	if err != nil {
		return fmt.Errorf("failed to list jobs: %w", err)
	}

	if len(jobs) == 0 {
		//nolint:errcheck // Writing to stdout in CLI context
		fmt.Fprintf(o.IOStreams.Out, "No jobs found.\n")
		return nil
	}

	// Print results in table format
	w := tabwriter.NewWriter(o.IOStreams.Out, 0, 0, 3, ' ', 0)
	defer func() {
		//nolint:errcheck,gosec // Flushing output in CLI context
		w.Flush()
	}()

	// Header
	if o.AllNamespaces {
		//nolint:errcheck // Writing to stdout in CLI context
		fmt.Fprintln(w, "NAMESPACE\tNAME\tTYPE\tACTION\tPHASE\tAGE")
	} else {
		//nolint:errcheck // Writing to stdout in CLI context
		fmt.Fprintln(w, "NAME\tTYPE\tACTION\tPHASE\tAGE")
	}

	// Rows
	for _, job := range jobs {
		if o.AllNamespaces {
			//nolint:errcheck // Writing to stdout in CLI context
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n",
				job.Namespace, job.Name, job.Type, job.Action, job.Phase, job.Age)
		} else {
			//nolint:errcheck // Writing to stdout in CLI context
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
				job.Name, job.Type, job.Action, job.Phase, job.Age)
		}
	}

	return nil
}
