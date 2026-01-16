// Package main implements the kubectl-forge CLI get job command
package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/kylegalloway/forge/pkg/constants"
	"github.com/kylegalloway/forge/pkg/kubectl"
	"github.com/spf13/cobra"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/cli-runtime/pkg/genericclioptions"
)

// JobDetails represents detailed job information for output
type JobDetails struct {
	Name        string            `json:"name" yaml:"name"`
	Namespace   string            `json:"namespace" yaml:"namespace"`
	Labels      map[string]string `json:"labels" yaml:"labels"`
	Annotations map[string]string `json:"annotations,omitempty" yaml:"annotations,omitempty"`
	Package     string            `json:"package" yaml:"package"`
	Action      string            `json:"action" yaml:"action"`
	JobType     string            `json:"jobType" yaml:"jobType"`
	CreatedAt   string            `json:"createdAt" yaml:"createdAt"`
	Age         string            `json:"age" yaml:"age"`
	Status      JobStatusDetails  `json:"status" yaml:"status"`
	Pods        []PodSummary      `json:"pods" yaml:"pods"`
	Volumes     []VolumeInfo      `json:"volumes,omitempty" yaml:"volumes,omitempty"`
	Conditions  []ConditionInfo   `json:"conditions,omitempty" yaml:"conditions,omitempty"`
}

// JobStatusDetails contains job status information
type JobStatusDetails struct {
	Active      int32  `json:"active" yaml:"active"`
	Succeeded   int32  `json:"succeeded" yaml:"succeeded"`
	Failed      int32  `json:"failed" yaml:"failed"`
	StartTime   string `json:"startTime,omitempty" yaml:"startTime,omitempty"`
	CompletedAt string `json:"completedAt,omitempty" yaml:"completedAt,omitempty"`
	Duration    string `json:"duration,omitempty" yaml:"duration,omitempty"`
	Phase       string `json:"phase" yaml:"phase"`
}

// PodSummary contains summary information about a pod
type PodSummary struct {
	Name     string `json:"name" yaml:"name"`
	Status   string `json:"status" yaml:"status"`
	Ready    string `json:"ready" yaml:"ready"`
	Restarts int32  `json:"restarts" yaml:"restarts"`
	Age      string `json:"age" yaml:"age"`
}

// VolumeInfo contains volume information
type VolumeInfo struct {
	Name      string `json:"name" yaml:"name"`
	Type      string `json:"type" yaml:"type"`
	ClaimName string `json:"claimName,omitempty" yaml:"claimName,omitempty"`
}

// ConditionInfo contains job condition information
type ConditionInfo struct {
	Type    string `json:"type" yaml:"type"`
	Status  string `json:"status" yaml:"status"`
	Reason  string `json:"reason,omitempty" yaml:"reason,omitempty"`
	Message string `json:"message,omitempty" yaml:"message,omitempty"`
}

// GetJobOptions holds options for the get job command
type GetJobOptions struct {
	configFlags *genericclioptions.ConfigFlags
	IOStreams   genericclioptions.IOStreams

	JobName      string
	OutputFormat string
}

// NewGetJobCommand creates the get job subcommand
func NewGetJobCommand(configFlags *genericclioptions.ConfigFlags) *cobra.Command {
	o := &GetJobOptions{
		configFlags: configFlags,
		IOStreams:   genericclioptions.IOStreams{In: os.Stdin, Out: os.Stdout, ErrOut: os.Stderr},
	}

	cmd := &cobra.Command{
		Use:   "job JOB_NAME",
		Short: "Get detailed information about a Forge job",
		Long: `Get detailed information about a Forge job, similar to kubectl describe.

Shows job metadata, status, conditions, pods, volumes, and labels.
Use --output json or --output yaml for machine-readable output.`,
		Example: `  # Get job details (describe-like format)
  kubectl forge get job my-package-build

  # Get job details in JSON format
  kubectl forge get job my-package-build --output json

  # Get job details in YAML format
  kubectl forge get job my-package-build --output yaml`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			o.JobName = args[0]
			return o.Run(cmd.Context())
		},
	}

	cmd.Flags().StringVarP(&o.OutputFormat, "output", "o", "table", "Output format (table, json, yaml)")

	return cmd
}

// Run executes the get job command
func (o *GetJobOptions) Run(ctx context.Context) error {
	format, err := kubectl.ParseOutputFormat(o.OutputFormat)
	if err != nil {
		return err
	}

	client, err := kubectl.NewClientFromFlags(o.configFlags)
	if err != nil {
		return fmt.Errorf("failed to create Kubernetes client: %w", err)
	}

	namespace := kubectl.GetNamespace(o.configFlags)

	job, err := client.FindJob(ctx, namespace, o.JobName)
	if err != nil {
		return fmt.Errorf("failed to find job %s: %w", o.JobName, err)
	}

	pods, err := client.FindJobPods(ctx, job, false)
	if err != nil {
		return fmt.Errorf("failed to find pods for job %s: %w", o.JobName, err)
	}

	details := o.toJobDetails(job, pods)

	printer := kubectl.NewPrinter(format, o.IOStreams.Out)

	switch format {
	case kubectl.OutputFormatJSON:
		return printer.PrintJSON(details)
	case kubectl.OutputFormatYAML:
		return printer.PrintYAML(details)
	default:
		return o.printDescribeFormat(details)
	}
}

func (o *GetJobOptions) toJobDetails(job *batchv1.Job, pods []*corev1.Pod) JobDetails {
	details := JobDetails{
		Name:        job.Name,
		Namespace:   job.Namespace,
		Labels:      job.Labels,
		Annotations: job.Annotations,
		Package:     job.Labels[constants.LabelPackage],
		Action:      job.Labels[constants.LabelAction],
		JobType:     job.Labels["resource-type"],
		CreatedAt:   job.CreationTimestamp.Format(time.RFC3339),
		Age:         formatDuration(time.Since(job.CreationTimestamp.Time)),
	}

	// Status
	details.Status = JobStatusDetails{
		Active:    job.Status.Active,
		Succeeded: job.Status.Succeeded,
		Failed:    job.Status.Failed,
	}

	if job.Status.StartTime != nil {
		details.Status.StartTime = job.Status.StartTime.Format(time.RFC3339)
	}
	if job.Status.CompletionTime != nil {
		details.Status.CompletedAt = job.Status.CompletionTime.Format(time.RFC3339)
		if job.Status.StartTime != nil {
			duration := job.Status.CompletionTime.Sub(job.Status.StartTime.Time)
			details.Status.Duration = duration.Round(time.Second).String()
		}
	}

	// Determine phase
	switch {
	case job.Status.Succeeded > 0:
		details.Status.Phase = "Completed"
	case job.Status.Failed > 0:
		details.Status.Phase = "Failed"
	case job.Status.Active > 0:
		details.Status.Phase = "Running"
	default:
		details.Status.Phase = "Pending"
	}

	// Pods
	for _, pod := range pods {
		ready := 0
		var restarts int32
		for _, cs := range pod.Status.ContainerStatuses {
			if cs.Ready {
				ready++
			}
			restarts += cs.RestartCount
		}
		details.Pods = append(details.Pods, PodSummary{
			Name:     pod.Name,
			Status:   string(pod.Status.Phase),
			Ready:    fmt.Sprintf("%d/%d", ready, len(pod.Spec.Containers)),
			Restarts: restarts,
			Age:      formatDuration(time.Since(pod.CreationTimestamp.Time)),
		})
	}

	// Volumes
	for _, vol := range job.Spec.Template.Spec.Volumes {
		v := VolumeInfo{Name: vol.Name}
		switch {
		case vol.PersistentVolumeClaim != nil:
			v.Type = "PersistentVolumeClaim"
			v.ClaimName = vol.PersistentVolumeClaim.ClaimName
		case vol.ConfigMap != nil:
			v.Type = "ConfigMap"
			v.ClaimName = vol.ConfigMap.Name
		case vol.Secret != nil: // pragma: allowlist secret
			v.Type = "Secret" // pragma: allowlist secret
			v.ClaimName = vol.Secret.SecretName
		case vol.EmptyDir != nil:
			v.Type = "EmptyDir"
		case vol.HostPath != nil:
			v.Type = "HostPath"
			v.ClaimName = vol.HostPath.Path
		default:
			v.Type = "Other"
		}
		details.Volumes = append(details.Volumes, v)
	}

	// Conditions
	for _, cond := range job.Status.Conditions {
		details.Conditions = append(details.Conditions, ConditionInfo{
			Type:    string(cond.Type),
			Status:  string(cond.Status),
			Reason:  cond.Reason,
			Message: cond.Message,
		})
	}

	return details
}

func (o *GetJobOptions) printDescribeFormat(d JobDetails) error {
	w := o.IOStreams.Out

	//nolint:errcheck // Writing to stdout in CLI context
	fmt.Fprintf(w, "Name:           %s\n", d.Name)
	//nolint:errcheck // Writing to stdout in CLI context
	fmt.Fprintf(w, "Namespace:      %s\n", d.Namespace)
	//nolint:errcheck // Writing to stdout in CLI context
	fmt.Fprintf(w, "Package:        %s\n", d.Package)
	//nolint:errcheck // Writing to stdout in CLI context
	fmt.Fprintf(w, "Action:         %s\n", d.Action)
	//nolint:errcheck // Writing to stdout in CLI context
	fmt.Fprintf(w, "Job Type:       %s\n", d.JobType)
	//nolint:errcheck // Writing to stdout in CLI context
	fmt.Fprintf(w, "Age:            %s\n", d.Age)
	//nolint:errcheck // Writing to stdout in CLI context
	fmt.Fprintln(w)

	//nolint:errcheck // Writing to stdout in CLI context
	fmt.Fprintf(w, "Status:\n")
	//nolint:errcheck // Writing to stdout in CLI context
	fmt.Fprintf(w, "  Phase:        %s\n", d.Status.Phase)
	//nolint:errcheck // Writing to stdout in CLI context
	fmt.Fprintf(w, "  Active:       %d\n", d.Status.Active)
	//nolint:errcheck // Writing to stdout in CLI context
	fmt.Fprintf(w, "  Succeeded:    %d\n", d.Status.Succeeded)
	//nolint:errcheck // Writing to stdout in CLI context
	fmt.Fprintf(w, "  Failed:       %d\n", d.Status.Failed)
	if d.Status.StartTime != "" {
		//nolint:errcheck // Writing to stdout in CLI context
		fmt.Fprintf(w, "  Start Time:   %s\n", d.Status.StartTime)
	}
	if d.Status.CompletedAt != "" {
		//nolint:errcheck // Writing to stdout in CLI context
		fmt.Fprintf(w, "  Completed At: %s\n", d.Status.CompletedAt)
	}
	if d.Status.Duration != "" {
		//nolint:errcheck // Writing to stdout in CLI context
		fmt.Fprintf(w, "  Duration:     %s\n", d.Status.Duration)
	}
	//nolint:errcheck // Writing to stdout in CLI context
	fmt.Fprintln(w)

	if len(d.Pods) > 0 {
		//nolint:errcheck // Writing to stdout in CLI context
		fmt.Fprintf(w, "Pods:\n")
		for _, p := range d.Pods {
			//nolint:errcheck // Writing to stdout in CLI context
			fmt.Fprintf(w, "  - %s\n", p.Name)
			//nolint:errcheck // Writing to stdout in CLI context
			fmt.Fprintf(w, "      Status:   %s\n", p.Status)
			//nolint:errcheck // Writing to stdout in CLI context
			fmt.Fprintf(w, "      Ready:    %s\n", p.Ready)
			//nolint:errcheck // Writing to stdout in CLI context
			fmt.Fprintf(w, "      Restarts: %d\n", p.Restarts)
			//nolint:errcheck // Writing to stdout in CLI context
			fmt.Fprintf(w, "      Age:      %s\n", p.Age)
		}
		//nolint:errcheck // Writing to stdout in CLI context
		fmt.Fprintln(w)
	}

	if len(d.Volumes) > 0 {
		//nolint:errcheck // Writing to stdout in CLI context
		fmt.Fprintf(w, "Volumes:\n")
		for _, v := range d.Volumes {
			if v.ClaimName != "" {
				//nolint:errcheck // Writing to stdout in CLI context
				fmt.Fprintf(w, "  - %s (%s: %s)\n", v.Name, v.Type, v.ClaimName)
			} else {
				//nolint:errcheck // Writing to stdout in CLI context
				fmt.Fprintf(w, "  - %s (%s)\n", v.Name, v.Type)
			}
		}
		//nolint:errcheck // Writing to stdout in CLI context
		fmt.Fprintln(w)
	}

	if len(d.Conditions) > 0 {
		//nolint:errcheck // Writing to stdout in CLI context
		fmt.Fprintf(w, "Conditions:\n")
		for _, c := range d.Conditions {
			//nolint:errcheck // Writing to stdout in CLI context
			fmt.Fprintf(w, "  %s: %s", c.Type, c.Status)
			if c.Reason != "" {
				//nolint:errcheck // Writing to stdout in CLI context
				fmt.Fprintf(w, " (%s)", c.Reason)
			}
			//nolint:errcheck // Writing to stdout in CLI context
			fmt.Fprintln(w)
			if c.Message != "" {
				//nolint:errcheck // Writing to stdout in CLI context
				fmt.Fprintf(w, "    Message: %s\n", c.Message)
			}
		}
		//nolint:errcheck // Writing to stdout in CLI context
		fmt.Fprintln(w)
	}

	if len(d.Labels) > 0 {
		//nolint:errcheck // Writing to stdout in CLI context
		fmt.Fprintf(w, "Labels:\n")
		for k, v := range d.Labels {
			//nolint:errcheck // Writing to stdout in CLI context
			fmt.Fprintf(w, "  %s=%s\n", k, v)
		}
	}

	return nil
}
