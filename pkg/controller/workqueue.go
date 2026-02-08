package controller

// ReconcileRequest identifies a resource to reconcile by namespace and name.
type ReconcileRequest struct {
	Namespace string
	Name      string
}

// keyFunc returns a namespace/name string key for a ReconcileRequest.
func keyFunc(namespace, name string) string {
	if namespace == "" {
		return name
	}
	return namespace + "/" + name
}
