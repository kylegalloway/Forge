// Package leaderelection provides leader election capabilities for high availability controller deployments.
package leaderelection

import (
	"context"
	"os"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
	"k8s.io/klog/v2"
)

const (
	// DefaultLeaseDuration is how long a leader holds the lease
	DefaultLeaseDuration = 15 * time.Second
	// DefaultRenewDeadline is how long the leader tries to renew
	DefaultRenewDeadline = 10 * time.Second
	// DefaultRetryPeriod is how often non-leaders try to acquire
	DefaultRetryPeriod = 2 * time.Second
)

// Config holds leader election configuration
type Config struct {
	// Lock name for the leader election
	LockName string
	// Lock namespace
	LockNamespace string
	// Identity of this instance (defaults to hostname)
	Identity string
	// LeaseDuration is how long a leader holds the lease
	LeaseDuration time.Duration
	// RenewDeadline is how long the leader tries to renew
	RenewDeadline time.Duration
	// RetryPeriod is how often non-leaders try to acquire
	RetryPeriod time.Duration
}

// DefaultConfig returns a default leader election config
func DefaultConfig() *Config {
	hostname, err := os.Hostname()
	if err != nil {
		hostname = "unknown"
	}

	return &Config{
		LockName:      "forge-controller-lock",
		LockNamespace: "forge-system",
		Identity:      hostname,
		LeaseDuration: DefaultLeaseDuration,
		RenewDeadline: DefaultRenewDeadline,
		RetryPeriod:   DefaultRetryPeriod,
	}
}

// RunWithLeaderElection runs the provided function with leader election
// The function will only run on the elected leader
func RunWithLeaderElection(ctx context.Context, client kubernetes.Interface, config *Config, run func(context.Context)) error {
	if config == nil {
		config = DefaultConfig()
	}

	// Create the resource lock for leader election
	lock := &resourcelock.LeaseLock{
		LeaseMeta: metav1.ObjectMeta{
			Name:      config.LockName,
			Namespace: config.LockNamespace,
		},
		Client: client.CoordinationV1(),
		LockConfig: resourcelock.ResourceLockConfig{
			Identity: config.Identity,
		},
	}

	// Start leader election
	leaderelection.RunOrDie(ctx, leaderelection.LeaderElectionConfig{
		Lock:            lock,
		ReleaseOnCancel: true,
		LeaseDuration:   config.LeaseDuration,
		RenewDeadline:   config.RenewDeadline,
		RetryPeriod:     config.RetryPeriod,
		Callbacks: leaderelection.LeaderCallbacks{
			OnStartedLeading: func(ctx context.Context) {
				klog.InfoS("Started leading", "identity", config.Identity)
				run(ctx)
			},
			OnStoppedLeading: func() {
				klog.InfoS("Stopped leading", "identity", config.Identity)
			},
			OnNewLeader: func(identity string) {
				if identity == config.Identity {
					klog.InfoS("I am the new leader", "identity", identity)
				} else {
					klog.InfoS("New leader elected", "leader", identity, "identity", config.Identity)
				}
			},
		},
	})

	return nil
}
