package steptypes

import (
	"errors"
	"fmt"
	"time"

	"github.com/bgrewell/dart/internal/config"
	"github.com/bgrewell/dart/internal/formatters"
	"github.com/bgrewell/dart/pkg/ifaces"
)

var _ ifaces.Step = &WaitForStep{}

// WaitForStep polls a command on the node until it exits zero — for setup
// that must wait on convergence (a service answering, DNS propagating, a
// cluster electing) before later steps run.
type WaitForStep struct {
	BaseStep
	node     ifaces.Node
	command  string
	timeout  time.Duration
	interval time.Duration
}

// newWaitForStep parses command (required), timeout seconds (default 60),
// and interval seconds (default 2).
func newWaitForStep(c *config.StepConfig, node ifaces.Node) (ifaces.Step, error) {
	command, present, err := optString(c, "command")
	if err != nil {
		return nil, err
	}
	if !present || command == "" {
		return nil, optionError(c, "command is required in step %q", c.Name)
	}

	timeoutSeconds, err := optFloat(c, "timeout", 60)
	if err != nil {
		return nil, err
	}
	if timeoutSeconds <= 0 {
		return nil, optionError(c, "timeout must be positive in step %q", c.Name)
	}
	intervalSeconds, err := optFloat(c, "interval", 2)
	if err != nil {
		return nil, err
	}
	if intervalSeconds <= 0 {
		return nil, optionError(c, "interval must be positive in step %q", c.Name)
	}

	return &WaitForStep{
		BaseStep: baseFor(c),
		node:     node,
		command:  command,
		timeout:  time.Duration(timeoutSeconds * float64(time.Second)),
		interval: time.Duration(intervalSeconds * float64(time.Second)),
	}, nil
}

// Run polls the command until success or timeout. The command is waited on
// in interval-sized slices against a single in-flight invocation: a check
// slower than the interval is re-awaited rather than re-launched, so
// invocations never overlap and a completion near the deadline is still
// observed. The step may overrun its deadline by at most one interval.
func (s *WaitForStep) Run(updater formatters.TaskCompleter) error {
	deadline := time.Now().Add(s.timeout)
	produce := ifaces.BoundedCommand(s.node, s.command, s.interval)
	attempt := 0
	for time.Now().Before(deadline) {
		attempt++
		updater.Update(fmt.Sprintf("waiting (attempt %d)", attempt))

		result, err := produce()
		if err == nil && result.ExitCode == 0 {
			updater.Complete()
			return nil
		}
		if err != nil && errors.Is(err, ifaces.ErrCommandTimeout) {
			// The check is still running; re-await it immediately
			continue
		}

		// Completed but not ready (or the node errored): pause before the
		// next poll, without overshooting the deadline
		pause := s.interval
		if remaining := time.Until(deadline); pause > remaining {
			pause = remaining
		}
		if pause > 0 {
			time.Sleep(pause)
		}
	}

	updater.Error()
	return fmt.Errorf("wait_for timed out after %s: %s", s.timeout, s.command)
}
