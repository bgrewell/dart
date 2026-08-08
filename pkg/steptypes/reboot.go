package steptypes

import (
	"fmt"
	"time"

	"github.com/bgrewell/dart/internal/config"
	"github.com/bgrewell/dart/internal/formatters"
	"github.com/bgrewell/dart/pkg/ifaces"
	"github.com/bgrewell/dart/pkg/nodetypes"
)

var _ ifaces.Step = &RebootStep{}

// RebootStep restarts the target node and blocks until it accepts commands
// again. Supported on node types implementing ifaces.Rebooter (lxd, ssh).
type RebootStep struct {
	BaseStep
	node         ifaces.Node
	force        bool
	readyCommand string
	timeout      time.Duration
}

// newRebootStep parses mode (graceful|force, default graceful),
// ready_command (optional override of the node's readiness check), and
// timeout seconds (0 uses the node's default).
func newRebootStep(c *config.StepConfig, node ifaces.Node) (ifaces.Step, error) {
	mode, present, err := optString(c, "mode")
	if err != nil {
		return nil, err
	}
	if !present || mode == "" {
		mode = "graceful"
	}
	if mode != "graceful" && mode != "force" {
		return nil, optionError(c, "mode must be \"graceful\" or \"force\" in step %q (got %q)", c.Name, mode)
	}

	readyCommand, _, err := optString(c, "ready_command")
	if err != nil {
		return nil, err
	}
	timeoutSeconds, err := optFloat(c, "timeout", 0)
	if err != nil {
		return nil, err
	}
	if timeoutSeconds < 0 {
		return nil, optionError(c, "timeout must be non-negative in step %q", c.Name)
	}

	if _, ok := node.(ifaces.Rebooter); !ok {
		return nil, optionError(c, "node %q does not support reboot (supported: %s) in step %q",
			c.Node[0], nodetypes.SupportingTypes(nodetypes.CapabilityReboot), c.Name)
	}

	return &RebootStep{
		BaseStep:     baseFor(c),
		node:         node,
		force:        mode == "force",
		readyCommand: readyCommand,
		timeout:      time.Duration(timeoutSeconds * float64(time.Second)),
	}, nil
}

// Run restarts the node and waits for readiness.
func (s *RebootStep) Run(updater formatters.TaskCompleter) error {
	rebooter, ok := s.node.(ifaces.Rebooter)
	if !ok {
		updater.Error()
		return fmt.Errorf("node does not support reboot")
	}

	updater.Update("rebooting")
	if err := rebooter.Reboot(s.force, s.readyCommand, s.timeout); err != nil {
		updater.Error()
		return err
	}

	updater.Complete()
	return nil
}
