package testtypes

import (
	"time"

	"fmt"

	"github.com/bgrewell/dart/internal/eval"
	"github.com/bgrewell/dart/pkg/ifaces"
)

// newExecuteTest runs a user-supplied command and applies the standard
// evaluate block to its result. Options beyond command/evaluate:
//
//   - extract: named values pulled from stdout via {jsonpath} or {regex}
//     specs; an evaluate entry with an extracted name takes a comparator
//     map ({gte, lte, eq, ne, within+tolerance_pct, ...}) applied to that
//     value instead of a standard evaluation type.
//   - capture: record values for later tests — a bare name captures the
//     whole trimmed stdout, a map of name → extractor captures pieces.
//     Later tests reference them as {{capture.name}} in command, skip_if,
//     and skip_unless.
func newExecuteTest(base BaseTest, opts map[string]interface{}) (ifaces.Test, error) {
	command, err := requiredString(base.name, opts, "command")
	if err != nil {
		return nil, err
	}
	timeoutSeconds, err := optFloat(base.name, opts, "timeout", 0)
	if err != nil {
		return nil, err
	}
	if timeoutSeconds < 0 {
		return nil, fmt.Errorf("timeout must be non-negative in test %q", base.name)
	}

	extractors := map[string]extractor{}
	noteOption("extract")
	if raw, ok := opts["extract"]; ok {
		spec, ok := raw.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("extract must be a map in test %q (got %T)", base.name, raw)
		}
		for name, extRaw := range spec {
			ext, err := parseExtractor(base.name, name, extRaw)
			if err != nil {
				return nil, err
			}
			extractors[name] = ext
		}
	}

	spec, err := evaluateSpec(base.name, opts)
	if err != nil {
		return nil, err
	}

	evaluations := make(map[string]eval.Evaluate, len(spec))
	for name, value := range spec {
		if ext, ok := extractors[name]; ok {
			comparators, err := parseComparators(base.name, name, value)
			if err != nil {
				return nil, err
			}
			evaluations[name] = &valueCheck{name: name, ext: ext, comparators: comparators}
			continue
		}
		evaluator, err := eval.New(name, value)
		if err != nil {
			return nil, err
		}
		evaluations[name] = evaluator
	}

	captureSpecs, err := parseCaptureSpecs(base.name, opts)
	if err != nil {
		return nil, err
	}

	base.evaluations = evaluations
	base.captureSpecs = captureSpecs
	return &commandTest{
		BaseTest: base,
		command:  command,
		timeout:  time.Duration(timeoutSeconds * float64(time.Second)),
	}, nil
}
