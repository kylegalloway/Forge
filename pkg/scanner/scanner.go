// Package scanner provides image vulnerability scanning capabilities
package scanner

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

// Config holds the scanner configuration
type Config struct {
	Enabled       bool
	Scanner       string // "trivy" or "grype"
	TrivyImage    string
	GrypeImage    string
	Severities    string
	FailOn        string
	Scanners      string
	EnforcePolicy bool
	IgnoredCVEs   []string
	MaxCVEs       CVELimits
}

// CVELimits defines maximum allowed CVE counts by severity
type CVELimits struct {
	Critical int
	High     int
	Medium   int
}

// Scanner provides image scanning functionality
type Scanner struct {
	config Config
}

// New creates a new Scanner instance
func New(config Config) *Scanner {
	return &Scanner{
		config: config,
	}
}

// GetScanInitContainer returns an init container that scans the target image
// Returns nil if scanning is disabled
func (s *Scanner) GetScanInitContainer(targetImage string) (*corev1.Container, error) {
	if !s.config.Enabled {
		return nil, nil
	}

	if targetImage == "" {
		return nil, fmt.Errorf("target image cannot be empty")
	}

	switch s.config.Scanner {
	case "trivy":
		return s.getTrivyInitContainer(targetImage), nil
	case "grype":
		return s.getGrypeInitContainer(targetImage), nil
	default:
		return nil, fmt.Errorf("unsupported scanner: %s", s.config.Scanner)
	}
}

// getTrivyInitContainer creates a Trivy scanner init container
func (s *Scanner) getTrivyInitContainer(targetImage string) *corev1.Container {
	// Build Trivy command
	// trivy image --exit-code 1 --severity CRITICAL,HIGH --scanners vuln,secret <image>
	command := []string{
		"trivy",
		"image",
		"--severity", s.config.Severities,
		"--scanners", s.config.Scanners,
	}

	// Add exit code to fail job if vulnerabilities found
	if s.config.EnforcePolicy {
		command = append(command, "--exit-code", "1")
	}

	// Add ignored CVEs if specified
	for _, cve := range s.config.IgnoredCVEs {
		command = append(command, "--ignore-unfixed", "--ignorefile", fmt.Sprintf("/tmp/trivyignore-%s", cve))
	}

	// Add target image
	command = append(command, targetImage)

	return &corev1.Container{
		Name:  "trivy-scan",
		Image: s.config.TrivyImage,
		Command: []string{
			"/bin/sh",
			"-c",
			fmt.Sprintf(`
echo "🔍 Scanning image: %s"
%s
SCAN_RESULT=$?
if [ $SCAN_RESULT -eq 0 ]; then
  echo "✅ Image scan passed - no critical vulnerabilities found"
else
  echo "❌ Image scan failed - vulnerabilities detected"
  exit $SCAN_RESULT
fi
`, targetImage, shellJoin(command)),
		},
		Resources: corev1.ResourceRequirements{
			Requests: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("100m"),
				corev1.ResourceMemory: resource.MustParse("128Mi"),
			},
			Limits: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("500m"),
				corev1.ResourceMemory: resource.MustParse("512Mi"),
			},
		},
		SecurityContext: &corev1.SecurityContext{
			AllowPrivilegeEscalation: boolPtr(false),
			RunAsNonRoot:             boolPtr(true),
			RunAsUser:                int64Ptr(65532),
			ReadOnlyRootFilesystem:   boolPtr(true),
			Capabilities: &corev1.Capabilities{
				Drop: []corev1.Capability{"ALL"},
			},
			SeccompProfile: &corev1.SeccompProfile{
				Type: corev1.SeccompProfileTypeRuntimeDefault,
			},
		},
		// Need write access to /tmp for Trivy cache
		VolumeMounts: []corev1.VolumeMount{
			{
				Name:      "trivy-cache",
				MountPath: "/tmp",
			},
		},
	}
}

// getGrypeInitContainer creates a Grype scanner init container
func (s *Scanner) getGrypeInitContainer(targetImage string) *corev1.Container {
	// Build Grype command
	// grype <image> --fail-on critical
	command := []string{
		"grype",
		targetImage,
	}

	if s.config.EnforcePolicy {
		command = append(command, "--fail-on", s.config.FailOn)
	}

	return &corev1.Container{
		Name:  "grype-scan",
		Image: s.config.GrypeImage,
		Command: []string{
			"/bin/sh",
			"-c",
			fmt.Sprintf(`
echo "🔍 Scanning image with Grype: %s"
%s
SCAN_RESULT=$?
if [ $SCAN_RESULT -eq 0 ]; then
  echo "✅ Image scan passed"
else
  echo "❌ Image scan failed"
  exit $SCAN_RESULT
fi
`, targetImage, shellJoin(command)),
		},
		Resources: corev1.ResourceRequirements{
			Requests: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("100m"),
				corev1.ResourceMemory: resource.MustParse("128Mi"),
			},
			Limits: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("500m"),
				corev1.ResourceMemory: resource.MustParse("512Mi"),
			},
		},
		SecurityContext: &corev1.SecurityContext{
			AllowPrivilegeEscalation: boolPtr(false),
			RunAsNonRoot:             boolPtr(true),
			RunAsUser:                int64Ptr(65532),
			ReadOnlyRootFilesystem:   boolPtr(true),
			Capabilities: &corev1.Capabilities{
				Drop: []corev1.Capability{"ALL"},
			},
			SeccompProfile: &corev1.SeccompProfile{
				Type: corev1.SeccompProfileTypeRuntimeDefault,
			},
		},
		VolumeMounts: []corev1.VolumeMount{
			{
				Name:      "grype-cache",
				MountPath: "/tmp",
			},
		},
	}
}

// GetCacheVolume returns a volume for scanner cache
func (s *Scanner) GetCacheVolume() *corev1.Volume {
	if !s.config.Enabled {
		return nil
	}

	volumeName := "trivy-cache"
	if s.config.Scanner == "grype" {
		volumeName = "grype-cache"
	}

	return &corev1.Volume{
		Name: volumeName,
		VolumeSource: corev1.VolumeSource{
			EmptyDir: &corev1.EmptyDirVolumeSource{},
		},
	}
}

// Helper functions

func shellJoin(args []string) string {
	result := ""
	for i, arg := range args {
		if i > 0 {
			result += " "
		}
		// Simple quoting - escape single quotes
		result += "'" + arg + "'"
	}
	return result
}

func boolPtr(b bool) *bool {
	return &b
}

func int64Ptr(i int64) *int64 {
	return &i
}
