package eval

import (
	"fmt"

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

// Factory constructs an evaluator from the raw value of its `evaluate` key.
// Invalid values produce a config-time error rather than a runtime failure.
type Factory func(value interface{}) (Evaluate, error)

// registry maps evaluation names, as written in YAML `evaluate` blocks, to
// their factories. New evaluation types are added here and become available
// to every test type that parses its checks through Parse.
var registry = map[string]Factory{
	"exit_code":       newExitCode(false),
	"exit_code_not":   newExitCode(true),
	"match":           newMatch(streamStdout),
	"stderr_match":    newMatch(streamStderr),
	"contains":        newContains(streamStdout, false),
	"not_contains":    newContains(streamStdout, true),
	"stderr_contains": newContains(streamStderr, false),
	"regex":           newRegex(streamStdout),
	"stderr_regex":    newRegex(streamStderr),
	"empty":           newEmpty(streamStdout),
	"stderr_empty":    newEmpty(streamStderr),
	"line_count":      newLineCount,
	"gt":              newNumeric("gt"),
	"lt":              newNumeric("lt"),
	"ge":              newNumeric("ge"),
	"le":              newNumeric("le"),
	"max_duration":    newMaxDuration,
	"json_path":       newJSONPath,
}

// New constructs the evaluator registered under name.
func New(name string, value interface{}) (Evaluate, error) {
	factory, ok := registry[name]
	if !ok {
		return nil, fmt.Errorf("unknown evaluation type %q", name)
	}
	evaluator, err := factory(value)
	if err != nil {
		return nil, fmt.Errorf("evaluation %q: %w", name, err)
	}
	return evaluator, nil
}

// Parse builds the evaluator set for an `evaluate` block.
func Parse(spec map[string]interface{}) (map[string]Evaluate, error) {
	evaluations := make(map[string]Evaluate, len(spec))
	for name, value := range spec {
		evaluator, err := New(name, value)
		if err != nil {
			return nil, err
		}
		evaluations[name] = evaluator
	}
	return evaluations, nil
}

// stream selects which output stream an evaluator inspects. Reads go through
// the cached ExecutionResult accessors so multiple evaluators on one test all
// see the full output despite the underlying streams being one-shot.
type stream int

const (
	streamStdout stream = iota
	streamStderr
)

func (s stream) read(execResult *execution.ExecutionResult) (string, error) {
	var data []byte
	var err error
	if s == streamStderr {
		data, err = execResult.StderrBytes()
	} else {
		data, err = execResult.StdoutBytes()
	}
	return string(data), err
}

func errResult(err error) *EvaluateResult {
	return &EvaluateResult{Passed: false, Err: err}
}

// asString / asInt / asIntList / asFloat / asBool normalize raw option
// values. Numbers arrive as float64 after the JSON round-trip in the config
// layer but may be native ints when the options map is built in Go.

func asString(v interface{}) (string, error) {
	s, ok := v.(string)
	if !ok {
		return "", fmt.Errorf("expected a string, got %T", v)
	}
	return s, nil
}

func asInt(v interface{}) (int, error) {
	switch n := v.(type) {
	case int:
		return n, nil
	case int64:
		return int(n), nil
	case float64:
		if n != float64(int(n)) {
			return 0, fmt.Errorf("expected an integer, got %v", n)
		}
		return int(n), nil
	default:
		return 0, fmt.Errorf("expected an integer, got %T", v)
	}
}

func asIntList(v interface{}) ([]int, error) {
	if list, ok := v.([]interface{}); ok {
		if len(list) == 0 {
			return nil, fmt.Errorf("expected at least one integer")
		}
		values := make([]int, 0, len(list))
		for _, item := range list {
			n, err := asInt(item)
			if err != nil {
				return nil, err
			}
			values = append(values, n)
		}
		return values, nil
	}
	n, err := asInt(v)
	if err != nil {
		return nil, err
	}
	return []int{n}, nil
}

func asFloat(v interface{}) (float64, error) {
	switch n := v.(type) {
	case int:
		return float64(n), nil
	case int64:
		return float64(n), nil
	case float64:
		return n, nil
	default:
		return 0, fmt.Errorf("expected a number, got %T", v)
	}
}

func asBool(v interface{}) (bool, error) {
	b, ok := v.(bool)
	if !ok {
		return false, fmt.Errorf("expected a boolean, got %T", v)
	}
	return b, nil
}
