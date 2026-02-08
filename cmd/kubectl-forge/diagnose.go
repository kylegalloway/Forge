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
		Use:   "diagnose RESOURCE_NAME",
		Short: "Diagnose problems with a Forge resource",
		Long: `Automatically diagnose problems with a ZarfPackageJob or UDSBundleJob.

This command analyzes the CRD status and its associated batch Jobs/pods to find common issues like:
- High retry counts (> 3 retries)
- Stuck in Retrying state (nextRetryTime in the past)
- Queued for too long (> 10 minutes)
- Failed with no batch Job (cleaned up)
- Container failures (OOMKilled, CrashLoopBackOff, ImagePullBackOff)
- Scheduling problems and resource constraints

It shows warning events, relevant logs, and provides suggestions for fixing issues.`,
		Example: `  # Diagnose a failed resource
  kubectl forge diagnose my-package

  # Diagnose with verbose output (all events, not just warnings)
  kubectl forge diagnose my-package --verbose

  # Show more log lines
  kubectl forge diagnose my-package --logs-lines 50

  # Output in JSON format for scripting
  kubectl forge diagnose my-package --output json`,
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

	// Get the CRD resource first
	resource, err := client.GetForgeResource(ctx, namespace, o.JobName)
	if err != nil {
		return fmt.Errorf("failed to find resource %s: %w", o.JobName, err)
	}

	// Build diagnosis starting from CRD-level info
	result := o.buildDiagnosisFromCRD(ctx, client, resource)

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

func (o *DiagnoseOptions) buildDiagnosisFromCRD(ctx context.Context, client *kubectl.Client, resource *kubectl.ForgeResource) DiagnoseResult {
	result := DiagnoseResult{
		Job: JobDiagnosis{
			Name:      resource.Name,
			Namespace: resource.Namespace,
			Type:      resource.ResourceType,
			Action:    resource.Action,
			Phase:     resource.Phase,
			Message:   resource.Message,
			Age:       formatDuration(time.Since(resource.CreatedAt)),
		},
	}

	// CRD-level problem detection
	o.detectCRDProblems(resource, &result)

	// For each operation with a jobName, run pod/container analysis
	for _, op := range resource.Operations {
		if op.JobName == "" {
			if op.Phase == constants.PhaseFailed {
				result.Problems = append(result.Problems, Problem{
					Severity: "warning",
					Resource: fmt.Sprintf("%s/%s (operation: %s)", resource.ResourceType, resource.Name, op.Name),
					Issue:    "Failed with no batch Job",
					Details:  "The batch Job was cleaned up or never created. Check controller logs for details.",
				})
			}
			continue
		}

		job, err := client.FindJob(ctx, resource.Namespace, op.JobName)
		if err != nil {
			result.Problems = append(result.Problems, Problem{
				Severity: "warning",
				Resource: fmt.Sprintf("Job/%s (operation: %s)", op.JobName, op.Name),
				Issue:    "Batch Job not found",
				Details:  fmt.Sprintf("Referenced job %s not found: %v", op.JobName, err),
			})
			continue
		}

		pods, err := client.FindJobPods(ctx, job, false)
		if err != nil {
			continue
		}

		events, err := client.GetJobEvents(ctx, resource.Namespace, op.JobName, o.Verbose)
		if err != nil {
			continue
		}

		// Analyze job-level conditions
		o.analyzeJobConditions(job, &result)

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
	}

	// Generate suggestions based on problems
	result.Suggestions = o.generateSuggestions(result.Problems, result.Events)

	return result
}

func (o *DiagnoseOptions) detectCRDProblems(resource *kubectl.ForgeResource, result *DiagnoseResult) {
	crdRef := fmt.Sprintf("%s/%s", resource.ResourceType, resource.Name)

	for _, op := range resource.Operations {
		opRef := fmt.Sprintf("%s (operation: %s)", crdRef, op.Name)

		// High retry count
		if op.RetryCount > 3 {
			result.Problems = append(result.Problems, Problem{
				Severity: "warning",
				Resource: opRef,
				Issue:    fmt.Sprintf("High retry count: %d", op.RetryCount),
				Details:  "Operation has been retried many times. Check for persistent failures.",
			})
		}

		// Stuck in Retrying with nextRetryTime in the past
		if op.Phase == constants.PhaseRetrying && op.NextRetryTime != nil && op.NextRetryTime.Before(time.Now()) {
			stuckFor := formatDuration(time.Since(*op.NextRetryTime))
			result.Problems = append(result.Problems, Problem{
				Severity: "error",
				Resource: opRef,
				Issue:    "Stuck in Retrying state",
				Details:  fmt.Sprintf("Next retry was scheduled %s ago but has not been processed. Controller may be down or stuck.", stuckFor),
			})
		}

		// Failed with last failure reason
		if op.Phase == constants.PhaseFailed && op.LastFailureReason != "" {
			result.Problems = append(result.Problems, Problem{
				Severity: "error",
				Resource: opRef,
				Issue:    "Operation failed",
				Details:  op.LastFailureReason,
			})
		}
	}

	// Queued for too long
	if resource.Phase == constants.PhaseQueued {
		queuedDuration := time.Since(resource.CreatedAt)
		if queuedDuration > 10*time.Minute {
			result.Problems = append(result.Problems, Problem{
				Severity: "warning",
				Resource: crdRef,
				Issue:    fmt.Sprintf("Queued for %s", formatDuration(queuedDuration)),
				Details:  "Resource has been queued for over 10 minutes. Check queue capacity and controller health.",
			})
		}
	}
}

func (o *DiagnoseOptions) analyzeJobConditions(job *batchv1.Job, result *DiagnoseResult) {
	for _, cond := range job.Status.Conditions {
		if cond.Type == batchv1.JobFailed && cond.Status == corev1.ConditionTrue {
			result.Problems = append(result.Problems, Problem{
				Severity: "error",
				Resource: fmt.Sprintf("Job/%s", job.Name),
				Issue:    "Job failed",
				Details:  cond.Message,
			})
		}
	}
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

func (o *DiagnoseOptions) generateSuggestions(problems []Problem, events []EventSummary) []string {
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
		case strings.Contains(p.Issue, "High retry count"):
			addSuggestion("Review the last failure reason and fix the root cause before retrying")
			addSuggestion("Consider increasing resource limits if failures are resource-related")
		case strings.Contains(p.Issue, "Stuck in Retrying"):
			addSuggestion("Check if the Forge controller is running: kubectl forge status")
			addSuggestion("Check controller logs: kubectl forge logs controller")
		case strings.Contains(p.Issue, "Queued for"):
			addSuggestion("Check queue capacity and concurrency limits in the controller configuration")
			addSuggestion("Check if the Forge controller is running: kubectl forge status")
		case strings.Contains(p.Issue, "Failed with no batch Job"):
			addSuggestion("Check controller logs for job creation failures: kubectl forge logs controller")
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
	colors := NewColorWriter(w)

	// Job header
	//nolint:errcheck // Writing to stdout in CLI context
	fmt.Fprintf(w, "Resource: %s (%s)\n", d.Job.Name, d.Job.Type)
	//nolint:errcheck // Writing to stdout in CLI context
	fmt.Fprintf(w, "Namespace: %s\n", d.Job.Namespace)
	//nolint:errcheck // Writing to stdout in CLI context
	fmt.Fprintf(w, "Action: %s\n", d.Job.Action)
	//nolint:errcheck // Writing to stdout in CLI context
	fmt.Fprintf(w, "Phase: %s\n", colors.Phase(d.Job.Phase))
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
			marker := colors.WarningMarker()
			if p.Severity == "error" {
				marker = colors.Status(false)
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
