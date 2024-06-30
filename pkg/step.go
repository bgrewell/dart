package pkg

import (
	"github.com/bgrewell/dart/internal/config"
	"github.com/bgrewell/dart/internal/formatters"
	"time"
)

func CreateSteps(configs []*config.StepConfig, nodes map[string]Node) (steps []Step, err error) {
	steps = make([]Step, 0)

	for _, c := range configs {
		switch c.Step.Type {
		case "simulated":
			step := &SimulatedStep{
				title:     c.Step.Options["message"].(string),
				sleepTime: c.Step.Options["time"].(int),
			}
			steps = append(steps, step)
		}
	}

	return steps, nil
}

type Step interface {
	Run(updater formatters.TaskCompleter) error
	TitleLen() int
	Title() string
}

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

func (s *SimulatedStep) TitleLen() int {
	return len(s.title)
}
