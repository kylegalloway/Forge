// Package constants provides status field names and keys for job resources.
//
// Status field names identify which operation's status is being accessed:
//   - StatusFieldBuild: Build operation status
//   - StatusFieldCreate: Create operation status
//   - StatusFieldPublish: Publish operation status
//   - StatusFieldDeploy: Deploy operation status
//
// Status keys are used within operation status objects to store specific values
// like state, message, retry information, and artifact locations.
package constants

const (
	// StatusFieldBuild is the field name for build operation status.
	StatusFieldBuild = "buildStatus"

	// StatusFieldCreate is the field name for create operation status.
	StatusFieldCreate = "createStatus"

	// StatusFieldPublish is the field name for publish operation status.
	StatusFieldPublish = "publishStatus"

	// StatusFieldDeploy is the field name for deploy operation status.
	StatusFieldDeploy = "deployStatus"

	// StatusKeyState is the key for the phase field in operation status.
	// Maps to the JSON field "phase" in OperationStatus types.
	StatusKeyState = "phase"

	// StatusKeyMessage is the key for the message value in operation status.
	StatusKeyMessage = "message"

	// StatusKeyRetryCount is the key for the retry count in operation status.
	StatusKeyRetryCount = "retryCount"

	// StatusKeyNextRetryTime is the key for the next retry time in operation status.
	StatusKeyNextRetryTime = "nextRetryTime"

	// StatusKeyLastFailureReason is the key for the last failure reason in operation status.
	StatusKeyLastFailureReason = "lastFailureReason"

	// StatusKeyArtifactLocation is the key for the artifact location in operation status.
	StatusKeyArtifactLocation = "artifactLocation"

	// StatusKeyCompletionTime is the key for the completion time in operation status.
	StatusKeyCompletionTime = "completionTime"
)
