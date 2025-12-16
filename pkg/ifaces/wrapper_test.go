package ifaces_test

import (
	"testing"

	"github.com/bgrewell/dart/pkg/ifaces"
	"github.com/stretchr/testify/assert"
)

// TestNoOpWrapper verifies the NoOpWrapper implements EnvironmentWrapper correctly
func TestNoOpWrapper(t *testing.T) {
	var wrapper ifaces.EnvironmentWrapper = &ifaces.NoOpWrapper{}

	assert.False(t, wrapper.Configured(), "NoOpWrapper should not be configured")
	assert.NoError(t, wrapper.Setup(), "NoOpWrapper.Setup should not return error")
	assert.NoError(t, wrapper.Teardown(), "NoOpWrapper.Teardown should not return error")
}

// TestEnvironmentWrapperInterface verifies the interface is properly defined
func TestEnvironmentWrapperInterface(t *testing.T) {
	// This test just ensures the interface can be used in type assertions
	var _ ifaces.EnvironmentWrapper = &ifaces.NoOpWrapper{}
}
