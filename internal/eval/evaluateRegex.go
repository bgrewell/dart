package eval

import (
	"fmt"
	"regexp"

	"github.com/bgrewell/dart/internal/execution"
	"github.com/bgrewell/dart/internal/results"
)

// EvaluateRegex checks the selected output stream against a regular
// expression. The pattern is compiled at config-load time so an invalid
// expression fails the run before any test executes.
type EvaluateRegex struct {
	Pattern *regexp.Regexp
	Source  stream
}

func newRegex(source stream) Factory {
	return func(value interface{}) (Evaluate, error) {
		pattern, err := asString(value)
		if err != nil {
			return nil, err
		}
		re, err := regexp.Compile(pattern)
		if err != nil {
			return nil, fmt.Errorf("invalid pattern: %w", err)
		}
		return &EvaluateRegex{Pattern: re, Source: source}, nil
	}
}

// Verify is a method that verifies the output matches the pattern
func (r *EvaluateRegex) Verify(execResult *execution.ExecutionResult) (result *EvaluateResult) {
	actual, err := r.Source.read(execResult)
	if err != nil {
		return errResult(err)
	}

	passed := r.Pattern.MatchString(actual)
	var details interface{} = actual
	if !passed {
		details = &results.ResultStringMatchFail{
			Expected: fmt.Sprintf("output matching /%s/", r.Pattern.String()),
			Actual:   actual,
		}
	}

	return &EvaluateResult{
		Passed:  passed,
		Details: details,
		Err:     nil,
	}
}
