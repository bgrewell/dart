package steptypes

import (
	"errors"
	"testing"

	"github.com/bgrewell/dart/pkg/nodetypes"

	"github.com/bgrewell/dart/internal/formatters"
	"github.com/stretchr/testify/assert"
)

// TestServiceCheckStep verifies service status checking.
func TestServiceCheckStep(t *testing.T) {
	mockNode := nodetypes.NewMockNode()
	mockNode.SetResponse("systemctl is-active 'nginx'", 0, "active\n", "")

	step := &ServiceCheckStep{
		BaseStep: BaseStep{title: "Service Check"},
		node:     mockNode,
		service:  "nginx",
	}

	updater := formatters.NewMockTaskCompleter()
	err := step.Run(updater)

	assert.NoError(t, err)
	assert.True(t, updater.IsCompleted())
}

// TestServiceCheckStepFailure verifies handling when service is inactive.
func TestServiceCheckStepFailure(t *testing.T) {
	mockNode := nodetypes.NewMockNode()
	mockNode.SetResponse("systemctl is-active 'nginx'", 3, "inactive\n", "")

	step := &ServiceCheckStep{
		BaseStep: BaseStep{title: "Service Check"},
		node:     mockNode,
		service:  "nginx",
	}

	updater := formatters.NewMockTaskCompleter()
	err := step.Run(updater)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "service nginx is not active")
	assert.Contains(t, err.Error(), "inactive")
}

// TestServiceCheckStepError verifies error handling.
func TestServiceCheckStepError(t *testing.T) {
	mockNode := nodetypes.NewMockNode()
	mockNode.SetError("systemctl is-active 'nginx'", errors.New("execution error"))

	step := &ServiceCheckStep{
		BaseStep: BaseStep{title: "Service Check"},
		node:     mockNode,
		service:  "nginx",
	}

	updater := formatters.NewMockTaskCompleter()
	err := step.Run(updater)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to check service")
}
