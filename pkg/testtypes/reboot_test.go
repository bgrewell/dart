package testtypes

import (
	"errors"
	"testing"
	"time"

	"github.com/bgrewell/dart/internal/formatters"
	"github.com/bgrewell/dart/pkg/nodetypes"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// rebootableMock wraps MockNode with a recorded, configurable Reboot.
type rebootableMock struct {
	*nodetypes.MockNode
	rebootErr   error
	calls       int
	lastForce   bool
	lastReady   string
	lastTimeout time.Duration
}

func (m *rebootableMock) Reboot(force bool, readyCommand string, timeout time.Duration) error {
	m.calls++
	m.lastForce = force
	m.lastReady = readyCommand
	m.lastTimeout = timeout
	return m.rebootErr
}

func TestRebootTest(t *testing.T) {
	node := &rebootableMock{MockNode: nodetypes.NewMockNode()}

	test, err := makeTest(t, node, TypeReboot, map[string]interface{}{
		"mode":          "force",
		"ready_command": "cat /etc/hostname",
		"timeout":       120,
	})
	require.NoError(t, err)
	allPassed(t, runTest(t, test))

	assert.Equal(t, 1, node.calls)
	assert.True(t, node.lastForce)
	assert.Equal(t, "cat /etc/hostname", node.lastReady)
	assert.Equal(t, 2*time.Minute, node.lastTimeout)
}

func TestRebootTestDefaultsGraceful(t *testing.T) {
	node := &rebootableMock{MockNode: nodetypes.NewMockNode()}
	test, err := makeTest(t, node, TypeReboot, map[string]interface{}{})
	require.NoError(t, err)
	allPassed(t, runTest(t, test))
	assert.False(t, node.lastForce)
	assert.Zero(t, node.lastTimeout)
}

func TestRebootTestFailure(t *testing.T) {
	node := &rebootableMock{MockNode: nodetypes.NewMockNode(), rebootErr: errors.New("did not come back")}
	test, err := makeTest(t, node, TypeReboot, map[string]interface{}{})
	require.NoError(t, err)

	_, err = test.Run(formatters.NewMockTestCompleter())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "did not come back")
}

func TestRebootTestUnsupportedNode(t *testing.T) {
	_, err := makeTest(t, nodetypes.NewMockNode(), TypeReboot, map[string]interface{}{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "does not support reboot")
}

func TestRebootTestInvalidMode(t *testing.T) {
	node := &rebootableMock{MockNode: nodetypes.NewMockNode()}
	_, err := makeTest(t, node, TypeReboot, map[string]interface{}{"mode": "gently"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "graceful")
}
