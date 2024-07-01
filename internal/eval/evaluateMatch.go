package eval

import (
	"github.com/bgrewell/dart/internal/execution"
	"github.com/bgrewell/dart/internal/results"
	"io"
	"strings"
)

// EvaluateMatch is a struct that contains the expected string to match in the output
type EvaluateMatch struct {
	Expected string
	Trim     bool
}

// Verify is a method that verifies that the expected string matches the output
func (m *EvaluateMatch) Verify(execResult *execution.ExecutionResult) (result *EvaluateResult) {
	actualBytes, err := io.ReadAll(execResult.Stdout)
	if err != nil {
		return &EvaluateResult{
			Passed:  false,
			Details: nil,
			Err:     err,
		}
	}

	actual := string(actualBytes)
	if m.Trim {
		actual = strings.TrimRight(actual, "\n ")
	}

	var details interface{} = actual
	passed := actual == m.Expected
	if !passed {
		details = &results.ResultStringMatchFail{
			Expected: m.Expected,
			Actual:   actual,
		}
	}

	return &EvaluateResult{
		Passed:  passed,
		Details: details,
		Err:     nil,
	}
}
