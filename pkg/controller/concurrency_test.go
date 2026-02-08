package controller

import (
	"testing"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"
)

func TestCanCreateJob_Unlimited(t *testing.T) {
	store := cache.NewStore(cache.MetaNamespaceKeyFunc)
	limiter := NewConcurrencyLimiter(ConcurrencyConfig{}, store)

	allowed, reason := limiter.CanCreateJob("default")
	if !allowed {
		t.Errorf("Expected unlimited config to always allow, got denied: %s", reason)
	}
}

func TestCanCreateJob_BelowPerNamespaceLimit(t *testing.T) {
	store := cache.NewStore(cache.MetaNamespaceKeyFunc)

	// Add 1 active job
	_ = store.Add(makeActiveJob("job-1", "default"))

	limiter := NewConcurrencyLimiter(ConcurrencyConfig{MaxConcurrentJobsPerNamespace: 3}, store)

	allowed, reason := limiter.CanCreateJob("default")
	if !allowed {
		t.Errorf("Expected allowed below limit, got denied: %s", reason)
	}
}

func TestCanCreateJob_AtPerNamespaceLimit(t *testing.T) {
	store := cache.NewStore(cache.MetaNamespaceKeyFunc)

	// Add 2 active jobs
	_ = store.Add(makeActiveJob("job-1", "default"))
	_ = store.Add(makeActiveJob("job-2", "default"))

	limiter := NewConcurrencyLimiter(ConcurrencyConfig{MaxConcurrentJobsPerNamespace: 2}, store)

	allowed, _ := limiter.CanCreateJob("default")
	if allowed {
		t.Error("Expected denied at per-namespace limit")
	}
}

func TestCanCreateJob_AbovePerNamespaceLimit(t *testing.T) {
	store := cache.NewStore(cache.MetaNamespaceKeyFunc)

	_ = store.Add(makeActiveJob("job-1", "default"))
	_ = store.Add(makeActiveJob("job-2", "default"))
	_ = store.Add(makeActiveJob("job-3", "default"))

	limiter := NewConcurrencyLimiter(ConcurrencyConfig{MaxConcurrentJobsPerNamespace: 2}, store)

	allowed, reason := limiter.CanCreateJob("default")
	if allowed {
		t.Error("Expected denied above per-namespace limit")
	}
	if reason == "" {
		t.Error("Expected a reason when denied")
	}
}

func TestCanCreateJob_BelowGlobalLimit(t *testing.T) {
	store := cache.NewStore(cache.MetaNamespaceKeyFunc)

	_ = store.Add(makeActiveJob("job-1", "ns-a"))
	_ = store.Add(makeActiveJob("job-2", "ns-b"))

	limiter := NewConcurrencyLimiter(ConcurrencyConfig{MaxConcurrentJobsGlobal: 5}, store)

	allowed, reason := limiter.CanCreateJob("ns-a")
	if !allowed {
		t.Errorf("Expected allowed below global limit, got denied: %s", reason)
	}
}

func TestCanCreateJob_AtGlobalLimit(t *testing.T) {
	store := cache.NewStore(cache.MetaNamespaceKeyFunc)

	_ = store.Add(makeActiveJob("job-1", "ns-a"))
	_ = store.Add(makeActiveJob("job-2", "ns-b"))
	_ = store.Add(makeActiveJob("job-3", "ns-c"))

	limiter := NewConcurrencyLimiter(ConcurrencyConfig{MaxConcurrentJobsGlobal: 3}, store)

	allowed, _ := limiter.CanCreateJob("ns-a")
	if allowed {
		t.Error("Expected denied at global limit")
	}
}

func TestCanCreateJob_CompletedJobsNotCounted(t *testing.T) {
	store := cache.NewStore(cache.MetaNamespaceKeyFunc)

	// 2 completed, 1 active
	_ = store.Add(makeCompletedJob("job-1", "default"))
	_ = store.Add(makeFailedJob("job-2", "default"))
	_ = store.Add(makeActiveJob("job-3", "default"))

	limiter := NewConcurrencyLimiter(ConcurrencyConfig{MaxConcurrentJobsPerNamespace: 2}, store)

	allowed, reason := limiter.CanCreateJob("default")
	if !allowed {
		t.Errorf("Expected allowed since completed/failed jobs don't count, got denied: %s", reason)
	}
}

func TestCanCreateJob_MultipleNamespaces(t *testing.T) {
	store := cache.NewStore(cache.MetaNamespaceKeyFunc)

	// 2 jobs in ns-a, 1 job in ns-b
	_ = store.Add(makeActiveJob("job-1", "ns-a"))
	_ = store.Add(makeActiveJob("job-2", "ns-a"))
	_ = store.Add(makeActiveJob("job-3", "ns-b"))

	limiter := NewConcurrencyLimiter(ConcurrencyConfig{MaxConcurrentJobsPerNamespace: 2}, store)

	// ns-a is at limit
	allowed, _ := limiter.CanCreateJob("ns-a")
	if allowed {
		t.Error("Expected ns-a to be at limit")
	}

	// ns-b has room
	allowed, reason := limiter.CanCreateJob("ns-b")
	if !allowed {
		t.Errorf("Expected ns-b to have room, got denied: %s", reason)
	}
}

func TestCanCreateJob_BothLimits(t *testing.T) {
	store := cache.NewStore(cache.MetaNamespaceKeyFunc)

	// 1 job in ns-a, 2 jobs in ns-b (total 3)
	_ = store.Add(makeActiveJob("job-1", "ns-a"))
	_ = store.Add(makeActiveJob("job-2", "ns-b"))
	_ = store.Add(makeActiveJob("job-3", "ns-b"))

	limiter := NewConcurrencyLimiter(ConcurrencyConfig{
		MaxConcurrentJobsPerNamespace: 2,
		MaxConcurrentJobsGlobal:       3,
	}, store)

	// ns-a has room per-namespace but global is at limit
	allowed, _ := limiter.CanCreateJob("ns-a")
	if allowed {
		t.Error("Expected denied because global limit is reached")
	}
}

func TestCanCreateJob_MultipleStores(t *testing.T) {
	storeA := cache.NewStore(cache.MetaNamespaceKeyFunc)
	storeB := cache.NewStore(cache.MetaNamespaceKeyFunc)

	_ = storeA.Add(makeActiveJob("zarf-job-1", "default"))
	_ = storeB.Add(makeActiveJob("uds-job-1", "default"))

	limiter := NewConcurrencyLimiter(ConcurrencyConfig{MaxConcurrentJobsGlobal: 2}, storeA, storeB)

	allowed, _ := limiter.CanCreateJob("default")
	if allowed {
		t.Error("Expected denied with combined store counts at global limit")
	}
}

// Helper functions to create test jobs

func makeActiveJob(name, namespace string) *batchv1.Job {
	return &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Status: batchv1.JobStatus{
			Active: 1,
		},
	}
}

func makeCompletedJob(name, namespace string) *batchv1.Job {
	return &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Status: batchv1.JobStatus{
			Succeeded: 1,
			Conditions: []batchv1.JobCondition{
				{
					Type:   batchv1.JobComplete,
					Status: corev1.ConditionTrue,
				},
			},
		},
	}
}

func makeFailedJob(name, namespace string) *batchv1.Job {
	return &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Status: batchv1.JobStatus{
			Failed: 1,
			Conditions: []batchv1.JobCondition{
				{
					Type:   batchv1.JobFailed,
					Status: corev1.ConditionTrue,
				},
			},
		},
	}
}
