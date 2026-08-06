package eval

import (
	"fmt"
	"time"

	"github.com/bgrewell/dart/internal/execution"
	"github.com/bgrewell/dart/internal/results"
)

// EvaluateMaxDuration checks that the test command completed within a bound.
// The bound is given in seconds (fractional values allowed).
type EvaluateMaxDuration struct {
	Max time.Duration
}

func newMaxDuration(value interface{}) (Evaluate, error) {
	seconds, err := asFloat(value)
	if err != nil {
		return nil, err
	}
	if seconds <= 0 {
		return nil, fmt.Errorf("expected a positive number of seconds, got %v", seconds)
	}
	return &EvaluateMaxDuration{Max: time.Duration(seconds * float64(time.Second))}, nil
}

// Verify is a method that verifies the command completed within the bound
func (d *EvaluateMaxDuration) Verify(execResult *execution.ExecutionResult) (result *EvaluateResult) {
	passed := execResult.Duration <= d.Max
	var details interface{} = execResult.Duration.String()
	if !passed {
		details = &results.ResultStringMatchFail{
			Expected: fmt.Sprintf("completion within %s", d.Max),
			Actual:   execResult.Duration.String(),
		}
	}

	return &EvaluateResult{
		Passed:  passed,
		Details: details,
		Err:     nil,
	}
}
