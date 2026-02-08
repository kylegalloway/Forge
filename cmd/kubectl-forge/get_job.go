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
	corev1 "k8s.io/api/core/v1"
	"k8s.io/cli-runtime/pkg/genericclioptions"
)

// JobDetails represents detailed job information for output
type JobDetails struct {
	Name        string            `json:"name" yaml:"name"`
	Namespace   string            `json:"namespace" yaml:"namespace"`
	Type        string            `json:"type" yaml:"type"`
	Action      string            `json:"action" yaml:"action"`
	Phase       string            `json:"phase" yaml:"phase"`
	Message     string            `json:"message,omitempty" yaml:"message,omitempty"`
	CreatedAt   string            `json:"createdAt" yaml:"createdAt"`
	Age         string            `json:"age" yaml:"age"`
	Labels      map[string]string `json:"labels,omitempty" yaml:"labels,omitempty"`
	Annotations map[string]string `json:"annotations,omitempty" yaml:"annotations,omitempty"`
	Operations  []OperationDetail `json:"operations,omitempty" yaml:"operations,omitempty"`
	Volumes     []VolumeInfo      `json:"volumes,omitempty" yaml:"volumes,omitempty"`
	Conditions  []ConditionInfo   `json:"conditions,omitempty" yaml:"conditions,omitempty"`
}

// OperationDetail contains detailed info for a single operation
type OperationDetail struct {
	Name              string       `json:"name" yaml:"name"`
	Phase             string       `json:"phase" yaml:"phase"`
	Message           string       `json:"message,omitempty" yaml:"message,omitempty"`
	JobName           string       `json:"jobName,omitempty" yaml:"jobName,omitempty"`
	RetryCount        int32        `json:"retryCount,omitempty" yaml:"retryCount,omitempty"`
	ArtifactLocation  string       `json:"artifactLocation,omitempty" yaml:"artifactLocation,omitempty"`
	LastFailureReason string       `json:"lastFailureReason,omitempty" yaml:"lastFailureReason,omitempty"`
	StartTime         string       `json:"startTime,omitempty" yaml:"startTime,omitempty"`
	CompletionTime    string       `json:"completionTime,omitempty" yaml:"completionTime,omitempty"`
	Duration          string       `json:"duration,omitempty" yaml:"duration,omitempty"`
	NextRetryTime     string       `json:"nextRetryTime,omitempty" yaml:"nextRetryTime,omitempty"`
	Pods              []PodSummary `json:"pods,omitempty" yaml:"pods,omitempty"`
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
	Action       string
}

// NewGetJobCommand creates the get job subcommand
func NewGetJobCommand(configFlags *genericclioptions.ConfigFlags) *cobra.Command {
	o := &GetJobOptions{
		configFlags: configFlags,
		IOStreams:   genericclioptions.IOStreams{In: os.Stdin, Out: os.Stdout, ErrOut: os.Stderr},
	}

	cmd := &cobra.Command{
		Use:   "job RESOURCE_NAME",
		Short: "Get detailed information about a Forge resource",
		Long: `Get detailed information about a ZarfPackageJob or UDSBundleJob, similar to kubectl describe.

Shows CRD-level metadata, status, per-operation details (phase, retry count,
artifact location, timing), associated batch Job pods, volumes, and conditions.
Use --output json or --output yaml for machine-readable output.`,
		Example: `  # Get resource details (describe-like format)
  kubectl forge get job my-package

  # Get resource details in JSON format
  kubectl forge get job my-package --output json

  # Show details for a specific operation
  kubectl forge get job my-package --action build`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			o.JobName = args[0]
			return o.Run(cmd.Context())
		},
	}

	cmd.Flags().StringVarP(&o.OutputFormat, "output", "o", "table", "Output format (table, json, yaml)")
	cmd.Flags().StringVarP(&o.Action, "action", "a", "", "Show details for a specific operation (build, create, publish, deploy)")

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

	resource, err := client.GetForgeResource(ctx, namespace, o.JobName)
	if err != nil {
		return fmt.Errorf("failed to find resource %s: %w", o.JobName, err)
	}

	details := o.toJobDetails(ctx, client, resource)

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

func (o *GetJobOptions) toJobDetails(ctx context.Context, client *kubectl.Client, resource *kubectl.ForgeResource) JobDetails {
	details := JobDetails{
		Name:        resource.Name,
		Namespace:   resource.Namespace,
		Type:        resource.ResourceType,
		Action:      resource.Action,
		Phase:       resource.Phase,
		Message:     resource.Message,
		CreatedAt:   resource.CreatedAt.Format(time.RFC3339),
		Age:         formatDuration(time.Since(resource.CreatedAt)),
		Labels:      resource.Labels,
		Annotations: resource.Annotations,
	}

	// Build operation details
	for _, op := range resource.Operations {
		// If --action is set, only show that operation
		if o.Action != "" && op.Name != o.Action {
			continue
		}

		opDetail := OperationDetail{
			Name:              op.Name,
			Phase:             op.Phase,
			Message:           op.Message,
			JobName:           op.JobName,
			RetryCount:        op.RetryCount,
			ArtifactLocation:  op.ArtifactLocation,
			LastFailureReason: op.LastFailureReason,
		}

		if op.StartTime != nil {
			opDetail.StartTime = op.StartTime.Format(time.RFC3339)
		}
		if op.CompletionTime != nil {
			opDetail.CompletionTime = op.CompletionTime.Format(time.RFC3339)
			if op.StartTime != nil {
				duration := op.CompletionTime.Sub(*op.StartTime)
				opDetail.Duration = duration.Round(time.Second).String()
			}
		}
		if op.NextRetryTime != nil {
			opDetail.NextRetryTime = op.NextRetryTime.Format(time.RFC3339)
		}

		// Fetch batch Job pods if the operation has a jobName
		if op.JobName != "" {
			job, jobErr := client.FindJob(ctx, resource.Namespace, op.JobName)
			if jobErr == nil {
				pods, podsErr := client.FindJobPods(ctx, job, false)
				if podsErr == nil {
					for _, pod := range pods {
						opDetail.Pods = append(opDetail.Pods, toPodSummary(pod))
					}
				}

				// Volumes (from first operation's job)
				if len(details.Volumes) == 0 {
					for _, vol := range job.Spec.Template.Spec.Volumes {
						details.Volumes = append(details.Volumes, toVolumeInfo(vol))
					}
				}

				// Conditions
				if len(details.Conditions) == 0 {
					for _, cond := range job.Status.Conditions {
						details.Conditions = append(details.Conditions, ConditionInfo{
							Type:    string(cond.Type),
							Status:  string(cond.Status),
							Reason:  cond.Reason,
							Message: cond.Message,
						})
					}
				}
			}
		}

		details.Operations = append(details.Operations, opDetail)
	}

	return details
}

func toPodSummary(pod *corev1.Pod) PodSummary {
	ready := 0
	var restarts int32
	for _, cs := range pod.Status.ContainerStatuses {
		if cs.Ready {
			ready++
		}
		restarts += cs.RestartCount
	}
	return PodSummary{
		Name:     pod.Name,
		Status:   string(pod.Status.Phase),
		Ready:    fmt.Sprintf("%d/%d", ready, len(pod.Spec.Containers)),
		Restarts: restarts,
		Age:      formatDuration(time.Since(pod.CreationTimestamp.Time)),
	}
}

func toVolumeInfo(vol corev1.Volume) VolumeInfo {
	v := VolumeInfo{Name: vol.Name}
	switch {
	case vol.PersistentVolumeClaim != nil:
		v.Type = constants.VolumeTypePersistentVolumeClaim
		v.ClaimName = vol.PersistentVolumeClaim.ClaimName
	case vol.ConfigMap != nil:
		v.Type = constants.VolumeTypeConfigMap
		v.ClaimName = vol.ConfigMap.Name
	case vol.Secret != nil: // pragma: allowlist secret
		v.Type = constants.VolumeTypeSecret // pragma: allowlist secret
		v.ClaimName = vol.Secret.SecretName
	case vol.EmptyDir != nil:
		v.Type = constants.VolumeTypeEmptyDir
	case vol.HostPath != nil:
		v.Type = constants.VolumeTypeHostPath
		v.ClaimName = vol.HostPath.Path
	default:
		v.Type = constants.VolumeTypeOther
	}
	return v
}

func (o *GetJobOptions) printDescribeFormat(d JobDetails) error {
	w := o.IOStreams.Out
	colors := NewColorWriter(w)

	//nolint:errcheck // Writing to stdout in CLI context
	fmt.Fprintf(w, "Name:           %s\n", d.Name)
	//nolint:errcheck // Writing to stdout in CLI context
	fmt.Fprintf(w, "Namespace:      %s\n", d.Namespace)
	//nolint:errcheck // Writing to stdout in CLI context
	fmt.Fprintf(w, "Type:           %s\n", d.Type)
	//nolint:errcheck // Writing to stdout in CLI context
	fmt.Fprintf(w, "Action:         %s\n", d.Action)
	//nolint:errcheck // Writing to stdout in CLI context
	fmt.Fprintf(w, "Phase:          %s\n", colors.Phase(d.Phase))
	if d.Message != "" {
		//nolint:errcheck // Writing to stdout in CLI context
		fmt.Fprintf(w, "Message:        %s\n", d.Message)
	}
	//nolint:errcheck // Writing to stdout in CLI context
	fmt.Fprintf(w, "Age:            %s\n", d.Age)
	//nolint:errcheck // Writing to stdout in CLI context
	fmt.Fprintln(w)

	// Operations
	if len(d.Operations) > 0 {
		//nolint:errcheck // Writing to stdout in CLI context
		fmt.Fprintf(w, "Operations:\n")
		for _, op := range d.Operations {
			phase := colors.Phase(op.Phase)
			if op.Phase == "" {
				phase = colors.Phase("Pending")
			}
			//nolint:errcheck // Writing to stdout in CLI context
			fmt.Fprintf(w, "  - %s:\n", op.Name)
			//nolint:errcheck // Writing to stdout in CLI context
			fmt.Fprintf(w, "      Phase:          %s\n", phase)
			if op.Message != "" {
				//nolint:errcheck // Writing to stdout in CLI context
				fmt.Fprintf(w, "      Message:        %s\n", op.Message)
			}
			if op.JobName != "" {
				//nolint:errcheck // Writing to stdout in CLI context
				fmt.Fprintf(w, "      Job:            %s\n", op.JobName)
			}
			if op.RetryCount > 0 {
				//nolint:errcheck // Writing to stdout in CLI context
				fmt.Fprintf(w, "      Retry Count:    %d\n", op.RetryCount)
			}
			if op.ArtifactLocation != "" {
				//nolint:errcheck // Writing to stdout in CLI context
				fmt.Fprintf(w, "      Artifacts:      %s\n", op.ArtifactLocation)
			}
			if op.LastFailureReason != "" {
				//nolint:errcheck // Writing to stdout in CLI context
				fmt.Fprintf(w, "      Last Failure:   %s\n", op.LastFailureReason)
			}
			if op.StartTime != "" {
				//nolint:errcheck // Writing to stdout in CLI context
				fmt.Fprintf(w, "      Start Time:     %s\n", op.StartTime)
			}
			if op.CompletionTime != "" {
				//nolint:errcheck // Writing to stdout in CLI context
				fmt.Fprintf(w, "      Completed At:   %s\n", op.CompletionTime)
			}
			if op.Duration != "" {
				//nolint:errcheck // Writing to stdout in CLI context
				fmt.Fprintf(w, "      Duration:       %s\n", op.Duration)
			}
			if op.NextRetryTime != "" {
				//nolint:errcheck // Writing to stdout in CLI context
				fmt.Fprintf(w, "      Next Retry:     %s\n", op.NextRetryTime)
			}

			// Pods for this operation
			if len(op.Pods) > 0 {
				//nolint:errcheck // Writing to stdout in CLI context
				fmt.Fprintf(w, "      Pods:\n")
				for _, p := range op.Pods {
					//nolint:errcheck // Writing to stdout in CLI context
					fmt.Fprintf(w, "        - %s (%s, Ready: %s, Restarts: %d, Age: %s)\n",
						p.Name, p.Status, p.Ready, p.Restarts, p.Age)
				}
			}
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
