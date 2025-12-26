// Package constants provides label keys and values for Kubernetes resource identification.
//
// Labels are used to:
//   - Identify Jobs and Pods created by Forge controllers
//   - Filter resources during Job monitoring and cleanup
//   - Distinguish between Zarf and UDS operations
//   - Track package names and action types in metrics
//
// Example usage:
//
//	labels := map[string]string{
//	    constants.LabelApp:     constants.LabelAppValueZarf,
//	    constants.LabelPackage: pkg.Name,
//	    constants.LabelAction:  constants.ActionBuild,
//	}
package constants

const (
	// LabelApp is the app label for Zarf package Jobs
	LabelApp = "app"
	// LabelAppValueZarf is the value for the app label for Zarf Jobs
	LabelAppValueZarf = "forge"
	// LabelAppValueUDS is the value for the app label for UDS bundle Jobs
	LabelAppValueUDS = "forge-uds"

	// LabelPackage is the label key for the package name
	LabelPackage = "forge.dev/package"
	// LabelAction is the label key for the action type
	LabelAction = "forge.dev/action"
	// LabelJobType is the label key for the job type
	LabelJobType = "forge.dev/job-type"
)
