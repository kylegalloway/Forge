// Package main implements the kubectl-forge CLI get pods command
package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/kylegalloway/forge/pkg/kubectl"
	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/cli-runtime/pkg/genericclioptions"
)

// PodInfo represents pod information for output
type PodInfo struct {
	Name       string          `json:"name" yaml:"name"`
	Namespace  string          `json:"namespace" yaml:"namespace"`
	Status     string          `json:"status" yaml:"status"`
	Ready      string          `json:"ready" yaml:"ready"`
	Restarts   int32           `json:"restarts" yaml:"restarts"`
	Age        string          `json:"age" yaml:"age"`
	Node       string          `json:"node" yaml:"node"`
	IP         string          `json:"ip" yaml:"ip"`
	Containers []ContainerInfo `json:"containers,omitempty" yaml:"containers,omitempty"`
}

// ContainerInfo represents container information within a pod
type ContainerInfo struct {
	Name         string `json:"name" yaml:"name"`
	Image        string `json:"image" yaml:"image"`
	Ready        bool   `json:"ready" yaml:"ready"`
	RestartCount int32  `json:"restartCount" yaml:"restartCount"`
	State        string `json:"state" yaml:"state"`
	ExitCode     *int32 `json:"exitCode,omitempty" yaml:"exitCode,omitempty"`
}

// GetPodsOptions holds options for the get pods command
type GetPodsOptions struct {
	configFlags *genericclioptions.ConfigFlags
	IOStreams   genericclioptions.IOStreams

	JobName      string
	Action       string
	OutputFormat string
	ShowAll      bool
}

// NewGetPodsCommand creates the get pods subcommand
func NewGetPodsCommand(configFlags *genericclioptions.ConfigFlags) *cobra.Command {
	o := &GetPodsOptions{
		configFlags: configFlags,
		IOStreams:   genericclioptions.IOStreams{In: os.Stdin, Out: os.Stdout, ErrOut: os.Stderr},
	}

	cmd := &cobra.Command{
		Use:   "pods RESOURCE_NAME",
		Short: "Get pods associated with a Forge resource",
		Long: `Get information about pods running a Forge resource's batch Job.

Shows pod name, status, ready state, restarts, age, node, and IP address.
Use --action to target a specific operation (build, create, publish, deploy).
Use --output to get detailed container information in JSON or YAML format.`,
		Example: `  # Get pods for a resource (table format)
  kubectl forge get pods my-package

  # Get pods for a specific operation
  kubectl forge get pods my-package --action build

  # Get pods in JSON format (includes container details)
  kubectl forge get pods my-package --output json

  # Get pods in YAML format
  kubectl forge get pods my-package --output yaml

  # Include all pods (including terminated)
  kubectl forge get pods my-package --show-all`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			o.JobName = args[0]
			return o.Run(cmd.Context())
		},
	}

	cmd.Flags().StringVarP(&o.Action, "action", "a", "", "Show pods for a specific operation (build, create, publish, deploy)")
	cmd.Flags().StringVarP(&o.OutputFormat, "output", "o", "table", "Output format (table, json, yaml)")
	cmd.Flags().BoolVar(&o.ShowAll, "show-all", false, "Show all pods including terminated")

	return cmd
}

// Run executes the get pods command
func (o *GetPodsOptions) Run(ctx context.Context) error {
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

	pods, err := client.FindJobPods(ctx, job, false)
	if err != nil {
		return fmt.Errorf("failed to find pods for job %s: %w", job.Name, err)
	}

	if len(pods) == 0 {
		//nolint:errcheck // Writing to stdout in CLI context
		fmt.Fprintf(o.IOStreams.Out, "No pods found for resource %s\n", o.JobName)
		return nil
	}

	// Convert to PodInfo structs
	podInfos := make([]PodInfo, 0, len(pods))
	for _, pod := range pods {
		podInfos = append(podInfos, o.toPodInfo(pod))
	}

	printer := kubectl.NewPrinter(format, o.IOStreams.Out)

	switch format {
	case kubectl.OutputFormatJSON:
		return printer.PrintJSON(podInfos)
	case kubectl.OutputFormatYAML:
		return printer.PrintYAML(podInfos)
	default:
		headers := []string{"NAME", "STATUS", "READY", "RESTARTS", "AGE", "NODE", "IP"}
		rows := make([][]string, 0, len(podInfos))
		for _, p := range podInfos {
			rows = append(rows, []string{
				p.Name, p.Status, p.Ready,
				fmt.Sprintf("%d", p.Restarts), p.Age, p.Node, p.IP,
			})
		}
		return printer.PrintTable(headers, rows)
	}
}

func (o *GetPodsOptions) toPodInfo(pod *corev1.Pod) PodInfo {
	ready := 0
	total := len(pod.Spec.Containers)
	var restarts int32

	containers := make([]ContainerInfo, 0, len(pod.Status.ContainerStatuses))

	// Include init container statuses
	for _, cs := range pod.Status.InitContainerStatuses {
		restarts += cs.RestartCount
		containers = append(containers, o.toContainerInfo(cs))
	}

	// Include regular container statuses
	for _, cs := range pod.Status.ContainerStatuses {
		if cs.Ready {
			ready++
		}
		restarts += cs.RestartCount
		containers = append(containers, o.toContainerInfo(cs))
	}

	age := time.Since(pod.CreationTimestamp.Time).Round(time.Second).String()

	// Determine pod status string
	status := string(pod.Status.Phase)
	if pod.DeletionTimestamp != nil {
		status = "Terminating"
	}

	// Check for specific container states that override phase
	for _, cs := range pod.Status.ContainerStatuses {
		if cs.State.Waiting != nil && cs.State.Waiting.Reason != "" {
			status = cs.State.Waiting.Reason
			break
		}
		if cs.State.Terminated != nil && cs.State.Terminated.Reason != "" {
			status = cs.State.Terminated.Reason
		}
	}

	return PodInfo{
		Name:       pod.Name,
		Namespace:  pod.Namespace,
		Status:     status,
		Ready:      fmt.Sprintf("%d/%d", ready, total),
		Restarts:   restarts,
		Age:        age,
		Node:       pod.Spec.NodeName,
		IP:         pod.Status.PodIP,
		Containers: containers,
	}
}

func (o *GetPodsOptions) toContainerInfo(cs corev1.ContainerStatus) ContainerInfo {
	state := "unknown"
	var exitCode *int32

	switch {
	case cs.State.Running != nil:
		state = "running"
	case cs.State.Waiting != nil:
		state = cs.State.Waiting.Reason
		if state == "" {
			state = "waiting"
		}
	case cs.State.Terminated != nil:
		state = cs.State.Terminated.Reason
		if state == "" {
			state = "terminated"
		}
		exitCode = &cs.State.Terminated.ExitCode
	}

	return ContainerInfo{
		Name:         cs.Name,
		Image:        cs.Image,
		Ready:        cs.Ready,
		RestartCount: cs.RestartCount,
		State:        state,
		ExitCode:     exitCode,
	}
}
