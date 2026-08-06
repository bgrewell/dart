package testtypes

import (
	"fmt"

	"github.com/bgrewell/dart/internal/eval"
	"github.com/bgrewell/dart/internal/helpers"
	"github.com/bgrewell/dart/pkg/ifaces"
)

// newPingTest pings a target from the node. Options: target (alias host,
// required), count (default 5). Evaluate keys: packet_loss (max percent,
// default check is 0 when no evaluate block is given), rtt_min (lower
// bound, ms), rtt_avg / rtt_max (upper bounds, ms). Other keys fall
// through to the standard evaluators. Requires `ping` on the node; works
// with iputils and busybox output formats.
func newPingTest(base BaseTest, opts map[string]interface{}) (ifaces.Test, error) {
	target, err := requiredString(base.name, opts, "target", "host")
	if err != nil {
		return nil, err
	}
	count, err := optInt(base.name, opts, "count", 5)
	if err != nil {
		return nil, err
	}
	if count < 1 {
		return nil, fmt.Errorf("count must be at least 1 in test %q", base.name)
	}

	spec, err := evaluateSpec(base.name, opts)
	if err != nil {
		return nil, err
	}

	evaluations := make(map[string]eval.Evaluate, len(spec))
	for name, value := range spec {
		switch name {
		case "packet_loss":
			max, ok := coerceFloat(value)
			if !ok {
				return nil, fmt.Errorf("packet_loss must be a number in test %q (got %v)", base.name, value)
			}
			evaluations[name] = &packetLossCheck{max: max}
		case "rtt_min", "rtt_avg", "rtt_max":
			bound, ok := coerceFloat(value)
			if !ok {
				return nil, fmt.Errorf("%s must be a number in test %q (got %v)", name, base.name, value)
			}
			evaluations[name] = &rttCheck{kind: name[len("rtt_"):], bound: bound}
		default:
			evaluator, err := eval.New(name, value)
			if err != nil {
				return nil, err
			}
			evaluations[name] = evaluator
		}
	}
	if len(evaluations) == 0 {
		evaluations["packet_loss"] = &packetLossCheck{max: 0}
	}

	base.evaluations = evaluations
	return &commandTest{
		BaseTest: base,
		command:  fmt.Sprintf("ping -q -c %d %s", count, helpers.ShellQuote(target)),
	}, nil
}
