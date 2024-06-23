package check

import (
	"github.com/bgrewell/dart/internal/eval"
	"github.com/bgrewell/dart/internal/execution"
)

// ExitCodeCheck is a struct that contains the expected exit code to check for
type ExitCodeCheck struct {
	Expected int
}

// Verify is a method that verifies that the expected exit code matches the actual exit code
func (e *ExitCodeCheck) Verify(execResult *execution.ExecutionResult) (result *CheckResult) {
	passed := execResult.ExitCode == e.Expected
	var details interface{} = execResult.ExitCode
	if !passed {
		details = &eval.EvalIntFailResult{
			Expected: e.Expected,
			Actual:   execResult.ExitCode,
		}
	}

	return &CheckResult{
		Passed:  passed,
		Details: details,
		Err:     nil,
	}
}
