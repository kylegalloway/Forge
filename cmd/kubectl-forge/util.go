// Package main provides shared utilities for kubectl-forge commands
package main

import (
	"fmt"
	"time"
)

// formatDuration formats a duration in a human-readable way similar to kubectl
// Examples: "5s", "3m", "2h", "7d"
func formatDuration(d time.Duration) string {
	if d < time.Second {
		return "0s"
	}

	d = d.Round(time.Second)

	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	if d < 24*time.Hour {
		return fmt.Sprintf("%dh", int(d.Hours()))
	}
	return fmt.Sprintf("%dd", int(d.Hours()/24))
}
