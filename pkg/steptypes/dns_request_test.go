package steptypes

import (
	"testing"

	"github.com/bgrewell/dart/internal/formatters"
	"github.com/stretchr/testify/assert"
)

// TestDNSRequestStep verifies DNS resolution.
func TestDNSRequestStep(t *testing.T) {
	step := &DNSRequestStep{
		BaseStep:    BaseStep{title: "DNS Test"},
		hostname:    "localhost",
		expectedIPs: []string{"127.0.0.1"},
	}

	// Run step
	updater := formatters.NewMockTaskCompleter()
	err := step.Run(updater)

	// Validate DNS resolution
	assert.NoError(t, err)
}
