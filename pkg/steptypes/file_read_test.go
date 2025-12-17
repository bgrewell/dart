package steptypes

import (
	"os"
	"testing"

	"github.com/bgrewell/dart/internal/formatters"
	"github.com/bgrewell/dart/pkg/nodetypes"
	"github.com/stretchr/testify/assert"
)

// TestFileReadStep verifies file reading and content validation.
func TestFileReadStep(t *testing.T) {
	tempFile := "/tmp/test_read.txt"
	expectedContent := "Hello, DART!"
	os.WriteFile(tempFile, []byte(expectedContent), 0644)

	step := &FileReadStep{
		BaseStep: BaseStep{title: "Read File"},
		node:     nodetypes.NewLocalNode(nil),
		filePath: tempFile,
		contains: "DART",
	}

	// Run step
	updater := formatters.NewMockTaskCompleter()
	err := step.Run(updater)

	// Validate content
	assert.NoError(t, err)

	// Clean up
	os.Remove(tempFile)
}
