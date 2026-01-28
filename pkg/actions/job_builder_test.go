package actions

import (
	"strings"
	"testing"
)

func TestJobBuilder_DebugMode(t *testing.T) {
	tests := []struct {
		name          string
		debugMode     bool
		wantDebugCmd  bool
		wantTTL       *int32
		wantOrigInMsg bool // expect original command to appear in debug message
	}{
		{
			name:          "debug mode disabled - normal command",
			debugMode:     false,
			wantDebugCmd:  false,
			wantTTL:       nil, // TTL not overridden
			wantOrigInMsg: false,
		},
		{
			name:          "debug mode enabled - debug script with completion marker",
			debugMode:     true,
			wantDebugCmd:  true,
			wantTTL:       Ptr(int32(3600)), // TTL set to 1 hour
			wantOrigInMsg: true,
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
			if tt.wantDebugCmd {
				// Debug mode should have a script that waits for completion marker
				if len(container.Args) != 1 {
					t.Fatalf("expected 1 arg, got %d", len(container.Args))
				}
				arg := container.Args[0]

				// Check for key elements of the debug script
				if !strings.Contains(arg, "DEBUG MODE ENABLED") {
					t.Error("debug script should contain 'DEBUG MODE ENABLED'")
				}
				if !strings.Contains(arg, "/tmp/debug-complete") {
					t.Error("debug script should reference /tmp/debug-complete marker")
				}
				if !strings.Contains(arg, "touch /tmp/debug-complete") {
					t.Error("debug script should show how to complete debugging")
				}
				if tt.wantOrigInMsg && !strings.Contains(arg, "echo hello") {
					t.Error("debug script should show original command")
				}
			} else if len(container.Args) != 1 || container.Args[0] != "echo hello" {
				t.Errorf("expected args = [echo hello], got %v", container.Args)
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
	// Debug mode should produce a script with completion marker logic
	if !strings.Contains(container.Args[0], "/tmp/debug-complete") {
		t.Errorf("debug mode should produce script with completion marker, got %s", container.Args[0])
	}
	// Original command should be shown in the debug output
	if !strings.Contains(container.Args[0], "actual command") {
		t.Errorf("debug script should show original command, got %s", container.Args[0])
	}
}

func TestShouldDebugAction(t *testing.T) {
	tests := []struct {
		name          string
		debugMode     bool
		debugActions  []string
		currentAction string
		want          bool
	}{
		{
			name:          "debugMode true, no debugActions - all actions debugged",
			debugMode:     true,
			debugActions:  nil,
			currentAction: "build",
			want:          true,
		},
		{
			name:          "debugMode true, no debugActions - publish also debugged",
			debugMode:     true,
			debugActions:  []string{},
			currentAction: "publish",
			want:          true,
		},
		{
			name:          "debugMode false, no debugActions - no debugging",
			debugMode:     false,
			debugActions:  nil,
			currentAction: "build",
			want:          false,
		},
		{
			name:          "debugActions specified - only listed action debugged",
			debugMode:     true,
			debugActions:  []string{"build"},
			currentAction: "build",
			want:          true,
		},
		{
			name:          "debugActions specified - unlisted action not debugged",
			debugMode:     true,
			debugActions:  []string{"build"},
			currentAction: "publish",
			want:          false,
		},
		{
			name:          "debugActions specified - multiple actions",
			debugMode:     true,
			debugActions:  []string{"build", "deploy"},
			currentAction: "deploy",
			want:          true,
		},
		{
			name:          "debugActions specified - debugMode false doesn't matter",
			debugMode:     false,
			debugActions:  []string{"publish"},
			currentAction: "publish",
			want:          true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ShouldDebugAction(tt.debugMode, tt.debugActions, tt.currentAction)
			if got != tt.want {
				t.Errorf("ShouldDebugAction(%v, %v, %q) = %v, want %v",
					tt.debugMode, tt.debugActions, tt.currentAction, got, tt.want)
			}
		})
	}
}
