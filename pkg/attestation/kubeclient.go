package attestation

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
)

// RealKubeClient is a real implementation of KubeClient using client-go
type RealKubeClient struct {
	clientset kubernetes.Interface
}

// NewRealKubeClient creates a new RealKubeClient
func NewRealKubeClient(clientset kubernetes.Interface) *RealKubeClient {
	return &RealKubeClient{
		clientset: clientset,
	}
}

// CreateConfigMap creates a ConfigMap in Kubernetes
func (r *RealKubeClient) CreateConfigMap(ctx context.Context, namespace string, cm *ConfigMap) error {
	k8sCM := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:   cm.Name,
			Labels: cm.Labels,
		},
		Data: cm.Data,
	}

	_, err := r.clientset.CoreV1().ConfigMaps(namespace).Create(ctx, k8sCM, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create ConfigMap: %w", err)
	}

	klog.V(4).InfoS("Created ConfigMap", "name", cm.Name, "namespace", namespace)
	return nil
}

// GetConfigMap retrieves a ConfigMap from Kubernetes
func (r *RealKubeClient) GetConfigMap(ctx context.Context, namespace, name string) (*ConfigMap, error) {
	k8sCM, err := r.clientset.CoreV1().ConfigMaps(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			return nil, fmt.Errorf("configmap %s not found", name)
		}
		return nil, fmt.Errorf("failed to get ConfigMap: %w", err)
	}

	cm := &ConfigMap{
		Name:   k8sCM.Name,
		Labels: k8sCM.Labels,
		Data:   k8sCM.Data,
	}

	klog.V(4).InfoS("Retrieved ConfigMap", "name", name, "namespace", namespace)
	return cm, nil
}

// ListConfigMaps lists ConfigMaps matching the given labels
func (r *RealKubeClient) ListConfigMaps(ctx context.Context, namespace string, labels map[string]string) ([]*ConfigMap, error) {
	// Build label selector
	labelSelector := metav1.LabelSelector{
		MatchLabels: labels,
	}

	selector, err := metav1.LabelSelectorAsSelector(&labelSelector)
	if err != nil {
		return nil, fmt.Errorf("failed to build label selector: %w", err)
	}

	k8sCMList, err := r.clientset.CoreV1().ConfigMaps(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: selector.String(),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list ConfigMaps: %w", err)
	}

	var results []*ConfigMap
	for i := range k8sCMList.Items {
		item := &k8sCMList.Items[i]
		cm := &ConfigMap{
			Name:   item.Name,
			Labels: item.Labels,
			Data:   item.Data,
		}
		results = append(results, cm)
	}

	klog.V(4).InfoS("Listed ConfigMaps", "namespace", namespace, "count", len(results))
	return results, nil
}

// UpdateConfigMap updates an existing ConfigMap in Kubernetes
func (r *RealKubeClient) UpdateConfigMap(ctx context.Context, namespace string, cm *ConfigMap) error {
	k8sCM := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:   cm.Name,
			Labels: cm.Labels,
		},
		Data: cm.Data,
	}

	_, err := r.clientset.CoreV1().ConfigMaps(namespace).Update(ctx, k8sCM, metav1.UpdateOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			return fmt.Errorf("configmap %s not found", cm.Name)
		}
		return fmt.Errorf("failed to update ConfigMap: %w", err)
	}

	klog.V(4).InfoS("Updated ConfigMap", "name", cm.Name, "namespace", namespace)
	return nil
}
