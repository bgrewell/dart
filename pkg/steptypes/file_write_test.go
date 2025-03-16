package steptypes

import (
	"os"
	"testing"

	"github.com/bgrewell/dart/internal/formatters"
	"github.com/stretchr/testify/assert"
)

// TestFileWriteStep verifies file creation and writing.
func TestFileWriteStep(t *testing.T) {
	tempFile := "/tmp/test_file.txt"
	step := &FileWriteStep{
		BaseStep:  BaseStep{title: "Write Test File"},
		filePath:  tempFile,
		contents:  "Hello World",
		overwrite: true,
	}

	// Run step
	updater := formatters.NewMockTaskCompleter()
	err := step.Run(updater)

	// Check file existence
	assert.NoError(t, err)
	assert.FileExists(t, tempFile)

	// Clean up
	os.Remove(tempFile)
}
