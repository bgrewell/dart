package eval

import (
	"github.com/bgrewell/dart/internal/execution"
	"io"
	"strings"
)

// EvaluateContains is a struct that contains the expected string to check for in the output
type EvaluateContains struct {
	Expected string
}

// Verify is a method that verifies that the expected string is contained in the output
func (c *EvaluateContains) Verify(execResult *execution.ExecutionResult) (result *EvaluateResult) {
	actual, err := io.ReadAll(execResult.Stdout)
	if err != nil {
		return &EvaluateResult{
			Passed:  false,
			Details: nil,
			Err:     err,
		}
	}
	return &EvaluateResult{
		Passed:  strings.Contains(string(actual), c.Expected),
		Details: string(actual),
		Err:     nil,
	}
}
