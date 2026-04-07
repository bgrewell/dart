package steptypes

import (
	"errors"
	"testing"

	"github.com/bgrewell/dart/internal/config"
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
			Node: config.NodeReference{"test-node"},
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
			Node: config.NodeReference{"test-node"},
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
			Node: config.NodeReference{"test-node"},
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
	var cfgErr *config.ConfigError
	assert.True(t, errors.As(err, &cfgErr))
	assert.Contains(t, cfgErr.Message, "command must be a string or array of strings")
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
			Node: config.NodeReference{"test-node"},
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
	var cfgErr2 *config.ConfigError
	assert.True(t, errors.As(err, &cfgErr2))
	assert.Contains(t, cfgErr2.Message, "command entry is not a string")
}

// TestCreateStepsFileCreate verifies creating a file_create step.
func TestCreateStepsFileCreate(t *testing.T) {
	mockNode := nodetypes.NewMockNode()
	nodes := map[string]ifaces.Node{
		"test-node": mockNode,
	}

	configs := []*config.StepConfig{
		{
			Name: "Create File",
			Node: config.NodeReference{"test-node"},
			Step: config.StepDetails{
				Type: "file_create",
				Options: map[string]interface{}{
					"path":       "/tmp/test.txt",
					"contents":   "Hello World",
					"overwrite":  true,
					"create_dir": true,
				},
			},
		},
	}

	steps, err := CreateSteps(configs, nodes)

	assert.NoError(t, err)
	assert.Len(t, steps, 1)

	createStep, ok := steps[0].(*FileCreateStep)
	assert.True(t, ok)
	assert.Equal(t, "Create File", createStep.Title())
	assert.Equal(t, "/tmp/test.txt", createStep.filePath)
	assert.Equal(t, "Hello World", createStep.contents)
	assert.True(t, createStep.overwrite)
	assert.True(t, createStep.createDir)
}

// TestCreateStepsFileCreateMissingPath verifies error when path is missing.
func TestCreateStepsFileCreateMissingPath(t *testing.T) {
	mockNode := nodetypes.NewMockNode()
	nodes := map[string]ifaces.Node{
		"test-node": mockNode,
	}

	configs := []*config.StepConfig{
		{
			Name: "Create File No Path",
			Node: config.NodeReference{"test-node"},
			Step: config.StepDetails{
				Type: "file_create",
				Options: map[string]interface{}{
					"contents": "Hello World",
				},
			},
		},
	}

	steps, err := CreateSteps(configs, nodes)

	assert.Nil(t, steps)
	var cfgErr3 *config.ConfigError
	assert.True(t, errors.As(err, &cfgErr3))
	assert.Contains(t, cfgErr3.Message, "file path is required")
}

// TestCreateStepsFileDelete verifies creating a file_delete step.
func TestCreateStepsFileDelete(t *testing.T) {
	mockNode := nodetypes.NewMockNode()
	nodes := map[string]ifaces.Node{
		"test-node": mockNode,
	}

	configs := []*config.StepConfig{
		{
			Name: "Delete File",
			Node: config.NodeReference{"test-node"},
			Step: config.StepDetails{
				Type: "file_delete",
				Options: map[string]interface{}{
					"path":          "/tmp/test.txt",
					"ignore_errors": true,
				},
			},
		},
	}

	steps, err := CreateSteps(configs, nodes)

	assert.NoError(t, err)
	assert.Len(t, steps, 1)

	deleteStep, ok := steps[0].(*FileDeleteStep)
	assert.True(t, ok)
	assert.Equal(t, "Delete File", deleteStep.Title())
	assert.Equal(t, "/tmp/test.txt", deleteStep.filePath)
	assert.True(t, deleteStep.ignoreErrors)
}

// TestCreateStepsFileEdit verifies creating a file_edit step.
func TestCreateStepsFileEdit(t *testing.T) {
	mockNode := nodetypes.NewMockNode()
	nodes := map[string]ifaces.Node{
		"test-node": mockNode,
	}

	configs := []*config.StepConfig{
		{
			Name: "Edit File",
			Node: config.NodeReference{"test-node"},
			Step: config.StepDetails{
				Type: "file_edit",
				Options: map[string]interface{}{
					"path":         "/tmp/test.txt",
					"operation":    "replace",
					"match_type":   "regex",
					"match":        `\d+`,
					"content":      "XXX",
					"use_captures": true,
				},
			},
		},
	}

	steps, err := CreateSteps(configs, nodes)

	assert.NoError(t, err)
	assert.Len(t, steps, 1)

	editStep, ok := steps[0].(*FileEditStep)
	assert.True(t, ok)
	assert.Equal(t, "Edit File", editStep.Title())
	assert.Equal(t, "/tmp/test.txt", editStep.filePath)
	assert.Equal(t, EditReplace, editStep.operation)
	assert.Equal(t, MatchRegex, editStep.matchType)
	assert.Equal(t, `\d+`, editStep.match)
	assert.Equal(t, "XXX", editStep.content)
	assert.True(t, editStep.useCaptures)
}

// TestCreateStepsFileEditInsertByLine verifies creating a file_edit step with line-based insert.
func TestCreateStepsFileEditInsertByLine(t *testing.T) {
	mockNode := nodetypes.NewMockNode()
	nodes := map[string]ifaces.Node{
		"test-node": mockNode,
	}

	configs := []*config.StepConfig{
		{
			Name: "Edit File Insert",
			Node: config.NodeReference{"test-node"},
			Step: config.StepDetails{
				Type: "file_edit",
				Options: map[string]interface{}{
					"path":        "/tmp/test.txt",
					"operation":   "insert",
					"position":    "before",
					"match_type":  "line",
					"line_number": 5,
					"content":     "new line",
				},
			},
		},
	}

	steps, err := CreateSteps(configs, nodes)

	assert.NoError(t, err)
	assert.Len(t, steps, 1)

	editStep, ok := steps[0].(*FileEditStep)
	assert.True(t, ok)
	assert.Equal(t, EditInsert, editStep.operation)
	assert.Equal(t, InsertBefore, editStep.position)
	assert.Equal(t, MatchLine, editStep.matchType)
	assert.Equal(t, 5, editStep.lineNumber)
}

// TestCreateStepsFileEditInvalidOperation verifies error for invalid operation.
func TestCreateStepsFileEditInvalidOperation(t *testing.T) {
	mockNode := nodetypes.NewMockNode()
	nodes := map[string]ifaces.Node{
		"test-node": mockNode,
	}

	configs := []*config.StepConfig{
		{
			Name: "Edit File Invalid Op",
			Node: config.NodeReference{"test-node"},
			Step: config.StepDetails{
				Type: "file_edit",
				Options: map[string]interface{}{
					"path":      "/tmp/test.txt",
					"operation": "invalid",
					"match":     "test",
				},
			},
		},
	}

	steps, err := CreateSteps(configs, nodes)

	assert.Nil(t, steps)
	var cfgErr4 *config.ConfigError
	assert.True(t, errors.As(err, &cfgErr4))
	assert.Contains(t, cfgErr4.Message, "invalid edit operation")
}

// TestCreateStepsFileEditMissingMatch verifies error when match is missing.
func TestCreateStepsFileEditMissingMatch(t *testing.T) {
	mockNode := nodetypes.NewMockNode()
	nodes := map[string]ifaces.Node{
		"test-node": mockNode,
	}

	configs := []*config.StepConfig{
		{
			Name: "Edit File No Match",
			Node: config.NodeReference{"test-node"},
			Step: config.StepDetails{
				Type: "file_edit",
				Options: map[string]interface{}{
					"path":       "/tmp/test.txt",
					"operation":  "replace",
					"match_type": "plain",
					// match is missing
					"content": "replacement",
				},
			},
		},
	}

	steps, err := CreateSteps(configs, nodes)

	assert.Nil(t, steps)
	var cfgErr5 *config.ConfigError
	assert.True(t, errors.As(err, &cfgErr5))
	assert.Contains(t, cfgErr5.Message, "match pattern is required")
}
