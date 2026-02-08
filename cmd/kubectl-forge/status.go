// Package main implements the kubectl-forge CLI status command
package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/kylegalloway/forge/pkg/constants"
	"github.com/kylegalloway/forge/pkg/kubectl"
	"github.com/spf13/cobra"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apiextclient "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/kubernetes"
)

const (
	// Default namespace for Forge system components
	forgeSystemNamespace = "forge-system"

	// Labels used to find Forge components
	controllerAppLabel = "forge-controller"
	webhookAppLabel    = "forge-webhook"
)

// StatusOptions holds options for the status command
type StatusOptions struct {
	configFlags *genericclioptions.ConfigFlags
	IOStreams   genericclioptions.IOStreams

	Namespace    string
	OutputFormat string
}

// SystemStatus contains overall system status
type SystemStatus struct {
	Controller  ComponentStatus `json:"controller" yaml:"controller"`
	Webhook     ComponentStatus `json:"webhook" yaml:"webhook"`
	CRDs        []CRDStatus     `json:"crds" yaml:"crds"`
	JobsSummary JobsSummary     `json:"jobsSummary" yaml:"jobsSummary"`
	Warnings    []string        `json:"warnings,omitempty" yaml:"warnings,omitempty"`
}

// ComponentStatus contains status for a system component
type ComponentStatus struct {
	Name      string      `json:"name" yaml:"name"`
	Available bool        `json:"available" yaml:"available"`
	Ready     string      `json:"ready" yaml:"ready"`
	Pods      []PodStatus `json:"pods" yaml:"pods"`
	Message   string      `json:"message,omitempty" yaml:"message,omitempty"`
	TLSExpiry string      `json:"tlsExpiry,omitempty" yaml:"tlsExpiry,omitempty"`
	TLSValid  bool        `json:"tlsValid,omitempty" yaml:"tlsValid,omitempty"`
}

// PodStatus contains pod status information
type PodStatus struct {
	Name     string `json:"name" yaml:"name"`
	Status   string `json:"status" yaml:"status"`
	Restarts int32  `json:"restarts" yaml:"restarts"`
	Age      string `json:"age" yaml:"age"`
	Node     string `json:"node" yaml:"node"`
}

// CRDStatus contains CRD status information
type CRDStatus struct {
	Name      string `json:"name" yaml:"name"`
	Installed bool   `json:"installed" yaml:"installed"`
	Version   string `json:"version,omitempty" yaml:"version,omitempty"`
}

// JobsSummary contains summary of jobs across the cluster
type JobsSummary struct {
	Pending   int `json:"pending" yaml:"pending"`
	Running   int `json:"running" yaml:"running"`
	Completed int `json:"completed" yaml:"completed"`
	Failed    int `json:"failed" yaml:"failed"`
	Retrying  int `json:"retrying" yaml:"retrying"`
}

// NewStatusCommand creates the status command
func NewStatusCommand(configFlags *genericclioptions.ConfigFlags) *cobra.Command {
	o := &StatusOptions{
		configFlags: configFlags,
		IOStreams:   genericclioptions.IOStreams{In: os.Stdin, Out: os.Stdout, ErrOut: os.Stderr},
	}

	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show Forge system status",
		Long: `Show the overall health status of the Forge system.

This command checks:
- Controller deployment health and pod status
- Webhook deployment health and TLS certificate validity
- CRD installation status
- Summary of jobs across the cluster
- Any warnings about stuck or problematic jobs`,
		Example: `  # Check system status
  kubectl forge status

  # Check status with JSON output
  kubectl forge status --output json

  # Check status in a specific namespace for system components
  kubectl forge status --namespace my-forge-system`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return o.Run(cmd.Context())
		},
	}

	cmd.Flags().StringVarP(&o.Namespace, "system-namespace", "s", forgeSystemNamespace, "Namespace where Forge system components are installed")
	cmd.Flags().StringVarP(&o.OutputFormat, "output", "o", "table", "Output format (table, json, yaml)")

	return cmd
}

// Run executes the status command
func (o *StatusOptions) Run(ctx context.Context) error {
	format, err := kubectl.ParseOutputFormat(o.OutputFormat)
	if err != nil {
		return err
	}

	restConfig, err := o.configFlags.ToRESTConfig()
	if err != nil {
		return fmt.Errorf("failed to create REST config: %w", err)
	}

	kubeClient, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return fmt.Errorf("failed to create Kubernetes client: %w", err)
	}

	apiextClient, err := apiextclient.NewForConfig(restConfig)
	if err != nil {
		return fmt.Errorf("failed to create API extensions client: %w", err)
	}

	status := SystemStatus{}

	// Check controller
	status.Controller = o.checkComponent(ctx, kubeClient, controllerAppLabel)

	// Check webhook
	status.Webhook = o.checkComponent(ctx, kubeClient, webhookAppLabel)

	// Check webhook TLS
	o.checkWebhookTLS(ctx, kubeClient, &status.Webhook)

	// Check CRDs
	status.CRDs = o.checkCRDs(ctx, apiextClient)

	// Get jobs summary
	forgeClient, err := kubectl.NewClientFromFlags(o.configFlags)
	if err == nil {
		status.JobsSummary, status.Warnings = o.getJobsSummary(ctx, forgeClient)
	}

	// Output
	printer := kubectl.NewPrinter(format, o.IOStreams.Out)

	switch format {
	case kubectl.OutputFormatJSON:
		return printer.PrintJSON(status)
	case kubectl.OutputFormatYAML:
		return printer.PrintYAML(status)
	default:
		return o.printStatus(status)
	}
}

func (o *StatusOptions) checkComponent(ctx context.Context, client kubernetes.Interface, appLabel string) ComponentStatus {
	status := ComponentStatus{
		Name:      appLabel,
		Available: false,
	}

	// Find deployment by label
	deployments, err := client.AppsV1().Deployments(o.Namespace).List(ctx, metav1.ListOptions{
		LabelSelector: fmt.Sprintf("app=%s", appLabel),
	})
	if err != nil {
		status.Message = fmt.Sprintf("Failed to list deployments: %v", err)
		return status
	}

	if len(deployments.Items) == 0 {
		status.Message = "Deployment not found"
		return status
	}

	deployment := deployments.Items[0]
	status.Name = deployment.Name
	status.Ready = fmt.Sprintf("%d/%d", deployment.Status.ReadyReplicas, deployment.Status.Replicas)
	status.Available = deployment.Status.ReadyReplicas > 0 && deployment.Status.ReadyReplicas == deployment.Status.Replicas

	// Check deployment conditions
	for _, cond := range deployment.Status.Conditions {
		if cond.Type == appsv1.DeploymentAvailable && cond.Status != corev1.ConditionTrue {
			status.Message = cond.Message
		}
	}

	// Get pods
	pods, err := client.CoreV1().Pods(o.Namespace).List(ctx, metav1.ListOptions{
		LabelSelector: fmt.Sprintf("app=%s", appLabel),
	})
	if err == nil {
		for _, pod := range pods.Items {
			var restarts int32
			for _, cs := range pod.Status.ContainerStatuses {
				restarts += cs.RestartCount
			}
			status.Pods = append(status.Pods, PodStatus{
				Name:     pod.Name,
				Status:   string(pod.Status.Phase),
				Restarts: restarts,
				Age:      formatDuration(time.Since(pod.CreationTimestamp.Time)),
				Node:     pod.Spec.NodeName,
			})
		}
	}

	return status
}

func (o *StatusOptions) checkWebhookTLS(ctx context.Context, client kubernetes.Interface, status *ComponentStatus) {
	// Try to find the webhook TLS secret
	secrets, err := client.CoreV1().Secrets(o.Namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return
	}

	for _, secret := range secrets.Items { //nolint:gosec // pragma: allowlist secret - iterating over k8s Secret objects
		if strings.Contains(secret.Name, "webhook") && strings.Contains(secret.Name, "tls") {
			certData, ok := secret.Data["tls.crt"]
			if !ok {
				continue
			}

			// Parse certificate
			block, _ := x509.ParseCertificate(certData) //nolint:errcheck // Best effort certificate parsing
			if block == nil {
				// Try PEM decode
				certs, err := tls.X509KeyPair(certData, secret.Data["tls.key"])
				if err != nil {
					continue
				}
				if len(certs.Certificate) > 0 {
					cert, err := x509.ParseCertificate(certs.Certificate[0])
					if err == nil {
						status.TLSValid = time.Now().Before(cert.NotAfter)
						daysUntilExpiry := int(time.Until(cert.NotAfter).Hours() / 24)
						status.TLSExpiry = fmt.Sprintf("%d days", daysUntilExpiry)
					}
				}
			}
			break
		}
	}
}

func (o *StatusOptions) checkCRDs(ctx context.Context, client apiextclient.Interface) []CRDStatus {
	expectedCRDs := []string{
		"zarfpackagejobs.forge.dev",
		"udsbundlejobs.forge.dev",
	}

	var crds []CRDStatus

	for _, name := range expectedCRDs {
		status := CRDStatus{
			Name:      name,
			Installed: false,
		}

		crd, err := client.ApiextensionsV1().CustomResourceDefinitions().Get(ctx, name, metav1.GetOptions{})
		if err == nil {
			status.Installed = true
			// Get served version
			for _, v := range crd.Spec.Versions {
				if v.Served {
					status.Version = v.Name
					break
				}
			}
			// Check if CRD is established
			for _, cond := range crd.Status.Conditions {
				if cond.Type == apiextv1.Established && cond.Status != apiextv1.ConditionTrue {
					status.Installed = false
				}
			}
		}

		crds = append(crds, status)
	}

	return crds
}

func (o *StatusOptions) getJobsSummary(ctx context.Context, client *kubectl.Client) (JobsSummary, []string) {
	summary := JobsSummary{}
	var warnings []string

	// List all resources across all namespaces via CRDs
	resources, err := client.ListForgeResources(ctx, "", "all")
	if err != nil {
		return summary, warnings
	}

	stuckThreshold := time.Hour
	retryThreshold := int32(3)

	for _, r := range resources {
		switch r.Phase {
		case constants.PhasePending:
			summary.Pending++
		case constants.PhaseQueued:
			summary.Pending++
			age := time.Since(r.CreatedAt)
			if age > 10*time.Minute {
				warnings = append(warnings, fmt.Sprintf("Resource queued > 10 minutes: %s/%s", r.Namespace, r.Name))
			}
		case constants.PhaseRunning:
			summary.Running++
			age := time.Since(r.CreatedAt)
			if age > stuckThreshold {
				warnings = append(warnings, fmt.Sprintf("Resource running > 1 hour: %s/%s", r.Namespace, r.Name))
			}
		case constants.PhaseCompleted:
			summary.Completed++
		case constants.PhaseFailed:
			summary.Failed++
		case constants.PhaseRetrying:
			summary.Retrying++
			// Check retry counts from CRD status
			for _, op := range r.Operations {
				if op.RetryCount > retryThreshold {
					warnings = append(warnings, fmt.Sprintf("High retry count (%d) for %s/%s operation %s",
						op.RetryCount, r.Namespace, r.Name, op.Name))
				}
			}
		}
	}

	return summary, warnings
}

func (o *StatusOptions) printStatus(s SystemStatus) error {
	w := o.IOStreams.Out

	//nolint:errcheck // Writing to stdout in CLI context
	fmt.Fprintf(w, "Forge System Status\n")
	//nolint:errcheck // Writing to stdout in CLI context
	fmt.Fprintf(w, "Namespace: %s\n\n", o.Namespace)

	// Controller
	//nolint:errcheck // Writing to stdout in CLI context
	fmt.Fprintf(w, "--- Controller ---\n")
	marker := "X"
	if s.Controller.Available {
		marker = "+"
	}
	//nolint:errcheck // Writing to stdout in CLI context
	fmt.Fprintf(w, "%s Deployment: %s (%s ready)\n", marker, s.Controller.Name, s.Controller.Ready)
	if s.Controller.Message != "" {
		//nolint:errcheck // Writing to stdout in CLI context
		fmt.Fprintf(w, "  Message: %s\n", s.Controller.Message)
	}
	for _, pod := range s.Controller.Pods {
		//nolint:errcheck // Writing to stdout in CLI context
		fmt.Fprintf(w, "  Pod: %s (%s, %d restarts, %s)\n", pod.Name, pod.Status, pod.Restarts, pod.Age)
	}
	//nolint:errcheck // Writing to stdout in CLI context
	fmt.Fprintln(w)

	// Webhook
	//nolint:errcheck // Writing to stdout in CLI context
	fmt.Fprintf(w, "--- Webhook ---\n")
	marker = "X"
	if s.Webhook.Available {
		marker = "+"
	}
	//nolint:errcheck // Writing to stdout in CLI context
	fmt.Fprintf(w, "%s Deployment: %s (%s ready)\n", marker, s.Webhook.Name, s.Webhook.Ready)
	if s.Webhook.TLSExpiry != "" {
		tlsMarker := "+"
		if !s.Webhook.TLSValid {
			tlsMarker = "X"
		}
		//nolint:errcheck // Writing to stdout in CLI context
		fmt.Fprintf(w, "%s TLS Certificate: expires in %s\n", tlsMarker, s.Webhook.TLSExpiry)
	}
	if s.Webhook.Message != "" {
		//nolint:errcheck // Writing to stdout in CLI context
		fmt.Fprintf(w, "  Message: %s\n", s.Webhook.Message)
	}

	// Check pod distribution
	if len(s.Webhook.Pods) > 1 {
		nodes := make(map[string]bool)
		for _, pod := range s.Webhook.Pods {
			nodes[pod.Node] = true
		}
		if len(nodes) > 1 {
			//nolint:errcheck // Writing to stdout in CLI context
			fmt.Fprintf(w, "+ Pods distributed across %d nodes\n", len(nodes))
		} else {
			//nolint:errcheck // Writing to stdout in CLI context
			fmt.Fprintf(w, "! Warning: All webhook pods on same node\n")
		}
	}

	for _, pod := range s.Webhook.Pods {
		//nolint:errcheck // Writing to stdout in CLI context
		fmt.Fprintf(w, "  Pod: %s (%s, %d restarts, %s)\n", pod.Name, pod.Status, pod.Restarts, pod.Age)
	}
	//nolint:errcheck // Writing to stdout in CLI context
	fmt.Fprintln(w)

	// CRDs
	//nolint:errcheck // Writing to stdout in CLI context
	fmt.Fprintf(w, "--- CRDs ---\n")
	for _, crd := range s.CRDs {
		marker = "X"
		if crd.Installed {
			marker = "+"
		}
		version := ""
		if crd.Version != "" {
			version = fmt.Sprintf(" (%s)", crd.Version)
		}
		//nolint:errcheck // Writing to stdout in CLI context
		fmt.Fprintf(w, "%s %s%s\n", marker, crd.Name, version)
	}
	//nolint:errcheck // Writing to stdout in CLI context
	fmt.Fprintln(w)

	// Jobs Summary
	//nolint:errcheck // Writing to stdout in CLI context
	fmt.Fprintf(w, "--- Jobs Summary ---\n")
	//nolint:errcheck // Writing to stdout in CLI context
	fmt.Fprintf(w, "  Pending:   %d\n", s.JobsSummary.Pending)
	//nolint:errcheck // Writing to stdout in CLI context
	fmt.Fprintf(w, "  Running:   %d\n", s.JobsSummary.Running)
	//nolint:errcheck // Writing to stdout in CLI context
	fmt.Fprintf(w, "  Completed: %d\n", s.JobsSummary.Completed)
	//nolint:errcheck // Writing to stdout in CLI context
	fmt.Fprintf(w, "  Failed:    %d\n", s.JobsSummary.Failed)
	//nolint:errcheck // Writing to stdout in CLI context
	fmt.Fprintf(w, "  Retrying:  %d\n", s.JobsSummary.Retrying)
	//nolint:errcheck // Writing to stdout in CLI context
	fmt.Fprintln(w)

	// Warnings
	if len(s.Warnings) > 0 {
		//nolint:errcheck // Writing to stdout in CLI context
		fmt.Fprintf(w, "--- Warnings ---\n")
		// Sort warnings for consistent output
		sort.Strings(s.Warnings)
		for _, warning := range s.Warnings {
			//nolint:errcheck // Writing to stdout in CLI context
			fmt.Fprintf(w, "! %s\n", warning)
		}
	}

	return nil
}
