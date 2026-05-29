package constants

const (
	// ConditionTypeReady indicates the overall job completed successfully.
	// True=succeeded, False=failed terminally, Unknown=in progress.
	ConditionTypeReady = "Ready"

	// ConditionTypeReconciling indicates the controller is actively working on the job.
	ConditionTypeReconciling = "Reconciling"

	// ConditionTypeBuildSucceeded reflects the build operation outcome.
	ConditionTypeBuildSucceeded = "BuildSucceeded"

	// ConditionTypeCreateSucceeded reflects the UDS create operation outcome.
	ConditionTypeCreateSucceeded = "CreateSucceeded"

	// ConditionTypePublishSucceeded reflects the publish operation outcome.
	ConditionTypePublishSucceeded = "PublishSucceeded"

	// ConditionTypeDeploySucceeded reflects the deploy operation outcome.
	ConditionTypeDeploySucceeded = "DeploySucceeded"
)

const (
	// ConditionReasonSucceeded is used when an operation or job completed successfully.
	ConditionReasonSucceeded = "Succeeded"
	// ConditionReasonFailed is used when an operation or job terminated with an error.
	ConditionReasonFailed = "Failed"
	// ConditionReasonProgressing is used while an operation or job is in flight.
	ConditionReasonProgressing = "Progressing"
	// ConditionReasonSuspended is reserved for when spec.suspend is added.
	// Not yet wired; defined here so the constant is stable when that feature lands.
	ConditionReasonSuspended = "Suspended"
)

// OperationConditionTypes maps OperationStatus JSON field names to condition types.
// Keys must match the json tags on the status types exactly — renaming a status
// field requires updating this list in sync.
// Slice order determines the order conditions appear in the status output.
var OperationConditionTypes = []struct {
	Field    string
	CondType string
}{
	{"buildStatus", ConditionTypeBuildSucceeded},
	{"createStatus", ConditionTypeCreateSucceeded},
	{"publishStatus", ConditionTypePublishSucceeded},
	{"deployStatus", ConditionTypeDeploySucceeded},
}
