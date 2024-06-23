package check

import (
	"github.com/bgrewell/dart/internal/eval"
	"github.com/bgrewell/dart/internal/execution"
	"io"
	"strings"
)

// MatchCheck is a struct that contains the expected string to match in the output
type MatchCheck struct {
	Expected string
	Trim     bool
}

// Verify is a method that verifies that the expected string matches the output
func (m *MatchCheck) Verify(execResult *execution.ExecutionResult) (result *CheckResult) {
	actualBytes, err := io.ReadAll(execResult.Stdout)
	if err != nil {
		return &CheckResult{
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
		details = &eval.EvalStringFailResult{
			Expected: m.Expected,
			Actual:   actual,
		}
	}

	return &CheckResult{
		Passed:  passed,
		Details: details,
		Err:     nil,
	}
}
