package internal

import (
	"github.com/bgrewell/dart/internal/formatters"
	"time"
)

type SimulatedStep struct {
	title     string
	sleepTime int
}

func (s *SimulatedStep) Title() string {
	return s.title
}

func (s *SimulatedStep) Run(updater formatters.TaskCompleter) error {
	time.Sleep(time.Duration(s.sleepTime) * time.Second)
	updater.Complete()
	return nil
}
