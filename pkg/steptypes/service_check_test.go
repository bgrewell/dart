package steptypes

import (
	"bytes"
	"errors"
	"github.com/bgrewell/dart/internal/execution"
	"github.com/bgrewell/dart/pkg/nodetypes"
	"io"
	"testing"

	"github.com/bgrewell/dart/internal/formatters"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockNode is a mock implementation of the ifaces.Node interface.
type MockNode struct {
	mock.Mock
}

// Execute simulates command execution.
func (m *MockNode) Execute(command string) (*execution.ExecutionResult, error) {
	args := m.Called(command)
	stdout := args.Get(0).(io.Reader)
	return &execution.ExecutionResult{
		Stdout:   stdout,
		Stderr:   new(bytes.Buffer),
		ExitCode: args.Int(1),
	}, args.Error(2)
}

// TestServiceCheckStep verifies service status checking.
func TestServiceCheckStep(t *testing.T) {
	mockNode := &nodetypes.MockNode{}
	mockNode.On("Execute", "systemctl is-active nginx").Return(io.NopCloser(bytes.NewBufferString("active\n")), 0, nil)

	step := &ServiceCheckStep{
		BaseStep: BaseStep{title: "Service Check"},
		node:     mockNode,
		service:  "nginx",
	}

	// Run step
	updater := formatters.NewMockTaskCompleter()
	err := step.Run(updater)

	// Validate service status check
	assert.NoError(t, err)
}

// TestServiceCheckStepFailure verifies handling when service is inactive.
func TestServiceCheckStepFailure(t *testing.T) {
	mockNode := &nodetypes.MockNode{}
	mockNode.On("Execute", "systemctl is-active nginx").Return(io.NopCloser(bytes.NewBufferString("inactive\n")), 3, nil)

	step := &ServiceCheckStep{
		BaseStep: BaseStep{title: "Service Check"},
		node:     mockNode,
		service:  "nginx",
	}

	// Run step
	updater := formatters.NewMockTaskCompleter()
	err := step.Run(updater)

	// Expect failure
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "service nginx is not active")
}

// TestServiceCheckStepError verifies error handling.
func TestServiceCheckStepError(t *testing.T) {
	mockNode := &nodetypes.MockNode{}
	mockNode.On("Execute", "systemctl is-active nginx").Return(io.NopCloser(bytes.NewBufferString("")), 1, errors.New("execution error"))

	step := &ServiceCheckStep{
		BaseStep: BaseStep{title: "Service Check"},
		node:     mockNode,
		service:  "nginx",
	}

	// Run step
	updater := formatters.NewMockTaskCompleter()
	err := step.Run(updater)

	// Expect failure
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to check service")
}
