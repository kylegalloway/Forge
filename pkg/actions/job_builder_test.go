package actions

import (
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"

	"github.com/kylegalloway/forge/pkg/apis/common"
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

func TestJobBuilder_WithExtraMounts(t *testing.T) {
	mounts := []common.ExtraMount{
		{
			ConfigMapRef: &common.LocalObjectReference{Name: "my-config"},
			MountPath:    "/etc/my-config",
		},
		{
			SecretRef: &common.LocalObjectReference{Name: "my-secret"},
			MountPath: "/etc/my-secret/credentials",
			SubPath:   "credentials",
			ReadOnly:  Ptr(false),
		},
	}

	builder := NewJobBuilder("extra-mount-test", "default").
		WithContainerName("test-container").
		WithContainerImage("test-image:latest").
		WithCommand([]string{"/bin/sh", "-c"}).
		WithArgs([]string{"echo hello"}).
		WithExtraMounts(mounts)

	job := builder.Build()

	if len(job.Spec.Template.Spec.Containers) == 0 {
		t.Fatal("expected at least one container")
	}

	// Collect volumes and volume mounts from the built job
	volumes := job.Spec.Template.Spec.Volumes
	container := job.Spec.Template.Spec.Containers[0]
	volumeMounts := container.VolumeMounts

	// --- Verify volumes ---

	// Find extra-mount-0 (ConfigMap)
	var foundCMVol bool
	for _, v := range volumes {
		if v.Name == "extra-mount-0" {
			foundCMVol = true
			if v.ConfigMap == nil {
				t.Error("extra-mount-0 should have a ConfigMapVolumeSource")
			} else if v.ConfigMap.Name != "my-config" {
				t.Errorf("extra-mount-0 ConfigMap name = %q, want %q", v.ConfigMap.Name, "my-config")
			}
			if v.Secret != nil { // pragma: allowlist secret
				t.Error("extra-mount-0 should not have a SecretVolumeSource")
			}
			break
		}
	}
	if !foundCMVol {
		t.Error("volume extra-mount-0 not found")
	}

	// Find extra-mount-1 (Secret)
	var foundSecretVol bool
	for _, v := range volumes {
		if v.Name == "extra-mount-1" {
			foundSecretVol = true // pragma: allowlist secret
			if v.Secret == nil {  // pragma: allowlist secret
				t.Error("extra-mount-1 should have a SecretVolumeSource")
			} else if v.Secret.SecretName != "my-secret" { // pragma: allowlist secret
				t.Errorf("extra-mount-1 Secret name = %q, want %q", v.Secret.SecretName, "my-secret")
			}
			if v.ConfigMap != nil {
				t.Error("extra-mount-1 should not have a ConfigMapVolumeSource")
			}
			break
		}
	}
	if !foundSecretVol {
		t.Error("volume extra-mount-1 not found")
	}

	// --- Verify volume mounts ---

	// Find mount for extra-mount-0 (ConfigMap mount)
	var foundCMMount bool
	for _, vm := range volumeMounts {
		if vm.Name == "extra-mount-0" {
			foundCMMount = true
			if vm.MountPath != "/etc/my-config" {
				t.Errorf("extra-mount-0 MountPath = %q, want %q", vm.MountPath, "/etc/my-config")
			}
			if !vm.ReadOnly {
				t.Error("extra-mount-0 should default to ReadOnly=true")
			}
			if vm.SubPath != "" {
				t.Errorf("extra-mount-0 SubPath = %q, want empty", vm.SubPath)
			}
			break
		}
	}
	if !foundCMMount {
		t.Error("volume mount extra-mount-0 not found")
	}

	// Find mount for extra-mount-1 (Secret mount with SubPath and ReadOnly=false)
	var foundSecretMount bool
	for _, vm := range volumeMounts {
		if vm.Name == "extra-mount-1" {
			foundSecretMount = true // pragma: allowlist secret
			if vm.MountPath != "/etc/my-secret/credentials" {
				t.Errorf("extra-mount-1 MountPath = %q, want %q", vm.MountPath, "/etc/my-secret/credentials")
			}
			if vm.ReadOnly {
				t.Error("extra-mount-1 should have ReadOnly=false when explicitly set")
			}
			if vm.SubPath != "credentials" {
				t.Errorf("extra-mount-1 SubPath = %q, want %q", vm.SubPath, "credentials")
			}
			break
		}
	}
	if !foundSecretMount {
		t.Error("volume mount extra-mount-1 not found")
	}
}

func findVolumeSizeLimit(t *testing.T, volumes []corev1.Volume, name string) resource.Quantity {
	t.Helper()
	for _, v := range volumes {
		if v.Name == name && v.EmptyDir != nil && v.EmptyDir.SizeLimit != nil {
			return *v.EmptyDir.SizeLimit
		}
	}
	t.Fatalf("volume %q not found or has no SizeLimit", name)
	return resource.Quantity{}
}

func TestJobBuilder_WithWorkspaceVolume_NilVolumeSizes(t *testing.T) {
	builder := NewJobBuilder("test-job", "default").
		WithContainerName("test").
		WithContainerImage("test:latest").
		WithCommand([]string{"/bin/sh", "-c"}).
		WithArgs([]string{"echo hello"}).
		WithWorkspaceVolume(nil)

	job := builder.Build()
	volumes := job.Spec.Template.Spec.Volumes

	workspace := findVolumeSizeLimit(t, volumes, "workspace")
	output := findVolumeSizeLimit(t, volumes, "output")

	if expected := resource.MustParse("10Gi"); !workspace.Equal(expected) {
		t.Errorf("workspace size = %s, want %s", workspace.String(), expected.String())
	}
	if expected := resource.MustParse("10Gi"); !output.Equal(expected) {
		t.Errorf("output size = %s, want %s", output.String(), expected.String())
	}
}

func TestJobBuilder_WithWorkspaceVolume_CustomSizes(t *testing.T) {
	vs := &common.VolumeSizes{
		Workspace: Ptr(resource.MustParse("50Gi")),
		Output:    Ptr(resource.MustParse("25Gi")),
	}

	builder := NewJobBuilder("test-job", "default").
		WithContainerName("test").
		WithContainerImage("test:latest").
		WithCommand([]string{"/bin/sh", "-c"}).
		WithArgs([]string{"echo hello"}).
		WithWorkspaceVolume(vs)

	job := builder.Build()
	volumes := job.Spec.Template.Spec.Volumes

	workspace := findVolumeSizeLimit(t, volumes, "workspace")
	output := findVolumeSizeLimit(t, volumes, "output")

	if expected := resource.MustParse("50Gi"); !workspace.Equal(expected) {
		t.Errorf("workspace size = %s, want %s", workspace.String(), expected.String())
	}
	if expected := resource.MustParse("25Gi"); !output.Equal(expected) {
		t.Errorf("output size = %s, want %s", output.String(), expected.String())
	}
}

func TestJobBuilder_WithWorkspaceVolume_PartialOverride(t *testing.T) {
	vs := &common.VolumeSizes{
		Workspace: Ptr(resource.MustParse("50Gi")),
		// Output not set — should default to 10Gi
	}

	builder := NewJobBuilder("test-job", "default").
		WithContainerName("test").
		WithContainerImage("test:latest").
		WithCommand([]string{"/bin/sh", "-c"}).
		WithArgs([]string{"echo hello"}).
		WithWorkspaceVolume(vs)

	job := builder.Build()
	volumes := job.Spec.Template.Spec.Volumes

	workspace := findVolumeSizeLimit(t, volumes, "workspace")
	output := findVolumeSizeLimit(t, volumes, "output")

	if expected := resource.MustParse("50Gi"); !workspace.Equal(expected) {
		t.Errorf("workspace size = %s, want %s", workspace.String(), expected.String())
	}
	if expected := resource.MustParse("10Gi"); !output.Equal(expected) {
		t.Errorf("output size = %s, want %s", output.String(), expected.String())
	}
}

func TestJobBuilder_Build_CustomTmpAndHomeSizes(t *testing.T) {
	vs := &common.VolumeSizes{
		Tmp:  Ptr(resource.MustParse("5Gi")),
		Home: Ptr(resource.MustParse("2Gi")),
	}

	builder := NewJobBuilder("test-job", "default").
		WithContainerName("test").
		WithContainerImage("test:latest").
		WithCommand([]string{"/bin/sh", "-c"}).
		WithArgs([]string{"echo hello"}).
		WithWorkspaceVolume(vs).
		WithUserConfig(1000) // sets homeDir

	job := builder.Build()
	volumes := job.Spec.Template.Spec.Volumes

	tmp := findVolumeSizeLimit(t, volumes, "tmp")
	home := findVolumeSizeLimit(t, volumes, "home")

	if expected := resource.MustParse("5Gi"); !tmp.Equal(expected) {
		t.Errorf("tmp size = %s, want %s", tmp.String(), expected.String())
	}
	if expected := resource.MustParse("2Gi"); !home.Equal(expected) {
		t.Errorf("home size = %s, want %s", home.String(), expected.String())
	}
}

func TestJobBuilder_Build_PartialTmpOnly(t *testing.T) {
	vs := &common.VolumeSizes{
		Tmp: Ptr(resource.MustParse("3Gi")),
		// Home not set — should default to 1Gi
	}

	builder := NewJobBuilder("test-job", "default").
		WithContainerName("test").
		WithContainerImage("test:latest").
		WithCommand([]string{"/bin/sh", "-c"}).
		WithArgs([]string{"echo hello"}).
		WithWorkspaceVolume(vs).
		WithUserConfig(1000)

	job := builder.Build()
	volumes := job.Spec.Template.Spec.Volumes

	tmp := findVolumeSizeLimit(t, volumes, "tmp")
	home := findVolumeSizeLimit(t, volumes, "home")

	if expected := resource.MustParse("3Gi"); !tmp.Equal(expected) {
		t.Errorf("tmp size = %s, want %s", tmp.String(), expected.String())
	}
	if expected := resource.MustParse("1Gi"); !home.Equal(expected) {
		t.Errorf("home size = %s, want %s", home.String(), expected.String())
	}
}
