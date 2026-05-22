package steptypes

import (
	"testing"

	"github.com/bgrewell/dart/internal/formatters"
	"github.com/bgrewell/dart/pkg/nodetypes"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestFileWriteStep verifies writing a file to the node.
func TestFileWriteStep(t *testing.T) {
	node := nodetypes.NewMockNode()

	step := &FileWriteStep{
		BaseStep:  BaseStep{title: "Write Test File"},
		node:      node,
		filePath:  "/etc/test_file_write.txt",
		contents:  "Hello World",
		overwrite: true,
	}

	updater := formatters.NewMockTaskCompleter()
	require.NoError(t, step.Run(updater))

	got, ok := node.GetFile("/etc/test_file_write.txt")
	require.True(t, ok)
	assert.Equal(t, "Hello World", string(got))
}

// TestFileWriteStepNoOverwrite verifies the no-overwrite guard.
func TestFileWriteStepNoOverwrite(t *testing.T) {
	node := nodetypes.NewMockNode()
	node.SeedFile("/etc/test_file_write_exists.txt", []byte("Initial"), 0644)

	step := &FileWriteStep{
		BaseStep:  BaseStep{title: "Write Existing No Overwrite"},
		node:      node,
		filePath:  "/etc/test_file_write_exists.txt",
		contents:  "Hello World",
		overwrite: false,
	}

	updater := formatters.NewMockTaskCompleter()
	err := step.Run(updater)
	assert.Error(t, err)
	assert.True(t, updater.IsErrored())
}
