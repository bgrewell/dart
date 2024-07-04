package internal

import (
	"github.com/bgrewell/dart/internal/config"
	"github.com/bgrewell/dart/internal/formatters"
)

func CreateSteps(configs []*config.StepConfig, nodes map[string]Node) (steps []Step, err error) {
	steps = make([]Step, 0)

	for _, c := range configs {
		switch c.Step.Type {
		case "simulated":
			step := &SimulatedStep{
				title:     c.Name,
				sleepTime: c.Step.Options["time"].(int),
			}
			steps = append(steps, step)
		case "execute":
			// Get the node that the step will be executed on
			node, ok := nodes[c.Node]
			if !ok {
				return nil, ErrNodeNotFound
			}
			step := &ExecuteStep{
				title:   c.Name,
				node:    node,
				command: c.Step.Options["command"].(string),
			}
			steps = append(steps, step)
		default:
			return nil, ErrUnknownStepType
		}
	}

	return steps, nil
}

type Step interface {
	Run(updater formatters.TaskCompleter) error
	TitleLen() int
	Title() string
}
