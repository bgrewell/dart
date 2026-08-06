package testtypes

import (
	"github.com/bgrewell/dart/internal/eval"
	"github.com/bgrewell/dart/pkg/ifaces"
)

// newExecuteTest runs a user-supplied command and applies the standard
// evaluate block to its result.
func newExecuteTest(base BaseTest, opts map[string]interface{}) (ifaces.Test, error) {
	command, err := requiredString(base.name, opts, "command")
	if err != nil {
		return nil, err
	}

	spec, err := evaluateSpec(base.name, opts)
	if err != nil {
		return nil, err
	}
	evaluations, err := eval.Parse(spec)
	if err != nil {
		return nil, err
	}

	base.evaluations = evaluations
	return &commandTest{
		BaseTest: base,
		command:  command,
	}, nil
}
