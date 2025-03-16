package steptypes

import (
	"os"
	"testing"

	"github.com/bgrewell/dart/internal/formatters"
	"github.com/stretchr/testify/assert"
)

// TestFileExistsStep verifies file existence check.
func TestFileExistsStep(t *testing.T) {
	tempFile := "/tmp/test_exists.txt"
	os.WriteFile(tempFile, []byte("test"), 0644)

	step := &FileExistsStep{
		BaseStep: BaseStep{title: "Check File Exists"},
		filePath: tempFile,
	}

	// Run step
	updater := formatters.NewMockTaskCompleter()
	err := step.Run(updater)

	// Check file exists
	assert.NoError(t, err)

	// Clean up
	os.Remove(tempFile)
}
