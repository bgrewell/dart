package check

import (
	"github.com/bgrewell/dart/internal/execution"
	"io"
	"strings"
)

// ContainsCheck is a struct that contains the expected string to check for in the output
type ContainsCheck struct {
	Expected string
}

// Verify is a method that verifies that the expected string is contained in the output
func (c *ContainsCheck) Verify(execResult *execution.ExecutionResult) (result *CheckResult) {
	actual, err := io.ReadAll(execResult.Stdout)
	if err != nil {
		return &CheckResult{
			Passed:  false,
			Details: nil,
			Err:     err,
		}
	}
	return &CheckResult{
		Passed:  strings.Contains(string(actual), c.Expected),
		Details: string(actual),
		Err:     nil,
	}
}
