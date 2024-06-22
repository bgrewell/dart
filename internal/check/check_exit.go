package check

import (
	"github.com/bgrewell/dart/internal/execution"
)

// ExitCodeCheck is a struct that contains the expected exit code to check for
type ExitCodeCheck struct {
	Expected int
}

// Verify is a method that verifies that the expected exit code matches the actual exit code
func (e *ExitCodeCheck) Verify(execResult *execution.ExecutionResult) (result *CheckResult) {
	return &CheckResult{
		Passed:  execResult.ExitCode == e.Expected,
		Details: execResult.ExitCode,
		Err:     nil,
	}
}
