package steptypes

import (
	"testing"

	"github.com/bgrewell/dart/internal/formatters"
	"github.com/bgrewell/dart/pkg/nodetypes"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestFileCreateStep verifies basic file creation on the node.
func TestFileCreateStep(t *testing.T) {
	node := nodetypes.NewMockNode()

	step := &FileCreateStep{
		BaseStep:  BaseStep{title: "Create Test File"},
		node:      node,
		filePath:  "/etc/test_file_create.txt",
		contents:  "Hello World",
		overwrite: false,
	}

	updater := formatters.NewMockTaskCompleter()
	require.NoError(t, step.Run(updater))

	got, ok := node.GetFile("/etc/test_file_create.txt")
	require.True(t, ok, "expected file to be created on the node")
	assert.Equal(t, "Hello World", string(got))
	assert.True(t, updater.IsCompleted())
}

// TestFileCreateStepOverwrite verifies file creation with overwrite.
func TestFileCreateStepOverwrite(t *testing.T) {
	node := nodetypes.NewMockNode()
	node.SeedFile("/etc/test_file_create_overwrite.txt", []byte("Initial content"), 0644)

	step := &FileCreateStep{
		BaseStep:  BaseStep{title: "Overwrite Test File"},
		node:      node,
		filePath:  "/etc/test_file_create_overwrite.txt",
		contents:  "New content",
		overwrite: true,
	}

	updater := formatters.NewMockTaskCompleter()
	require.NoError(t, step.Run(updater))

	got, ok := node.GetFile("/etc/test_file_create_overwrite.txt")
	require.True(t, ok)
	assert.Equal(t, "New content", string(got))
}

// TestFileCreateStepNoOverwrite verifies error when file exists without overwrite.
func TestFileCreateStepNoOverwrite(t *testing.T) {
	node := nodetypes.NewMockNode()
	node.SeedFile("/etc/test_file_create_no_overwrite.txt", []byte("Initial content"), 0644)

	step := &FileCreateStep{
		BaseStep:  BaseStep{title: "No Overwrite Test File"},
		node:      node,
		filePath:  "/etc/test_file_create_no_overwrite.txt",
		contents:  "New content",
		overwrite: false,
	}

	updater := formatters.NewMockTaskCompleter()
	err := step.Run(updater)

	assert.Error(t, err)
	assert.True(t, updater.IsErrored())
}

// TestFileCreateStepWithCreateDir verifies creating directories as needed.
func TestFileCreateStepWithCreateDir(t *testing.T) {
	node := nodetypes.NewMockNode()

	step := &FileCreateStep{
		BaseStep:  BaseStep{title: "Create File With Dir"},
		node:      node,
		filePath:  "/srv/sub/dir/test_file.txt",
		contents:  "Test content",
		createDir: true,
	}

	updater := formatters.NewMockTaskCompleter()
	require.NoError(t, step.Run(updater))

	got, ok := node.GetFile("/srv/sub/dir/test_file.txt")
	require.True(t, ok)
	assert.Equal(t, "Test content", string(got))
}
