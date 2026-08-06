package eval

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/bgrewell/dart/internal/execution"
	"github.com/bgrewell/dart/internal/results"
)

// EvaluateNumeric compares stdout, parsed as a number, against a bound.
// Registered as gt / lt / ge / le.
type EvaluateNumeric struct {
	Op    string
	Bound float64
}

var numericOpSymbols = map[string]string{
	"gt": ">",
	"lt": "<",
	"ge": ">=",
	"le": "<=",
}

func newNumeric(op string) Factory {
	return func(value interface{}) (Evaluate, error) {
		bound, err := asFloat(value)
		if err != nil {
			return nil, err
		}
		return &EvaluateNumeric{Op: op, Bound: bound}, nil
	}
}

// Verify is a method that verifies the numeric comparison against the output
func (n *EvaluateNumeric) Verify(execResult *execution.ExecutionResult) (result *EvaluateResult) {
	actual, err := streamStdout.read(execResult)
	if err != nil {
		return errResult(err)
	}

	trimmed := strings.TrimSpace(actual)
	expected := fmt.Sprintf("%s %v", numericOpSymbols[n.Op], n.Bound)

	value, parseErr := strconv.ParseFloat(trimmed, 64)
	if parseErr != nil {
		return &EvaluateResult{
			Passed: false,
			Details: &results.ResultStringMatchFail{
				Expected: fmt.Sprintf("numeric output %s", expected),
				Actual:   trimmed,
			},
			Err: nil,
		}
	}

	var passed bool
	switch n.Op {
	case "gt":
		passed = value > n.Bound
	case "lt":
		passed = value < n.Bound
	case "ge":
		passed = value >= n.Bound
	case "le":
		passed = value <= n.Bound
	}

	var details interface{} = trimmed
	if !passed {
		details = &results.ResultStringMatchFail{
			Expected: expected,
			Actual:   trimmed,
		}
	}

	return &EvaluateResult{
		Passed:  passed,
		Details: details,
		Err:     nil,
	}
}
