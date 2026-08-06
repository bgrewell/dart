package eval

import (
	"fmt"
	"strings"

	"github.com/bgrewell/dart/internal/execution"
	"github.com/bgrewell/dart/internal/results"
)

// EvaluateMatch checks the selected output stream for an exact match.
// Trim removes trailing whitespace from the output before comparing.
type EvaluateMatch struct {
	Expected string
	Trim     bool
	Source   stream
}

// newMatch accepts either a plain string (trailing whitespace trimmed before
// comparison) or a {value, trim} map for exact matching with trim disabled.
func newMatch(source stream) Factory {
	return func(value interface{}) (Evaluate, error) {
		switch v := value.(type) {
		case string:
			return &EvaluateMatch{Expected: v, Trim: true, Source: source}, nil
		case map[string]interface{}:
			raw, ok := v["value"]
			if !ok {
				return nil, fmt.Errorf("missing required key %q", "value")
			}
			expected, err := asString(raw)
			if err != nil {
				return nil, fmt.Errorf("key %q: %w", "value", err)
			}
			trim := true
			if rawTrim, ok := v["trim"]; ok {
				if trim, err = asBool(rawTrim); err != nil {
					return nil, fmt.Errorf("key %q: %w", "trim", err)
				}
			}
			for key := range v {
				if key != "value" && key != "trim" {
					return nil, fmt.Errorf("unknown key %q", key)
				}
			}
			return &EvaluateMatch{Expected: expected, Trim: trim, Source: source}, nil
		default:
			return nil, fmt.Errorf("expected a string or a {value, trim} map, got %T", value)
		}
	}
}

// Verify is a method that verifies that the expected string matches the output
func (m *EvaluateMatch) Verify(execResult *execution.ExecutionResult) (result *EvaluateResult) {
	actual, err := m.Source.read(execResult)
	if err != nil {
		return errResult(err)
	}
	if m.Trim {
		actual = strings.TrimRight(actual, " \t\n\r")
	}

	var details interface{} = actual
	passed := actual == m.Expected
	if !passed {
		details = &results.ResultStringMatchFail{
			Expected: m.Expected,
			Actual:   actual,
		}
	}

	return &EvaluateResult{
		Passed:  passed,
		Details: details,
		Err:     nil,
	}
}
