package eval

import (
	"github.com/bgrewell/dart/internal/execution"
	"github.com/bgrewell/dart/internal/results"
)

// EvaluateExitCode is a struct that contains the expected exit code to check for
type EvaluateExitCode struct {
	Expected int
}

// Verify is a method that verifies that the expected exit code matches the actual exit code
func (e *EvaluateExitCode) Verify(execResult *execution.ExecutionResult) (result *EvaluateResult) {
	passed := execResult.ExitCode == e.Expected
	var details interface{} = execResult.ExitCode
	if !passed {
		details = &results.ResultIntMatchFail{
			Expected: e.Expected,
			Actual:   execResult.ExitCode,
		}
	}

	return &EvaluateResult{
		Passed:  passed,
		Details: details,
		Err:     nil,
	}
}
