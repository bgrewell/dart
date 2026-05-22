package steptypes

import (
	"testing"

	"github.com/bgrewell/dart/internal/formatters"
	"github.com/bgrewell/dart/pkg/nodetypes"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestFileExistsStep verifies file existence check on the node.
func TestFileExistsStep(t *testing.T) {
	node := nodetypes.NewMockNode()
	node.SeedFile("/etc/test_exists.txt", []byte("test"), 0644)

	step := &FileExistsStep{
		BaseStep: BaseStep{title: "Check File Exists"},
		node:     node,
		filePath: "/etc/test_exists.txt",
	}

	updater := formatters.NewMockTaskCompleter()
	require.NoError(t, step.Run(updater))
	assert.True(t, updater.IsCompleted())
}

// TestFileExistsStepMissing verifies error when file is absent on the node.
func TestFileExistsStepMissing(t *testing.T) {
	node := nodetypes.NewMockNode()

	step := &FileExistsStep{
		BaseStep: BaseStep{title: "Check File Missing"},
		node:     node,
		filePath: "/etc/no_such_file.txt",
	}

	updater := formatters.NewMockTaskCompleter()
	err := step.Run(updater)
	assert.Error(t, err)
	assert.True(t, updater.IsErrored())
}
