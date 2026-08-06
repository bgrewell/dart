package eval

import (
	"fmt"
	"strings"

	"github.com/bgrewell/dart/internal/execution"
	"github.com/bgrewell/dart/internal/results"
)

// EvaluateLineCount checks the number of lines on stdout. Trailing newlines
// are ignored, so `printf "a\nb\n"` and `printf "a\nb"` both count as 2.
type EvaluateLineCount struct {
	Expected int
}

func newLineCount(value interface{}) (Evaluate, error) {
	expected, err := asInt(value)
	if err != nil {
		return nil, err
	}
	if expected < 0 {
		return nil, fmt.Errorf("expected a non-negative integer, got %d", expected)
	}
	return &EvaluateLineCount{Expected: expected}, nil
}

// Verify is a method that verifies the line count of the output
func (l *EvaluateLineCount) Verify(execResult *execution.ExecutionResult) (result *EvaluateResult) {
	actual, err := streamStdout.read(execResult)
	if err != nil {
		return errResult(err)
	}

	trimmed := strings.TrimRight(actual, "\r\n")
	count := 0
	if trimmed != "" {
		count = strings.Count(trimmed, "\n") + 1
	}

	passed := count == l.Expected
	var details interface{} = count
	if !passed {
		details = &results.ResultIntMatchFail{
			Expected: l.Expected,
			Actual:   count,
		}
	}

	return &EvaluateResult{
		Passed:  passed,
		Details: details,
		Err:     nil,
	}
}
