package testtypes

import (
	"fmt"
	"testing"

	"github.com/bgrewell/dart/internal/config"
	"github.com/bgrewell/dart/internal/results"
	"github.com/bgrewell/dart/pkg/ifaces"
	"github.com/bgrewell/dart/pkg/nodetypes"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// clusterFixture builds N mock nodes with per-node canned responses.
func clusterFixture(t *testing.T, command string, outputs map[string]string) map[string]ifaces.Node {
	t.Helper()
	nodes := map[string]ifaces.Node{}
	for name, output := range outputs {
		mock := nodetypes.NewMockNode()
		mock.SetResponse(command, 0, output, "")
		nodes[name] = mock
	}
	return nodes
}

func makeConsistencyTest(t *testing.T, nodes map[string]ifaces.Node, nodeNames []string, options map[string]interface{}) (ifaces.Test, error) {
	t.Helper()
	configs := []*config.TestConfig{{
		Name:    "cluster check",
		Node:    config.NodeReference(nodeNames),
		Type:    TypeConsistency,
		Options: options,
	}}
	tests, err := CreateTests(configs, nodes)
	if err != nil {
		return nil, err
	}
	require.Len(t, tests, 1, "consistency tests must not be expanded per node")
	return tests[0], nil
}

func TestConsistencyAllAgree(t *testing.T) {
	nodes := clusterFixture(t, "cat /etc/app.conf", map[string]string{
		"n1": "version=3\n", "n2": "version=3\n", "n3": "version=3\n",
	})
	test, err := makeConsistencyTest(t, nodes, []string{"n1", "n2", "n3"},
		map[string]interface{}{"command": "cat /etc/app.conf"})
	require.NoError(t, err)
	allPassed(t, runTest(t, test))
}

// Config drift fails and the failure names which nodes disagree.
func TestConsistencyDriftReportsNodes(t *testing.T) {
	nodes := clusterFixture(t, "cat /etc/app.conf", map[string]string{
		"n1": "version=3\n", "n2": "version=3\n", "n3": "version=2\n",
	})
	test, err := makeConsistencyTest(t, nodes, []string{"n1", "n2", "n3"},
		map[string]interface{}{"command": "cat /etc/app.conf"})
	require.NoError(t, err)

	results := runTest(t, test)
	require.False(t, results["all_equal"].Passed)
	details := results["all_equal"].Details
	assert.Contains(t, describeDetails(details), "n3")
	assert.Contains(t, describeDetails(details), "version=2")
}

// An unreachable node is a consistency finding, not an aborted run.
func TestConsistencyUnreachableNodeFails(t *testing.T) {
	nodes := clusterFixture(t, "cat /etc/app.conf", map[string]string{
		"n1": "version=3\n", "n2": "version=3\n",
	})
	broken := nodetypes.NewMockNode() // no canned response: Execute errors
	nodes["n3"] = broken

	test, err := makeConsistencyTest(t, nodes, []string{"n1", "n2", "n3"},
		map[string]interface{}{"command": "cat /etc/app.conf"})
	require.NoError(t, err)

	results := runTest(t, test)
	assert.False(t, results["all_equal"].Passed)
	assert.Contains(t, describeDetails(results["all_equal"].Details), "n3")
}

// Leader election: exactly one node reports leader.
func TestConsistencyExactlyOneLeader(t *testing.T) {
	nodes := clusterFixture(t, "cluster-role", map[string]string{
		"n1": "follower\n", "n2": "leader\n", "n3": "follower\n",
	})
	test, err := makeConsistencyTest(t, nodes, []string{"n1", "n2", "n3"},
		map[string]interface{}{
			"command": "cluster-role",
			"evaluate": map[string]interface{}{
				"matching": map[string]interface{}{"pattern": "^leader$", "count": 1},
			},
		})
	require.NoError(t, err)
	allPassed(t, runTest(t, test))
}

// Split brain: two leaders fails the same check.
func TestConsistencySplitBrainFails(t *testing.T) {
	nodes := clusterFixture(t, "cluster-role", map[string]string{
		"n1": "leader\n", "n2": "leader\n", "n3": "follower\n",
	})
	test, err := makeConsistencyTest(t, nodes, []string{"n1", "n2", "n3"},
		map[string]interface{}{
			"command": "cluster-role",
			"evaluate": map[string]interface{}{
				"matching": map[string]interface{}{"pattern": "^leader$"},
			},
		})
	require.NoError(t, err)

	results := runTest(t, test)
	require.False(t, results["matching"].Passed)
	assert.Contains(t, describeDetails(results["matching"].Details), "2 matching")
}

// all_equal: false asserts nodes deliberately differ (e.g. unique ids).
func TestConsistencyExpectDifference(t *testing.T) {
	nodes := clusterFixture(t, "hostname", map[string]string{
		"n1": "alpha\n", "n2": "beta\n",
	})
	test, err := makeConsistencyTest(t, nodes, []string{"n1", "n2"},
		map[string]interface{}{
			"command":  "hostname",
			"evaluate": map[string]interface{}{"all_equal": false},
		})
	require.NoError(t, err)
	allPassed(t, runTest(t, test))
}

// The JSON report is on stdout, so standard evaluators apply too.
func TestConsistencyStandardEvaluators(t *testing.T) {
	nodes := clusterFixture(t, "cluster-role", map[string]string{
		"n1": "leader\n", "n2": "follower\n",
	})
	test, err := makeConsistencyTest(t, nodes, []string{"n1", "n2"},
		map[string]interface{}{
			"command": "cluster-role",
			"evaluate": map[string]interface{}{
				"contains":  "leader",
				"json_path": map[string]interface{}{"path": "outputs[0].node", "equals": "n1"},
			},
		})
	require.NoError(t, err)
	allPassed(t, runTest(t, test))
}

// An explicit nodes: list overrides the test's node reference.
func TestConsistencyExplicitNodeList(t *testing.T) {
	nodes := clusterFixture(t, "check", map[string]string{
		"n1": "same\n", "n2": "same\n", "n3": "different\n",
	})
	test, err := makeConsistencyTest(t, nodes, []string{"n1", "n2", "n3"},
		map[string]interface{}{
			"command": "check",
			"nodes":   []interface{}{"n1", "n2"},
		})
	require.NoError(t, err)
	allPassed(t, runTest(t, test))
}

func TestConsistencyValidation(t *testing.T) {
	nodes := clusterFixture(t, "x", map[string]string{"n1": "a", "n2": "b"})

	_, err := makeConsistencyTest(t, nodes, []string{"n1", "n2"}, map[string]interface{}{})
	assert.ErrorContains(t, err, "command is required")

	_, err = makeConsistencyTest(t, nodes, []string{"n1"}, map[string]interface{}{"command": "x"})
	assert.ErrorContains(t, err, "at least two nodes")

	_, err = makeConsistencyTest(t, nodes, []string{"n1", "n2"}, map[string]interface{}{
		"command": "x", "nodes": []interface{}{"n1", "ghost"}})
	assert.ErrorContains(t, err, "not listed in the node: reference")

	_, err = makeConsistencyTest(t, nodes, []string{"n1", "n1", "n2"}, map[string]interface{}{"command": "x"})
	assert.ErrorContains(t, err, "listed more than once")

	_, err = makeConsistencyTest(t, nodes, []string{"n1", "n2"}, map[string]interface{}{
		"command":  "x",
		"evaluate": map[string]interface{}{"matching": map[string]interface{}{"pattern": "("}}})
	assert.ErrorContains(t, err, "invalid")

	_, err = makeConsistencyTest(t, nodes, []string{"n1", "n2"}, map[string]interface{}{
		"command":  "x",
		"evaluate": map[string]interface{}{"matching": map[string]interface{}{"pattern": "a", "bogus": 1}}})
	assert.ErrorContains(t, err, "unknown matching key")
}

// describeDetails renders a details value for assertions.
func describeDetails(details interface{}) string {
	if text, ok := details.(string); ok {
		return text
	}
	if fail, ok := details.(*results.ResultStringMatchFail); ok {
		return fail.Expected + " | " + fail.Actual
	}
	return fmt.Sprint(details)
}

// An unreachable node must fail BOTH polarities: with all_equal: false an
// outage would otherwise satisfy "nodes differ" and go green.
func TestConsistencyErrorFailsDifferExpectation(t *testing.T) {
	nodes := clusterFixture(t, "check", map[string]string{
		"n1": "same\n", "n2": "same\n",
	})
	nodes["n3"] = nodetypes.NewMockNode() // unmapped command: errors

	test, err := makeConsistencyTest(t, nodes, []string{"n1", "n2", "n3"},
		map[string]interface{}{
			"command":  "check",
			"evaluate": map[string]interface{}{"all_equal": false},
		})
	require.NoError(t, err)

	results := runTest(t, test)
	require.False(t, results["all_equal"].Passed, "an unreachable node is never a valid comparison")
	assert.Contains(t, describeDetails(results["all_equal"].Details), "n3")
}

// Different invalid-UTF-8 outputs must not collapse into agreement (JSON
// marshaling replaces them all with U+FFFD).
func TestConsistencyInvalidUTF8NotEqual(t *testing.T) {
	nodes := map[string]ifaces.Node{}
	for name, output := range map[string]string{
		"n1": string([]byte{0xff, 0xfe}),
		"n2": string([]byte{0xfa, 0xfb}),
	} {
		mock := nodetypes.NewMockNode()
		mock.SetResponse("dump", 0, output, "")
		nodes[name] = mock
	}

	test, err := makeConsistencyTest(t, nodes, []string{"n1", "n2"},
		map[string]interface{}{"command": "dump"})
	require.NoError(t, err)

	results := runTest(t, test)
	assert.False(t, results["all_equal"].Passed, "different binary outputs must not read as equal")
}

// The reported node names every compared node, not just the first.
func TestConsistencyNodeNameListsAll(t *testing.T) {
	nodes := clusterFixture(t, "x", map[string]string{"n1": "a", "n2": "a", "n3": "a"})
	test, err := makeConsistencyTest(t, nodes, []string{"n1", "n2", "n3"},
		map[string]interface{}{"command": "x"})
	require.NoError(t, err)
	assert.Equal(t, "n1,n2,n3", test.NodeName())
}

// The worst (highest) exit code wins, not the last one seen.
func TestConsistencyWorstExitCode(t *testing.T) {
	nodes := map[string]ifaces.Node{}
	for name, code := range map[string]int{"n1": 5, "n2": 2} {
		mock := nodetypes.NewMockNode()
		mock.SetResponse("failing", code, "out\n", "")
		nodes[name] = mock
	}

	test, err := makeConsistencyTest(t, nodes, []string{"n1", "n2"},
		map[string]interface{}{
			"command":  "failing",
			"evaluate": map[string]interface{}{"exit_code": 5},
		})
	require.NoError(t, err)
	allPassed(t, runTest(t, test))
}
