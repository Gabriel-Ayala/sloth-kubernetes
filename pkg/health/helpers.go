// Package health provides cluster health checking functionality
// This file contains helper functions used by health checks.
package health

import (
	"strings"
)

// contains checks if a string contains a substring
// Returns false for empty substrings
func contains(s, substr string) bool {
	if substr == "" {
		return false
	}
	return strings.Contains(s, substr)
}

// containsMiddle checks if a string contains a substring in the middle
// (not at the beginning or end)
func containsMiddle(s, substr string) bool {
	if !strings.Contains(s, substr) {
		return false
	}
	idx := strings.Index(s, substr)
	return idx > 0 && idx+len(substr) < len(s)
}

// isRecoverableError checks if an error is recoverable (non-fatal)
// All errors are considered recoverable by default to allow retry logic
func isRecoverableError(err error) bool {
	if err == nil {
		return false
	}
	// All errors are considered recoverable to support retry logic
	return true
}
