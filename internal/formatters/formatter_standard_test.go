package formatters

import (
	"bytes"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/bgrewell/dart/internal/results"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func captureFormatter() (*StandardFormatter, *bytes.Buffer) {
	buf := &bytes.Buffer{}
	return NewStandardFormatterWithWriter(buf), buf
}

func TestPrintHeader(t *testing.T) {
	sf, buf := captureFormatter()
	sf.PrintHeader("Running tests")
	assert.Contains(t, buf.String(), "[+] Running tests")
}

func TestPrintPassDetails(t *testing.T) {
	sf, buf := captureFormatter()
	sf.PrintPass("match", "hello\nworld")
	out := buf.String()
	assert.Contains(t, out, "+match:")
	assert.Contains(t, out, "hello")
	assert.Contains(t, out, "world")

	buf.Reset()
	sf.PrintPass("exit_code", 3)
	assert.Contains(t, buf.String(), "3")
}

func TestPrintFailStructuredDetails(t *testing.T) {
	sf, buf := captureFormatter()
	sf.PrintFail("match", &results.ResultStringMatchFail{Expected: "want", Actual: "got"})
	out := buf.String()
	assert.Contains(t, out, "-match:")
	assert.Contains(t, out, "Expected: want")
	assert.Contains(t, out, "Actual: got")

	buf.Reset()
	sf.PrintFail("exit_code", &results.ResultIntMatchFail{Expected: 0, Actual: 2})
	out = buf.String()
	assert.Contains(t, out, "Expected: 0")
	assert.Contains(t, out, "Actual: 2")
}

// Unknown detail types render via fmt.Sprint instead of printing nothing.
func TestPrintFailUnknownDetailType(t *testing.T) {
	sf, buf := captureFormatter()
	sf.PrintFail("weird", 12.5)
	assert.Contains(t, buf.String(), "12.5")

	buf.Reset()
	sf.PrintFail("nil details", nil)
	assert.Contains(t, buf.String(), "-nil details:")
}

func TestPrintSkip(t *testing.T) {
	sf, buf := captureFormatter()
	sf.PrintSkip("conditional", "skip_if condition met: true")
	out := buf.String()
	assert.Contains(t, out, "~conditional:")
	assert.Contains(t, out, "skip_if condition met")
}

func TestPrintError(t *testing.T) {
	sf, buf := captureFormatter()
	sf.PrintError(errors.New("node exploded"))
	assert.Contains(t, buf.String(), "node exploded")
}

func TestPrintResultsCounts(t *testing.T) {
	sf, buf := captureFormatter()
	sf.PrintResults(3, 1, 2, 4, 1500*time.Millisecond)
	out := buf.String()
	assert.Contains(t, out, "Pass: 00003")
	assert.Contains(t, out, "Fail: 00001")
	assert.Contains(t, out, "Skip: 00002")
	assert.Contains(t, out, "Ran:  00004")
}

func TestPrintResultsOmitsZeroSkipAndRan(t *testing.T) {
	sf, buf := captureFormatter()
	sf.PrintResults(1, 0, 0, 0, 0)
	out := buf.String()
	assert.NotContains(t, out, "Skip:")
	assert.NotContains(t, out, "Ran:")
}

// Output through a buffer must not contain raw ANSI escapes: the color
// package disables itself for non-terminal writers, and node-name
// rendering must follow it rather than hardcoding escape codes.
func TestNoAnsiEscapesInCapturedOutput(t *testing.T) {
	sf, buf := captureFormatter()
	sf.SetNodeNameWidth(8)
	sf.SetTestColumnWidth(20)

	completer := sf.StartTest("1", "escape check", "node-a")
	completer.Complete([]bool{true})
	sf.PrintPass("check", "value")
	sf.PrintResults(1, 0, 0, 0, 0)

	require.NotEmpty(t, buf.String())
	assert.NotContains(t, buf.String(), "\033[38;5;", "raw 256-color escapes must not reach non-terminal output")
}

func TestPadRightWithPeriods(t *testing.T) {
	assert.Equal(t, "name ... ", padRightWithPeriods("name", 3))
	// A name longer than the column still gets at least one period
	assert.Equal(t, "long-name . ", padRightWithPeriods("long-name", -5))
}

func TestFormatNodeBox(t *testing.T) {
	sf, _ := captureFormatter()
	assert.Empty(t, sf.formatNodeBox("node"), "no node box without a width")

	sf.SetNodeNameWidth(6)
	box := sf.formatNodeBox("db")
	assert.Contains(t, box, "[ ")
	assert.Contains(t, box, "db    ")
	assert.Contains(t, box, " ]")
}

// Six-digit test IDs must not panic the zero-padding.
func TestStartTestLongIdDoesNotPanic(t *testing.T) {
	sf, _ := captureFormatter()
	completer := sf.StartTest("123456", "long id", "n")
	completer.Complete([]bool{true})
}

func TestCompleterStates(t *testing.T) {
	sf, buf := captureFormatter()
	sf.SetTestColumnWidth(10)

	sf.StartTest("1", "passes", "n").Complete([]bool{true, true})
	sf.StartTest("2", "fails", "n").Complete([]bool{true, false})
	sf.StartTest("3", "ran only", "n").Complete(nil)
	sf.StartTest("4", "skipped", "n").Skip()

	out := buf.String()
	assert.Contains(t, out, "passed")
	assert.Contains(t, out, "failed")
	assert.Contains(t, out, "ran")
	assert.Contains(t, out, "skipped")
}

func TestTaskCompleterStates(t *testing.T) {
	sf, buf := captureFormatter()
	sf.SetTaskColumnWidth(10)

	sf.StartTask("succeeds", "n", "running").Complete()
	sf.StartTask("errors", "n", "running").Error()

	out := buf.String()
	assert.Contains(t, out, "done")
	assert.Contains(t, out, "error")
	assert.True(t, strings.Contains(out, "succeeds"))
}
