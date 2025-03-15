package internal

import (
	"github.com/bgrewell/dart/internal/config"
	"github.com/bgrewell/dart/internal/formatters"
)

// Step defines an executable action within a test or automation suite.
//
// Each Step represents a discrete operation, such as executing a command,
// simulating conditions, installing packages, or introducing delays. Steps
// are associated with nodes that provide the execution context.
//
// Implementations must provide logic for execution, identification, and
// clear human-readable descriptions for ease of debugging and readability.
//
// ### Typical Use Cases:
// - **Command Execution:** Run specific commands on designated nodes.
// - **Package Installation:** Install or manage software packages via apt or similar.
// - **Simulation:** Simulate network delays, interruptions, or other conditions.
//
// Methods:
// - `Run()`: Executes the step's action.
// - `Title()`: Provides a descriptive title for display purposes.
//
// Example:
// ```go
// step := &ExecuteStep{title: "Ensure Ubuntu version", node: targetNode, command: "grep -q "24.04" /etc/lsb-release"}
// err := step.Run()
//
//	if err != nil {
//	    log.Fatal(err)
//	}
//
// ```
type Step interface {
	Run(updater formatters.TaskCompleter) error
	Title() string
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
func CreateSteps(configs []*config.StepConfig, nodes map[string]Node) ([]Step, error) {
	var steps []Step

	for _, c := range configs {
		node, ok := nodes[c.Node]
		if !ok {
			return nil, ErrNodeNotFound
		}

		switch c.Step.Type {
		case "simulated":
			steps = append(steps, &SimulatedStep{
				title:     c.Name,
				sleepTime: c.Step.Options["time"].(int),
			})
		case "execute":
			steps = append(steps, &ExecuteStep{
				title:   c.Name,
				node:    node,
				command: c.Step.Options["command"].(string),
			})
		case "apt":
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

			steps = append(steps, &AptStep{
				title:    c.Name,
				node:     node,
				packages: packages,
			})
		default:
			return nil, ErrUnknownStepType
		}
	}

	return steps, nil
}
