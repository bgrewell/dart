package steptypes

import (
	"testing"

	"github.com/bgrewell/dart/internal/formatters"
	"github.com/bgrewell/dart/pkg/nodetypes"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestFileReadStep verifies file reading and contains validation against the node.
func TestFileReadStep(t *testing.T) {
	node := nodetypes.NewMockNode()
	node.SeedFile("/etc/test_read.txt", []byte("Hello, DART!"), 0644)

	step := &FileReadStep{
		BaseStep: BaseStep{title: "Read File"},
		node:     node,
		filePath: "/etc/test_read.txt",
		contains: "DART",
	}

	updater := formatters.NewMockTaskCompleter()
	require.NoError(t, step.Run(updater))
	assert.True(t, updater.IsCompleted())
}

// TestFileReadStepContainsMissing verifies error when expected content is absent.
func TestFileReadStepContainsMissing(t *testing.T) {
	node := nodetypes.NewMockNode()
	node.SeedFile("/etc/test_read.txt", []byte("nothing of interest"), 0644)

	step := &FileReadStep{
		BaseStep: BaseStep{title: "Read File Contains Missing"},
		node:     node,
		filePath: "/etc/test_read.txt",
		contains: "DART",
	}

	updater := formatters.NewMockTaskCompleter()
	err := step.Run(updater)
	assert.Error(t, err)
	assert.True(t, updater.IsErrored())
}
