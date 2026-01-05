package audit

import (
	"context"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/kylegalloway/forge/pkg/logging"
)

func TestNewAuditTrail(t *testing.T) {
	client := fake.NewSimpleClientset() //nolint:staticcheck // Using standard fake client for tests //nolint:staticcheck // Using standard fake client for tests
	config := DefaultConfig()

	trail := NewAuditTrail(client, config)

	if trail == nil {
		t.Fatal("NewAuditTrail returned nil")
	}

	if trail.kubeClient != client {
		t.Error("kubeClient not set correctly")
	}

	if !trail.enabled {
		t.Error("Expected audit trail to be enabled")
	}
}

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	if !config.Enabled {
		t.Error("Expected audit to be enabled by default")
	}

	if !config.StoreAsEvents {
		t.Error("Expected StoreAsEvents to be true by default")
	}

	if !config.EnableChecksums {
		t.Error("Expected EnableChecksums to be true by default")
	}

	if config.MaxEventsPerConfigMap != 100 {
		t.Errorf("Expected MaxEventsPerConfigMap to be 100, got %d", config.MaxEventsPerConfigMap)
	}
}

func TestRecordEvent(t *testing.T) {
	client := fake.NewSimpleClientset() //nolint:staticcheck // Using standard fake client for tests
	config := DefaultConfig()
	trail := NewAuditTrail(client, config)

	ctx := context.Background()
	ctx = logging.WithCorrelationID(ctx, "test-correlation-123")

	event := AuditEvent{
		Type:              EventJobCreated,
		ResourceKind:      "ZarfPackageJob",
		ResourceName:      "test-job",
		ResourceNamespace: "default",
		ResourceUID:       "test-uid-123",
		User:              "test-user",
		Result:            "Success",
		Message:           "Job created",
	}

	err := trail.RecordEvent(ctx, event)
	if err != nil {
		t.Fatalf("RecordEvent failed: %v", err)
	}

	// Verify event was created
	events, err := client.CoreV1().Events("default").List(ctx, metav1.ListOptions{
		LabelSelector: "forge.dev/audit=true",
	})
	if err != nil {
		t.Fatalf("Failed to list events: %v", err)
	}

	if len(events.Items) != 1 {
		t.Fatalf("Expected 1 event, got %d", len(events.Items))
	}

	k8sEvent := events.Items[0]

	// Verify event fields
	if k8sEvent.InvolvedObject.Kind != "ZarfPackageJob" {
		t.Errorf("Expected kind 'ZarfPackageJob', got '%s'", k8sEvent.InvolvedObject.Kind)
	}

	if k8sEvent.InvolvedObject.Name != "test-job" {
		t.Errorf("Expected name 'test-job', got '%s'", k8sEvent.InvolvedObject.Name)
	}

	if k8sEvent.Labels["forge.dev/audit"] != "true" {
		t.Error("Expected audit label to be 'true'")
	}

	if k8sEvent.Labels["forge.dev/event-type"] != string(EventJobCreated) {
		t.Errorf("Expected event-type '%s', got '%s'", EventJobCreated, k8sEvent.Labels["forge.dev/event-type"])
	}

	if k8sEvent.Annotations["forge.dev/correlation-id"] != "test-correlation-123" {
		t.Errorf("Expected correlation-id 'test-correlation-123', got '%s'", k8sEvent.Annotations["forge.dev/correlation-id"])
	}

	if k8sEvent.Annotations["forge.dev/checksum"] == "" {
		t.Error("Expected checksum to be set")
	}
}

func TestRecordEventDisabled(t *testing.T) {
	client := fake.NewSimpleClientset() //nolint:staticcheck // Using standard fake client for tests
	config := DefaultConfig()
	config.Enabled = false
	trail := NewAuditTrail(client, config)

	ctx := context.Background()
	event := AuditEvent{
		Type:              EventJobCreated,
		ResourceKind:      "ZarfPackageJob",
		ResourceName:      "test-job",
		ResourceNamespace: "default",
		Result:            "Success",
		Message:           "Job created",
	}

	err := trail.RecordEvent(ctx, event)
	if err != nil {
		t.Fatalf("RecordEvent failed: %v", err)
	}

	// Verify no events were created
	events, err := client.CoreV1().Events("default").List(ctx, metav1.ListOptions{})
	if err != nil {
		t.Fatalf("Failed to list events: %v", err)
	}

	if len(events.Items) != 0 {
		t.Fatalf("Expected 0 events when disabled, got %d", len(events.Items))
	}
}

func TestRecordJobCreated(t *testing.T) {
	client := fake.NewSimpleClientset() //nolint:staticcheck // Using standard fake client for tests
	config := DefaultConfig()
	trail := NewAuditTrail(client, config)

	ctx := context.Background()

	err := trail.RecordJobCreated(ctx, "ZarfPackageJob", "default", "test-job", "uid-123", "admin")
	if err != nil {
		t.Fatalf("RecordJobCreated failed: %v", err)
	}

	// Verify event
	events, _ := client.CoreV1().Events("default").List(ctx, metav1.ListOptions{})
	if len(events.Items) != 1 {
		t.Fatalf("Expected 1 event, got %d", len(events.Items))
	}

	event := events.Items[0]
	if event.Reason != "ForgeJobCreated" {
		t.Errorf("Expected reason 'ForgeJobCreated', got '%s'", event.Reason)
	}
}

func TestRecordJobValidated(t *testing.T) {
	client := fake.NewSimpleClientset() //nolint:staticcheck // Using standard fake client for tests
	config := DefaultConfig()
	trail := NewAuditTrail(client, config)

	ctx := context.Background()
	details := map[string]string{
		"serviceAccount": "forge-job",
		"policy":         "default",
	}

	err := trail.RecordJobValidated(ctx, "ZarfPackageJob", "default", "test-job", "admin", details)
	if err != nil {
		t.Fatalf("RecordJobValidated failed: %v", err)
	}

	events, _ := client.CoreV1().Events("default").List(ctx, metav1.ListOptions{})
	if len(events.Items) != 1 {
		t.Fatalf("Expected 1 event, got %d", len(events.Items))
	}
}

func TestRecordJobValidationFailed(t *testing.T) {
	client := fake.NewSimpleClientset() //nolint:staticcheck // Using standard fake client for tests
	config := DefaultConfig()
	trail := NewAuditTrail(client, config)

	ctx := context.Background()

	err := trail.RecordJobValidationFailed(ctx, "ZarfPackageJob", "default", "test-job", "admin", "Invalid ServiceAccount")
	if err != nil {
		t.Fatalf("RecordJobValidationFailed failed: %v", err)
	}

	events, _ := client.CoreV1().Events("default").List(ctx, metav1.ListOptions{})
	if len(events.Items) != 1 {
		t.Fatalf("Expected 1 event, got %d", len(events.Items))
	}

	event := events.Items[0]
	if event.Type != corev1.EventTypeWarning {
		t.Errorf("Expected warning event type, got '%s'", event.Type)
	}
}

func TestRecordActionEvents(t *testing.T) {
	client := fake.NewSimpleClientset() //nolint:staticcheck // Using standard fake client for tests
	config := DefaultConfig()
	trail := NewAuditTrail(client, config)

	ctx := context.Background()

	tests := []struct {
		name       string
		recordFunc func() error
		eventType  EventType
	}{
		{
			name: "Action Started",
			recordFunc: func() error {
				return trail.RecordActionStarted(ctx, "ZarfPackageJob", "default", "test-job", "Build")
			},
			eventType: EventActionStarted,
		},
		{
			name: "Action Completed",
			recordFunc: func() error {
				return trail.RecordActionCompleted(ctx, "ZarfPackageJob", "default", "test-job", "Build", map[string]string{"duration": "5m"})
			},
			eventType: EventActionCompleted,
		},
		{
			name: "Action Failed",
			recordFunc: func() error {
				return trail.RecordActionFailed(ctx, "ZarfPackageJob", "default", "test-job", "Build", "timeout")
			},
			eventType: EventActionFailed,
		},
	}

	for i, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.recordFunc()
			if err != nil {
				t.Fatalf("Record function failed: %v", err)
			}

			events, _ := client.CoreV1().Events("default").List(ctx, metav1.ListOptions{})
			if len(events.Items) != i+1 {
				t.Fatalf("Expected %d events, got %d", i+1, len(events.Items))
			}

			event := events.Items[i]
			if event.Labels["forge.dev/event-type"] != string(tt.eventType) {
				t.Errorf("Expected event type '%s', got '%s'", tt.eventType, event.Labels["forge.dev/event-type"])
			}
		})
	}
}

func TestRecordJobCompleted(t *testing.T) {
	client := fake.NewSimpleClientset() //nolint:staticcheck // Using standard fake client for tests
	config := DefaultConfig()
	trail := NewAuditTrail(client, config)

	ctx := context.Background()
	details := map[string]string{
		"duration":  "10m",
		"artifacts": "3",
	}

	err := trail.RecordJobCompleted(ctx, "ZarfPackageJob", "default", "test-job", details)
	if err != nil {
		t.Fatalf("RecordJobCompleted failed: %v", err)
	}

	events, _ := client.CoreV1().Events("default").List(ctx, metav1.ListOptions{})
	if len(events.Items) != 1 {
		t.Fatalf("Expected 1 event, got %d", len(events.Items))
	}

	event := events.Items[0]
	if event.Type != corev1.EventTypeNormal {
		t.Errorf("Expected normal event type, got '%s'", event.Type)
	}
}

func TestRecordPolicyViolation(t *testing.T) {
	client := fake.NewSimpleClientset() //nolint:staticcheck // Using standard fake client for tests
	config := DefaultConfig()
	trail := NewAuditTrail(client, config)

	ctx := context.Background()

	err := trail.RecordPolicyViolation(ctx, "ZarfPackageJob", "default", "test-job", "ResourceQuotaPolicy", "CPU limit exceeded")
	if err != nil {
		t.Fatalf("RecordPolicyViolation failed: %v", err)
	}

	events, _ := client.CoreV1().Events("default").List(ctx, metav1.ListOptions{})
	if len(events.Items) != 1 {
		t.Fatalf("Expected 1 event, got %d", len(events.Items))
	}

	event := events.Items[0]
	if event.Type != corev1.EventTypeWarning {
		t.Errorf("Expected warning event type, got '%s'", event.Type)
	}
}

func TestGenerateChecksum(t *testing.T) {
	client := fake.NewSimpleClientset() //nolint:staticcheck // Using standard fake client for tests
	config := DefaultConfig()
	trail := NewAuditTrail(client, config)

	event1 := AuditEvent{
		Type:              EventJobCreated,
		ResourceKind:      "ZarfPackageJob",
		ResourceName:      "test-job",
		ResourceNamespace: "default",
		Result:            "Success",
		Message:           "Job created",
	}

	event2 := event1 // Same event

	checksum1 := trail.generateChecksum(event1)
	checksum2 := trail.generateChecksum(event2)

	if checksum1 == "" {
		t.Error("Expected non-empty checksum")
	}

	if checksum1 != checksum2 {
		t.Error("Expected identical checksums for identical events")
	}

	// Modify event
	event2.Message = "Different message"
	checksum3 := trail.generateChecksum(event2)

	if checksum1 == checksum3 {
		t.Error("Expected different checksums for different events")
	}
}

func TestVerifyChecksum(t *testing.T) {
	client := fake.NewSimpleClientset() //nolint:staticcheck // Using standard fake client for tests
	config := DefaultConfig()
	trail := NewAuditTrail(client, config)

	event := AuditEvent{
		Type:              EventJobCreated,
		ResourceKind:      "ZarfPackageJob",
		ResourceName:      "test-job",
		ResourceNamespace: "default",
		Result:            "Success",
		Message:           "Job created",
	}

	// Generate checksum
	event.Checksum = trail.generateChecksum(event)

	// Verify valid checksum
	if !trail.VerifyChecksum(event) {
		t.Error("Expected checksum verification to succeed")
	}

	// Tamper with event
	tamperedEvent := event
	tamperedEvent.Message = "Tampered message"
	tamperedEvent.Checksum = event.Checksum // Keep original checksum

	// Verify tampered checksum fails
	if trail.VerifyChecksum(tamperedEvent) {
		t.Error("Expected checksum verification to fail for tampered event")
	}
}

func TestParseEvent(t *testing.T) {
	k8sEvent := &corev1.Event{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-event",
			Namespace: "default",
			Labels: map[string]string{
				"forge.dev/audit":      "true",
				"forge.dev/event-type": string(EventJobCreated),
			},
			Annotations: map[string]string{
				"forge.dev/correlation-id": "test-corr-123",
				"forge.dev/checksum":       "abc123",
			},
		},
		InvolvedObject: corev1.ObjectReference{
			Kind:      "ZarfPackageJob",
			Name:      "test-job",
			Namespace: "default",
			UID:       "uid-123",
		},
		Message:        "Job created",
		FirstTimestamp: metav1.NewTime(time.Now()),
	}

	event, err := ParseEvent(k8sEvent)
	if err != nil {
		t.Fatalf("ParseEvent failed: %v", err)
	}

	if event.Type != EventJobCreated {
		t.Errorf("Expected type '%s', got '%s'", EventJobCreated, event.Type)
	}

	if event.ResourceKind != "ZarfPackageJob" {
		t.Errorf("Expected resource kind 'ZarfPackageJob', got '%s'", event.ResourceKind)
	}

	if event.ResourceName != "test-job" {
		t.Errorf("Expected resource name 'test-job', got '%s'", event.ResourceName)
	}

	if event.CorrelationID != "test-corr-123" {
		t.Errorf("Expected correlation ID 'test-corr-123', got '%s'", event.CorrelationID)
	}

	if event.Checksum != "abc123" {
		t.Errorf("Expected checksum 'abc123', got '%s'", event.Checksum)
	}
}

func TestParseEventNonAudit(t *testing.T) {
	k8sEvent := &corev1.Event{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-event",
			Namespace: "default",
			// No audit label
		},
	}

	_, err := ParseEvent(k8sEvent)
	if err == nil {
		t.Error("Expected error for non-audit event")
	}
}

func TestListAuditEvents(t *testing.T) {
	client := fake.NewSimpleClientset() //nolint:staticcheck // Using standard fake client for tests
	ctx := context.Background()

	// Create audit events
	events := []*corev1.Event{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "event-1",
				Namespace: "default",
				Labels: map[string]string{
					"forge.dev/audit":      "true",
					"forge.dev/event-type": string(EventJobCreated),
				},
				Annotations: map[string]string{
					"forge.dev/correlation-id": "corr-1",
				},
			},
			InvolvedObject: corev1.ObjectReference{
				Kind: "ZarfPackageJob",
				Name: "test-job",
			},
			FirstTimestamp: metav1.NewTime(time.Now()),
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "event-2",
				Namespace: "default",
				Labels: map[string]string{
					"forge.dev/audit":      "true",
					"forge.dev/event-type": string(EventJobStarted),
				},
				Annotations: map[string]string{
					"forge.dev/correlation-id": "corr-1",
				},
			},
			InvolvedObject: corev1.ObjectReference{
				Kind: "ZarfPackageJob",
				Name: "test-job",
			},
			FirstTimestamp: metav1.NewTime(time.Now()),
		},
	}

	for _, event := range events {
		_, err := client.CoreV1().Events("default").Create(ctx, event, metav1.CreateOptions{})
		if err != nil {
			t.Fatalf("Failed to create test event: %v", err)
		}
	}

	// List audit events for the job
	auditEvents, err := ListAuditEvents(ctx, client, "default", "test-job")
	if err != nil {
		t.Fatalf("ListAuditEvents failed: %v", err)
	}

	if len(auditEvents) != 2 {
		t.Fatalf("Expected 2 audit events, got %d", len(auditEvents))
	}
}

func TestGetEventType(t *testing.T) {
	client := fake.NewSimpleClientset() //nolint:staticcheck // Using standard fake client for tests
	config := DefaultConfig()
	trail := NewAuditTrail(client, config)

	tests := []struct {
		eventType EventType
		expected  string
	}{
		{EventJobCreated, corev1.EventTypeNormal},
		{EventJobValidated, corev1.EventTypeNormal},
		{EventJobValidationFailed, corev1.EventTypeWarning},
		{EventJobStarted, corev1.EventTypeNormal},
		{EventJobCompleted, corev1.EventTypeNormal},
		{EventJobFailed, corev1.EventTypeWarning},
		{EventActionStarted, corev1.EventTypeNormal},
		{EventActionCompleted, corev1.EventTypeNormal},
		{EventActionFailed, corev1.EventTypeWarning},
		{EventPolicyViolation, corev1.EventTypeWarning},
	}

	for _, tt := range tests {
		t.Run(string(tt.eventType), func(t *testing.T) {
			result := trail.getEventType(tt.eventType)
			if result != tt.expected {
				t.Errorf("Expected event type '%s', got '%s'", tt.expected, result)
			}
		})
	}
}
