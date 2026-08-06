package steptypes

import (
	"time"

	"github.com/bgrewell/dart/internal/config"
	"github.com/bgrewell/dart/internal/formatters"
	"github.com/bgrewell/dart/pkg/ifaces"
)

var _ ifaces.Step = &SimulatedStep{}

// SimulatedStep introduces an artificial delay.
type SimulatedStep struct {
	BaseStep
	sleepTime time.Duration
}

// newSimulatedStep accepts a `time` option in seconds (fractional allowed).
func newSimulatedStep(c *config.StepConfig, _ ifaces.Node) (ifaces.Step, error) {
	if _, ok := c.Step.Options["time"]; !ok {
		return nil, optionError(c, "time is required in step %q", c.Name)
	}
	seconds, err := optFloat(c, "time", 0)
	if err != nil {
		return nil, err
	}
	if seconds < 0 {
		return nil, optionError(c, "time must be non-negative in step %q", c.Name)
	}

	return &SimulatedStep{
		BaseStep:  baseFor(c),
		sleepTime: time.Duration(seconds * float64(time.Second)),
	}, nil
}

// Run sleeps for the specified time and marks completion.
func (s *SimulatedStep) Run(updater formatters.TaskCompleter) error {
	time.Sleep(s.sleepTime)
	updater.Complete()
	return nil
}
