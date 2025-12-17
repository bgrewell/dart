package ifaces

import "github.com/bgrewell/dart/internal/formatters"

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
	NodeName() string
}
