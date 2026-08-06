package testtypes

import (
	"fmt"

	"github.com/bgrewell/dart/internal/eval"
	"github.com/bgrewell/dart/internal/helpers"
	"github.com/bgrewell/dart/pkg/ifaces"
)

// newExistsTest checks for a path's existence on the node via `test -e`.
// Options: path (alias filename); evaluate.exists (bool, default true).
// Other evaluate keys fall through to the standard evaluators.
func newExistsTest(base BaseTest, opts map[string]interface{}) (ifaces.Test, error) {
	path, err := requiredString(base.name, opts, "path", "filename")
	if err != nil {
		return nil, err
	}

	spec, err := evaluateSpec(base.name, opts)
	if err != nil {
		return nil, err
	}

	evaluations := make(map[string]eval.Evaluate, len(spec))
	expected := true
	for name, value := range spec {
		if name == "exists" {
			b, ok := value.(bool)
			if !ok {
				return nil, fmt.Errorf("exists must be a boolean in test %q (got %T)", base.name, value)
			}
			expected = b
			continue
		}
		evaluator, err := eval.New(name, value)
		if err != nil {
			return nil, err
		}
		evaluations[name] = evaluator
	}
	evaluations["exists"] = &existsCheck{expected: expected}

	base.evaluations = evaluations
	return &commandTest{
		BaseTest: base,
		command:  "test -e " + helpers.ShellQuote(path),
	}, nil
}
