package common

import (
	"testing"

	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestActionResult(t *testing.T) {
	now := metav1.Now()
	result := &ActionResult{
		JobName:   "test-job",
		Phase:     "Running",
		Message:   "Test message",
		StartTime: now,
		Completed: false,
	}

	if result.JobName != "test-job" {
		t.Errorf("Expected JobName to be 'test-job', got '%s'", result.JobName)
	}

	if result.Phase != "Running" {
		t.Errorf("Expected Phase to be 'Running', got '%s'", result.Phase)
	}

	if result.Completed {
		t.Error("Expected Completed to be false")
	}

	// Verify all fields are populated
	if result.Message == "" {
		t.Error("Expected Message to be set")
	}

	if result.StartTime != now {
		t.Error("Expected StartTime to be set")
	}
}

func TestPtr(t *testing.T) {
	// Test with int
	intVal := 42
	intPtr := Ptr(intVal)
	if intPtr == nil {
		t.Fatal("Expected Ptr to return non-nil pointer")
	}
	if *intPtr != intVal {
		t.Errorf("Expected *Ptr to be %d, got %d", intVal, *intPtr)
	}

	// Test with string
	strVal := "test"
	strPtr := Ptr(strVal)
	if strPtr == nil {
		t.Fatal("Expected Ptr to return non-nil pointer")
	}
	if *strPtr != strVal {
		t.Errorf("Expected *Ptr to be '%s', got '%s'", strVal, *strPtr)
	}

	// Test with bool
	boolVal := true
	boolPtr := Ptr(boolVal)
	if boolPtr == nil {
		t.Fatal("Expected Ptr to return non-nil pointer")
	}
	if *boolPtr != boolVal {
		t.Errorf("Expected *Ptr to be %v, got %v", boolVal, *boolPtr)
	}
}

func TestMustParseQuantity(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Parse CPU quantity",
			input:    "100m",
			expected: "100m",
		},
		{
			name:     "Parse memory quantity",
			input:    "128Mi",
			expected: "128Mi",
		},
		{
			name:     "Parse storage quantity",
			input:    "1Gi",
			expected: "1Gi",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := MustParseQuantity(tt.input)
			expected := resource.MustParse(tt.expected)
			if !result.Equal(expected) {
				t.Errorf("Expected quantity %s, got %s", expected.String(), result.String())
			}
		})
	}
}

func TestMustParseQuantityPanic(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("Expected MustParseQuantity to panic on invalid input")
		}
	}()

	// This should panic
	MustParseQuantity("invalid")
}
