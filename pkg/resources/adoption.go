// Package resources provides resource discovery and adoption functionality for Kubernetes resources.
package resources

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/klog/v2"
)

// Adopter manages resource adoption by adding OwnerReferences
type Adopter struct {
	dynamicClient dynamic.Interface
}

// NewAdopter creates a new resource adopter
func NewAdopter(dynamicClient dynamic.Interface) *Adopter {
	return &Adopter{
		dynamicClient: dynamicClient,
	}
}

// AdoptResources adds OwnerReferences to existing resources to establish ownership
func (a *Adopter) AdoptResources(ctx context.Context, owner metav1.Object, ownerGVK schema.GroupVersionKind, resources []ResourceReference, validateOwnership bool) error {
	if len(resources) == 0 {
		klog.V(4).InfoS("No resources to adopt")
		return nil
	}

	// Validate no conflicting owners if requested
	if validateOwnership {
		if err := a.ValidateNoConflictingOwners(resources); err != nil {
			return fmt.Errorf("ownership validation failed: %w", err)
		}
	}

	klog.InfoS("Adopting resources", "count", len(resources), "owner", owner.GetName())

	// Create OwnerReference for the adopting resource
	ownerRef := metav1.OwnerReference{
		APIVersion: ownerGVK.GroupVersion().String(),
		Kind:       ownerGVK.Kind,
		Name:       owner.GetName(),
		UID:        owner.GetUID(),
		Controller: boolPtr(true),
	}

	// Add OwnerReference to each resource
	var errors []error
	for _, ref := range resources {
		gvr := schema.GroupVersionResource{
			Group:   ref.Group,
			Version: ref.Version,
			// Kind is not used in GVR, need to determine Resource name
			// For simplicity, lowercase Kind and add 's' (works for most cases)
			Resource: kindToResource(ref.Kind),
		}

		// Get the resource
		resource, err := a.dynamicClient.Resource(gvr).Namespace(ref.Namespace).Get(ctx, ref.Name, metav1.GetOptions{})
		if err != nil {
			errors = append(errors, fmt.Errorf("failed to get %s/%s in %s: %w", ref.Kind, ref.Name, ref.Namespace, err))
			continue
		}

		// Add OwnerReference if not already present
		ownerRefs := resource.GetOwnerReferences()
		if !hasOwnerReference(ownerRefs, ownerRef) {
			ownerRefs = append(ownerRefs, ownerRef)
			resource.SetOwnerReferences(ownerRefs)

			// Update the resource
			_, err = a.dynamicClient.Resource(gvr).Namespace(ref.Namespace).Update(ctx, resource, metav1.UpdateOptions{})
			if err != nil {
				errors = append(errors, fmt.Errorf("failed to update %s/%s in %s: %w", ref.Kind, ref.Name, ref.Namespace, err))
				continue
			}

			klog.InfoS("Adopted resource", "kind", ref.Kind, "name", ref.Name, "namespace", ref.Namespace)
		} else {
			klog.V(4).InfoS("Resource already has owner reference", "kind", ref.Kind, "name", ref.Name)
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("failed to adopt %d resources: %v", len(errors), errors)
	}

	return nil
}

// ValidateNoConflictingOwners checks that resources don't have conflicting controller owners
func (a *Adopter) ValidateNoConflictingOwners(resources []ResourceReference) error {
	for _, ref := range resources {
		if hasControllerOwner(ref.OwnerRefs) {
			return fmt.Errorf("resource %s/%s in %s already has a controller owner", ref.Kind, ref.Name, ref.Namespace)
		}
	}
	return nil
}

// hasControllerOwner checks if any OwnerReference has Controller=true
func hasControllerOwner(ownerRefs []metav1.OwnerReference) bool {
	for _, ref := range ownerRefs {
		if ref.Controller != nil && *ref.Controller {
			return true
		}
	}
	return false
}

// hasOwnerReference checks if an OwnerReference already exists in the list
func hasOwnerReference(ownerRefs []metav1.OwnerReference, target metav1.OwnerReference) bool {
	for _, ref := range ownerRefs {
		if ref.UID == target.UID && ref.Kind == target.Kind && ref.Name == target.Name {
			return true
		}
	}
	return false
}

// kindToResource converts a Kind to a resource name (simple pluralization)
// This is a simplified version - production code might use REST mapper
func kindToResource(kind string) string {
	// Special cases
	switch kind {
	case "Ingress":
		return "ingresses"
	case "Endpoints":
		return "endpoints"
	default:
		// Simple pluralization: lowercase and add 's'
		return kind + "s"
	}
}

// boolPtr returns a pointer to a bool value
func boolPtr(b bool) *bool {
	return &b
}
