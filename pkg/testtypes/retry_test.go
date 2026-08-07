package testtypes

import (
	"strings"
	"testing"
	"time"

	"github.com/bgrewell/dart/internal/config"
	"github.com/bgrewell/dart/internal/execution"
	"github.com/bgrewell/dart/internal/formatters"
	"github.com/bgrewell/dart/pkg/ifaces"
	"github.com/bgrewell/dart/pkg/nodetypes"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func makeRetryTest(t *testing.T, node ifaces.Node, retry *config.RetryConfig, options map[string]interface{}) ifaces.Test {
	t.Helper()
	nodes := map[string]ifaces.Node{"test-node": node}
	configs := []*config.TestConfig{
		{
			Name:    "retrying",
			Node:    config.NodeReference{"test-node"},
			Type:    TypeExecute,
			Options: options,
			Retry:   retry,
		},
	}
	tests, err := CreateTests(configs, nodes)
	require.NoError(t, err)
	return tests[0]
}

// A test that fails its first attempts passes once the system converges.
func TestRetryEventuallyPasses(t *testing.T) {
	node := nodetypes.NewMockNode()
	node.QueueResponse("svc-status", 0, "starting\n", "")
	node.QueueResponse("svc-status", 0, "starting\n", "")
	node.SetResponse("svc-status", 0, "active\n", "")

	test := makeRetryTest(t, node,
		&config.RetryConfig{Timeout: 10, Interval: 0.01},
		map[string]interface{}{
			"command":  "svc-status",
			"evaluate": map[string]interface{}{"match": "active"},
		})
	allPassed(t, runTest(t, test))
}

// Retry also covers command errors, not just failed evaluations.
func TestRetryCoversExecuteErrors(t *testing.T) {
	node := nodetypes.NewMockNode()
	// No response mapped initially: first attempts error. Map it after a
	// short delay via a goroutine? Simpler: queue nothing and set the
	// persistent response — the mock errors only for unmapped commands, so
	// use a wrapper that errors twice.
	flaky := &flakyNode{MockNode: node, failures: 2}
	node.SetResponse("flaky-cmd", 0, "ok\n", "")

	test := makeRetryTest(t, flaky,
		&config.RetryConfig{Timeout: 10, Interval: 0.01},
		map[string]interface{}{
			"command":  "flaky-cmd",
			"evaluate": map[string]interface{}{"exit_code": 0},
		})
	allPassed(t, runTest(t, test))
	assert.Equal(t, 3, flaky.calls)
}

type flakyNode struct {
	*nodetypes.MockNode
	failures int
	calls    int
}

func (f *flakyNode) Execute(command string, options ...execution.ExecutionOption) (*execution.ExecutionResult, error) {
	f.calls++
	if f.calls <= f.failures {
		return nil, assert.AnError
	}
	return f.MockNode.Execute(command, options...)
}

// Retry gives up at the timeout and reports the final failing attempt.
func TestRetryTimesOut(t *testing.T) {
	node := nodetypes.NewMockNode()
	node.SetResponse("never-ready", 0, "starting\n", "")

	test := makeRetryTest(t, node,
		&config.RetryConfig{Timeout: 0.05, Interval: 0.01},
		map[string]interface{}{
			"command":  "never-ready",
			"evaluate": map[string]interface{}{"match": "active"},
		})
	results := runTest(t, test)
	assert.False(t, results["match"].Passed)
}

func TestRetryValidation(t *testing.T) {
	node := nodetypes.NewMockNode()
	nodes := map[string]ifaces.Node{"test-node": node}
	_, err := CreateTests([]*config.TestConfig{{
		Name: "bad retry", Node: config.NodeReference{"test-node"}, Type: TypeExecute,
		Options: map[string]interface{}{"command": "true"},
		Retry:   &config.RetryConfig{Timeout: 0},
	}}, nodes)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "retry.timeout must be positive")
}

// slowNode delays every execution, for timeout tests.
type slowNode struct {
	*nodetypes.MockNode
	delay time.Duration
}

func (s *slowNode) Execute(command string, options ...execution.ExecutionOption) (*execution.ExecutionResult, error) {
	time.Sleep(s.delay)
	return s.MockNode.Execute(command, options...)
}

// A hung command fails with a timeout error instead of hanging the suite.
func TestExecuteTimeout(t *testing.T) {
	mock := nodetypes.NewMockNode()
	mock.SetResponse("slow-cmd", 0, "done\n", "")
	node := &slowNode{MockNode: mock, delay: 300 * time.Millisecond}

	test, err := makeTest(t, node, TypeExecute, map[string]interface{}{
		"command":  "slow-cmd",
		"timeout":  0.05,
		"evaluate": map[string]interface{}{"exit_code": 0},
	})
	require.NoError(t, err)

	results, err := test.Run(formatters.NewMockTestCompleter())
	require.NoError(t, err, "a timeout is a test failure, not an infrastructure error")
	require.Contains(t, results, "timeout")
	assert.False(t, results["timeout"].Passed)
	details, ok := results["timeout"].Details.(string)
	require.True(t, ok)
	assert.Contains(t, details, "timed out after")
	assert.True(t, strings.Contains(details, "slow-cmd"))
}

// Within the timeout, execution proceeds normally.
func TestExecuteTimeoutNotTriggered(t *testing.T) {
	mock := nodetypes.NewMockNode()
	mock.SetResponse("quick-cmd", 0, "done\n", "")
	node := &slowNode{MockNode: mock, delay: 5 * time.Millisecond}

	test, err := makeTest(t, node, TypeExecute, map[string]interface{}{
		"command":  "quick-cmd",
		"timeout":  5,
		"evaluate": map[string]interface{}{"exit_code": 0},
	})
	require.NoError(t, err)
	allPassed(t, runTest(t, test))
}

// Single-flight: retrying a timed-out command re-awaits the same in-flight
// invocation instead of stacking new executions against the node.
func TestRetryTimeoutSingleFlight(t *testing.T) {
	mock := nodetypes.NewMockNode()
	mock.SetResponse("hung-cmd", 0, "late\n", "")
	node := &flakyCounter{slowNode: slowNode{MockNode: mock, delay: 150 * time.Millisecond}}

	test := makeRetryTest(t, node,
		&config.RetryConfig{Timeout: 0.4, Interval: 0.02},
		map[string]interface{}{
			"command":  "hung-cmd",
			"timeout":  0.02,
			"evaluate": map[string]interface{}{"match": "late"},
		})
	allPassed(t, runTest(t, test))
	assert.Equal(t, 1, node.calls, "one invocation total despite timed-out attempts")
}

type flakyCounter struct {
	slowNode
	calls int
}

func (f *flakyCounter) Execute(command string, options ...execution.ExecutionOption) (*execution.ExecutionResult, error) {
	f.calls++
	return f.slowNode.Execute(command, options...)
}

func TestRetryRejectedOnReboot(t *testing.T) {
	node := &rebootableMock{MockNode: nodetypes.NewMockNode()}
	nodes := map[string]ifaces.Node{"test-node": node}
	_, err := CreateTests([]*config.TestConfig{{
		Name: "reboot retry", Node: config.NodeReference{"test-node"}, Type: TypeReboot,
		Options: map[string]interface{}{},
		Retry:   &config.RetryConfig{Timeout: 10},
	}}, nodes)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "retry is not supported on reboot tests")
}

func TestRetryIntervalValidation(t *testing.T) {
	node := nodetypes.NewMockNode()
	nodes := map[string]ifaces.Node{"test-node": node}
	_, err := CreateTests([]*config.TestConfig{{
		Name: "interval too big", Node: config.NodeReference{"test-node"}, Type: TypeExecute,
		Options: map[string]interface{}{"command": "true"},
		Retry:   &config.RetryConfig{Timeout: 1, Interval: 5},
	}}, nodes)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "retry can never engage")

	_, err = CreateTests([]*config.TestConfig{{
		Name: "negative interval", Node: config.NodeReference{"test-node"}, Type: TypeExecute,
		Options: map[string]interface{}{"command": "true"},
		Retry:   &config.RetryConfig{Timeout: 10, Interval: -1},
	}}, nodes)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "must not be negative")
}
