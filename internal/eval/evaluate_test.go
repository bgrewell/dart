package eval

import (
	"strings"
	"testing"
	"time"

	"github.com/bgrewell/dart/internal/execution"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func execResult(exitCode int, stdout, stderr string) *execution.ExecutionResult {
	return &execution.ExecutionResult{
		ExitCode: exitCode,
		Stdout:   strings.NewReader(stdout),
		Stderr:   strings.NewReader(stderr),
	}
}

func mustNew(t *testing.T, name string, value interface{}) Evaluate {
	t.Helper()
	evaluator, err := New(name, value)
	require.NoError(t, err)
	return evaluator
}

// Multiple output-based evaluators on one result must all see the full
// output; the underlying streams are one-shot readers.
func TestMultipleEvaluatorsShareOutput(t *testing.T) {
	evaluations, err := Parse(map[string]interface{}{
		"match":    "hello world",
		"contains": "world",
		"regex":    "^hello",
	})
	require.NoError(t, err)

	result := execResult(0, "hello world\n", "")
	for name, evaluator := range evaluations {
		verdict := evaluator.Verify(result)
		assert.NoError(t, verdict.Err, name)
		assert.True(t, verdict.Passed, name)
	}
}

func TestParseErrors(t *testing.T) {
	cases := map[string]map[string]interface{}{
		"unknown type":            {"bogus": "x"},
		"match non-string":        {"match": 123.0},
		"match unknown map key":   {"match": map[string]interface{}{"value": "x", "trmi": true}},
		"match map missing value": {"match": map[string]interface{}{"trim": false}},
		"contains non-string":     {"contains": 5.0},
		"exit_code fractional":    {"exit_code": 1.7},
		"exit_code string":        {"exit_code": "zero"},
		"exit_code empty list":    {"exit_code": []interface{}{}},
		"regex invalid pattern":   {"regex": "("},
		"line_count negative":     {"line_count": -1.0},
		"max_duration zero":       {"max_duration": 0.0},
		"json_path non-map":       {"json_path": "a.b"},
		"json_path missing path":  {"json_path": map[string]interface{}{"equals": 1.0}},
		"json_path bad path":      {"json_path": map[string]interface{}{"path": "a..b", "equals": 1.0}},
		"empty non-bool":          {"empty": "yes"},
	}
	for name, spec := range cases {
		_, err := Parse(spec)
		assert.Error(t, err, name)
	}
}

func TestExitCode(t *testing.T) {
	single := mustNew(t, "exit_code", 0.0)
	assert.True(t, single.Verify(execResult(0, "", "")).Passed)
	assert.False(t, single.Verify(execResult(1, "", "")).Passed)

	list := mustNew(t, "exit_code", []interface{}{0.0, 1.0})
	assert.True(t, list.Verify(execResult(1, "", "")).Passed)
	assert.False(t, list.Verify(execResult(2, "", "")).Passed)

	negated := mustNew(t, "exit_code_not", 0.0)
	assert.False(t, negated.Verify(execResult(0, "", "")).Passed)
	assert.True(t, negated.Verify(execResult(3, "", "")).Passed)
}

func TestMatch(t *testing.T) {
	trimmed := mustNew(t, "match", "hello")
	assert.True(t, trimmed.Verify(execResult(0, "hello \t\r\n", "")).Passed)
	assert.False(t, trimmed.Verify(execResult(0, "hello!", "")).Passed)

	exact := mustNew(t, "match", map[string]interface{}{"value": "hello\n", "trim": false})
	assert.True(t, exact.Verify(execResult(0, "hello\n", "")).Passed)
	assert.False(t, exact.Verify(execResult(0, "hello", "")).Passed)

	stderr := mustNew(t, "stderr_match", "oops")
	assert.True(t, stderr.Verify(execResult(0, "", "oops\n")).Passed)
	assert.False(t, stderr.Verify(execResult(0, "oops", "")).Passed)
}

func TestContains(t *testing.T) {
	contains := mustNew(t, "contains", "needle")
	assert.True(t, contains.Verify(execResult(0, "hay needle hay", "")).Passed)
	assert.False(t, contains.Verify(execResult(0, "hay hay", "")).Passed)

	negated := mustNew(t, "not_contains", "error")
	assert.True(t, negated.Verify(execResult(0, "all good", "")).Passed)
	assert.False(t, negated.Verify(execResult(0, "an error occurred", "")).Passed)

	stderr := mustNew(t, "stderr_contains", "warning")
	assert.True(t, stderr.Verify(execResult(0, "", "warning: x")).Passed)
	assert.False(t, stderr.Verify(execResult(0, "warning: x", "")).Passed)
}

func TestRegex(t *testing.T) {
	re := mustNew(t, "regex", `^v\d+\.\d+\.\d+$`)
	assert.True(t, re.Verify(execResult(0, "v1.2.3", "")).Passed)
	assert.False(t, re.Verify(execResult(0, "version 1.2.3", "")).Passed)
}

func TestEmpty(t *testing.T) {
	empty := mustNew(t, "empty", true)
	assert.True(t, empty.Verify(execResult(0, " \n", "")).Passed)
	assert.False(t, empty.Verify(execResult(0, "output", "")).Passed)

	nonEmpty := mustNew(t, "empty", false)
	assert.True(t, nonEmpty.Verify(execResult(0, "output", "")).Passed)
	assert.False(t, nonEmpty.Verify(execResult(0, "", "")).Passed)

	stderrEmpty := mustNew(t, "stderr_empty", true)
	assert.True(t, stderrEmpty.Verify(execResult(0, "noise", "")).Passed)
	assert.False(t, stderrEmpty.Verify(execResult(0, "", "oops")).Passed)
}

func TestLineCount(t *testing.T) {
	three := mustNew(t, "line_count", 3.0)
	assert.True(t, three.Verify(execResult(0, "a\nb\nc\n", "")).Passed)
	assert.True(t, three.Verify(execResult(0, "a\nb\nc", "")).Passed)
	assert.False(t, three.Verify(execResult(0, "a\nb\n", "")).Passed)

	zero := mustNew(t, "line_count", 0.0)
	assert.True(t, zero.Verify(execResult(0, "", "")).Passed)
	assert.False(t, zero.Verify(execResult(0, "x", "")).Passed)
}

func TestNumeric(t *testing.T) {
	gt := mustNew(t, "gt", 10.0)
	assert.True(t, gt.Verify(execResult(0, "42\n", "")).Passed)
	assert.False(t, gt.Verify(execResult(0, "10", "")).Passed)

	ge := mustNew(t, "ge", 10.0)
	assert.True(t, ge.Verify(execResult(0, "10", "")).Passed)

	lt := mustNew(t, "lt", 0.5)
	assert.True(t, lt.Verify(execResult(0, "0.25", "")).Passed)
	assert.False(t, lt.Verify(execResult(0, "0.75", "")).Passed)

	le := mustNew(t, "le", 5.0)
	assert.True(t, le.Verify(execResult(0, "5", "")).Passed)

	// Non-numeric output fails the check rather than erroring
	verdict := gt.Verify(execResult(0, "not a number", ""))
	assert.False(t, verdict.Passed)
	assert.NoError(t, verdict.Err)
}

func TestMaxDuration(t *testing.T) {
	bound := mustNew(t, "max_duration", 2.0)

	fast := execResult(0, "", "")
	fast.Duration = 500 * time.Millisecond
	assert.True(t, bound.Verify(fast).Passed)

	slow := execResult(0, "", "")
	slow.Duration = 3 * time.Second
	assert.False(t, bound.Verify(slow).Passed)
}

func TestJSONPath(t *testing.T) {
	doc := `{"result": {"status": "ok", "items": [{"name": "first"}, {"name": "second"}], "count": 2}}`

	status := mustNew(t, "json_path", map[string]interface{}{"path": "result.status", "equals": "ok"})
	assert.True(t, status.Verify(execResult(0, doc, "")).Passed)

	item := mustNew(t, "json_path", map[string]interface{}{"path": "result.items[1].name", "equals": "second"})
	assert.True(t, item.Verify(execResult(0, doc, "")).Passed)

	// Integer expected values compare numerically against JSON floats
	count := mustNew(t, "json_path", map[string]interface{}{"path": "result.count", "equals": 2.0})
	assert.True(t, count.Verify(execResult(0, doc, "")).Passed)

	mismatch := mustNew(t, "json_path", map[string]interface{}{"path": "result.status", "equals": "failed"})
	assert.False(t, mismatch.Verify(execResult(0, doc, "")).Passed)

	missing := mustNew(t, "json_path", map[string]interface{}{"path": "result.absent", "equals": "x"})
	assert.False(t, missing.Verify(execResult(0, doc, "")).Passed)

	invalid := mustNew(t, "json_path", map[string]interface{}{"path": "result.status", "equals": "ok"})
	assert.False(t, invalid.Verify(execResult(0, "not json", "")).Passed)
}

func TestExitCodeAcceptsNativeInts(t *testing.T) {
	evaluator := mustNew(t, "exit_code", 0)
	assert.True(t, evaluator.Verify(execResult(0, "", "")).Passed)

	list := mustNew(t, "exit_code", []interface{}{0, 1})
	assert.True(t, list.Verify(execResult(1, "", "")).Passed)
}
