// Package audit provides audit trail functionality for job execution events
package audit

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"

	"github.com/kylegalloway/forge/pkg/logging"
)

// EventType represents the type of audit event
type EventType string

const (
	// EventJobCreated is emitted when a job resource is created
	EventJobCreated EventType = "JobCreated"
	// EventJobValidated is emitted when job validation succeeds
	EventJobValidated EventType = "JobValidated"
	// EventJobValidationFailed is emitted when job validation fails
	EventJobValidationFailed EventType = "JobValidationFailed"
	// EventJobStarted is emitted when job execution begins
	EventJobStarted EventType = "JobStarted"
	// EventJobCompleted is emitted when job completes successfully
	EventJobCompleted EventType = "JobCompleted"
	// EventJobFailed is emitted when job execution fails
	EventJobFailed EventType = "JobFailed"
	// EventJobDeleted is emitted when a job is deleted
	EventJobDeleted EventType = "JobDeleted"

	// EventActionStarted is emitted when an individual action begins
	EventActionStarted EventType = "ActionStarted"
	// EventActionCompleted is emitted when an action completes successfully
	EventActionCompleted EventType = "ActionCompleted"
	// EventActionFailed is emitted when an action fails
	EventActionFailed EventType = "ActionFailed"

	// EventPVCCreated is emitted when a PVC is created for artifacts
	EventPVCCreated EventType = "PVCCreated"
	// EventPVCDeleted is emitted when an artifact PVC is deleted
	EventPVCDeleted EventType = "PVCDeleted"

	// EventPolicyViolation is emitted when a policy is violated
	EventPolicyViolation EventType = "PolicyViolation"
	// EventPolicyEnforced is emitted when a policy is successfully enforced
	EventPolicyEnforced EventType = "PolicyEnforced"

	// EventConfigChanged is emitted when configuration changes
	EventConfigChanged EventType = "ConfigChanged"

	// Audit event reason prefix
	auditReasonPrefix = "Forge"
)

// AuditEvent represents a single audit trail entry
type AuditEvent struct {
	// Timestamp when the event occurred
	Timestamp time.Time `json:"timestamp"`

	// Event type
	Type EventType `json:"type"`

	// Resource being audited
	ResourceKind      string `json:"resourceKind"`
	ResourceName      string `json:"resourceName"`
	ResourceNamespace string `json:"resourceNamespace"`
	ResourceUID       string `json:"resourceUID,omitempty"`

	// User/actor information
	User      string `json:"user,omitempty"`
	UserAgent string `json:"userAgent,omitempty"`

	// Correlation ID for request tracing
	CorrelationID string `json:"correlationID,omitempty"`

	// Action being performed
	Action string `json:"action,omitempty"`

	// Result of the action
	Result string `json:"result"`

	// Additional details
	Message string            `json:"message"`
	Details map[string]string `json:"details,omitempty"`

	// Checksum for tamper detection
	Checksum string `json:"checksum,omitempty"`
}

// AuditTrail manages audit logging for job execution
type AuditTrail struct {
	kubeClient kubernetes.Interface
	logger     *logging.Logger
	enabled    bool
}

// Config holds audit trail configuration
type Config struct {
	// Enable audit trail
	Enabled bool

	// Store audit events as Kubernetes Events
	StoreAsEvents bool

	// Store audit events as ConfigMaps (for longer retention)
	StoreAsConfigMaps bool

	// Maximum events per ConfigMap
	MaxEventsPerConfigMap int

	// Enable checksum verification
	EnableChecksums bool
}

// DefaultConfig returns default audit configuration
func DefaultConfig() Config {
	return Config{
		Enabled:               true,
		StoreAsEvents:         true,
		StoreAsConfigMaps:     false,
		MaxEventsPerConfigMap: 100,
		EnableChecksums:       true,
	}
}

// NewAuditTrail creates a new audit trail manager
func NewAuditTrail(kubeClient kubernetes.Interface, config Config) *AuditTrail {
	return &AuditTrail{
		kubeClient: kubeClient,
		logger:     logging.NewLogger("audit-trail"),
		enabled:    config.Enabled,
	}
}

// RecordEvent records an audit event
func (at *AuditTrail) RecordEvent(ctx context.Context, event AuditEvent) error {
	if !at.enabled {
		at.logger.Debug(ctx, "Audit trail disabled, skipping event", "type", event.Type)
		return nil
	}

	// Set timestamp if not provided
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now()
	}

	// Add correlation ID from context if present
	if event.CorrelationID == "" {
		event.CorrelationID = logging.GetCorrelationID(ctx)
	}

	// Generate checksum
	event.Checksum = at.generateChecksum(event)

	// Log the audit event
	at.logEvent(ctx, event)

	// Store as Kubernetes Event
	if err := at.storeAsEvent(ctx, event); err != nil {
		at.logger.Error(ctx, err, "Failed to store audit event as Kubernetes Event",
			"type", event.Type,
			"resource", event.ResourceName)
		return err
	}

	return nil
}

// RecordJobCreated records a job creation event
func (at *AuditTrail) RecordJobCreated(ctx context.Context, resourceKind, namespace, name, uid, user string) error {
	return at.RecordEvent(ctx, AuditEvent{
		Type:              EventJobCreated,
		ResourceKind:      resourceKind,
		ResourceNamespace: namespace,
		ResourceName:      name,
		ResourceUID:       uid,
		User:              user,
		Result:            "Success",
		Message:           fmt.Sprintf("%s created", resourceKind),
	})
}

// RecordJobValidated records successful job validation
func (at *AuditTrail) RecordJobValidated(ctx context.Context, resourceKind, namespace, name, user string, details map[string]string) error {
	return at.RecordEvent(ctx, AuditEvent{
		Type:              EventJobValidated,
		ResourceKind:      resourceKind,
		ResourceNamespace: namespace,
		ResourceName:      name,
		User:              user,
		Result:            "Success",
		Message:           "Validation succeeded",
		Details:           details,
	})
}

// RecordJobValidationFailed records failed job validation
func (at *AuditTrail) RecordJobValidationFailed(ctx context.Context, resourceKind, namespace, name, user, reason string) error {
	return at.RecordEvent(ctx, AuditEvent{
		Type:              EventJobValidationFailed,
		ResourceKind:      resourceKind,
		ResourceNamespace: namespace,
		ResourceName:      name,
		User:              user,
		Result:            "Failure",
		Message:           fmt.Sprintf("Validation failed: %s", reason),
	})
}

// RecordJobStarted records job execution start
func (at *AuditTrail) RecordJobStarted(ctx context.Context, resourceKind, namespace, name, action string) error {
	return at.RecordEvent(ctx, AuditEvent{
		Type:              EventJobStarted,
		ResourceKind:      resourceKind,
		ResourceNamespace: namespace,
		ResourceName:      name,
		Action:            action,
		Result:            "Started",
		Message:           fmt.Sprintf("Job started with action: %s", action),
	})
}

// RecordActionStarted records individual action start
func (at *AuditTrail) RecordActionStarted(ctx context.Context, resourceKind, namespace, name, action string) error {
	return at.RecordEvent(ctx, AuditEvent{
		Type:              EventActionStarted,
		ResourceKind:      resourceKind,
		ResourceNamespace: namespace,
		ResourceName:      name,
		Action:            action,
		Result:            "Started",
		Message:           fmt.Sprintf("Action started: %s", action),
	})
}

// RecordActionCompleted records successful action completion
func (at *AuditTrail) RecordActionCompleted(ctx context.Context, resourceKind, namespace, name, action string, details map[string]string) error {
	return at.RecordEvent(ctx, AuditEvent{
		Type:              EventActionCompleted,
		ResourceKind:      resourceKind,
		ResourceNamespace: namespace,
		ResourceName:      name,
		Action:            action,
		Result:            "Success",
		Message:           fmt.Sprintf("Action completed: %s", action),
		Details:           details,
	})
}

// RecordActionFailed records failed action
func (at *AuditTrail) RecordActionFailed(ctx context.Context, resourceKind, namespace, name, action, reason string) error {
	return at.RecordEvent(ctx, AuditEvent{
		Type:              EventActionFailed,
		ResourceKind:      resourceKind,
		ResourceNamespace: namespace,
		ResourceName:      name,
		Action:            action,
		Result:            "Failure",
		Message:           fmt.Sprintf("Action failed: %s - %s", action, reason),
	})
}

// RecordJobCompleted records successful job completion
func (at *AuditTrail) RecordJobCompleted(ctx context.Context, resourceKind, namespace, name string, details map[string]string) error {
	return at.RecordEvent(ctx, AuditEvent{
		Type:              EventJobCompleted,
		ResourceKind:      resourceKind,
		ResourceNamespace: namespace,
		ResourceName:      name,
		Result:            "Success",
		Message:           "Job completed successfully",
		Details:           details,
	})
}

// RecordJobFailed records job failure
func (at *AuditTrail) RecordJobFailed(ctx context.Context, resourceKind, namespace, name, reason string) error {
	return at.RecordEvent(ctx, AuditEvent{
		Type:              EventJobFailed,
		ResourceKind:      resourceKind,
		ResourceNamespace: namespace,
		ResourceName:      name,
		Result:            "Failure",
		Message:           fmt.Sprintf("Job failed: %s", reason),
	})
}

// RecordPolicyViolation records a policy violation
func (at *AuditTrail) RecordPolicyViolation(ctx context.Context, resourceKind, namespace, name, policy, violation string) error {
	return at.RecordEvent(ctx, AuditEvent{
		Type:              EventPolicyViolation,
		ResourceKind:      resourceKind,
		ResourceNamespace: namespace,
		ResourceName:      name,
		Result:            "Blocked",
		Message:           fmt.Sprintf("Policy violation: %s", violation),
		Details: map[string]string{
			"policy":    policy,
			"violation": violation,
		},
	})
}

// logEvent logs the audit event using structured logging
func (at *AuditTrail) logEvent(ctx context.Context, event AuditEvent) {
	// Create context with audit metadata
	auditCtx := logging.WithNamespace(ctx, event.ResourceNamespace)
	if event.CorrelationID != "" {
		auditCtx = logging.WithCorrelationID(auditCtx, event.CorrelationID)
	}

	at.logger.Info(auditCtx, "Audit event recorded",
		"eventType", event.Type,
		"resource", fmt.Sprintf("%s/%s", event.ResourceKind, event.ResourceName),
		"action", event.Action,
		"result", event.Result,
		"user", event.User,
		"message", event.Message)
}

// storeAsEvent stores the audit event as a Kubernetes Event
func (at *AuditTrail) storeAsEvent(ctx context.Context, event AuditEvent) error {
	// Create event object
	k8sEvent := &corev1.Event{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s.%x", event.ResourceName, time.Now().UnixNano()),
			Namespace: event.ResourceNamespace,
			Labels: map[string]string{
				"app":                     "forge",
				"forge.dev/audit":         "true",
				"forge.dev/resource-kind": event.ResourceKind,
				"forge.dev/event-type":    string(event.Type),
			},
			Annotations: map[string]string{
				"forge.dev/correlation-id": event.CorrelationID,
				"forge.dev/checksum":       event.Checksum,
			},
		},
		InvolvedObject: corev1.ObjectReference{
			Kind:      event.ResourceKind,
			Name:      event.ResourceName,
			Namespace: event.ResourceNamespace,
			UID:       types.UID(event.ResourceUID),
		},
		Reason:  fmt.Sprintf("%s%s", auditReasonPrefix, event.Type),
		Message: at.formatEventMessage(event),
		Source: corev1.EventSource{
			Component: "forge-controller",
		},
		FirstTimestamp: metav1.NewTime(event.Timestamp),
		LastTimestamp:  metav1.NewTime(event.Timestamp),
		Count:          1,
		Type:           at.getEventType(event.Type),
	}

	// Create the event
	_, err := at.kubeClient.CoreV1().Events(event.ResourceNamespace).Create(ctx, k8sEvent, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create event: %w", err)
	}

	return nil
}

// formatEventMessage formats the audit event as a message
func (at *AuditTrail) formatEventMessage(event AuditEvent) string {
	msg := event.Message

	// Add user if present
	if event.User != "" {
		msg = fmt.Sprintf("%s (user: %s)", msg, event.User)
	}

	// Add action if present
	if event.Action != "" {
		msg = fmt.Sprintf("%s [action: %s]", msg, event.Action)
	}

	// Add details if present
	if len(event.Details) > 0 {
		detailsJSON, err := json.Marshal(event.Details)
		if err != nil {
			klog.V(4).ErrorS(err, "Failed to marshal event details")
		} else {
			msg = fmt.Sprintf("%s - Details: %s", msg, string(detailsJSON))
		}
	}

	return msg
}

// getEventType maps audit event type to Kubernetes event type
func (at *AuditTrail) getEventType(eventType EventType) string {
	switch eventType {
	case EventJobValidationFailed, EventJobFailed, EventActionFailed, EventPolicyViolation:
		return corev1.EventTypeWarning
	default:
		return corev1.EventTypeNormal
	}
}

// generateChecksum generates a SHA256 checksum of the audit event for tamper detection
func (at *AuditTrail) generateChecksum(event AuditEvent) string {
	// Create a copy without checksum for hashing
	eventCopy := event
	eventCopy.Checksum = ""

	// Serialize to JSON
	data, err := json.Marshal(eventCopy)
	if err != nil {
		klog.V(4).ErrorS(err, "Failed to marshal event for checksum")
		return ""
	}

	// Generate SHA256 hash
	hash := sha256.Sum256(data)
	return fmt.Sprintf("%x", hash)
}

// VerifyChecksum verifies the checksum of an audit event
func (at *AuditTrail) VerifyChecksum(event AuditEvent) bool {
	expected := event.Checksum
	actual := at.generateChecksum(event)
	return expected == actual
}

// ParseEvent parses a Kubernetes Event into an AuditEvent
func ParseEvent(k8sEvent *corev1.Event) (*AuditEvent, error) {
	if k8sEvent.Labels["forge.dev/audit"] != "true" {
		return nil, fmt.Errorf("not an audit event")
	}

	event := &AuditEvent{
		Timestamp:         k8sEvent.FirstTimestamp.Time,
		Type:              EventType(k8sEvent.Labels["forge.dev/event-type"]),
		ResourceKind:      k8sEvent.InvolvedObject.Kind,
		ResourceName:      k8sEvent.InvolvedObject.Name,
		ResourceNamespace: k8sEvent.InvolvedObject.Namespace,
		ResourceUID:       string(k8sEvent.InvolvedObject.UID),
		CorrelationID:     k8sEvent.Annotations["forge.dev/correlation-id"],
		Checksum:          k8sEvent.Annotations["forge.dev/checksum"],
		Message:           k8sEvent.Message,
	}

	return event, nil
}

// ListAuditEvents retrieves audit events for a resource
func ListAuditEvents(ctx context.Context, kubeClient kubernetes.Interface, namespace, resourceName string) ([]*AuditEvent, error) {
	// List events with audit label
	eventList, err := kubeClient.CoreV1().Events(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: "forge.dev/audit=true",
		FieldSelector: fmt.Sprintf("involvedObject.name=%s", resourceName),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list events: %w", err)
	}

	// Parse events
	events := make([]*AuditEvent, 0, len(eventList.Items))
	for i := range eventList.Items {
		event, err := ParseEvent(&eventList.Items[i])
		if err != nil {
			klog.V(4).ErrorS(err, "Failed to parse event", "event", eventList.Items[i].Name)
			continue
		}
		events = append(events, event)
	}

	return events, nil
}

// GetObjectReference creates an ObjectReference for audit events
func GetObjectReference(_ runtime.Object, kind, name, namespace, uid string) corev1.ObjectReference {
	return corev1.ObjectReference{
		Kind:      kind,
		Name:      name,
		Namespace: namespace,
		UID:       types.UID(uid),
	}
}
