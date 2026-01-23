// Package main implements the kubectl-forge CLI logs command for system components
package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/kubernetes"
)

// LogsOptions holds options for the logs command
type LogsOptions struct {
	configFlags *genericclioptions.ConfigFlags
	IOStreams   genericclioptions.IOStreams

	Component       string
	SystemNamespace string
	Follow          bool
	Errors          bool
	Since           string
	TailLines       int64
	AllPods         bool
}

// NewLogsCommand creates the logs command for system components
func NewLogsCommand(configFlags *genericclioptions.ConfigFlags) *cobra.Command {
	o := &LogsOptions{
		configFlags:     configFlags,
		IOStreams:       genericclioptions.IOStreams{In: os.Stdin, Out: os.Stdout, ErrOut: os.Stderr},
		SystemNamespace: forgeSystemNamespace,
		TailLines:       100,
	}

	cmd := &cobra.Command{
		Use:   "logs COMPONENT",
		Short: "Get logs from Forge system components",
		Long: `Get logs from Forge controller or webhook pods.

This is a convenience command that auto-discovers the Forge system pods
and retrieves their logs without needing to know the exact pod names.

Available components:
  controller - The Forge controller that manages jobs
  webhook    - The validating webhook for policy enforcement`,
		Example: `  # Get controller logs
  kubectl forge logs controller

  # Get webhook logs
  kubectl forge logs webhook

  # Follow controller logs in real-time
  kubectl forge logs controller --follow

  # Get logs from last 5 minutes
  kubectl forge logs controller --since 5m

  # Get last 50 lines
  kubectl forge logs controller --tail 50

  # Get logs from all pods (when multiple replicas)
  kubectl forge logs webhook --all`,
		Args:      cobra.ExactArgs(1),
		ValidArgs: []string{"controller", "webhook"},
		RunE: func(cmd *cobra.Command, args []string) error {
			o.Component = strings.ToLower(args[0])
			if o.Component != "controller" && o.Component != "webhook" {
				return fmt.Errorf("invalid component %q: must be 'controller' or 'webhook'", o.Component)
			}
			return o.Run(cmd.Context())
		},
	}

	cmd.Flags().StringVarP(&o.SystemNamespace, "system-namespace", "s", forgeSystemNamespace, "Namespace where Forge system components are installed")
	cmd.Flags().BoolVarP(&o.Follow, "follow", "f", false, "Follow log output in real-time")
	cmd.Flags().BoolVarP(&o.Errors, "errors", "e", false, "Filter to error-level logs only")
	cmd.Flags().StringVar(&o.Since, "since", "", "Only return logs newer than a relative duration (e.g., 5m, 1h)")
	cmd.Flags().Int64Var(&o.TailLines, "tail", 100, "Number of lines to show from end of logs")
	cmd.Flags().BoolVar(&o.AllPods, "all", false, "Get logs from all pods (when multiple replicas)")

	return cmd
}

// Run executes the logs command
func (o *LogsOptions) Run(ctx context.Context) error {
	restConfig, err := o.configFlags.ToRESTConfig()
	if err != nil {
		return fmt.Errorf("failed to create REST config: %w", err)
	}

	client, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return fmt.Errorf("failed to create Kubernetes client: %w", err)
	}

	// Determine label selector based on component
	var labelSelector string
	switch o.Component {
	case "controller":
		labelSelector = fmt.Sprintf("app=%s", controllerAppLabel)
	case "webhook":
		labelSelector = fmt.Sprintf("app=%s", webhookAppLabel)
	}

	// Find pods
	pods, err := client.CoreV1().Pods(o.SystemNamespace).List(ctx, metav1.ListOptions{
		LabelSelector: labelSelector,
	})
	if err != nil {
		return fmt.Errorf("failed to list pods: %w", err)
	}

	if len(pods.Items) == 0 {
		return fmt.Errorf("no %s pods found in namespace %s", o.Component, o.SystemNamespace)
	}

	// Filter to running pods
	var runningPods []corev1.Pod
	for _, pod := range pods.Items {
		if pod.Status.Phase == corev1.PodRunning {
			runningPods = append(runningPods, pod)
		}
	}

	if len(runningPods) == 0 {
		return fmt.Errorf("no running %s pods found", o.Component)
	}

	// If not --all, just use first pod
	if !o.AllPods {
		runningPods = runningPods[:1]
	}

	// Build log options
	logOpts := &corev1.PodLogOptions{
		Follow: o.Follow,
	}

	if o.TailLines > 0 {
		logOpts.TailLines = &o.TailLines
	}

	if o.Since != "" {
		duration, err := parseDuration(o.Since)
		if err != nil {
			return fmt.Errorf("invalid --since duration: %w", err)
		}
		seconds := int64(duration.Seconds())
		logOpts.SinceSeconds = &seconds
	}

	// Get logs from each pod
	for i, pod := range runningPods {
		if len(runningPods) > 1 {
			//nolint:errcheck // Writing to stdout in CLI context
			fmt.Fprintf(o.IOStreams.Out, "==> %s <==\n", pod.Name)
		}

		req := client.CoreV1().Pods(o.SystemNamespace).GetLogs(pod.Name, logOpts)
		stream, err := req.Stream(ctx)
		if err != nil {
			//nolint:errcheck // Writing to stderr in CLI context
			fmt.Fprintf(o.IOStreams.ErrOut, "Warning: failed to get logs from %s: %v\n", pod.Name, err)
			continue
		}

		if o.Errors {
			// Filter to error lines
			if err := o.filterErrorLogs(stream, o.IOStreams.Out); err != nil {
				//nolint:errcheck,gosec // Best-effort close in error path
				stream.Close()
				//nolint:errcheck // Writing to stderr in CLI context
				fmt.Fprintf(o.IOStreams.ErrOut, "Warning: error reading logs from %s: %v\n", pod.Name, err)
				continue
			}
		} else {
			//nolint:errcheck,gosec // Best effort copy to stdout
			io.Copy(o.IOStreams.Out, stream)
		}
		//nolint:errcheck,gosec // Best-effort close
		stream.Close()

		if len(runningPods) > 1 && i < len(runningPods)-1 {
			//nolint:errcheck // Writing to stdout in CLI context
			fmt.Fprintln(o.IOStreams.Out)
		}
	}

	return nil
}

func (o *LogsOptions) filterErrorLogs(r io.Reader, w io.Writer) error {
	buf := make([]byte, 4096)
	var line strings.Builder

	for {
		n, err := r.Read(buf)
		if n > 0 {
			for i := 0; i < n; i++ {
				if buf[i] == '\n' {
					lineStr := line.String()
					// Check for error indicators
					if strings.Contains(strings.ToLower(lineStr), "error") ||
						strings.Contains(lineStr, "level=error") ||
						strings.Contains(lineStr, "\"level\":\"error\"") ||
						strings.Contains(lineStr, "severity=ERROR") ||
						strings.Contains(lineStr, "\"severity\":\"ERROR\"") {
						//nolint:errcheck // Writing to stdout in CLI context
						fmt.Fprintln(w, lineStr)
					}
					line.Reset()
				} else {
					line.WriteByte(buf[i])
				}
			}
		}
		if err != nil {
			if err == io.EOF {
				// Handle last line without newline
				if line.Len() > 0 {
					lineStr := line.String()
					if strings.Contains(strings.ToLower(lineStr), "error") {
						//nolint:errcheck // Writing to stdout in CLI context
						fmt.Fprintln(w, lineStr)
					}
				}
				return nil
			}
			return err
		}
	}
}

// parseDuration parses duration strings like "5m", "1h", "30s"
func parseDuration(s string) (time.Duration, error) {
	return time.ParseDuration(s)
}
