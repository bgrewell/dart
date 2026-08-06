package testtypes

import (
	"errors"
	"testing"

	"github.com/bgrewell/dart/internal/config"
	"github.com/bgrewell/dart/pkg/ifaces"
	"github.com/bgrewell/dart/pkg/nodetypes"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func makeSkippableTest(t *testing.T, node ifaces.Node, skipIf, skipUnless string) ifaces.Test {
	t.Helper()
	nodes := map[string]ifaces.Node{"test-node": node}
	configs := []*config.TestConfig{
		{
			Name:       "skippable",
			Node:       config.NodeReference{"test-node"},
			Type:       TypeExecute,
			Options:    map[string]interface{}{"command": "true"},
			SkipIf:     skipIf,
			SkipUnless: skipUnless,
		},
	}
	tests, err := CreateTests(configs, nodes)
	require.NoError(t, err)
	require.Len(t, tests, 1)
	return tests[0]
}

func TestShouldSkipNoConditions(t *testing.T) {
	test := makeSkippableTest(t, nodetypes.NewMockNode(), "", "")
	skip, _, err := test.ShouldSkip()
	require.NoError(t, err)
	assert.False(t, skip)
}

func TestShouldSkipSkipIf(t *testing.T) {
	node := nodetypes.NewMockNode()
	node.SetResponse("in-maintenance-mode", 0, "", "")
	test := makeSkippableTest(t, node, "in-maintenance-mode", "")

	skip, reason, err := test.ShouldSkip()
	require.NoError(t, err)
	assert.True(t, skip)
	assert.Contains(t, reason, "skip_if")
	assert.Contains(t, reason, "in-maintenance-mode")

	// Condition command failing means no skip
	node.SetResponse("in-maintenance-mode", 1, "", "")
	skip, _, err = test.ShouldSkip()
	require.NoError(t, err)
	assert.False(t, skip)
}

func TestShouldSkipSkipUnless(t *testing.T) {
	node := nodetypes.NewMockNode()
	node.SetResponse("which aether-ops-bootstrap", 1, "", "")
	test := makeSkippableTest(t, node, "", "which aether-ops-bootstrap")

	skip, reason, err := test.ShouldSkip()
	require.NoError(t, err)
	assert.True(t, skip)
	assert.Contains(t, reason, "skip_unless")

	node.SetResponse("which aether-ops-bootstrap", 0, "/usr/bin/aether-ops-bootstrap\n", "")
	skip, _, err = test.ShouldSkip()
	require.NoError(t, err)
	assert.False(t, skip)
}

// A condition command that cannot run at all is an error, never a silent
// skip or pass.
func TestShouldSkipConditionError(t *testing.T) {
	node := nodetypes.NewMockNode()
	node.SetError("broken-cond", errors.New("node unreachable"))
	test := makeSkippableTest(t, node, "broken-cond", "")

	_, _, err := test.ShouldSkip()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "skip_if command failed to run")
}
