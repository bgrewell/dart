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
		case "apt":
			// Get the node that the step will be executed on
			node, ok := nodes[c.Node]
			if !ok {
				return nil, ErrNodeNotFound
			}

			// Convert []interface{} to []string
			rawPackages, ok := c.Step.Options["packages"].([]interface{})
			if !ok {
				return nil, ErrPackagesNotArray
			}

			packages := make([]string, len(rawPackages))
			for i, pkg := range rawPackages {
				packages[i], ok = pkg.(string)
				if !ok {
					return nil, ErrPackageNotString
				}
			}

			step := &AptStep{
				title:    c.Name,
				node:     node,
				packages: packages,
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
