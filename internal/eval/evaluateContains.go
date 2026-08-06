package eval

import (
	"fmt"
	"strings"

	"github.com/bgrewell/dart/internal/execution"
	"github.com/bgrewell/dart/internal/results"
)

// EvaluateContains checks the selected output stream for a substring.
// With Negate set, the check passes only when the substring is absent.
type EvaluateContains struct {
	Expected string
	Negate   bool
	Source   stream
}

func newContains(source stream, negate bool) Factory {
	return func(value interface{}) (Evaluate, error) {
		expected, err := asString(value)
		if err != nil {
			return nil, err
		}
		return &EvaluateContains{Expected: expected, Negate: negate, Source: source}, nil
	}
}

// Verify is a method that verifies the substring condition against the output
func (c *EvaluateContains) Verify(execResult *execution.ExecutionResult) (result *EvaluateResult) {
	actual, err := c.Source.read(execResult)
	if err != nil {
		return errResult(err)
	}

	passed := strings.Contains(actual, c.Expected) != c.Negate
	var details interface{} = actual
	if !passed {
		qualifier := "output containing"
		if c.Negate {
			qualifier = "output not containing"
		}
		details = &results.ResultStringMatchFail{
			Expected: fmt.Sprintf("%s %q", qualifier, c.Expected),
			Actual:   actual,
		}
	}

	return &EvaluateResult{
		Passed:  passed,
		Details: details,
		Err:     nil,
	}
}
