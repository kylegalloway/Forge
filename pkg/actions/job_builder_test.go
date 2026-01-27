package actions

import (
	"testing"
)

func TestJobBuilder_DebugMode(t *testing.T) {
	tests := []struct {
		name         string
		debugMode    bool
		wantSleepCmd bool
		wantTTL      *int32
	}{
		{
			name:         "debug mode disabled - normal command",
			debugMode:    false,
			wantSleepCmd: false,
			wantTTL:      nil, // TTL not overridden
		},
		{
			name:         "debug mode enabled - sleep infinity",
			debugMode:    true,
			wantSleepCmd: true,
			wantTTL:      Ptr(int32(3600)), // TTL set to 1 hour
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			builder := NewJobBuilder("test-job", "default").
				WithContainerName("test-container").
				WithContainerImage("test-image:latest").
				WithCommand([]string{"/bin/sh", "-c"}).
				WithArgs([]string{"echo hello"}).
				WithDebugMode(tt.debugMode)

			job := builder.Build()

			// Verify container args
			if len(job.Spec.Template.Spec.Containers) == 0 {
				t.Fatal("expected at least one container")
			}

			container := job.Spec.Template.Spec.Containers[0]
			if tt.wantSleepCmd {
				if len(container.Args) != 1 || container.Args[0] != "sleep infinity" {
					t.Errorf("expected args = [sleep infinity], got %v", container.Args)
				}
			} else {
				if len(container.Args) != 1 || container.Args[0] != "echo hello" {
					t.Errorf("expected args = [echo hello], got %v", container.Args)
				}
			}

			// Verify TTL
			if tt.wantTTL != nil {
				if job.Spec.TTLSecondsAfterFinished == nil {
					t.Error("expected TTLSecondsAfterFinished to be set")
				} else if *job.Spec.TTLSecondsAfterFinished != *tt.wantTTL {
					t.Errorf("expected TTLSecondsAfterFinished = %d, got %d",
						*tt.wantTTL, *job.Spec.TTLSecondsAfterFinished)
				}
			}
		})
	}
}

func TestJobBuilder_DebugModePrecedence(t *testing.T) {
	// Test that per-job debug mode works correctly
	// The precedence logic (specDebugMode || globalDebugMode) is handled by the action handlers,
	// but the JobBuilder should correctly apply whatever value it receives

	builder := NewJobBuilder("debug-test", "default").
		WithContainerName("test").
		WithContainerImage("test:latest").
		WithCommand([]string{"/bin/sh", "-c"}).
		WithArgs([]string{"actual command"}).
		WithDebugMode(true) // Simulates per-job override

	job := builder.Build()

	container := job.Spec.Template.Spec.Containers[0]
	if container.Args[0] != "sleep infinity" {
		t.Errorf("debug mode override should result in sleep infinity, got %s", container.Args[0])
	}
}
