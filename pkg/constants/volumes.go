// Package constants provides volume type names for Kubernetes volume sources.
//
// These constants are used when displaying or categorizing volume types
// in CLI output and logs.
package constants

const (
	// VolumeTypePersistentVolumeClaim identifies a PersistentVolumeClaim volume source.
	VolumeTypePersistentVolumeClaim = "PersistentVolumeClaim"

	// VolumeTypeConfigMap identifies a ConfigMap volume source.
	VolumeTypeConfigMap = "ConfigMap"

	// VolumeTypeSecret identifies a Secret volume source.
	VolumeTypeSecret = "Secret" // pragma: allowlist secret

	// VolumeTypeEmptyDir identifies an EmptyDir volume source.
	VolumeTypeEmptyDir = "EmptyDir"

	// VolumeTypeHostPath identifies a HostPath volume source.
	VolumeTypeHostPath = "HostPath"

	// VolumeTypeOther identifies an unknown or unsupported volume source type.
	VolumeTypeOther = "Other"
)
