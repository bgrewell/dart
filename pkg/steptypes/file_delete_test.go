package steptypes

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/bgrewell/dart/internal/formatters"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestFileDeleteStep verifies basic file deletion.
func TestFileDeleteStep(t *testing.T) {
	tempFile := filepath.Join(os.TempDir(), "test_file_delete.txt")

	// Create the file first
	err := os.WriteFile(tempFile, []byte("Test content"), 0644)
	require.NoError(t, err)

	step := &FileDeleteStep{
		BaseStep: BaseStep{title: "Delete Test File"},
		filePath: tempFile,
	}

	updater := formatters.NewMockTaskCompleter()
	err = step.Run(updater)

	assert.NoError(t, err)
	assert.NoFileExists(t, tempFile)
	assert.True(t, updater.IsCompleted())
}

// TestFileDeleteStepNotExists verifies error when file doesn't exist.
func TestFileDeleteStepNotExists(t *testing.T) {
	tempFile := filepath.Join(os.TempDir(), "test_file_delete_not_exists.txt")

	step := &FileDeleteStep{
		BaseStep: BaseStep{title: "Delete Non-existent File"},
		filePath: tempFile,
	}

	updater := formatters.NewMockTaskCompleter()
	err := step.Run(updater)

	assert.Error(t, err)
	assert.True(t, updater.IsErrored())
}

// TestFileDeleteStepIgnoreErrors verifies ignore_errors option.
func TestFileDeleteStepIgnoreErrors(t *testing.T) {
	tempFile := filepath.Join(os.TempDir(), "test_file_delete_ignore.txt")

	step := &FileDeleteStep{
		BaseStep:     BaseStep{title: "Delete Non-existent File Ignore"},
		filePath:     tempFile,
		ignoreErrors: true,
	}

	updater := formatters.NewMockTaskCompleter()
	err := step.Run(updater)

	assert.NoError(t, err)
	assert.True(t, updater.IsCompleted())
}
