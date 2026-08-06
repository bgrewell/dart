package testtypes

import (
	"fmt"

	"github.com/bgrewell/dart/internal/eval"
	"github.com/bgrewell/dart/internal/helpers"
	"github.com/bgrewell/dart/pkg/ifaces"
)

// newServiceStatusTest checks a systemd unit's state on the node via
// `systemctl is-active`. Options: service (required); evaluate.status
// (string, default "active" — compared against the command's output, so
// "inactive" or "failed" can be asserted too). Other evaluate keys fall
// through to the standard evaluators.
func newServiceStatusTest(base BaseTest, opts map[string]interface{}) (ifaces.Test, error) {
	service, err := requiredString(base.name, opts, "service")
	if err != nil {
		return nil, err
	}

	spec, err := evaluateSpec(base.name, opts)
	if err != nil {
		return nil, err
	}

	evaluations := make(map[string]eval.Evaluate, len(spec))
	status := "active"
	for name, value := range spec {
		if name == "status" {
			s, ok := value.(string)
			if !ok {
				return nil, fmt.Errorf("status must be a string in test %q (got %T)", base.name, value)
			}
			status = s
			continue
		}
		evaluator, err := eval.New(name, value)
		if err != nil {
			return nil, err
		}
		evaluations[name] = evaluator
	}
	evaluations["status"] = &eval.EvaluateMatch{Expected: status, Trim: true}

	base.evaluations = evaluations
	return &commandTest{
		BaseTest: base,
		command:  "systemctl is-active " + helpers.ShellQuote(service),
	}, nil
}
