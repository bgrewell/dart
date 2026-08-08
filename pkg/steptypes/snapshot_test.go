package steptypes

import (
	"errors"
	"testing"

	"github.com/bgrewell/dart/internal/formatters"
	"github.com/bgrewell/dart/pkg/nodetypes"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// snapshotMock records snapshot calls.
type snapshotMock struct {
	*nodetypes.MockNode
	created         []string
	restored        []string
	deleted         []string
	stateful        bool
	restoreStateful bool
	failWith        error
	callCount       int
}

func (m *snapshotMock) Snapshot(name string, stateful bool) error {
	m.callCount++
	m.stateful = stateful
	m.created = append(m.created, name)
	return m.failWith
}

func (m *snapshotMock) RestoreSnapshot(name string, stateful bool) error {
	m.callCount++
	m.restoreStateful = stateful
	m.restored = append(m.restored, name)
	return m.failWith
}

func (m *snapshotMock) DeleteSnapshot(name string) error {
	m.callCount++
	m.deleted = append(m.deleted, name)
	return m.failWith
}

func TestSnapshotActions(t *testing.T) {
	node := &snapshotMock{MockNode: nodetypes.NewMockNode()}

	create, err := makeStepOn(t, node, TypeSnapshot, map[string]interface{}{"name": "pre-test"})
	require.NoError(t, err)
	require.NoError(t, create.Run(formatters.NewMockTaskCompleter()))
	assert.Equal(t, []string{"pre-test"}, node.created)
	assert.False(t, node.stateful, "stateless by default")

	restore, err := makeStepOn(t, node, TypeSnapshot, map[string]interface{}{
		"name": "pre-test", "action": "restore"})
	require.NoError(t, err)
	require.NoError(t, restore.Run(formatters.NewMockTaskCompleter()))
	assert.Equal(t, []string{"pre-test"}, node.restored)

	del, err := makeStepOn(t, node, TypeSnapshot, map[string]interface{}{
		"name": "pre-test", "action": "delete"})
	require.NoError(t, err)
	require.NoError(t, del.Run(formatters.NewMockTaskCompleter()))
	assert.Equal(t, []string{"pre-test"}, node.deleted)
}

func TestSnapshotStateful(t *testing.T) {
	node := &snapshotMock{MockNode: nodetypes.NewMockNode()}
	step, err := makeStepOn(t, node, TypeSnapshot, map[string]interface{}{
		"name": "running-state", "stateful": true})
	require.NoError(t, err)
	require.NoError(t, step.Run(formatters.NewMockTaskCompleter()))
	assert.True(t, node.stateful)
}

func TestSnapshotFailureSurfaces(t *testing.T) {
	node := &snapshotMock{MockNode: nodetypes.NewMockNode(), failWith: errors.New("no space left")}
	step, err := makeStepOn(t, node, TypeSnapshot, map[string]interface{}{"name": "x"})
	require.NoError(t, err)

	err = step.Run(formatters.NewMockTaskCompleter())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no space left")
}

func TestSnapshotUnsupportedNode(t *testing.T) {
	_, err := makeStepOn(t, nodetypes.NewMockNode(), TypeSnapshot, map[string]interface{}{"name": "x"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "does not support snapshots")
}

func TestSnapshotValidation(t *testing.T) {
	node := &snapshotMock{MockNode: nodetypes.NewMockNode()}

	_, err := makeStepOn(t, node, TypeSnapshot, map[string]interface{}{})
	assert.ErrorContains(t, err, "snapshot name is required")

	_, err = makeStepOn(t, node, TypeSnapshot, map[string]interface{}{
		"name": "x", "action": "rollback"})
	assert.ErrorContains(t, err, "create, restore, or delete")

	_, err = makeStepOn(t, node, TypeSnapshot, map[string]interface{}{
		"name": "x", "action": "delete", "stateful": true})
	assert.ErrorContains(t, err, "not delete")
}

// A stateful snapshot must be restored statefully: without the flag LXD
// silently performs a disk-only restore and drops the saved memory.
func TestSnapshotStatefulRestorePropagates(t *testing.T) {
	node := &snapshotMock{MockNode: nodetypes.NewMockNode()}
	step, err := makeStepOn(t, node, TypeSnapshot, map[string]interface{}{
		"name": "running-state", "action": "restore", "stateful": true})
	require.NoError(t, err)
	require.NoError(t, step.Run(formatters.NewMockTaskCompleter()))
	assert.True(t, node.restoreStateful, "stateful must reach the restore call")
}
