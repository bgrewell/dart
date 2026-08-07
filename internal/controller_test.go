package internal

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/bgrewell/dart/internal/config"
	"github.com/bgrewell/dart/internal/formatters"
	"github.com/bgrewell/dart/internal/report"
	"github.com/bgrewell/dart/pkg/ifaces"
	"github.com/bgrewell/dart/pkg/nodetypes"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// recordingFormatter captures formatter calls so controller flows can be
// asserted without terminal output.
type recordingFormatter struct {
	mu      sync.Mutex
	headers []string
	passes  []string
	fails   []string
	skips   []string
	tasks   []string
	tests   []string
	results struct {
		pass, fail, skipped, ran int
		printed                  bool
	}
}

func (r *recordingFormatter) SetTaskColumnWidth(int) {}
func (r *recordingFormatter) SetTestColumnWidth(int) {}
func (r *recordingFormatter) SetNodeNameWidth(int)   {}

func (r *recordingFormatter) StartTask(task, nodeName, status string) formatters.TaskCompleter {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.tasks = append(r.tasks, fmt.Sprintf("%s@%s", task, nodeName))
	return formatters.NewMockTaskCompleter()
}

func (r *recordingFormatter) StartTest(id, name, nodeName string) formatters.TestCompleter {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.tests = append(r.tests, fmt.Sprintf("%s@%s", name, nodeName))
	return formatters.NewMockTestCompleter()
}

func (r *recordingFormatter) PrintHeader(header string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.headers = append(r.headers, header)
}

func (r *recordingFormatter) PrintResults(pass, fail, skipped, ran int, elapsed time.Duration) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.results.pass, r.results.fail, r.results.skipped, r.results.ran = pass, fail, skipped, ran
	r.results.printed = true
}

func (r *recordingFormatter) PrintPass(name string, details interface{}) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.passes = append(r.passes, name)
}

func (r *recordingFormatter) PrintFail(name string, details interface{}) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.fails = append(r.fails, name)
}

func (r *recordingFormatter) PrintSkip(name string, reason string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.skips = append(r.skips, name)
}

func (r *recordingFormatter) PrintEmpty()          {}
func (r *recordingFormatter) PrintError(err error) {}

// fakePlatform records lifecycle calls.
type fakePlatform struct {
	name       string
	configured bool
	setupErr   error
	setups     int
	teardowns  int
}

func (p *fakePlatform) Configured() bool { return p.configured }
func (p *fakePlatform) Name() string     { return p.name }
func (p *fakePlatform) Setup() error {
	p.setups++
	return p.setupErr
}
func (p *fakePlatform) Teardown() error {
	p.teardowns++
	return nil
}

// trackingNode wraps MockNode and records lifecycle calls with a shared
// event log so ordering across nodes can be asserted.
type trackingNode struct {
	*nodetypes.MockNode
	name     string
	events   *[]string
	mu       *sync.Mutex
	setupErr error
}

func (n *trackingNode) Setup() error {
	n.mu.Lock()
	*n.events = append(*n.events, "setup:"+n.name)
	n.mu.Unlock()
	return n.setupErr
}

func (n *trackingNode) Teardown() error {
	n.mu.Lock()
	*n.events = append(*n.events, "teardown:"+n.name)
	n.mu.Unlock()
	return nil
}

type controllerFixture struct {
	nodes     map[string]ifaces.Node
	configs   []*config.NodeConfig
	events    []string
	mu        sync.Mutex
	formatter *recordingFormatter
}

func newFixture(nodeNames ...string) *controllerFixture {
	f := &controllerFixture{
		nodes:     map[string]ifaces.Node{},
		formatter: &recordingFormatter{},
	}
	for _, name := range nodeNames {
		mock := nodetypes.NewMockNode()
		mock.SetResponse("true", 0, "", "")
		mock.SetResponse("false", 1, "", "")
		mock.SetResponse("echo ok", 0, "ok\n", "")
		f.nodes[name] = &trackingNode{MockNode: mock, name: name, events: &f.events, mu: &f.mu}
		f.configs = append(f.configs, &config.NodeConfig{Name: name})
	}
	return f
}

func (f *controllerFixture) controller(tests []*config.TestConfig, opts ...func(*TestController)) *TestController {
	tc := NewTestController("suite", nil, f.nodes, f.configs, nil, nil, tests,
		false, false, false, false, false, false, "", "", f.formatter)
	for _, opt := range opts {
		opt(tc)
	}
	return tc
}

func execTest(name, node, command string, evaluate map[string]interface{}) *config.TestConfig {
	options := map[string]interface{}{"command": command}
	if evaluate != nil {
		options["evaluate"] = evaluate
	}
	return &config.TestConfig{
		Name:    name,
		Node:    config.NodeReference{node},
		Type:    "execute",
		Options: options,
	}
}

func TestControllerCountsPassFailSkip(t *testing.T) {
	f := newFixture("n1")
	pass := execTest("passing", "n1", "echo ok", map[string]interface{}{"exit_code": 0})
	fail := execTest("failing", "n1", "false", map[string]interface{}{"exit_code": 0})
	skipped := execTest("skipped", "n1", "echo ok", map[string]interface{}{"exit_code": 0})
	skipped.SkipUnless = "false"
	ran := execTest("just runs", "n1", "echo ok", nil)

	tc := f.controller([]*config.TestConfig{pass, fail, skipped, ran})
	err := tc.Run()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "1 tests failed")
	require.True(t, f.formatter.results.printed)
	assert.Equal(t, 1, f.formatter.results.pass)
	assert.Equal(t, 1, f.formatter.results.fail)
	assert.Equal(t, 1, f.formatter.results.skipped)
	assert.Equal(t, 1, f.formatter.results.ran)
}

// Multi-node expansion reuses the test name per node; each executed test
// must count individually.
func TestControllerDuplicateNamesCountSeparately(t *testing.T) {
	f := newFixture("n1", "n2")
	tests := []*config.TestConfig{
		execTest("same name", "n1", "echo ok", map[string]interface{}{"exit_code": 0}),
		execTest("same name", "n2", "echo ok", map[string]interface{}{"exit_code": 0}),
	}

	tc := f.controller(tests)
	require.NoError(t, tc.Run())
	assert.Equal(t, 2, f.formatter.results.pass, "both same-named tests must count")
}

func TestControllerAllPassingReturnsNil(t *testing.T) {
	f := newFixture("n1")
	tc := f.controller([]*config.TestConfig{
		execTest("t1", "n1", "echo ok", map[string]interface{}{"exit_code": 0}),
	})
	assert.NoError(t, tc.Run())
}

func TestControllerStopOnFailReportsAllChecks(t *testing.T) {
	f := newFixture("n1")
	fail := execTest("multi-check fail", "n1", "false", map[string]interface{}{
		"exit_code": 0,
		"match":     "never",
	})
	after := execTest("never runs", "n1", "echo ok", nil)

	tc := f.controller([]*config.TestConfig{fail, after}, func(tc *TestController) {
		tc.stopOnFail = true
	})
	err := tc.Run()

	require.Error(t, err)
	// Both failing checks of the test are reported before stopping
	assert.Contains(t, f.formatter.fails, "exit_code")
	assert.Contains(t, f.formatter.fails, "match")
	// The following test never started
	assert.Len(t, f.formatter.tests, 1)
}

func TestControllerNodeOrderDeterministic(t *testing.T) {
	f := newFixture("zebra", "alpha", "middle")
	tc := f.controller([]*config.TestConfig{
		execTest("t", "alpha", "echo ok", nil),
	})
	require.NoError(t, tc.Run())

	// Setup follows config order (zebra, alpha, middle), teardown likewise
	require.GreaterOrEqual(t, len(f.events), 6)
	assert.Equal(t, []string{"setup:zebra", "setup:alpha", "setup:middle"}, f.events[:3])
	assert.Equal(t, []string{"teardown:zebra", "teardown:alpha", "teardown:middle"}, f.events[3:6])
}

func TestControllerUntilValidation(t *testing.T) {
	f := newFixture("n1")
	tc := f.controller([]*config.TestConfig{
		execTest("known test", "n1", "echo ok", nil),
	}, func(tc *TestController) {
		tc.until = "no such target"
	})
	err := tc.Run()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--until target")
	assert.Contains(t, err.Error(), "known test")
}

func TestControllerUntilStopsAfterTarget(t *testing.T) {
	f := newFixture("n1")
	tc := f.controller([]*config.TestConfig{
		execTest("first", "n1", "echo ok", map[string]interface{}{"exit_code": 0}),
		execTest("second", "n1", "echo ok", map[string]interface{}{"exit_code": 0}),
	}, func(tc *TestController) {
		tc.until = "first"
	})
	require.NoError(t, tc.Run())

	assert.Len(t, f.formatter.tests, 1)
	// until-exit leaves nodes running by design: no teardown events
	for _, event := range f.events {
		assert.NotContains(t, event, "teardown")
	}
}

func TestControllerSetupOnlySkipsTests(t *testing.T) {
	f := newFixture("n1")
	tc := f.controller([]*config.TestConfig{
		execTest("t", "n1", "echo ok", nil),
	}, func(tc *TestController) {
		tc.setupOnly = true
	})
	require.NoError(t, tc.Run())
	assert.Empty(t, f.formatter.tests, "no tests run in setup-only mode")
	for _, event := range f.events {
		assert.NotContains(t, event, "teardown", "setup-only leaves nodes running")
	}
}

func TestControllerTeardownOnlyRunsTeardownSteps(t *testing.T) {
	f := newFixture("n1")
	teardownSteps := []*config.StepConfig{
		{
			Name: "cleanup files",
			Node: config.NodeReference{"n1"},
			Step: config.StepDetails{
				Type:    "execute",
				Options: map[string]interface{}{"command": "echo ok"},
			},
		},
	}
	tc := NewTestController("suite", nil, f.nodes, f.configs, nil, teardownSteps, nil,
		false, false, false, false, false, true, "", "", f.formatter)
	require.NoError(t, tc.Run())

	assert.Contains(t, f.formatter.tasks, "cleanup files@n1", "teardown steps must run in teardown-only mode")
	assert.Contains(t, f.events, "teardown:n1")
}

func TestControllerNodeSetupErrorCleansUpPriorNodes(t *testing.T) {
	f := newFixture("first", "second")
	f.nodes["second"].(*trackingNode).setupErr = errors.New("boom")

	tc := f.controller([]*config.TestConfig{
		execTest("t", "first", "echo ok", nil),
	})
	err := tc.Run()

	require.Error(t, err)
	assert.Contains(t, f.events, "setup:first")
	assert.Contains(t, f.events, "teardown:first", "successfully set-up node must be cleaned up")
	assert.NotContains(t, f.events, "teardown:second", "failed node was never set up")
}

func TestControllerPlatformLifecycle(t *testing.T) {
	f := newFixture("n1")
	platform := &fakePlatform{name: "fake", configured: true}
	tc := NewTestController("suite", []ifaces.PlatformManager{platform}, f.nodes, f.configs,
		nil, nil, []*config.TestConfig{execTest("t", "n1", "echo ok", nil)},
		false, false, false, false, false, false, "", "", f.formatter)

	require.NoError(t, tc.Run())
	assert.Equal(t, 1, platform.setups)
	assert.Equal(t, 1, platform.teardowns)
}

func TestControllerPlatformSetupErrorTornDown(t *testing.T) {
	f := newFixture("n1")
	good := &fakePlatform{name: "good", configured: true}
	bad := &fakePlatform{name: "bad", configured: true, setupErr: errors.New("no daemon")}
	tc := NewTestController("suite", []ifaces.PlatformManager{good, bad}, f.nodes, f.configs,
		nil, nil, nil,
		false, false, false, false, false, false, "", "", f.formatter)

	err := tc.Run()
	require.Error(t, err)
	assert.Equal(t, 1, good.teardowns, "successfully set-up platform must be cleaned up")
	assert.Equal(t, 0, bad.teardowns, "failed platform was never set up")
}

func TestControllerRunErrorAborts(t *testing.T) {
	f := newFixture("n1")
	// No mock response for this command: the node errors, the run aborts
	tc := f.controller([]*config.TestConfig{
		execTest("broken", "n1", "unmapped-command", map[string]interface{}{"exit_code": 0}),
		execTest("after", "n1", "echo ok", nil),
	})
	err := tc.Run()
	require.Error(t, err)
	assert.Len(t, f.formatter.tests, 1, "run aborts on test error")
	assert.Contains(t, f.events, "teardown:n1", "error path still cleans up nodes")
}

// A teardown failure after passing tests must still produce a report:
// CI needs the artifact showing what actually ran.
func TestControllerReportWrittenOnTeardownFailure(t *testing.T) {
	f := newFixture("n1")
	reportPath := filepath.Join(t.TempDir(), "results.json")

	teardownSteps := []*config.StepConfig{{
		Name: "failing cleanup",
		Node: config.NodeReference{"n1"},
		Step: config.StepDetails{
			Type:    "execute",
			Options: map[string]interface{}{"command": "unmapped-cleanup-cmd"},
		},
	}}
	tc := NewTestController("suite", nil, f.nodes, f.configs, nil, teardownSteps,
		[]*config.TestConfig{execTest("passes", "n1", "echo ok", map[string]interface{}{"exit_code": 0})},
		false, false, false, false, false, false, "", "", f.formatter)
	tc.SetReports([]report.Spec{{Format: "json", Path: reportPath}})

	err := tc.Run()
	require.Error(t, err, "teardown failure aborts the run")

	data, readErr := os.ReadFile(reportPath)
	require.NoError(t, readErr, "report must exist despite the teardown failure")
	var r report.Report
	require.NoError(t, json.Unmarshal(data, &r))
	assert.Equal(t, 1, r.Passed, "the passing test's result survives")
}

// stop-on-error still gets its report via the deferred writer.
func TestControllerReportWrittenOnStopOnFail(t *testing.T) {
	f := newFixture("n1")
	reportPath := filepath.Join(t.TempDir(), "results.json")

	tc := f.controller([]*config.TestConfig{
		execTest("fails", "n1", "false", map[string]interface{}{"exit_code": 0}),
	}, func(tc *TestController) {
		tc.stopOnFail = true
	})
	tc.SetReports([]report.Spec{{Format: "json", Path: reportPath}})

	require.Error(t, tc.Run())
	data, err := os.ReadFile(reportPath)
	require.NoError(t, err)
	var r report.Report
	require.NoError(t, json.Unmarshal(data, &r))
	assert.Equal(t, 1, r.Failed)
}

// Iteration-suffixed report paths keep every iteration's outcome.
func TestControllerIterationReportPaths(t *testing.T) {
	f := newFixture("n1")
	dir := t.TempDir()
	base := filepath.Join(dir, "results.json")

	tc := f.controller([]*config.TestConfig{
		execTest("t", "n1", "echo ok", map[string]interface{}{"exit_code": 0}),
	})
	tc.SetReports([]report.Spec{{Format: "json", Path: base}})

	tc.SetReportIteration(1)
	require.NoError(t, tc.Run())
	tc.SetReportIteration(2)
	require.NoError(t, tc.Run())

	assert.FileExists(t, filepath.Join(dir, "results-1.json"))
	assert.FileExists(t, filepath.Join(dir, "results-2.json"))
	assert.NoFileExists(t, base)
}

func taggedTest(name, node string, tags ...string) *config.TestConfig {
	cfg := execTest(name, node, "echo ok", map[string]interface{}{"exit_code": 0})
	cfg.Tags = tags
	return cfg
}

func TestControllerTagFiltering(t *testing.T) {
	f := newFixture("n1")
	tc := f.controller([]*config.TestConfig{
		taggedTest("net test", "n1", "network"),
		taggedTest("smoke test", "n1", "smoke"),
		taggedTest("slow net", "n1", "network", "slow"),
		taggedTest("untagged", "n1"),
	})
	tc.SetTagFilters([]string{"network"}, []string{"slow"})
	require.NoError(t, tc.Run())

	// Only "net test": network-tagged but not slow
	assert.Len(t, f.formatter.tests, 1)
	assert.Contains(t, f.formatter.tests[0], "net test")
	assert.Equal(t, 1, f.formatter.results.pass)
}

func TestControllerSkipTagsOnly(t *testing.T) {
	f := newFixture("n1")
	tc := f.controller([]*config.TestConfig{
		taggedTest("fast", "n1", "smoke"),
		taggedTest("slow", "n1", "slow"),
		taggedTest("untagged", "n1"),
	})
	tc.SetTagFilters(nil, []string{"slow"})
	require.NoError(t, tc.Run())
	assert.Len(t, f.formatter.tests, 2, "untagged and non-slow tests run")
}
