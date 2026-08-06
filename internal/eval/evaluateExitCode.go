package eval

import (
	"fmt"
	"strconv"

	"github.com/bgrewell/dart/internal/execution"
	"github.com/bgrewell/dart/internal/results"
)

// EvaluateExitCode checks the exit code against a set of accepted values.
// With Negate set, the check passes only when the exit code is not in the set.
type EvaluateExitCode struct {
	Expected []int
	Negate   bool
}

// newExitCode accepts a single integer or a list of integers.
func newExitCode(negate bool) Factory {
	return func(value interface{}) (Evaluate, error) {
		expected, err := asIntList(value)
		if err != nil {
			return nil, err
		}
		return &EvaluateExitCode{Expected: expected, Negate: negate}, nil
	}
}

// Verify is a method that verifies that the exit code satisfies the check
func (e *EvaluateExitCode) Verify(execResult *execution.ExecutionResult) (result *EvaluateResult) {
	matched := false
	for _, want := range e.Expected {
		if execResult.ExitCode == want {
			matched = true
			break
		}
	}
	passed := matched != e.Negate

	var details interface{} = execResult.ExitCode
	if !passed {
		if len(e.Expected) == 1 && !e.Negate {
			details = &results.ResultIntMatchFail{
				Expected: e.Expected[0],
				Actual:   execResult.ExitCode,
			}
		} else {
			qualifier := "one of"
			if e.Negate {
				qualifier = "none of"
			}
			details = &results.ResultStringMatchFail{
				Expected: fmt.Sprintf("%s %v", qualifier, e.Expected),
				Actual:   strconv.Itoa(execResult.ExitCode),
			}
		}
	}

	return &EvaluateResult{
		Passed:  passed,
		Details: details,
		Err:     nil,
	}
}
