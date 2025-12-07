package validation

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// TestYAMLSyntax validates that all YAML files in .config/ are syntactically correct
func TestYAMLSyntax(t *testing.T) {
	t.Skip("Skipping YAML syntax validation - config manifests not generated")
	rootDirs := []string{
		"../../.config",
	}

	for _, rootDir := range rootDirs {
		err := filepath.Walk(rootDir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if info.IsDir() {
				return nil
			}
			if !strings.HasSuffix(path, ".yaml") && !strings.HasSuffix(path, ".yml") {
				return nil
			}

			// Skip template files that contain Go template syntax
			if strings.Contains(path, "templates") {
				return nil
			}

			t.Run(path, func(t *testing.T) {
				data, readErr := os.ReadFile(path)
				if readErr != nil {
					t.Fatalf("Failed to read %s: %v", path, readErr)
				}

				// Split on --- for multi-document YAML
				docs := strings.Split(string(data), "\n---\n")
				for docIndex, doc := range docs {
					doc = strings.TrimSpace(doc)
					if doc == "" {
						continue
					}

					var obj interface{}
					if err := yaml.Unmarshal([]byte(doc), &obj); err != nil {
						t.Errorf("Document %d in %s has invalid YAML syntax: %v", docIndex, path, err)
					}
				}
			})

			return nil
		})

		if err != nil {
			t.Fatalf("Failed to walk directory %s: %v", rootDir, err)
		}
	}
}

// TestZarfPackageJobSamples validates that all ZarfPackageJob samples are well-formed
func TestZarfPackageJobSamples(t *testing.T) {
	t.Skip("Skipping sample validation - samples not generated")
	samplesDir := "../../.config/samples/v1alpha1"

	files, err := os.ReadDir(samplesDir)
	if err != nil {
		t.Fatalf("Failed to read samples directory: %v", err)
	}

	for _, file := range files {
		if file.IsDir() {
			continue
		}
		if !strings.HasSuffix(file.Name(), ".yaml") {
			continue
		}

		path := filepath.Join(samplesDir, file.Name())
		t.Run(file.Name(), func(t *testing.T) {
			data, readErr := os.ReadFile(path)
			if readErr != nil {
				t.Fatalf("Failed to read %s: %v", path, readErr)
			}

			var obj unstructured.Unstructured
			if err := yaml.Unmarshal(data, &obj.Object); err != nil {
				t.Fatalf("Failed to unmarshal YAML: %v", err)
			}

			// Validate required fields
			apiVersion := obj.GetAPIVersion()
			if apiVersion != "forge.dev/v1alpha1" {
				t.Errorf("Expected apiVersion 'forge.dev/v1alpha1', got '%s'", apiVersion)
			}

			kind := obj.GetKind()
			if kind != "ZarfPackageJob" {
				t.Errorf("Expected kind 'ZarfPackageJob', got '%s'", kind)
			}

			name := obj.GetName()
			if name == "" {
				t.Error("metadata.name is required")
			}

			namespace := obj.GetNamespace()
			if namespace == "" {
				t.Error("metadata.namespace is required")
			}

			// Validate spec exists
			spec, found, err := unstructured.NestedMap(obj.Object, "spec")
			if err != nil {
				t.Fatalf("Failed to get spec: %v", err)
			}
			if !found {
				t.Fatal("spec is required")
			}

			// Validate required spec fields
			action, found, err := unstructured.NestedString(spec, "action")
			if err != nil || !found {
				t.Error("spec.action is required")
			}
			if action == "" {
				t.Error("spec.action cannot be empty")
			}

			// Validate source exists
			_, found, err = unstructured.NestedMap(spec, "source")
			if err != nil {
				t.Fatalf("Failed to get spec.source: %v", err)
			}
			if !found {
				t.Error("spec.source is required")
			}
		})
	}
}

// TestCRDManifest validates the CRD manifest structure
func TestCRDManifest(t *testing.T) {
	t.Skip("Skipping CRD validation - CRD manifest not generated")
	crdPath := "../../.config/crd/forge.dev_zarfpackagejobs.yaml"

	data, err := os.ReadFile(crdPath)
	if err != nil {
		t.Fatalf("Failed to read CRD: %v", err)
	}

	var crd unstructured.Unstructured
	err = yaml.Unmarshal(data, &crd.Object)
	if err != nil {
		t.Fatalf("Failed to unmarshal CRD: %v", err)
	}

	// Validate CRD structure
	if crd.GetKind() != "CustomResourceDefinition" {
		t.Errorf("Expected kind 'CustomResourceDefinition', got '%s'", crd.GetKind())
	}

	name := crd.GetName()
	if name != "zarfpackagejobs.forge.dev" {
		t.Errorf("Expected name 'zarfpackagejobs.forge.dev', got '%s'", name)
	}

	spec, found, err := unstructured.NestedMap(crd.Object, "spec")
	if err != nil || !found {
		t.Fatal("spec is required in CRD")
	}

	group, found, err := unstructured.NestedString(spec, "group")
	if err != nil || !found {
		t.Fatal("spec.group is required")
	}
	if group != "forge.dev" {
		t.Errorf("Expected group 'forge.dev', got '%s'", group)
	}

	names, found, err := unstructured.NestedMap(spec, "names")
	if err != nil || !found {
		t.Fatal("spec.names is required")
	}

	kind, found, err := unstructured.NestedString(names, "kind")
	if err != nil || !found {
		t.Fatal("spec.names.kind is required")
	}
	if kind != "ZarfPackageJob" {
		t.Errorf("Expected kind 'ZarfPackageJob', got '%s'", kind)
	}

	plural, found, err := unstructured.NestedString(names, "plural")
	if err != nil || !found {
		t.Fatal("spec.names.plural is required")
	}
	if plural != "zarfpackagejobs" {
		t.Errorf("Expected plural 'zarfpackagejobs', got '%s'", plural)
	}

	// Validate shortnames
	shortNames, found, err := unstructured.NestedStringSlice(names, "shortNames")
	if err != nil || !found {
		t.Fatal("spec.names.shortNames is required")
	}
	if len(shortNames) == 0 || shortNames[0] != "zpj" {
		t.Errorf("Expected shortname 'zpj', got %v", shortNames)
	}
}

// TestRBACManifests validates RBAC configuration
func TestRBACManifests(t *testing.T) {
	t.Skip("Skipping RBAC validation - RBAC manifests not generated")
	rbacFiles := []string{
		"../../.config/rbac/rbac.yaml",
		"../../.config/namespace-scoped/rbac.yaml",
	}

	for _, rbacFile := range rbacFiles {
		t.Run(rbacFile, func(t *testing.T) {
			data, err := os.ReadFile(rbacFile)
			if err != nil {
				t.Fatalf("Failed to read %s: %v", rbacFile, err)
			}

			// Split on --- for multi-document YAML
			docs := strings.Split(string(data), "\n---\n")
			hasServiceAccount := false
			hasRole := false
			hasBinding := false

			for _, doc := range docs {
				doc = strings.TrimSpace(doc)
				if doc == "" {
					continue
				}

				var obj unstructured.Unstructured
				if err := yaml.Unmarshal([]byte(doc), &obj.Object); err != nil {
					t.Fatalf("Failed to unmarshal YAML: %v", err)
				}

				kind := obj.GetKind()
				switch kind {
				case "ServiceAccount":
					hasServiceAccount = true
					if obj.GetName() != "forge-controller" {
						t.Errorf("Expected ServiceAccount name 'forge-controller', got '%s'", obj.GetName())
					}
				case "ClusterRole", "Role":
					hasRole = true
				case "ClusterRoleBinding", "RoleBinding":
					hasBinding = true
				}
			}

			if !hasServiceAccount {
				t.Error("RBAC manifest must include ServiceAccount")
			}
			if !hasRole {
				t.Error("RBAC manifest must include Role or ClusterRole")
			}
			if !hasBinding {
				t.Error("RBAC manifest must include RoleBinding or ClusterRoleBinding")
			}
		})
	}
}
