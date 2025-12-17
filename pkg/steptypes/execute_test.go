package steptypes

import (
	"testing"

	"github.com/bgrewell/dart/internal/formatters"
	"github.com/bgrewell/dart/pkg/nodetypes"
	"github.com/stretchr/testify/assert"
)

// TestExecuteStepSingleCommand verifies execution of a single command.
func TestExecuteStepSingleCommand(t *testing.T) {
	mockNode := nodetypes.NewMockNode()
	mockNode.SetResponse("echo hello", 0, "hello\n", "")

	step := &ExecuteStep{
		BaseStep: BaseStep{title: "Single Command Test"},
		node:     mockNode,
		commands: []string{"echo hello"},
	}

	updater := formatters.NewMockTaskCompleter()
	err := step.Run(updater)

	assert.NoError(t, err)
	assert.True(t, updater.IsCompleted())
}

// TestExecuteStepMultipleCommands verifies execution of multiple commands sequentially.
func TestExecuteStepMultipleCommands(t *testing.T) {
	mockNode := nodetypes.NewMockNode()
	mockNode.SetResponse("echo first", 0, "first\n", "")
	mockNode.SetResponse("echo second", 0, "second\n", "")
	mockNode.SetResponse("echo third", 0, "third\n", "")

	step := &ExecuteStep{
		BaseStep: BaseStep{title: "Multiple Commands Test"},
		node:     mockNode,
		commands: []string{"echo first", "echo second", "echo third"},
	}

	updater := formatters.NewMockTaskCompleter()
	err := step.Run(updater)

	assert.NoError(t, err)
	assert.True(t, updater.IsCompleted())
}

// TestExecuteStepFailsOnFirstError verifies that execution stops on the first failing command.
func TestExecuteStepFailsOnFirstError(t *testing.T) {
	mockNode := nodetypes.NewMockNode()
	mockNode.SetResponse("echo first", 0, "first\n", "")
	mockNode.SetResponse("failing command", 1, "", "error message")
	mockNode.SetResponse("echo third", 0, "third\n", "")

	step := &ExecuteStep{
		BaseStep: BaseStep{title: "Fail on First Error Test"},
		node:     mockNode,
		commands: []string{"echo first", "failing command", "echo third"},
	}

	updater := formatters.NewMockTaskCompleter()
	err := step.Run(updater)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "exit code 1")
	assert.True(t, updater.IsErrored())
}

// TestExecuteStepEmptyCommands verifies that an empty command list succeeds.
func TestExecuteStepEmptyCommands(t *testing.T) {
	mockNode := nodetypes.NewMockNode()

	step := &ExecuteStep{
		BaseStep: BaseStep{title: "Empty Commands Test"},
		node:     mockNode,
		commands: []string{},
	}

	updater := formatters.NewMockTaskCompleter()
	err := step.Run(updater)

	assert.NoError(t, err)
	assert.True(t, updater.IsCompleted())
}
