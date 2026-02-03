package uds

import (
	"strings"
	"testing"

	udsv1alpha3 "github.com/kylegalloway/forge/pkg/apis/uds/v1alpha3"
)

func TestBuildPreTaskCommands_Empty(t *testing.T) {
	result := buildPreTaskCommands(nil)
	if result != "" {
		t.Errorf("buildPreTaskCommands(nil) = %q, want empty string", result)
	}

	result = buildPreTaskCommands([]udsv1alpha3.RunnerPreTask{})
	if result != "" {
		t.Errorf("buildPreTaskCommands([]) = %q, want empty string", result)
	}
}

func TestBuildPreTaskCommands_SingleTaskNoVars(t *testing.T) {
	tasks := []udsv1alpha3.RunnerPreTask{
		{Name: "setup-deps"},
	}
	result := buildPreTaskCommands(tasks)
	if result != "uds run setup-deps" {
		t.Errorf("buildPreTaskCommands() = %q, want %q", result, "uds run setup-deps")
	}
}

func TestBuildPreTaskCommands_SingleTaskWithVars(t *testing.T) {
	tasks := []udsv1alpha3.RunnerPreTask{
		{
			Name: "setup-deps",
			Variables: map[string]string{
				"ZARF_VERSION": "0.40.0",
			},
		},
	}
	result := buildPreTaskCommands(tasks)
	if !strings.HasPrefix(result, "uds run setup-deps") {
		t.Errorf("buildPreTaskCommands() = %q, want prefix %q", result, "uds run setup-deps")
	}
	if !strings.Contains(result, "--set ZARF_VERSION=0.40.0") {
		t.Errorf("buildPreTaskCommands() = %q, want to contain %q", result, "--set ZARF_VERSION=0.40.0")
	}
}

func TestBuildPreTaskCommands_MultipleTasks(t *testing.T) {
	tasks := []udsv1alpha3.RunnerPreTask{
		{
			Name: "setup-deps",
			Variables: map[string]string{
				"ZARF_VERSION": "0.40.0",
			},
		},
		{Name: "generate-config"},
	}
	result := buildPreTaskCommands(tasks)

	// Should contain both tasks joined by &&
	if !strings.Contains(result, "uds run setup-deps") {
		t.Errorf("buildPreTaskCommands() = %q, missing 'uds run setup-deps'", result)
	}
	if !strings.Contains(result, "uds run generate-config") {
		t.Errorf("buildPreTaskCommands() = %q, missing 'uds run generate-config'", result)
	}
	if !strings.Contains(result, " && ") {
		t.Errorf("buildPreTaskCommands() = %q, missing ' && ' separator", result)
	}

	// Verify order: setup-deps comes before generate-config
	setupIdx := strings.Index(result, "uds run setup-deps")
	configIdx := strings.Index(result, "uds run generate-config")
	if setupIdx >= configIdx {
		t.Errorf("buildPreTaskCommands() tasks in wrong order: setup-deps at %d, generate-config at %d", setupIdx, configIdx)
	}
}

func TestBuildPreTaskCommands_TaskWithMultipleVars(t *testing.T) {
	tasks := []udsv1alpha3.RunnerPreTask{
		{
			Name: "configure",
			Variables: map[string]string{
				"KEY1": "val1",
				"KEY2": "val2",
			},
		},
	}
	result := buildPreTaskCommands(tasks)
	if !strings.Contains(result, "--set KEY1=val1") {
		t.Errorf("buildPreTaskCommands() = %q, missing '--set KEY1=val1'", result)
	}
	if !strings.Contains(result, "--set KEY2=val2") {
		t.Errorf("buildPreTaskCommands() = %q, missing '--set KEY2=val2'", result)
	}
}
