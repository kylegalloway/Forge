package resources

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ResourceReference identifies a Kubernetes resource
type ResourceReference struct {
	Group     string
	Version   string
	Kind      string
	Namespace string
	Name      string
	OwnerRefs []metav1.OwnerReference
	UID       string
}
