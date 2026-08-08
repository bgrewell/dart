package steptypes

import (
	"time"

	"github.com/bgrewell/dart/internal/config"
	"github.com/bgrewell/dart/internal/formatters"
	"github.com/bgrewell/dart/pkg/ifaces"
)

var _ ifaces.Step = &SimulatedStep{}

// SimulatedStep introduces an artificial delay, standing in for work that
// is not being performed — a placeholder while a suite is being written, or
// a pause that models a slow operation.
type SimulatedStep struct {
	BaseStep
	sleepTime time.Duration
	message   string
}

// newSimulatedStep accepts a `time` option in seconds (fractional allowed)
// and an optional `message` describing what is being stood in for.
func newSimulatedStep(c *config.StepConfig, _ ifaces.Node) (ifaces.Step, error) {
	noteOption("time")
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
	message, _, err := optString(c, "message")
	if err != nil {
		return nil, err
	}

	return &SimulatedStep{
		BaseStep:  baseFor(c),
		sleepTime: time.Duration(seconds * float64(time.Second)),
		message:   message,
	}, nil
}

// Run sleeps for the specified time and marks completion. The message, when
// given, is what the step reports while it waits.
func (s *SimulatedStep) Run(updater formatters.TaskCompleter) error {
	if s.message != "" {
		updater.Update(s.message)
	}
	time.Sleep(s.sleepTime)
	updater.Complete()
	return nil
}
