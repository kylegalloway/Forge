package controller

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"

	zarfv1alpha1 "github.com/kylegalloway/forge/pkg/apis/zarf/v1alpha1"
)

const (
	// DefaultArtifactStorageSize is the default size for artifact PVCs
	DefaultArtifactStorageSize = "10Gi"
	// ArtifactVolumeName is the name of the artifact volume
	ArtifactVolumeName = "artifacts"
	// ArtifactMountPath is the mount path for the artifact volume
	ArtifactMountPath = "/artifacts"
	// ArtifactStorageLabel is the label applied to artifact PVCs
	ArtifactStorageLabel = "forge.dev/artifact-storage"
)

// isMultiActionZarfJob checks if an action requires artifact sharing between jobs
func isMultiActionZarfJob(action zarfv1alpha1.Action) bool {
	return action == zarfv1alpha1.ActionBuildPublish ||
		action == zarfv1alpha1.ActionBuildDeploy ||
		action == zarfv1alpha1.ActionPublishDeploy ||
		action == zarfv1alpha1.ActionBuildPublishDeploy
}

// ensureArtifactPVC creates or retrieves the shared artifact PVC for multi-action jobs
func ensureArtifactPVC(
	ctx context.Context,
	kubeClient kubernetes.Interface,
	name, namespace string,
	ownerRef metav1.OwnerReference,
) (*corev1.PersistentVolumeClaim, error) {
	pvcName := fmt.Sprintf("%s-artifacts", name)

	// Check if PVC already exists
	existingPVC, err := kubeClient.CoreV1().PersistentVolumeClaims(namespace).Get(ctx, pvcName, metav1.GetOptions{})
	if err == nil {
		klog.V(2).InfoS("Artifact PVC already exists", "pvc", pvcName, "namespace", namespace)
		return existingPVC, nil
	}

	if !errors.IsNotFound(err) {
		return nil, fmt.Errorf("failed to check for existing PVC: %w", err)
	}

	// Create new PVC
	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      pvcName,
			Namespace: namespace,
			Labels: map[string]string{
				"app":                        "forge",
				"forge.dev/artifact-storage": "true",
				"forge.dev/managed-by":       "forge-controller",
			},
			OwnerReferences: []metav1.OwnerReference{ownerRef},
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{
				corev1.ReadWriteOnce,
			},
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: resource.MustParse(DefaultArtifactStorageSize),
				},
			},
		},
	}

	createdPVC, err := kubeClient.CoreV1().PersistentVolumeClaims(namespace).Create(ctx, pvc, metav1.CreateOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to create artifact PVC: %w", err)
	}

	klog.InfoS("Created artifact PVC", "pvc", pvcName, "namespace", namespace, "size", DefaultArtifactStorageSize)
	return createdPVC, nil
}
