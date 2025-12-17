package attestation

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestRealKubeClient_CreateConfigMap(t *testing.T) {
	clientset := fake.NewSimpleClientset()
	client := NewRealKubeClient(clientset)

	cm := &ConfigMap{
		Name: "test-cm",
		Labels: map[string]string{
			"app": "test",
		},
		Data: map[string]string{
			"key": "value",
		},
	}

	err := client.CreateConfigMap(context.Background(), "default", cm)
	if err != nil {
		t.Fatalf("CreateConfigMap failed: %v", err)
	}

	// Verify ConfigMap was created
	k8sCM, err := clientset.CoreV1().ConfigMaps("default").Get(context.Background(), "test-cm", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Failed to get created ConfigMap: %v", err)
	}

	if k8sCM.Name != "test-cm" {
		t.Errorf("Expected name 'test-cm', got %s", k8sCM.Name)
	}

	if k8sCM.Labels["app"] != "test" {
		t.Errorf("Expected label app='test', got %s", k8sCM.Labels["app"])
	}

	if k8sCM.Data["key"] != "value" {
		t.Errorf("Expected data key='value', got %s", k8sCM.Data["key"])
	}
}

func TestRealKubeClient_GetConfigMap(t *testing.T) {
	clientset := fake.NewSimpleClientset()
	client := NewRealKubeClient(clientset)

	// Create a ConfigMap first
	k8sCM := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cm",
			Namespace: "default",
			Labels: map[string]string{
				"app": "test",
			},
		},
		Data: map[string]string{
			"key": "value",
		},
	}

	_, err := clientset.CoreV1().ConfigMaps("default").Create(context.Background(), k8sCM, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("Failed to create test ConfigMap: %v", err)
	}

	// Get the ConfigMap
	cm, err := client.GetConfigMap(context.Background(), "default", "test-cm")
	if err != nil {
		t.Fatalf("GetConfigMap failed: %v", err)
	}

	if cm.Name != "test-cm" {
		t.Errorf("Expected name 'test-cm', got %s", cm.Name)
	}

	if cm.Labels["app"] != "test" {
		t.Errorf("Expected label app='test', got %s", cm.Labels["app"])
	}

	if cm.Data["key"] != "value" {
		t.Errorf("Expected data key='value', got %s", cm.Data["key"])
	}
}

func TestRealKubeClient_GetConfigMap_NotFound(t *testing.T) {
	clientset := fake.NewSimpleClientset()
	client := NewRealKubeClient(clientset)

	_, err := client.GetConfigMap(context.Background(), "default", "nonexistent")
	if err == nil {
		t.Fatal("Expected error for nonexistent ConfigMap, got nil")
	}

	expectedMsg := "configmap nonexistent not found"
	if err.Error() != expectedMsg {
		t.Errorf("Expected error message '%s', got '%s'", expectedMsg, err.Error())
	}
}

func TestRealKubeClient_ListConfigMaps(t *testing.T) {
	clientset := fake.NewSimpleClientset()
	client := NewRealKubeClient(clientset)

	// Create multiple ConfigMaps
	for i := 1; i <= 3; i++ {
		k8sCM := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-cm-" + string(rune('0'+i)),
				Namespace: "default",
				Labels: map[string]string{
					"app":  "test",
					"type": "attestation",
				},
			},
			Data: map[string]string{
				"key": "value",
			},
		}

		_, err := clientset.CoreV1().ConfigMaps("default").Create(context.Background(), k8sCM, metav1.CreateOptions{})
		if err != nil {
			t.Fatalf("Failed to create test ConfigMap: %v", err)
		}
	}

	// Create one ConfigMap without the labels
	k8sCM := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "other-cm",
			Namespace: "default",
			Labels: map[string]string{
				"app": "other",
			},
		},
		Data: map[string]string{
			"key": "value",
		},
	}

	_, err := clientset.CoreV1().ConfigMaps("default").Create(context.Background(), k8sCM, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("Failed to create test ConfigMap: %v", err)
	}

	// List ConfigMaps with labels
	labels := map[string]string{
		"app":  "test",
		"type": "attestation",
	}

	cms, err := client.ListConfigMaps(context.Background(), "default", labels)
	if err != nil {
		t.Fatalf("ListConfigMaps failed: %v", err)
	}

	if len(cms) != 3 {
		t.Errorf("Expected 3 ConfigMaps, got %d", len(cms))
	}

	for _, cm := range cms {
		if cm.Labels["app"] != "test" {
			t.Errorf("Expected label app='test', got %s", cm.Labels["app"])
		}
		if cm.Labels["type"] != "attestation" {
			t.Errorf("Expected label type='attestation', got %s", cm.Labels["type"])
		}
	}
}

func TestRealKubeClient_ListConfigMaps_Empty(t *testing.T) {
	clientset := fake.NewSimpleClientset()
	client := NewRealKubeClient(clientset)

	labels := map[string]string{
		"app": "nonexistent",
	}

	cms, err := client.ListConfigMaps(context.Background(), "default", labels)
	if err != nil {
		t.Fatalf("ListConfigMaps failed: %v", err)
	}

	if len(cms) != 0 {
		t.Errorf("Expected 0 ConfigMaps, got %d", len(cms))
	}
}

func TestRealKubeClient_UpdateConfigMap(t *testing.T) {
	clientset := fake.NewSimpleClientset()
	client := NewRealKubeClient(clientset)

	// Create a ConfigMap first
	k8sCM := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cm",
			Namespace: "default",
			Labels: map[string]string{
				"app": "test",
			},
		},
		Data: map[string]string{
			"key": "value",
		},
	}

	_, err := clientset.CoreV1().ConfigMaps("default").Create(context.Background(), k8sCM, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("Failed to create test ConfigMap: %v", err)
	}

	// Update the ConfigMap
	cm := &ConfigMap{
		Name: "test-cm",
		Labels: map[string]string{
			"app":     "test",
			"updated": "true",
		},
		Data: map[string]string{
			"key":    "newvalue",
			"newkey": "newvalue",
		},
	}

	err = client.UpdateConfigMap(context.Background(), "default", cm)
	if err != nil {
		t.Fatalf("UpdateConfigMap failed: %v", err)
	}

	// Verify ConfigMap was updated
	k8sCM, err = clientset.CoreV1().ConfigMaps("default").Get(context.Background(), "test-cm", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Failed to get updated ConfigMap: %v", err)
	}

	if k8sCM.Labels["updated"] != "true" {
		t.Errorf("Expected label updated='true', got %s", k8sCM.Labels["updated"])
	}

	if k8sCM.Data["key"] != "newvalue" {
		t.Errorf("Expected data key='newvalue', got %s", k8sCM.Data["key"])
	}

	if k8sCM.Data["newkey"] != "newvalue" {
		t.Errorf("Expected data newkey='newvalue', got %s", k8sCM.Data["newkey"])
	}
}

func TestRealKubeClient_UpdateConfigMap_NotFound(t *testing.T) {
	clientset := fake.NewSimpleClientset()
	client := NewRealKubeClient(clientset)

	cm := &ConfigMap{
		Name: "nonexistent",
		Labels: map[string]string{
			"app": "test",
		},
		Data: map[string]string{
			"key": "value",
		},
	}

	err := client.UpdateConfigMap(context.Background(), "default", cm)
	if err == nil {
		t.Fatal("Expected error for nonexistent ConfigMap, got nil")
	}

	expectedMsg := "configmap nonexistent not found"
	if err.Error() != expectedMsg {
		t.Errorf("Expected error message '%s', got '%s'", expectedMsg, err.Error())
	}
}
