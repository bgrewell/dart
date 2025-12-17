package steptypes

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/bgrewell/dart/internal/formatters"
	"github.com/bgrewell/dart/pkg/nodetypes"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestFileCreateStep verifies basic file creation.
func TestFileCreateStep(t *testing.T) {
	tempFile := filepath.Join(os.TempDir(), "test_file_create.txt")
	defer os.Remove(tempFile)

	step := &FileCreateStep{
		BaseStep:  BaseStep{title: "Create Test File"},
		node:      nodetypes.NewLocalNode(nil),
		filePath:  tempFile,
		contents:  "Hello World",
		overwrite: false,
	}

	updater := formatters.NewMockTaskCompleter()
	err := step.Run(updater)

	assert.NoError(t, err)
	assert.FileExists(t, tempFile)

	content, err := os.ReadFile(tempFile)
	require.NoError(t, err)
	assert.Equal(t, "Hello World", string(content))
	assert.True(t, updater.IsCompleted())
}

// TestFileCreateStepOverwrite verifies file creation with overwrite.
func TestFileCreateStepOverwrite(t *testing.T) {
	tempFile := filepath.Join(os.TempDir(), "test_file_create_overwrite.txt")
	defer os.Remove(tempFile)

	// Create initial file
	err := os.WriteFile(tempFile, []byte("Initial content"), 0644)
	require.NoError(t, err)

	step := &FileCreateStep{
		BaseStep:  BaseStep{title: "Overwrite Test File"},
		node:      nodetypes.NewLocalNode(nil),
		filePath:  tempFile,
		contents:  "New content",
		overwrite: true,
	}

	updater := formatters.NewMockTaskCompleter()
	err = step.Run(updater)

	assert.NoError(t, err)

	content, err := os.ReadFile(tempFile)
	require.NoError(t, err)
	assert.Equal(t, "New content", string(content))
}

// TestFileCreateStepNoOverwrite verifies error when file exists without overwrite.
func TestFileCreateStepNoOverwrite(t *testing.T) {
	tempFile := filepath.Join(os.TempDir(), "test_file_create_no_overwrite.txt")
	defer os.Remove(tempFile)

	// Create initial file
	err := os.WriteFile(tempFile, []byte("Initial content"), 0644)
	require.NoError(t, err)

	step := &FileCreateStep{
		BaseStep:  BaseStep{title: "No Overwrite Test File"},
		node:      nodetypes.NewLocalNode(nil),
		filePath:  tempFile,
		contents:  "New content",
		overwrite: false,
	}

	updater := formatters.NewMockTaskCompleter()
	err = step.Run(updater)

	assert.Error(t, err)
	assert.True(t, updater.IsErrored())
}

// TestFileCreateStepWithCreateDir verifies creating directories as needed.
func TestFileCreateStepWithCreateDir(t *testing.T) {
	tempDir := filepath.Join(os.TempDir(), "test_create_dir_"+randomSuffix())
	tempFile := filepath.Join(tempDir, "subdir", "test_file.txt")
	defer os.RemoveAll(tempDir)

	step := &FileCreateStep{
		BaseStep:  BaseStep{title: "Create File With Dir"},
		node:      nodetypes.NewLocalNode(nil),
		filePath:  tempFile,
		contents:  "Test content",
		createDir: true,
	}

	updater := formatters.NewMockTaskCompleter()
	err := step.Run(updater)

	assert.NoError(t, err)
	assert.FileExists(t, tempFile)
}

// Helper function to generate random suffix for unique directories
func randomSuffix() string {
	return fmt.Sprintf("%d", os.Getpid())
}
