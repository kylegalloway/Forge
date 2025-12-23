// Package util provides common utility functions used across the codebase.
package util

import "k8s.io/apimachinery/pkg/api/resource"

// Ptr returns a pointer to the given value.
// This is useful for inline pointer conversions, especially when constructing
// Kubernetes API objects that require pointer fields.
func Ptr[T any](v T) *T {
	return &v
}

// MustParseQuantity parses a resource quantity string or panics.
// This is intended for use with compile-time constants where parsing
// should always succeed. For user input, use resource.ParseQuantity instead.
func MustParseQuantity(quantityStr string) resource.Quantity {
	return resource.MustParse(quantityStr)
}
