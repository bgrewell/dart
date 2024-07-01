package eval

import (
	"github.com/bgrewell/dart/internal/execution"
)

type EvaluateResult struct {
	Passed  bool
	Details interface{}
	Err     error
}

type Evaluate interface {
	Verify(execResult *execution.ExecutionResult) (result *EvaluateResult)
}
