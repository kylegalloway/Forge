package constants

import "time"

const (
	// JobMonitorInterval is how often to check Job statuses
	JobMonitorInterval = 10 * time.Second

	// Default timeout values (in seconds)
	DefaultBuildTimeout   = 3600 // 1 hour
	DefaultPublishTimeout = 1800 // 30 minutes
	DefaultDeployTimeout  = 1800 // 30 minutes
	DefaultCreateTimeout  = 3600 // 1 hour for UDS bundle creation

	// Security context UIDs
	DefaultZarfUID = 1000
	DefaultUDSUID  = 65532
)
