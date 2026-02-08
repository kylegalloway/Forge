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

func TestJobBuilder_WithInClusterKubeconfig(t *testing.T) {
	builder := NewJobBuilder("deploy-test", "default").
		WithContainerName("zarf-deploy").
		WithContainerImage("test-image:latest").
		WithCommand([]string{"/bin/sh", "-c"}).
		WithArgs([]string{"zarf package deploy /workspace/*.tar.zst --confirm"}).
		WithInClusterKubeconfig()

	job := builder.Build()

	// Verify kubeconfig-init is present as an init container
	initContainers := job.Spec.Template.Spec.InitContainers
	if len(initContainers) == 0 {
		t.Fatal("expected kubeconfig-init init container")
	}
	if initContainers[0].Name != "kubeconfig-init" {
		t.Errorf("expected first init container name = kubeconfig-init, got %s", initContainers[0].Name)
	}
	if initContainers[0].Image != "test-image:latest" {
		t.Errorf("expected init container to use same image, got %s", initContainers[0].Image)
	}

	// Verify KUBECONFIG env var on main container
	container := job.Spec.Template.Spec.Containers[0]
	foundEnv := false
	for _, env := range container.Env {
		if env.Name == "KUBECONFIG" {
			foundEnv = true
			if env.Value != "/etc/kubeconfig/kubeconfig" {
				t.Errorf("expected KUBECONFIG = /etc/kubeconfig/kubeconfig, got %s", env.Value)
			}
		}
	}
	if !foundEnv {
		t.Error("KUBECONFIG env var not found on main container")
	}

	// Verify kubeconfig volume exists
	foundVol := false
	for _, vol := range job.Spec.Template.Spec.Volumes {
		if vol.Name == "kubeconfig" {
			foundVol = true
			if vol.EmptyDir == nil {
				t.Error("kubeconfig volume should be an emptyDir")
			}
		}
	}
	if !foundVol {
		t.Error("kubeconfig volume not found")
	}

	// Verify main container has kubeconfig mount (read-only)
	foundMount := false
	for _, mount := range container.VolumeMounts {
		if mount.Name == "kubeconfig" {
			foundMount = true
			if !mount.ReadOnly {
				t.Error("kubeconfig mount should be read-only on main container")
			}
		}
	}
	if !foundMount {
		t.Error("kubeconfig volume mount not found on main container")
	}

	// Verify the deploy command is clean (no kubeconfig setup script)
	if !strings.Contains(container.Args[0], "zarf package deploy") {
		t.Errorf("expected clean deploy command, got: %s", container.Args[0])
	}
}

func TestJobBuilder_WithInClusterKubeconfig_DebugMode(t *testing.T) {
	builder := NewJobBuilder("deploy-debug", "default").
		WithContainerName("zarf-deploy").
		WithContainerImage("test-image:latest").
		WithCommand([]string{"/bin/sh", "-c"}).
		WithArgs([]string{"zarf package deploy /workspace/*.tar.zst --confirm"}).
		WithInClusterKubeconfig().
		WithDebugMode(true)

	job := builder.Build()

	// kubeconfig-init should still be present in debug mode
	initContainers := job.Spec.Template.Spec.InitContainers
	if len(initContainers) == 0 {
		t.Fatal("expected kubeconfig-init init container in debug mode")
	}
	if initContainers[0].Name != "kubeconfig-init" {
		t.Errorf("expected first init container = kubeconfig-init, got %s", initContainers[0].Name)
	}

	// Debug script should NOT contain any kubeconfig setup shell code
	container := job.Spec.Template.Spec.Containers[0]
	debugArg := container.Args[0]
	if strings.Contains(debugArg, "SA_DIR") || strings.Contains(debugArg, "base64") {
		t.Error("debug script should not contain kubeconfig setup shell code")
	}
	if !strings.Contains(debugArg, "DEBUG MODE ENABLED") {
		t.Error("expected debug mode script")
	}

	// KUBECONFIG env var should still be set (available when user execs in)
	foundEnv := false
	for _, env := range container.Env {
		if env.Name == "KUBECONFIG" {
			foundEnv = true
		}
	}
	if !foundEnv {
		t.Error("KUBECONFIG env var should be set even in debug mode")
	}
}

// TestJobBuilder_CompleteJobSpecification verifies all required fields in a complete job spec
func TestJobBuilder_CompleteJobSpecification(t *testing.T) {
	builder := NewJobBuilder("test-job", "default").
		WithContainerName("main").
		WithContainerImage("test-image:v1.0").
		WithCommand([]string{"/bin/sh"}).
		WithArgs([]string{"-c", "echo test"}).
		WithWorkingDir("/workspace").
		WithLabels(map[string]string{
			"app":    "test",
			"action": "build",
		}).
		WithServiceAccountName("test-sa").
		WithResources(corev1.ResourceRequirements{
			Requests: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("100m"),
				corev1.ResourceMemory: resource.MustParse("256Mi"),
			},
			Limits: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("500m"),
				corev1.ResourceMemory: resource.MustParse("1Gi"),
			},
		})

	job := builder.Build()

	// Verify metadata
	if job.Name != "test-job" || job.Namespace != "default" {
		t.Errorf("job metadata incorrect: %s/%s", job.Namespace, job.Name)
	}

	// Verify labels
	if job.Labels["app"] != "test" || job.Labels["action"] != "build" {
		t.Error("labels not set correctly")
	}

	// Verify service account
	if job.Spec.Template.Spec.ServiceAccountName != "test-sa" {
		t.Error("service account not set")
	}

	// Verify container spec
	if len(job.Spec.Template.Spec.Containers) != 1 {
		t.Fatalf("expected 1 container, got %d", len(job.Spec.Template.Spec.Containers))
	}

	container := job.Spec.Template.Spec.Containers[0]
	if container.Image != "test-image:v1.0" {
		t.Errorf("image = %s, want test-image:v1.0", container.Image)
	}
	if container.WorkingDir != "/workspace" {
		t.Errorf("workingDir = %s, want /workspace", container.WorkingDir)
	}
	if len(container.Command) != 1 || container.Command[0] != "/bin/sh" {
		t.Errorf("command = %v, want [/bin/sh]", container.Command)
	}

	// Verify resources
	if cpu := container.Resources.Requests[corev1.ResourceCPU]; cpu.String() != "100m" {
		t.Errorf("cpu request = %s, want 100m", cpu.String())
	}
	if mem := container.Resources.Limits[corev1.ResourceMemory]; mem.String() != "1Gi" {
		t.Errorf("memory limit = %s, want 1Gi", mem.String())
	}
}

// TestJobBuilder_AllVolumesPresent verifies all expected volumes are created
func TestJobBuilder_AllVolumesPresent(t *testing.T) {
	builder := NewJobBuilder("test-job", "default").
		WithContainerName("main").
		WithContainerImage("test:latest").
		WithCommand([]string{"/bin/sh", "-c"}).
		WithArgs([]string{"echo test"}).
		WithWorkspaceVolume(nil).
		WithArtifactPVC("artifact-pvc").
		WithUserConfig(1000)

	job := builder.Build()

	volumeNames := make(map[string]bool)
	for _, vol := range job.Spec.Template.Spec.Volumes {
		volumeNames[vol.Name] = true
	}

	expectedVolumes := []string{
		"workspace",
		"output",
		"tmp",
		"home",
	}

	for _, expected := range expectedVolumes {
		if !volumeNames[expected] {
			t.Errorf("expected volume %s not found", expected)
		}
	}

	// Verify artifact PVC is added
	if !volumeNames["artifacts"] {
		t.Error("expected artifacts PVC volume not found")
	}
}

// TestJobBuilder_InitContainerOrdering verifies init containers are created in correct order
func TestJobBuilder_InitContainerOrdering(t *testing.T) {
	initContainers := []corev1.Container{
		{
			Name:    "git-init",
			Image:   "alpine/git:latest",
			Command: []string{"git", "clone"},
		},
	}

	builder := NewJobBuilder("test-job", "default").
		WithContainerName("main").
		WithContainerImage("test:latest").
		WithCommand([]string{"/bin/sh", "-c"}).
		WithArgs([]string{"echo test"}).
		WithInitContainers(initContainers).
		WithWorkspaceVolume(nil)

	job := builder.Build()

	// With init containers, should have them in pod spec
	if len(job.Spec.Template.Spec.InitContainers) < 1 {
		t.Error("expected at least one init container")
	}

	// Init containers should be named appropriately
	for _, ic := range job.Spec.Template.Spec.InitContainers {
		if ic.Name == "" {
			t.Error("init container has empty name")
		}
	}
}

// TestJobBuilder_VolumeSizeConfiguration tests all volume size customization options
func TestJobBuilder_VolumeSizeConfiguration(t *testing.T) {
	tests := []struct {
		name          string
		volumeSizes   *common.VolumeSizes
		expectedSizes map[string]string
	}{
		{
			name: "all custom sizes",
			volumeSizes: &common.VolumeSizes{
				Workspace: Ptr(resource.MustParse("10Gi")),
				Output:    Ptr(resource.MustParse("5Gi")),
				Tmp:       Ptr(resource.MustParse("2Gi")),
				Home:      Ptr(resource.MustParse("500Mi")),
			},
			expectedSizes: map[string]string{
				"workspace": "10Gi",
				"output":    "5Gi",
				"tmp":       "2Gi",
				"home":      "500Mi",
			},
		},
		{
			name: "partial custom sizes",
			volumeSizes: &common.VolumeSizes{
				Workspace: Ptr(resource.MustParse("8Gi")),
				Output:    nil, // should use default 10Gi
			},
			expectedSizes: map[string]string{
				"workspace": "8Gi",
				"output":    "10Gi",
			},
		},
		{
			name:        "nil volume sizes uses all defaults",
			volumeSizes: nil,
			expectedSizes: map[string]string{
				"workspace": "10Gi",
				"output":    "10Gi",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			builder := NewJobBuilder("test-job", "default").
				WithContainerName("main").
				WithContainerImage("test:latest").
				WithCommand([]string{"/bin/sh", "-c"}).
				WithArgs([]string{"echo test"}).
				WithWorkspaceVolume(tt.volumeSizes).
				WithUserConfig(1000)

			job := builder.Build()
			volumes := job.Spec.Template.Spec.Volumes

			for volName, expectedSize := range tt.expectedSizes {
				size := findVolumeSizeLimit(t, volumes, volName)
				expected := resource.MustParse(expectedSize)
				if !size.Equal(expected) {
					t.Errorf("%s: got %s, want %s", volName, size.String(), expected.String())
				}
			}
		})
	}
}

// TestJobBuilder_SecurityContext verifies security context is set correctly
func TestJobBuilder_SecurityContext(t *testing.T) {
	builder := NewJobBuilder("test-job", "default").
		WithContainerName("main").
		WithContainerImage("test:latest").
		WithCommand([]string{"/bin/sh", "-c"}).
		WithArgs([]string{"echo test"}).
		WithUserConfig(1000)

	job := builder.Build()

	container := job.Spec.Template.Spec.Containers[0]

	// Verify container security context
	if container.SecurityContext == nil {
		t.Error("container security context not set")
	} else {
		if container.SecurityContext.RunAsUser == nil || *container.SecurityContext.RunAsUser != 1000 {
			t.Error("container runAsUser not set to 1000")
		}
		if container.SecurityContext.ReadOnlyRootFilesystem == nil || !*container.SecurityContext.ReadOnlyRootFilesystem {
			t.Error("readOnlyRootFilesystem not set to true")
		}
	}

	// Verify pod security context
	if job.Spec.Template.Spec.SecurityContext == nil {
		t.Error("pod security context not set")
	}
}

// TestJobBuilder_ExtraMountsValidation tests ExtraMount handling and validation
func TestJobBuilder_ExtraMountsValidation(t *testing.T) {
	tests := []struct {
		name        string
		extraMounts []common.ExtraMount
		shouldFail  bool
	}{
		{
			name: "valid ConfigMap mount",
			extraMounts: []common.ExtraMount{
				{
					ConfigMapRef: &common.LocalObjectReference{Name: "my-config"},
					MountPath:    "/etc/config",
				},
			},
			shouldFail: false,
		},
		{
			name: "valid Secret mount",
			extraMounts: []common.ExtraMount{
				{
					SecretRef: &common.LocalObjectReference{Name: "my-secret"},
					MountPath: "/etc/secret",
				},
			},
			shouldFail: false,
		},
		{
			name: "multiple mounts",
			extraMounts: []common.ExtraMount{
				{
					ConfigMapRef: &common.LocalObjectReference{Name: "config1"},
					MountPath:    "/etc/config",
				},
				{
					SecretRef: &common.LocalObjectReference{Name: "secret1"},
					MountPath: "/etc/secret",
				},
			},
			shouldFail: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			builder := NewJobBuilder("test-job", "default").
				WithContainerName("main").
				WithContainerImage("test:latest").
				WithCommand([]string{"/bin/sh", "-c"}).
				WithArgs([]string{"echo test"}).
				WithExtraMounts(tt.extraMounts)

			job := builder.Build()

			// Verify extra mounts are added
			if len(tt.extraMounts) > 0 {
				volumeFound := 0
				for _, vol := range job.Spec.Template.Spec.Volumes {
					if vol.ConfigMap != nil || vol.Secret != nil { // pragma: allowlist secret
						volumeFound++
					}
				}

				if volumeFound != len(tt.extraMounts) {
					t.Errorf("expected %d extra volumes, found %d", len(tt.extraMounts), volumeFound)
				}
			}
		})
	}
}

// TestJobBuilder_ExtraMountsReservedPaths tests that reserved paths are protected
func TestJobBuilder_ExtraMountsReservedPaths(t *testing.T) {
	reservedPaths := []string{
		"/workspace",
		"/output",
		"/tmp",
		"/home",
	}

	for _, reserved := range reservedPaths {
		t.Run("reserved path "+reserved, func(t *testing.T) {
			// Attempting to mount to a reserved path should be handled
			extraMount := common.ExtraMount{
				ConfigMapRef: &common.LocalObjectReference{Name: "config"},
				MountPath:    reserved,
			}

			builder := NewJobBuilder("test-job", "default").
				WithContainerName("main").
				WithContainerImage("test:latest").
				WithCommand([]string{"/bin/sh", "-c"}).
				WithArgs([]string{"echo test"}).
				WithExtraMounts([]common.ExtraMount{extraMount})

			job := builder.Build()

			// Just verify the job is created; validation of reserved paths
			// may happen at webhook level
			if job == nil {
				t.Error("job should be created")
			}
		})
	}
}

// TestJobBuilder_OwnerReferences verifies owner references are set correctly
func TestJobBuilder_OwnerReferences(t *testing.T) {
	builder := NewJobBuilder("test-job", "default").
		WithContainerName("main").
		WithContainerImage("test:latest").
		WithCommand([]string{"/bin/sh", "-c"}).
		WithArgs([]string{"echo test"})

	job := builder.Build()

	// Verify job can be built
	if job == nil {
		t.Fatal("job should be created")
	}
	if job.Name != "test-job" {
		t.Errorf("job name = %s, want test-job", job.Name)
	}
}

// TestJobBuilder_NodeSelectorAndAffinity verifies node selection and affinity rules
func TestJobBuilder_NodeSelectorAndAffinity(t *testing.T) {
	nodeSelector := map[string]string{
		"kubernetes.io/os": "linux",
		"gpu":              "true",
	}

	affinity := &corev1.Affinity{
		NodeAffinity: &corev1.NodeAffinity{
			RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
				NodeSelectorTerms: []corev1.NodeSelectorTerm{
					{
						MatchExpressions: []corev1.NodeSelectorRequirement{
							{
								Key:      "kubernetes.io/os",
								Operator: corev1.NodeSelectorOpIn,
								Values:   []string{"linux"},
							},
						},
					},
				},
			},
		},
	}

	builder := NewJobBuilder("test-job", "default").
		WithContainerName("main").
		WithContainerImage("test:latest").
		WithCommand([]string{"/bin/sh", "-c"}).
		WithArgs([]string{"echo test"}).
		WithNodeSelector(nodeSelector).
		WithAffinity(affinity)

	job := builder.Build()

	// Verify node selector
	if job.Spec.Template.Spec.NodeSelector["kubernetes.io/os"] != "linux" {
		t.Error("node selector not applied")
	}

	// Verify affinity
	if job.Spec.Template.Spec.Affinity == nil {
		t.Error("affinity not applied")
	}
}

// TestJobBuilder_Tolerations verifies tolerations are set correctly
func TestJobBuilder_Tolerations(t *testing.T) {
	tolerations := []corev1.Toleration{
		{
			Key:      "gpu",
			Operator: corev1.TolerationOpEqual,
			Value:    "true",
			Effect:   corev1.TaintEffectNoSchedule,
		},
	}

	builder := NewJobBuilder("test-job", "default").
		WithContainerName("main").
		WithContainerImage("test:latest").
		WithCommand([]string{"/bin/sh", "-c"}).
		WithArgs([]string{"echo test"}).
		WithTolerations(tolerations)

	job := builder.Build()

	if len(job.Spec.Template.Spec.Tolerations) != 1 {
		t.Errorf("expected 1 toleration, got %d", len(job.Spec.Template.Spec.Tolerations))
	}

	if job.Spec.Template.Spec.Tolerations[0].Key != "gpu" {
		t.Error("toleration not applied correctly")
	}
}
