package steptypes

import (
	"testing"

	"github.com/bgrewell/dart/internal/formatters"
	"github.com/bgrewell/dart/pkg/nodetypes"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestFileDeleteStep verifies basic file deletion on the node.
func TestFileDeleteStep(t *testing.T) {
	node := nodetypes.NewMockNode()
	node.SeedFile("/etc/test_file_delete.txt", []byte("Test content"), 0644)

	step := &FileDeleteStep{
		BaseStep: BaseStep{title: "Delete Test File"},
		node:     node,
		filePath: "/etc/test_file_delete.txt",
	}

	updater := formatters.NewMockTaskCompleter()
	require.NoError(t, step.Run(updater))

	_, ok := node.GetFile("/etc/test_file_delete.txt")
	assert.False(t, ok, "expected file to be removed from the node")
	assert.True(t, updater.IsCompleted())
}

// TestFileDeleteStepNotExists verifies error when file doesn't exist.
func TestFileDeleteStepNotExists(t *testing.T) {
	node := nodetypes.NewMockNode()

	step := &FileDeleteStep{
		BaseStep: BaseStep{title: "Delete Non-existent File"},
		node:     node,
		filePath: "/etc/test_file_delete_missing.txt",
	}

	updater := formatters.NewMockTaskCompleter()
	err := step.Run(updater)

	assert.Error(t, err)
	assert.True(t, updater.IsErrored())
}

// TestFileDeleteStepIgnoreErrors verifies ignore_errors option.
func TestFileDeleteStepIgnoreErrors(t *testing.T) {
	node := nodetypes.NewMockNode()

	step := &FileDeleteStep{
		BaseStep:     BaseStep{title: "Delete Non-existent File Ignore"},
		node:         node,
		filePath:     "/etc/test_file_delete_ignore.txt",
		ignoreErrors: true,
	}

	updater := formatters.NewMockTaskCompleter()
	require.NoError(t, step.Run(updater))
	assert.True(t, updater.IsCompleted())
}
