package testtypes

import (
	"fmt"
	"strings"
	"time"

	"github.com/bgrewell/dart/internal/eval"
	"github.com/bgrewell/dart/internal/execution"
	"github.com/bgrewell/dart/internal/formatters"
	"github.com/bgrewell/dart/pkg/ifaces"
)

var _ ifaces.Test = &RebootTest{}

// RebootTest restarts the target node mid-suite and blocks until it
// accepts commands again — for suites that verify reboot-dependent
// behavior between tests (rollbacks, kernel updates, first-boot
// services). mode: force models a power cut. Supported on node types
// implementing ifaces.Rebooter (lxd, ssh).
type RebootTest struct {
	BaseTest
	force        bool
	readyCommand string
	timeout      time.Duration
}

// newRebootTest parses mode (graceful|force, default graceful),
// ready_command (optional override of the node's readiness check), and
// timeout seconds (0 uses the node's default). The evaluate block accepts
// the standard evaluators (e.g. max_duration to bound the reboot); the
// default asserts the reboot completed.
func newRebootTest(base BaseTest, opts map[string]interface{}) (ifaces.Test, error) {
	if _, ok := base.node.(ifaces.Rebooter); !ok {
		return nil, fmt.Errorf("node %q does not support reboot (supported: lxd, ssh) in test %q", base.nodeName, base.name)
	}
	// Retrying a reboot test would power-cycle the target on every failed
	// evaluation — reject rather than surprise
	if base.retryTimeout > 0 {
		return nil, fmt.Errorf("retry is not supported on reboot tests (test %q): a failing evaluation would reboot the target repeatedly", base.name)
	}

	mode, present, err := optString(base.name, opts, "mode")
	if err != nil {
		return nil, err
	}
	if !present || mode == "" {
		mode = "graceful"
	}
	if mode != "graceful" && mode != "force" {
		return nil, fmt.Errorf("mode must be \"graceful\" or \"force\" in test %q (got %q)", base.name, mode)
	}

	readyCommand, _, err := optString(base.name, opts, "ready_command")
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

	spec, err := evaluateSpec(base.name, opts)
	if err != nil {
		return nil, err
	}
	evaluations, err := eval.Parse(spec)
	if err != nil {
		return nil, err
	}
	if len(evaluations) == 0 {
		evaluations["rebooted"] = &eval.EvaluateExitCode{Expected: []int{0}}
	}

	base.evaluations = evaluations
	return &RebootTest{
		BaseTest:     base,
		force:        mode == "force",
		readyCommand: readyCommand,
		timeout:      time.Duration(timeoutSeconds * float64(time.Second)),
	}, nil
}

func (t *RebootTest) Run(updater formatters.TestCompleter) (map[string]*eval.EvaluateResult, error) {
	return t.runProducer(func() (*execution.ExecutionResult, error) {
		rebooter := t.node.(ifaces.Rebooter)
		if err := rebooter.Reboot(t.force, t.readyCommand, t.timeout); err != nil {
			return nil, err
		}
		return &execution.ExecutionResult{
			ExitCode: 0,
			Stdout:   strings.NewReader("rebooted"),
			Stderr:   strings.NewReader(""),
		}, nil
	}, updater)
}
