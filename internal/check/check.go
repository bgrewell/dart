package check

import (
	"github.com/bgrewell/dart/internal/execution"
)

type CheckResult struct {
	Passed  bool
	Details interface{}
	Err     error
}

type Check interface {
	Verify(execResult *execution.ExecutionResult) (result *CheckResult)
}
