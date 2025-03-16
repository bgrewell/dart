package steptypes

import (
	"github.com/bgrewell/dart/internal/config"
	"github.com/bgrewell/dart/internal/formatters"
	"github.com/bgrewell/dart/internal/helpers"
	"github.com/bgrewell/dart/pkg/ifaces"
)

// BaseStep provides a common structure for all step types.
type BaseStep struct {
	title string
}

// Title returns the title of the step.
func (s *BaseStep) Title() string {
	return s.title
}

// Run is defined by specific step implementations.
func (s *BaseStep) Run(updater formatters.TaskCompleter) error {
	return nil // Should be overridden
}

// CreateSteps constructs a slice of executable Steps based on provided configuration.
//
// This function processes a slice of step configurations and maps each step configuration
// to its corresponding concrete implementation (e.g., `ExecuteStep`, `AptStep`, `SimulateStep`).
//
// Parameters:
// - `configs`: Slice of step configurations specifying the type, node, and parameters for each step.
// - `nodes`: Map of node names to Node interfaces, used to associate a step with the correct execution context.
//
// Returns:
//   - A slice of initialized Step implementations ready for execution.
//   - An error if configuration parsing fails, if required parameters are missing or incorrectly typed,
//     or if an unknown step type is encountered.
//
// Example usage:
// ```go
// steps, err := CreateSteps(stepConfigs, availableNodes)
//
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	for _, step := range steps {
//	    step.Run()
//	}
//
// ```
//
// Errors:
// - `ErrUnknownStepType` if a configuration includes a type that is not supported.
// - `ErrPackageNotString` if package entries for an apt step are not strings.
func CreateSteps(configs []*config.StepConfig, nodes map[string]ifaces.Node) ([]ifaces.Step, error) {
	var steps []ifaces.Step

	for _, c := range configs {
		node, ok := nodes[c.Node]
		if !ok {
			return nil, helpers.ErrNodeNotFound
		}

		switch c.Step.Type {
		case "simulated":
			steps = append(steps, &SimulatedStep{
				BaseStep:  BaseStep{title: c.Name},
				sleepTime: c.Step.Options["time"].(int),
			})
		case "execute":
			steps = append(steps, &ExecuteStep{
				BaseStep: BaseStep{title: c.Name},
				node:     node,
				command:  c.Step.Options["command"].(string),
			})
		case "apt":
			rawPackages, ok := c.Step.Options["packages"].([]interface{})
			if !ok {
				return nil, helpers.ErrPackagesNotArray
			}

			packages := make([]string, len(rawPackages))
			for i, pkg := range rawPackages {
				packages[i], ok = pkg.(string)
				if !ok {
					return nil, helpers.ErrPackageNotString
				}
			}

			steps = append(steps, &AptStep{
				BaseStep: BaseStep{title: c.Name},
				node:     node,
				packages: packages,
			})
		default:
			return nil, helpers.ErrUnknownStepType
		}
	}

	return steps, nil
}
