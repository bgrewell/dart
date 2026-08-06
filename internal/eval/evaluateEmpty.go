package eval

import (
	"strings"

	"github.com/bgrewell/dart/internal/execution"
	"github.com/bgrewell/dart/internal/results"
)

// EvaluateEmpty checks whether the selected output stream is empty, ignoring
// whitespace. Expected false inverts the check (output must be non-empty).
type EvaluateEmpty struct {
	Expected bool
	Source   stream
}

func newEmpty(source stream) Factory {
	return func(value interface{}) (Evaluate, error) {
		expected, err := asBool(value)
		if err != nil {
			return nil, err
		}
		return &EvaluateEmpty{Expected: expected, Source: source}, nil
	}
}

// Verify is a method that verifies the emptiness condition against the output
func (e *EvaluateEmpty) Verify(execResult *execution.ExecutionResult) (result *EvaluateResult) {
	actual, err := e.Source.read(execResult)
	if err != nil {
		return errResult(err)
	}

	isEmpty := strings.TrimSpace(actual) == ""
	passed := isEmpty == e.Expected
	var details interface{} = actual
	if !passed {
		expected := "non-empty output"
		if e.Expected {
			expected = "no output"
		}
		details = &results.ResultStringMatchFail{
			Expected: expected,
			Actual:   actual,
		}
	}

	return &EvaluateResult{
		Passed:  passed,
		Details: details,
		Err:     nil,
	}
}
