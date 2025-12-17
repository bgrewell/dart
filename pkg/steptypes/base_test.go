package steptypes

import (
	"testing"

	"github.com/bgrewell/dart/internal/config"
	"github.com/bgrewell/dart/internal/helpers"
	"github.com/bgrewell/dart/pkg/ifaces"
	"github.com/bgrewell/dart/pkg/nodetypes"
	"github.com/stretchr/testify/assert"
)

// TestCreateStepsExecuteSingleCommand verifies creating an execute step with a single command string.
func TestCreateStepsExecuteSingleCommand(t *testing.T) {
	mockNode := nodetypes.NewMockNode()
	nodes := map[string]ifaces.Node{
		"test-node": mockNode,
	}

	configs := []*config.StepConfig{
		{
			Name: "Single Command",
			Node: "test-node",
			Step: config.StepDetails{
				Type: "execute",
				Options: map[string]interface{}{
					"command": "echo hello",
				},
			},
		},
	}

	steps, err := CreateSteps(configs, nodes)

	assert.NoError(t, err)
	assert.Len(t, steps, 1)

	execStep, ok := steps[0].(*ExecuteStep)
	assert.True(t, ok)
	assert.Equal(t, "Single Command", execStep.Title())
	assert.Equal(t, []string{"echo hello"}, execStep.commands)
}

// TestCreateStepsExecuteMultipleCommands verifies creating an execute step with an array of commands.
func TestCreateStepsExecuteMultipleCommands(t *testing.T) {
	mockNode := nodetypes.NewMockNode()
	nodes := map[string]ifaces.Node{
		"test-node": mockNode,
	}

	configs := []*config.StepConfig{
		{
			Name: "Multiple Commands",
			Node: "test-node",
			Step: config.StepDetails{
				Type: "execute",
				Options: map[string]interface{}{
					"command": []interface{}{"echo first", "echo second", "echo third"},
				},
			},
		},
	}

	steps, err := CreateSteps(configs, nodes)

	assert.NoError(t, err)
	assert.Len(t, steps, 1)

	execStep, ok := steps[0].(*ExecuteStep)
	assert.True(t, ok)
	assert.Equal(t, "Multiple Commands", execStep.Title())
	assert.Equal(t, []string{"echo first", "echo second", "echo third"}, execStep.commands)
}

// TestCreateStepsExecuteInvalidCommandType verifies error handling for invalid command types.
func TestCreateStepsExecuteInvalidCommandType(t *testing.T) {
	mockNode := nodetypes.NewMockNode()
	nodes := map[string]ifaces.Node{
		"test-node": mockNode,
	}

	configs := []*config.StepConfig{
		{
			Name: "Invalid Command",
			Node: "test-node",
			Step: config.StepDetails{
				Type: "execute",
				Options: map[string]interface{}{
					"command": 12345, // Invalid type
				},
			},
		},
	}

	steps, err := CreateSteps(configs, nodes)

	assert.Nil(t, steps)
	assert.ErrorIs(t, err, helpers.ErrInvalidCommandType)
}

// TestCreateStepsExecuteNonStringInArray verifies error handling when array contains non-string values.
func TestCreateStepsExecuteNonStringInArray(t *testing.T) {
	mockNode := nodetypes.NewMockNode()
	nodes := map[string]ifaces.Node{
		"test-node": mockNode,
	}

	configs := []*config.StepConfig{
		{
			Name: "Invalid Array Command",
			Node: "test-node",
			Step: config.StepDetails{
				Type: "execute",
				Options: map[string]interface{}{
					"command": []interface{}{"echo hello", 12345}, // Contains non-string
				},
			},
		},
	}

	steps, err := CreateSteps(configs, nodes)

	assert.Nil(t, steps)
	assert.ErrorIs(t, err, helpers.ErrCommandNotString)
}
