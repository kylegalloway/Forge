// Package main implements the kubectl-forge CLI diagnose command
package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/kylegalloway/forge/pkg/constants"
	"github.com/kylegalloway/forge/pkg/kubectl"
	"github.com/spf13/cobra"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/cli-runtime/pkg/genericclioptions"
)

// DiagnoseOptions holds options for the diagnose command
type DiagnoseOptions struct {
	configFlags *genericclioptions.ConfigFlags
	IOStreams   genericclioptions.IOStreams

	JobName      string
	Verbose      bool
	LogsLines    int64
	OutputFormat string
}

// DiagnoseResult contains all diagnostic information
type DiagnoseResult struct {
	Job         JobDiagnosis    `json:"job" yaml:"job"`
	Problems    []Problem       `json:"problems" yaml:"problems"`
	Events      []EventSummary  `json:"events" yaml:"events"`
	Logs        []ContainerLogs `json:"logs,omitempty" yaml:"logs,omitempty"`
	Suggestions []string        `json:"suggestions,omitempty" yaml:"suggestions,omitempty"`
}

// JobDiagnosis contains job-level diagnostic information
type JobDiagnosis struct {
	Name      string `json:"name" yaml:"name"`
	Namespace string `json:"namespace" yaml:"namespace"`
	Type      string `json:"type" yaml:"type"`
	Action    string `json:"action" yaml:"action"`
	Phase     string `json:"phase" yaml:"phase"`
	Age       string `json:"age" yaml:"age"`
	Message   string `json:"message,omitempty" yaml:"message,omitempty"`
}

// Problem represents a detected issue
type Problem struct {
	Severity string `json:"severity" yaml:"severity"` // "error", "warning"
	Resource string `json:"resource" yaml:"resource"` // "Pod/name", "Job/name"
	Issue    string `json:"issue" yaml:"issue"`
	Details  string `json:"details,omitempty" yaml:"details,omitempty"`
}

// EventSummary contains summarized event information
type EventSummary struct {
	Age     string `json:"age" yaml:"age"`
	Type    string `json:"type" yaml:"type"`
	Reason  string `json:"reason" yaml:"reason"`
	Object  string `json:"object" yaml:"object"`
	Message string `json:"message" yaml:"message"`
}

// ContainerLogs contains logs from a container
type ContainerLogs struct {
	Pod       string `json:"pod" yaml:"pod"`
	Container string `json:"container" yaml:"container"`
	Logs      string `json:"logs" yaml:"logs"`
}

// NewDiagnoseCommand creates the diagnose command
func NewDiagnoseCommand(configFlags *genericclioptions.ConfigFlags) *cobra.Command {
	o := &DiagnoseOptions{
		configFlags: configFlags,
		IOStreams:   genericclioptions.IOStreams{In: os.Stdin, Out: os.Stdout, ErrOut: os.Stderr},
		LogsLines:   20,
	}

	cmd := &cobra.Command{
		Use:   "diagnose JOB_NAME",
		Short: "Diagnose problems with a Forge job",
		Long: `Automatically diagnose problems with a Forge job.

This command analyzes a job and its pods to find common issues like:
- Container failures (OOMKilled, CrashLoopBackOff, ImagePullBackOff)
- Scheduling problems
- Resource constraints
- Configuration errors

It shows warning events, relevant logs, and provides suggestions for fixing issues.`,
		Example: `  # Diagnose a failed job
  kubectl forge diagnose my-package-build

  # Diagnose with verbose output (all events, not just warnings)
  kubectl forge diagnose my-package-build --verbose

  # Show more log lines
  kubectl forge diagnose my-package-build --logs-lines 50

  # Output in JSON format for scripting
  kubectl forge diagnose my-package-build --output json`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			o.JobName = args[0]
			return o.Run(cmd.Context())
		},
	}

	cmd.Flags().BoolVarP(&o.Verbose, "verbose", "v", false, "Show all events, not just warnings")
	cmd.Flags().Int64Var(&o.LogsLines, "logs-lines", 20, "Number of log lines to show from failed containers")
	cmd.Flags().StringVarP(&o.OutputFormat, "output", "o", "table", "Output format (table, json, yaml)")

	return cmd
}

// Run executes the diagnose command
func (o *DiagnoseOptions) Run(ctx context.Context) error {
	format, err := kubectl.ParseOutputFormat(o.OutputFormat)
	if err != nil {
		return err
	}

	client, err := kubectl.NewClientFromFlags(o.configFlags)
	if err != nil {
		return fmt.Errorf("failed to create Kubernetes client: %w", err)
	}

	namespace := kubectl.GetNamespace(o.configFlags)

	// Find the job
	job, err := client.FindJob(ctx, namespace, o.JobName)
	if err != nil {
		return fmt.Errorf("failed to find job %s: %w", o.JobName, err)
	}

	// Find pods for the job
	pods, err := client.FindJobPods(ctx, job, false)
	if err != nil {
		return fmt.Errorf("failed to find pods for job %s: %w", o.JobName, err)
	}

	// Get events
	events, err := client.GetJobEvents(ctx, namespace, o.JobName, o.Verbose)
	if err != nil {
		return fmt.Errorf("failed to get events: %w", err)
	}

	// Build diagnosis
	result := o.buildDiagnosis(ctx, client, job, pods, events)

	// Output
	printer := kubectl.NewPrinter(format, o.IOStreams.Out)

	switch format {
	case kubectl.OutputFormatJSON:
		return printer.PrintJSON(result)
	case kubectl.OutputFormatYAML:
		return printer.PrintYAML(result)
	default:
		return o.printDiagnosis(result)
	}
}

func (o *DiagnoseOptions) buildDiagnosis(ctx context.Context, client *kubectl.Client, job *batchv1.Job, pods []*corev1.Pod, events []corev1.Event) DiagnoseResult {
	result := DiagnoseResult{
		Job: JobDiagnosis{
			Name:      job.Name,
			Namespace: job.Namespace,
			Type:      job.Labels["resource-type"],
			Action:    job.Labels[constants.LabelAction],
			Age:       formatDuration(time.Since(job.CreationTimestamp.Time)),
		},
	}

	// Determine phase
	switch {
	case job.Status.Succeeded > 0:
		result.Job.Phase = constants.PhaseCompleted
	case job.Status.Failed > 0:
		result.Job.Phase = constants.PhaseFailed
	case job.Status.Active > 0:
		result.Job.Phase = constants.PhaseRunning
	default:
		result.Job.Phase = constants.PhasePending
	}

	// Check for job-level issues
	for _, cond := range job.Status.Conditions {
		if cond.Type == batchv1.JobFailed && cond.Status == corev1.ConditionTrue {
			result.Problems = append(result.Problems, Problem{
				Severity: "error",
				Resource: fmt.Sprintf("Job/%s", job.Name),
				Issue:    "Job failed",
				Details:  cond.Message,
			})
			if cond.Message != "" {
				result.Job.Message = cond.Message
			}
		}
	}

	// Analyze pods
	for _, pod := range pods {
		problems, logs := o.analyzePod(ctx, client, pod)
		result.Problems = append(result.Problems, problems...)
		result.Logs = append(result.Logs, logs...)
	}

	// Process events
	sort.Slice(events, func(i, j int) bool {
		return events[i].LastTimestamp.Before(&events[j].LastTimestamp)
	})

	for _, e := range events {
		age := "unknown"
		if !e.LastTimestamp.IsZero() {
			age = formatDuration(time.Since(e.LastTimestamp.Time))
		}
		result.Events = append(result.Events, EventSummary{
			Age:     age,
			Type:    e.Type,
			Reason:  e.Reason,
			Object:  fmt.Sprintf("%s/%s", e.InvolvedObject.Kind, e.InvolvedObject.Name),
			Message: e.Message,
		})
	}

	// Generate suggestions based on problems
	result.Suggestions = o.generateSuggestions(result.Problems, events)

	return result
}

func (o *DiagnoseOptions) analyzePod(ctx context.Context, client *kubectl.Client, pod *corev1.Pod) ([]Problem, []ContainerLogs) {
	var problems []Problem
	var logs []ContainerLogs

	podRef := fmt.Sprintf("Pod/%s", pod.Name)

	// Check pod phase
	switch pod.Status.Phase {
	case corev1.PodFailed:
		problems = append(problems, Problem{
			Severity: "error",
			Resource: podRef,
			Issue:    "Pod failed",
			Details:  pod.Status.Message,
		})
	case corev1.PodPending:
		// Check for scheduling issues
		for _, cond := range pod.Status.Conditions {
			if cond.Type == corev1.PodScheduled && cond.Status == corev1.ConditionFalse {
				problems = append(problems, Problem{
					Severity: "warning",
					Resource: podRef,
					Issue:    "Pod not scheduled",
					Details:  cond.Message,
				})
			}
		}
	}

	// Analyze init containers
	for _, cs := range pod.Status.InitContainerStatuses {
		p, l := o.analyzeContainerStatus(ctx, client, pod, cs, true)
		problems = append(problems, p...)
		logs = append(logs, l...)
	}

	// Analyze regular containers
	for _, cs := range pod.Status.ContainerStatuses {
		p, l := o.analyzeContainerStatus(ctx, client, pod, cs, false)
		problems = append(problems, p...)
		logs = append(logs, l...)
	}

	return problems, logs
}

func (o *DiagnoseOptions) analyzeContainerStatus(ctx context.Context, client *kubectl.Client, pod *corev1.Pod, cs corev1.ContainerStatus, isInit bool) ([]Problem, []ContainerLogs) {
	var problems []Problem
	var logs []ContainerLogs

	containerType := "Container"
	if isInit {
		containerType = "InitContainer"
	}
	ref := fmt.Sprintf("Pod/%s %s/%s", pod.Name, containerType, cs.Name)

	// Check waiting state
	if cs.State.Waiting != nil {
		w := cs.State.Waiting
		switch w.Reason {
		case "ImagePullBackOff", "ErrImagePull":
			problems = append(problems, Problem{
				Severity: "error",
				Resource: ref,
				Issue:    w.Reason,
				Details:  w.Message,
			})
		case "CrashLoopBackOff":
			problems = append(problems, Problem{
				Severity: "error",
				Resource: ref,
				Issue:    "CrashLoopBackOff",
				Details:  fmt.Sprintf("Container is crash looping (restarts: %d)", cs.RestartCount),
			})
			// Get logs for crash looping container
			if l := o.getContainerLogs(ctx, client, pod, cs.Name, true); l != "" {
				logs = append(logs, ContainerLogs{
					Pod:       pod.Name,
					Container: cs.Name,
					Logs:      l,
				})
			}
		case "ContainerCreating":
			if time.Since(pod.CreationTimestamp.Time) > 5*time.Minute {
				problems = append(problems, Problem{
					Severity: "warning",
					Resource: ref,
					Issue:    "Container stuck creating",
					Details:  "Container has been creating for over 5 minutes",
				})
			}
		case "CreateContainerConfigError":
			problems = append(problems, Problem{
				Severity: "error",
				Resource: ref,
				Issue:    "Container config error",
				Details:  w.Message,
			})
		}
	}

	// Check terminated state
	if cs.State.Terminated != nil {
		t := cs.State.Terminated
		if t.ExitCode != 0 {
			issue := fmt.Sprintf("Exited with code %d", t.ExitCode)
			details := t.Message

			// Check for OOMKilled
			if t.Reason == "OOMKilled" {
				issue = "OOMKilled"
				details = "Container was killed due to out of memory"
			}

			problems = append(problems, Problem{
				Severity: "error",
				Resource: ref,
				Issue:    issue,
				Details:  details,
			})

			// Get logs for failed container
			if l := o.getContainerLogs(ctx, client, pod, cs.Name, false); l != "" {
				logs = append(logs, ContainerLogs{
					Pod:       pod.Name,
					Container: cs.Name,
					Logs:      l,
				})
			}
		}
	}

	// Check last terminated state (for containers that restarted)
	if cs.LastTerminationState.Terminated != nil {
		t := cs.LastTerminationState.Terminated
		if t.Reason == "OOMKilled" {
			problems = append(problems, Problem{
				Severity: "warning",
				Resource: ref,
				Issue:    "Previously OOMKilled",
				Details:  fmt.Sprintf("Container was OOMKilled at %s", t.FinishedAt.Format(time.RFC3339)),
			})
		}
	}

	return problems, logs
}

func (o *DiagnoseOptions) getContainerLogs(ctx context.Context, client *kubectl.Client, pod *corev1.Pod, container string, previous bool) string {
	var buf bytes.Buffer
	opts := &kubectl.LogOptions{
		Container: container,
		TailLines: o.LogsLines,
		Previous:  previous,
	}

	if err := client.GetPodLogs(ctx, pod, opts, &buf); err != nil {
		return ""
	}

	return strings.TrimSpace(buf.String())
}

func (o *DiagnoseOptions) generateSuggestions(problems []Problem, events []corev1.Event) []string {
	var suggestions []string
	seen := make(map[string]bool)

	addSuggestion := func(s string) {
		if !seen[s] {
			seen[s] = true
			suggestions = append(suggestions, s)
		}
	}

	for _, p := range problems {
		switch {
		case strings.Contains(p.Issue, "OOMKilled"):
			addSuggestion("Increase memory limit in the job spec or reduce memory usage in the build")
		case p.Issue == "CrashLoopBackOff":
			addSuggestion("Check container logs above for the crash reason")
			addSuggestion("Verify the container image and entrypoint are correct")
		case strings.Contains(p.Issue, "ImagePullBackOff") || strings.Contains(p.Issue, "ErrImagePull"):
			addSuggestion("Verify the image name and tag are correct")
			addSuggestion("Check if image pull secrets are configured correctly")
			addSuggestion("Verify network access to the container registry")
		case strings.Contains(p.Issue, "not scheduled"):
			addSuggestion("Check cluster resource availability (CPU, memory, nodes)")
			addSuggestion("Verify node selectors and tolerations match available nodes")
		case strings.Contains(p.Issue, "config error"):
			addSuggestion("Check ConfigMap and Secret references in the job spec")
			addSuggestion("Verify all referenced resources exist in the namespace")
		}
	}

	// Check events for additional insights
	for _, e := range events {
		switch e.Reason {
		case "FailedScheduling":
			if strings.Contains(e.Message, "insufficient") {
				addSuggestion("Cluster has insufficient resources - consider scaling up or freeing resources")
			}
		case "FailedMount":
			addSuggestion("Check that PVCs exist and are in Bound state")
			addSuggestion("Verify storage class is available and has capacity")
		case "FailedAttachVolume":
			addSuggestion("Check that the volume is not attached to another node")
			addSuggestion("Verify the storage provider is functioning correctly")
		}
	}

	return suggestions
}

func (o *DiagnoseOptions) printDiagnosis(d DiagnoseResult) error {
	w := o.IOStreams.Out

	// Job header
	//nolint:errcheck // Writing to stdout in CLI context
	fmt.Fprintf(w, "Job: %s (%s)\n", d.Job.Name, d.Job.Type)
	//nolint:errcheck // Writing to stdout in CLI context
	fmt.Fprintf(w, "Namespace: %s\n", d.Job.Namespace)
	//nolint:errcheck // Writing to stdout in CLI context
	fmt.Fprintf(w, "Action: %s\n", d.Job.Action)
	//nolint:errcheck // Writing to stdout in CLI context
	fmt.Fprintf(w, "Phase: %s\n", d.Job.Phase)
	//nolint:errcheck // Writing to stdout in CLI context
	fmt.Fprintf(w, "Age: %s\n", d.Job.Age)
	if d.Job.Message != "" {
		//nolint:errcheck // Writing to stdout in CLI context
		fmt.Fprintf(w, "Message: %s\n", d.Job.Message)
	}
	//nolint:errcheck // Writing to stdout in CLI context
	fmt.Fprintln(w)

	// Problems
	if len(d.Problems) > 0 {
		//nolint:errcheck // Writing to stdout in CLI context
		fmt.Fprintf(w, "--- Problems Found ---\n")
		for _, p := range d.Problems {
			marker := "!"
			if p.Severity == "error" {
				marker = "X"
			}
			//nolint:errcheck // Writing to stdout in CLI context
			fmt.Fprintf(w, "%s %s: %s\n", marker, p.Resource, p.Issue)
			if p.Details != "" {
				//nolint:errcheck // Writing to stdout in CLI context
				fmt.Fprintf(w, "  %s\n", p.Details)
			}
		}
		//nolint:errcheck // Writing to stdout in CLI context
		fmt.Fprintln(w)
	} else {
		//nolint:errcheck // Writing to stdout in CLI context
		fmt.Fprintf(w, "--- No Problems Found ---\n")
		//nolint:errcheck // Writing to stdout in CLI context
		fmt.Fprintln(w)
	}

	// Events
	if len(d.Events) > 0 {
		eventType := "Warning Events"
		if o.Verbose {
			eventType = "Events"
		}
		//nolint:errcheck // Writing to stdout in CLI context
		fmt.Fprintf(w, "--- %s ---\n", eventType)
		for _, e := range d.Events {
			marker := " "
			if e.Type == "Warning" {
				marker = "!"
			}
			msg := e.Message
			if len(msg) > 70 {
				msg = msg[:67] + "..."
			}
			//nolint:errcheck // Writing to stdout in CLI context
			fmt.Fprintf(w, "%s [%s] %s: %s\n", marker, e.Age, e.Reason, msg)
		}
		//nolint:errcheck // Writing to stdout in CLI context
		fmt.Fprintln(w)
	}

	// Logs from failed containers
	if len(d.Logs) > 0 {
		//nolint:errcheck // Writing to stdout in CLI context
		fmt.Fprintf(w, "--- Container Logs ---\n")
		for _, l := range d.Logs {
			//nolint:errcheck // Writing to stdout in CLI context
			fmt.Fprintf(w, "==> %s/%s <==\n", l.Pod, l.Container)
			//nolint:errcheck // Writing to stdout in CLI context
			fmt.Fprintln(w, l.Logs)
			//nolint:errcheck // Writing to stdout in CLI context
			fmt.Fprintln(w)
		}
	}

	// Suggestions
	if len(d.Suggestions) > 0 {
		//nolint:errcheck // Writing to stdout in CLI context
		fmt.Fprintf(w, "--- Suggestions ---\n")
		for _, s := range d.Suggestions {
			//nolint:errcheck // Writing to stdout in CLI context
			fmt.Fprintf(w, "* %s\n", s)
		}
	}

	return nil
}
