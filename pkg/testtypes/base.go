package testtypes

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/bgrewell/dart/internal/config"
	"github.com/bgrewell/dart/internal/eval"
	"github.com/bgrewell/dart/internal/execution"
	"github.com/bgrewell/dart/internal/formatters"
	"github.com/bgrewell/dart/pkg/ifaces"
)

const (
	TypeExecute       = "execute"
	TypeExists        = "exists"
	TypeFileContent   = "file_content"
	TypeFileHash      = "file_hash"
	TypeHTTPRequest   = "http_request"
	TypePing          = "ping"
	TypePortCheck     = "port_check"
	TypeServiceStatus = "service_status"
	TypeReboot        = "reboot"
)

// testFactory constructs a test from its base and raw options. Invalid
// options produce a config-time error rather than a runtime failure.
type testFactory func(base BaseTest, opts map[string]interface{}) (ifaces.Test, error)

// testFactories maps test type names, as written in YAML, to their
// factories. New test types register here.
var testFactories = map[string]testFactory{
	TypeExecute:       newExecuteTest,
	TypeExists:        newExistsTest,
	TypeFileContent:   newFileContentTest,
	TypeFileHash:      newFileHashTest,
	TypeHTTPRequest:   newHTTPRequestTest,
	TypePing:          newPingTest,
	TypePortCheck:     newPortCheckTest,
	TypeServiceStatus: newServiceStatusTest,
	TypeReboot:        newRebootTest,
}

type BaseTest struct {
	name         string
	nodeName     string
	node         ifaces.Node
	testType     string
	setup        []string
	teardown     []string
	skipIf       string
	skipUnless   string
	evaluations  map[string]eval.Evaluate
	captures     *captureStore
	captureSpecs []captureSpec
}

func (t *BaseTest) Name() string {
	return t.name
}

func (t *BaseTest) NodeName() string {
	return t.nodeName
}

// ShouldSkip evaluates the test's skip conditions on its node: skip_if
// skips when its command succeeds, skip_unless skips when its command
// fails. An error running a condition command is an error, not a skip —
// a broken condition must never silently pass as green or vanish as
// skipped.
func (t *BaseTest) ShouldSkip() (skip bool, reason string, err error) {
	if t.skipIf != "" {
		cmd, err := t.interpolateCaptures(t.skipIf)
		if err != nil {
			return false, "", err
		}
		result, err := t.node.Execute(cmd)
		if err != nil {
			return false, "", fmt.Errorf("skip_if command failed to run: %w", err)
		}
		if result.ExitCode == 0 {
			return true, fmt.Sprintf("skip_if condition met: %s", cmd), nil
		}
	}
	if t.skipUnless != "" {
		cmd, err := t.interpolateCaptures(t.skipUnless)
		if err != nil {
			return false, "", err
		}
		result, err := t.node.Execute(cmd)
		if err != nil {
			return false, "", fmt.Errorf("skip_unless command failed to run: %w", err)
		}
		if result.ExitCode != 0 {
			return true, fmt.Sprintf("skip_unless condition not met: %s", cmd), nil
		}
	}
	return false, "", nil
}

// runProducer is the shared test flow: run setup commands on the node,
// produce an execution result, always run teardown, then apply the test's
// evaluations. Evaluations run in sorted-name order so reported results are
// deterministic. A teardown failure is surfaced after evaluation so the
// test outcome isn't lost, but it still aborts the run since the system
// state is unknown at that point.
func (t *BaseTest) runProducer(produce func() (*execution.ExecutionResult, error), updater formatters.TestCompleter) (results map[string]*eval.EvaluateResult, err error) {

	// Run pre-execute commands; a failure here fails the test before it runs
	updater.Update("preparing")
	for _, cmd := range t.setup {
		if _, err = t.node.Execute(cmd); err != nil {
			updater.Error()
			return nil, err
		}
	}

	updater.Update("running")
	start := time.Now()
	testResult, testErr := produce()
	if testResult != nil && testResult.Duration == 0 {
		testResult.Duration = time.Since(start)
	}

	// Post-execute commands always run, even after a test failure, since
	// they are part of cleanup
	updater.Update("cleanup")
	var teardownErr error
	for _, cmd := range t.teardown {
		if _, cmdErr := t.node.Execute(cmd); cmdErr != nil {
			teardownErr = cmdErr
			break
		}
	}

	if testErr != nil {
		updater.Error()
		return nil, testErr
	}

	// Drain both streams up front so buffered output is captured even when
	// no output-based evaluations are configured
	_, _ = testResult.StdoutBytes()
	_, _ = testResult.StderrBytes()

	// Record captured values for later tests. A capture that cannot be
	// extracted is an error: downstream tests depend on it.
	if len(t.captureSpecs) > 0 {
		stdout, _ := testResult.StdoutBytes()
		for _, spec := range t.captureSpecs {
			value := strings.TrimSpace(string(stdout))
			if spec.ext != nil {
				var extractErr error
				value, extractErr = spec.ext.extract(string(stdout))
				if extractErr != nil {
					updater.Error()
					return nil, fmt.Errorf("capture %q in test %q: %w", spec.name, t.name, extractErr)
				}
			}
			t.captures.set(spec.name, value)
		}
	}

	names := make([]string, 0, len(t.evaluations))
	for name := range t.evaluations {
		names = append(names, name)
	}
	sort.Strings(names)

	results = make(map[string]*eval.EvaluateResult, len(names))
	passed := make([]bool, 0, len(names))
	for _, name := range names {
		result := t.evaluations[name].Verify(testResult)
		passed = append(passed, result.Passed)
		results[name] = result
	}

	if teardownErr != nil {
		updater.Error()
		return results, teardownErr
	}

	updater.Complete(passed)
	return results, nil
}

// runCommand is runProducer for the common case of a single node command.
// Capture references ({{capture.name}}) in the command resolve against
// values recorded by earlier tests.
func (t *BaseTest) runCommand(command string, updater formatters.TestCompleter) (map[string]*eval.EvaluateResult, error) {
	command, err := t.interpolateCaptures(command)
	if err != nil {
		updater.Error()
		return nil, err
	}
	return t.runProducer(func() (*execution.ExecutionResult, error) {
		return t.node.Execute(command)
	}, updater)
}

var _ ifaces.Test = &commandTest{}

// commandTest runs a node command and applies evaluations. Most test types
// are specializations that derive the command and checks from their options.
type commandTest struct {
	BaseTest
	command string
}

func (t *commandTest) Run(updater formatters.TestCompleter) (map[string]*eval.EvaluateResult, error) {
	return t.runCommand(t.command, updater)
}

// CreateTests creates a slice of Test objects from a slice of TestConfig objects
func CreateTests(configs []*config.TestConfig, nodes map[string]ifaces.Node) (tests []ifaces.Test, err error) {
	tests = make([]ifaces.Test, 0)

	// Sort tests by order; a stable sort preserves config-file order for
	// tests that share an Order value
	sort.SliceStable(configs, func(i, j int) bool {
		return configs[i].Order < configs[j].Order
	})

	// One capture store per suite: values recorded by earlier tests are
	// available to later ones
	captures := newCaptureStore()

	for _, cfg := range configs {

		// After expansion, each config has exactly one node
		nodeName := cfg.Node[0]
		node, ok := nodes[nodeName]
		if !ok {
			return nil, &config.ConfigError{
				Message:  fmt.Sprintf("node %q not found (referenced in test %q)", nodeName, cfg.Name),
				Location: cfg.NodeLoc,
			}
		}

		factory, ok := testFactories[cfg.Type]
		if !ok {
			return nil, &config.ConfigError{
				Message:  fmt.Sprintf("unknown test type %q", cfg.Type),
				Location: cfg.TypeLoc,
			}
		}

		base := BaseTest{
			name:       cfg.Name,
			nodeName:   nodeName,
			node:       node,
			testType:   cfg.Type,
			setup:      cfg.Setup,
			teardown:   cfg.Teardown,
			skipIf:     cfg.SkipIf,
			skipUnless: cfg.SkipUnless,
			captures:   captures,
		}

		test, err := factory(base, cfg.Options)
		if err != nil {
			return nil, err
		}
		tests = append(tests, test)

	}
	return tests, nil
}

// The opt* helpers validate raw option values for tests. A
// present-but-wrong-typed option is a config error, never a silent zero
// value. Keys are tried in order so documented aliases (path/filename) work.

func optString(testName string, opts map[string]interface{}, keys ...string) (value string, present bool, err error) {
	for _, key := range keys {
		raw, ok := opts[key]
		if !ok {
			continue
		}
		s, ok := raw.(string)
		if !ok {
			return "", true, fmt.Errorf("%s must be a string in test %q (got %T)", key, testName, raw)
		}
		return s, true, nil
	}
	return "", false, nil
}

func requiredString(testName string, opts map[string]interface{}, keys ...string) (string, error) {
	value, present, err := optString(testName, opts, keys...)
	if err != nil {
		return "", err
	}
	if !present || value == "" {
		return "", fmt.Errorf("%s is required in test %q", keys[0], testName)
	}
	return value, nil
}

func coerceInt(v interface{}) (int, bool) {
	switch n := v.(type) {
	case int:
		return n, true
	case int64:
		return int(n), true
	case float64:
		if n != float64(int(n)) {
			return 0, false
		}
		return int(n), true
	}
	return 0, false
}

func coerceFloat(v interface{}) (float64, bool) {
	switch n := v.(type) {
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	case float64:
		return n, true
	}
	return 0, false
}

func optInt(testName string, opts map[string]interface{}, key string, def int) (int, error) {
	raw, ok := opts[key]
	if !ok {
		return def, nil
	}
	n, ok := coerceInt(raw)
	if !ok {
		return 0, fmt.Errorf("%s must be an integer in test %q (got %v)", key, testName, raw)
	}
	return n, nil
}

func optFloat(testName string, opts map[string]interface{}, key string, def float64) (float64, error) {
	raw, ok := opts[key]
	if !ok {
		return def, nil
	}
	f, ok := coerceFloat(raw)
	if !ok {
		return 0, fmt.Errorf("%s must be a number in test %q (got %v)", key, testName, raw)
	}
	return f, nil
}

// evaluateSpec returns the raw `evaluate` block, or an empty map when absent.
func evaluateSpec(testName string, opts map[string]interface{}) (map[string]interface{}, error) {
	raw, ok := opts["evaluate"]
	if !ok {
		return map[string]interface{}{}, nil
	}
	spec, ok := raw.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("evaluate must be a map in test %q (got %T)", testName, raw)
	}
	return spec, nil
}
