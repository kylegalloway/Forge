// Package main implements the kubectl-forge CLI get events command
package main

import (
	"context"
	"fmt"
	"os"
	"sort"
	"time"

	"github.com/kylegalloway/forge/pkg/kubectl"
	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"
)

// EventInfo represents event information for output
type EventInfo struct {
	LastSeen  string `json:"lastSeen" yaml:"lastSeen"`
	Type      string `json:"type" yaml:"type"`
	Reason    string `json:"reason" yaml:"reason"`
	Object    string `json:"object" yaml:"object"`
	Message   string `json:"message" yaml:"message"`
	Count     int32  `json:"count" yaml:"count"`
	FirstSeen string `json:"firstSeen" yaml:"firstSeen"`
	Source    string `json:"source" yaml:"source"`
}

// GetEventsOptions holds options for the get events command
type GetEventsOptions struct {
	configFlags *genericclioptions.ConfigFlags
	IOStreams   genericclioptions.IOStreams

	JobName      string
	Action       string
	OutputFormat string
	AllEvents    bool
}

// NewGetEventsCommand creates the get events subcommand
func NewGetEventsCommand(configFlags *genericclioptions.ConfigFlags) *cobra.Command {
	o := &GetEventsOptions{
		configFlags: configFlags,
		IOStreams:   genericclioptions.IOStreams{In: os.Stdin, Out: os.Stdout, ErrOut: os.Stderr},
	}

	cmd := &cobra.Command{
		Use:   "events RESOURCE_NAME",
		Short: "Get events for a Forge resource",
		Long: `Get Kubernetes events associated with a Forge resource's batch Job and its pods.

By default, only Warning events are shown. Use --all to see all event types.
Use --action to target a specific operation (build, create, publish, deploy).
Events are sorted by last seen time, most recent last.`,
		Example: `  # Get warning events for a resource
  kubectl forge get events my-package

  # Get events for a specific operation
  kubectl forge get events my-package --action build

  # Get all events (including Normal)
  kubectl forge get events my-package --all

  # Get events in JSON format
  kubectl forge get events my-package --output json

  # Get events in YAML format
  kubectl forge get events my-package --output yaml`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			o.JobName = args[0]
			return o.Run(cmd.Context())
		},
	}

	cmd.Flags().StringVarP(&o.Action, "action", "a", "", "Show events for a specific operation (build, create, publish, deploy)")
	cmd.Flags().StringVarP(&o.OutputFormat, "output", "o", "table", "Output format (table, json, yaml)")
	cmd.Flags().BoolVar(&o.AllEvents, "all", false, "Show all events, not just warnings")

	return cmd
}

// Run executes the get events command
func (o *GetEventsOptions) Run(ctx context.Context) error {
	format, err := kubectl.ParseOutputFormat(o.OutputFormat)
	if err != nil {
		return err
	}

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

	// Get events for the batch Job and related resources
	events, err := client.GetJobEvents(ctx, namespace, job.Name, o.AllEvents)
	if err != nil {
		return fmt.Errorf("failed to get events: %w", err)
	}

	if len(events) == 0 {
		if o.AllEvents {
			//nolint:errcheck // Writing to stdout in CLI context
			fmt.Fprintf(o.IOStreams.Out, "No events found for resource %s\n", o.JobName)
		} else {
			//nolint:errcheck // Writing to stdout in CLI context
			fmt.Fprintf(o.IOStreams.Out, "No warning events found for resource %s (use --all to see all events)\n", o.JobName)
		}
		return nil
	}

	// Sort by LastTimestamp (oldest first, most recent last)
	sort.Slice(events, func(i, j int) bool {
		return events[i].LastTimestamp.Before(&events[j].LastTimestamp)
	})

	// Convert to EventInfo structs
	eventInfos := make([]EventInfo, 0, len(events))
	for _, e := range events {
		lastSeen := "unknown"
		firstSeen := "unknown"

		if !e.LastTimestamp.IsZero() {
			lastSeen = formatDuration(time.Since(e.LastTimestamp.Time))
		}
		if !e.FirstTimestamp.IsZero() {
			firstSeen = formatDuration(time.Since(e.FirstTimestamp.Time))
		}

		eventInfos = append(eventInfos, EventInfo{
			LastSeen:  lastSeen,
			Type:      e.Type,
			Reason:    e.Reason,
			Object:    fmt.Sprintf("%s/%s", e.InvolvedObject.Kind, e.InvolvedObject.Name),
			Message:   e.Message,
			Count:     e.Count,
			FirstSeen: firstSeen,
			Source:    e.Source.Component,
		})
	}

	printer := kubectl.NewPrinter(format, o.IOStreams.Out)

	switch format {
	case kubectl.OutputFormatJSON:
		return printer.PrintJSON(eventInfos)
	case kubectl.OutputFormatYAML:
		return printer.PrintYAML(eventInfos)
	default:
		headers := []string{"LAST SEEN", "TYPE", "REASON", "OBJECT", "MESSAGE"}
		rows := make([][]string, 0, len(eventInfos))
		for _, e := range eventInfos {
			// Truncate message for table display
			msg := e.Message
			if len(msg) > 60 {
				msg = msg[:57] + "..."
			}
			rows = append(rows, []string{e.LastSeen, e.Type, e.Reason, e.Object, msg})
		}
		return printer.PrintTable(headers, rows)
	}
}
