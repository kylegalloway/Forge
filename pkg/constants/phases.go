// Package constants provides status phase values for job and operation states.
//
// Phases represent the current state of a job or operation:
//   - Pending: Not yet started
//   - Running: Currently executing
//   - Completed: Successfully finished
//   - Failed: Terminated with an error
//   - Retrying: Failed but will be retried
//
// These constants should be used consistently across controllers, monitors,
// and CLI tools when checking or setting status phases.
package constants

const (
	// PhasePending indicates a job or operation has not yet started.
	PhasePending = "Pending"

	// PhaseRunning indicates a job or operation is currently executing.
	PhaseRunning = "Running"

	// PhaseCompleted indicates a job or operation finished successfully.
	PhaseCompleted = "Completed"

	// PhaseFailed indicates a job or operation terminated with an error.
	PhaseFailed = "Failed"

	// PhaseRetrying indicates a job or operation failed but will be retried.
	PhaseRetrying = "Retrying"

	// PhaseQueued indicates a resource has been accepted but is waiting for capacity.
	PhaseQueued = "Queued"
)
