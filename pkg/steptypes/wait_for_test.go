package steptypes

import (
	"testing"

	"github.com/bgrewell/dart/internal/formatters"
	"github.com/bgrewell/dart/pkg/nodetypes"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWaitForEventuallySucceeds(t *testing.T) {
	node := nodetypes.NewMockNode()
	node.QueueResponse("svc-ready", 1, "", "")
	node.QueueResponse("svc-ready", 1, "", "")
	node.SetResponse("svc-ready", 0, "", "")

	step, err := makeStep(t, TypeWaitFor, map[string]interface{}{
		"command":  "svc-ready",
		"timeout":  10,
		"interval": 0.01,
	})
	require.NoError(t, err)

	waitStep := step.(*WaitForStep)
	waitStep.node = node
	updater := formatters.NewMockTaskCompleter()
	require.NoError(t, waitStep.Run(updater))
	assert.True(t, updater.IsCompleted())
}

func TestWaitForTimesOut(t *testing.T) {
	node := nodetypes.NewMockNode()
	node.SetResponse("never-ready", 1, "", "")

	step, err := makeStep(t, TypeWaitFor, map[string]interface{}{
		"command":  "never-ready",
		"timeout":  0.05,
		"interval": 0.01,
	})
	require.NoError(t, err)

	waitStep := step.(*WaitForStep)
	waitStep.node = node
	err = waitStep.Run(formatters.NewMockTaskCompleter())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "wait_for timed out")
}

func TestWaitForValidation(t *testing.T) {
	_, err := makeStep(t, TypeWaitFor, map[string]interface{}{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "command is required")

	_, err = makeStep(t, TypeWaitFor, map[string]interface{}{"command": "x", "timeout": -1})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "timeout must be positive")
}
