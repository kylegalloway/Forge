package controller

import (
	"fmt"

	batchv1 "k8s.io/api/batch/v1"
	"k8s.io/client-go/tools/cache"
)

// ConcurrencyConfig holds configuration for job concurrency limits.
type ConcurrencyConfig struct {
	// MaxConcurrentJobsPerNamespace limits concurrent jobs per namespace.
	// 0 means unlimited.
	MaxConcurrentJobsPerNamespace int

	// MaxConcurrentJobsGlobal limits total concurrent jobs across all namespaces.
	// 0 means unlimited.
	MaxConcurrentJobsGlobal int
}

// ConcurrencyLimiter checks whether new jobs can be created based on
// configured concurrency limits and current active job counts from the
// informer cache.
type ConcurrencyLimiter struct {
	config    ConcurrencyConfig
	jobStores []cache.Store
}

// NewConcurrencyLimiter creates a new ConcurrencyLimiter that reads active job
// counts from the provided informer cache stores.
func NewConcurrencyLimiter(config ConcurrencyConfig, jobStores ...cache.Store) *ConcurrencyLimiter {
	return &ConcurrencyLimiter{
		config:    config,
		jobStores: jobStores,
	}
}

// CanCreateJob checks whether a new job can be created in the given namespace
// without exceeding concurrency limits. Returns true if allowed, or false with
// a reason message if at capacity.
func (cl *ConcurrencyLimiter) CanCreateJob(namespace string) (bool, string) {
	if cl.config.MaxConcurrentJobsPerNamespace == 0 && cl.config.MaxConcurrentJobsGlobal == 0 {
		return true, ""
	}

	var activeInNamespace, activeGlobal int

	for _, store := range cl.jobStores {
		for _, item := range store.List() {
			job, ok := item.(*batchv1.Job)
			if !ok {
				continue
			}
			if isJobActive(job) {
				activeGlobal++
				if job.Namespace == namespace {
					activeInNamespace++
				}
			}
		}
	}

	if cl.config.MaxConcurrentJobsPerNamespace > 0 && activeInNamespace >= cl.config.MaxConcurrentJobsPerNamespace {
		return false, fmt.Sprintf("namespace %s at capacity: %d/%d concurrent jobs",
			namespace, activeInNamespace, cl.config.MaxConcurrentJobsPerNamespace)
	}

	if cl.config.MaxConcurrentJobsGlobal > 0 && activeGlobal >= cl.config.MaxConcurrentJobsGlobal {
		return false, fmt.Sprintf("global capacity reached: %d/%d concurrent jobs",
			activeGlobal, cl.config.MaxConcurrentJobsGlobal)
	}

	return true, ""
}

// isJobActive returns true if the job is not in a terminal state
// (i.e., it has no Complete or Failed condition set to True).
func isJobActive(job *batchv1.Job) bool {
	for _, c := range job.Status.Conditions {
		if (c.Type == batchv1.JobComplete || c.Type == batchv1.JobFailed) &&
			c.Status == "True" {
			return false
		}
	}
	return true
}
