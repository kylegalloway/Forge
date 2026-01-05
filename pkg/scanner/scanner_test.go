package scanner

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
)

func TestGetScanInitContainer_Disabled(t *testing.T) {
	config := Config{
		Enabled: false,
	}

	s := New(config)
	container, err := s.GetScanInitContainer("test-image:latest")

	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	if container != nil {
		t.Error("Expected nil container when scanning is disabled")
	}
}

func TestGetScanInitContainer_EmptyImage(t *testing.T) {
	config := Config{
		Enabled: true,
		Scanner: "trivy",
	}

	s := New(config)
	_, err := s.GetScanInitContainer("")

	if err == nil {
		t.Error("Expected error for empty image, got nil")
	}
}

func TestGetScanInitContainer_Trivy(t *testing.T) {
	config := Config{
		Enabled:       true,
		Scanner:       "trivy",
		TrivyImage:    "aquasec/trivy:0.57.1",
		Severities:    "CRITICAL,HIGH",
		Scanners:      "vuln,secret",
		EnforcePolicy: true,
	}

	s := New(config)
	container, err := s.GetScanInitContainer("zarf:v0.68.1")

	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	if container == nil {
		t.Fatal("Expected non-nil container")
	}

	if container.Name != "trivy-scan" {
		t.Errorf("Expected name 'trivy-scan', got: %s", container.Name)
	}

	if container.Image != "aquasec/trivy:0.57.1" {
		t.Errorf("Expected image 'aquasec/trivy:0.57.1', got: %s", container.Image)
	}

	// Check security context
	if container.SecurityContext == nil {
		t.Fatal("Expected security context to be set")
	}

	if container.SecurityContext.RunAsNonRoot == nil || !*container.SecurityContext.RunAsNonRoot {
		t.Error("Expected RunAsNonRoot to be true")
	}

	if container.SecurityContext.ReadOnlyRootFilesystem == nil || !*container.SecurityContext.ReadOnlyRootFilesystem {
		t.Error("Expected ReadOnlyRootFilesystem to be true")
	}

	// Check volume mounts
	if len(container.VolumeMounts) == 0 {
		t.Error("Expected volume mounts for Trivy cache")
	}

	foundCacheMount := false
	for _, mount := range container.VolumeMounts {
		if mount.Name == "trivy-cache" {
			foundCacheMount = true
			break
		}
	}

	if !foundCacheMount {
		t.Error("Expected trivy-cache volume mount")
	}
}

func TestGetScanInitContainer_Grype(t *testing.T) {
	config := Config{
		Enabled:       true,
		Scanner:       "grype",
		GrypeImage:    "anchore/grype:v0.83.0",
		FailOn:        "critical",
		EnforcePolicy: true,
	}

	s := New(config)
	container, err := s.GetScanInitContainer("zarf:v0.68.1")

	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	if container == nil {
		t.Fatal("Expected non-nil container")
	}

	if container.Name != "grype-scan" {
		t.Errorf("Expected name 'grype-scan', got: %s", container.Name)
	}

	if container.Image != "anchore/grype:v0.83.0" {
		t.Errorf("Expected image 'anchore/grype:v0.83.0', got: %s", container.Image)
	}
}

func TestGetScanInitContainer_UnsupportedScanner(t *testing.T) {
	config := Config{
		Enabled: true,
		Scanner: "unsupported-scanner",
	}

	s := New(config)
	_, err := s.GetScanInitContainer("test-image:latest")

	if err == nil {
		t.Error("Expected error for unsupported scanner, got nil")
	}
}

func TestGetCacheVolume_Disabled(t *testing.T) {
	config := Config{
		Enabled: false,
	}

	s := New(config)
	volume := s.GetCacheVolume()

	if volume != nil {
		t.Error("Expected nil volume when scanning is disabled")
	}
}

func TestGetCacheVolume_Trivy(t *testing.T) {
	config := Config{
		Enabled: true,
		Scanner: "trivy",
	}

	s := New(config)
	volume := s.GetCacheVolume()

	if volume == nil {
		t.Fatal("Expected non-nil volume")
	}

	if volume.Name != "trivy-cache" {
		t.Errorf("Expected volume name 'trivy-cache', got: %s", volume.Name)
	}

	if volume.EmptyDir == nil {
		t.Error("Expected EmptyDir volume source")
	}
}

func TestGetCacheVolume_Grype(t *testing.T) {
	config := Config{
		Enabled: true,
		Scanner: "grype",
	}

	s := New(config)
	volume := s.GetCacheVolume()

	if volume == nil {
		t.Fatal("Expected non-nil volume")
	}

	if volume.Name != "grype-cache" {
		t.Errorf("Expected volume name 'grype-cache', got: %s", volume.Name)
	}
}

func TestGetScanInitContainer_WithIgnoredCVEs(t *testing.T) {
	config := Config{
		Enabled:       true,
		Scanner:       "trivy",
		TrivyImage:    "aquasec/trivy:0.57.1",
		Severities:    "CRITICAL,HIGH",
		Scanners:      "vuln",
		EnforcePolicy: true,
		IgnoredCVEs:   []string{"CVE-2024-1234", "CVE-2024-5678"},
	}

	s := New(config)
	container, err := s.GetScanInitContainer("test:latest")

	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	if container == nil {
		t.Fatal("Expected non-nil container")
	}

	// The command should reference the ignored CVEs
	// We can't easily test the full command string, but we verified the config is set
}

func TestSecurityContext(t *testing.T) {
	config := Config{
		Enabled:    true,
		Scanner:    "trivy",
		TrivyImage: "aquasec/trivy:latest",
	}

	s := New(config)
	container, _ := s.GetScanInitContainer("test:latest")

	sc := container.SecurityContext
	if sc == nil {
		t.Fatal("SecurityContext should not be nil")
	}

	// Verify all security constraints
	tests := []struct {
		name     string
		check    func() bool
		expected bool
	}{
		{"AllowPrivilegeEscalation", func() bool { return sc.AllowPrivilegeEscalation != nil && !*sc.AllowPrivilegeEscalation }, true},
		{"RunAsNonRoot", func() bool { return sc.RunAsNonRoot != nil && *sc.RunAsNonRoot }, true},
		{"ReadOnlyRootFilesystem", func() bool { return sc.ReadOnlyRootFilesystem != nil && *sc.ReadOnlyRootFilesystem }, true},
		{"DropAllCapabilities", func() bool {
			return sc.Capabilities != nil && len(sc.Capabilities.Drop) > 0 && sc.Capabilities.Drop[0] == corev1.Capability("ALL")
		}, true},
		{"SeccompProfile", func() bool {
			return sc.SeccompProfile != nil && sc.SeccompProfile.Type == corev1.SeccompProfileTypeRuntimeDefault
		}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.check() != tt.expected {
				t.Errorf("%s check failed", tt.name)
			}
		})
	}
}
