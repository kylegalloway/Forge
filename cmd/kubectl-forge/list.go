// Package main implements the kubectl-forge CLI list command
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"text/tabwriter"
	"time"

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
	Watch         bool
	WatchInterval time.Duration
	Wide          bool
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
		Short:   "List Forge resources",
		Long: `List ZarfPackageJobs and UDSBundleJobs in the cluster.

Shows resource name, type, action, phase, message, and age from the CRD status.`,
		Example: `  # List resources in current namespace
  kubectl forge list

  # List resources in all namespaces
  kubectl forge list --all-namespaces

  # List only Zarf package jobs
  kubectl forge list --type zarf

  # List only UDS bundle jobs
  kubectl forge list --type uds

  # Watch resources with live updates
  kubectl forge list --watch

  # Show per-operation status details
  kubectl forge list --wide`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return o.Run(cmd.Context())
		},
	}

	cmd.Flags().BoolVarP(&o.AllNamespaces, "all-namespaces", "A", false, "List resources across all namespaces")
	cmd.Flags().StringVarP(&o.JobType, "type", "t", "all", "Filter by resource type (zarf, uds, all)")
	cmd.Flags().BoolVarP(&o.Watch, "watch", "w", false, "Watch for changes and update the display")
	cmd.Flags().DurationVar(&o.WatchInterval, "watch-interval", 2*time.Second, "Interval between updates when watching")
	cmd.Flags().BoolVar(&o.Wide, "wide", false, "Show per-operation status summary")

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

	if o.Watch {
		return o.runWatch(ctx, client, namespace)
	}

	return o.printResources(ctx, client, namespace)
}

func (o *ListOptions) runWatch(ctx context.Context, client *kubectl.Client, namespace string) error {
	// Set up signal handling for clean exit
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(sigCh)

	ticker := time.NewTicker(o.WatchInterval)
	defer ticker.Stop()

	// Print initial state
	o.clearScreen()
	if err := o.printResources(ctx, client, namespace); err != nil {
		return err
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-sigCh:
			//nolint:errcheck // Writing to stdout in CLI context
			fmt.Fprintln(o.IOStreams.Out)
			return nil
		case <-ticker.C:
			o.clearScreen()
			if err := o.printResources(ctx, client, namespace); err != nil {
				//nolint:errcheck // Writing to stderr in CLI context
				fmt.Fprintf(o.IOStreams.ErrOut, "Error: %v\n", err)
			}
		}
	}
}

func (o *ListOptions) clearScreen() {
	// ANSI escape codes to clear screen and move cursor to top-left
	//nolint:errcheck // Writing to stdout in CLI context
	fmt.Fprint(o.IOStreams.Out, "\033[2J\033[H")
}

func (o *ListOptions) printResources(ctx context.Context, client *kubectl.Client, namespace string) error {
	// List resources from CRDs
	resources, err := client.ListForgeResources(ctx, namespace, o.JobType)
	if err != nil {
		return fmt.Errorf("failed to list resources: %w", err)
	}

	// Create color writer for phase colorization
	colors := NewColorWriter(o.IOStreams.Out)

	if o.Watch {
		//nolint:errcheck // Writing to stdout in CLI context
		fmt.Fprintf(o.IOStreams.Out, "Every %s: kubectl forge list", o.WatchInterval)
		if o.AllNamespaces {
			//nolint:errcheck // Writing to stdout in CLI context
			fmt.Fprintf(o.IOStreams.Out, " --all-namespaces")
		}
		//nolint:errcheck // Writing to stdout in CLI context
		fmt.Fprintf(o.IOStreams.Out, "    %s\n\n", time.Now().Format(time.RFC1123))
	}

	if len(resources) == 0 {
		//nolint:errcheck // Writing to stdout in CLI context
		fmt.Fprintf(o.IOStreams.Out, "No resources found.\n")
		return nil
	}

	// Print results in table format
	w := tabwriter.NewWriter(o.IOStreams.Out, 0, 0, 3, ' ', 0)

	// Header
	if o.AllNamespaces {
		if o.Wide {
			//nolint:errcheck // Writing to stdout in CLI context
			fmt.Fprintln(w, "NAMESPACE\tNAME\tTYPE\tACTION\tPHASE\tMESSAGE\tOPERATIONS\tAGE")
		} else {
			//nolint:errcheck // Writing to stdout in CLI context
			fmt.Fprintln(w, "NAMESPACE\tNAME\tTYPE\tACTION\tPHASE\tMESSAGE\tAGE")
		}
	} else {
		if o.Wide {
			//nolint:errcheck // Writing to stdout in CLI context
			fmt.Fprintln(w, "NAME\tTYPE\tACTION\tPHASE\tMESSAGE\tOPERATIONS\tAGE")
		} else {
			//nolint:errcheck // Writing to stdout in CLI context
			fmt.Fprintln(w, "NAME\tTYPE\tACTION\tPHASE\tMESSAGE\tAGE")
		}
	}

	// Rows
	for _, r := range resources {
		phase := colors.Phase(r.Phase)
		age := formatDuration(time.Since(r.CreatedAt))
		typeStr := resourceTypeShort(r.ResourceType)

		// Truncate message for table display
		msg := r.Message
		if len(msg) > 40 {
			msg = msg[:37] + "..."
		}

		if o.AllNamespaces {
			if o.Wide {
				ops := formatOperationsSummary(r.Operations, colors)
				//nolint:errcheck // Writing to stdout in CLI context
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
					r.Namespace, r.Name, typeStr, r.Action, phase, msg, ops, age)
			} else {
				//nolint:errcheck // Writing to stdout in CLI context
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
					r.Namespace, r.Name, typeStr, r.Action, phase, msg, age)
			}
		} else {
			if o.Wide {
				ops := formatOperationsSummary(r.Operations, colors)
				//nolint:errcheck // Writing to stdout in CLI context
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
					r.Name, typeStr, r.Action, phase, msg, ops, age)
			} else {
				//nolint:errcheck // Writing to stdout in CLI context
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n",
					r.Name, typeStr, r.Action, phase, msg, age)
			}
		}
	}

	//nolint:errcheck,gosec // Flushing output in CLI context
	w.Flush()
	return nil
}

// resourceTypeShort returns a short display string for the resource type
func resourceTypeShort(resourceType string) string {
	switch resourceType {
	case "ZarfPackageJob":
		return "zarf"
	case "UDSBundleJob":
		return "uds"
	default:
		return resourceType
	}
}

// formatOperationsSummary returns a compact summary of operation statuses
func formatOperationsSummary(ops []kubectl.OperationInfo, colors *ColorWriter) string {
	if len(ops) == 0 {
		return "-"
	}
	var parts []string
	for _, op := range ops {
		phase := op.Phase
		if phase == "" {
			phase = "Pending"
		}
		parts = append(parts, fmt.Sprintf("%s:%s", op.Name, colors.Phase(phase)))
	}
	return strings.Join(parts, " ")
}
