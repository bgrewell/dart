package steptypes

import (
	"github.com/bgrewell/dart/internal/formatters"
	"github.com/bgrewell/dart/pkg/ifaces"
	"time"
)

var _ ifaces.Step = &SimulatedStep{}

// SimulatedStep introduces an artificial delay.
type SimulatedStep struct {
	BaseStep
	sleepTime int
}

// Run sleeps for the specified time and marks completion.
func (s *SimulatedStep) Run(updater formatters.TaskCompleter) error {
	time.Sleep(time.Duration(s.sleepTime) * time.Second)
	updater.Complete()
	return nil
}
