package steptypes

import (
	"fmt"
	"github.com/bgrewell/dart/internal/formatters"
	"github.com/bgrewell/dart/pkg/ifaces"
)

var _ ifaces.Step = &ExecuteStep{}

// ExecuteStep runs shell commands on a node.
type ExecuteStep struct {
	BaseStep
	node     ifaces.Node
	commands []string
}

// Run executes the commands sequentially and evaluates success.
func (s *ExecuteStep) Run(updater formatters.TaskCompleter) error {
	for _, command := range s.commands {
		result, err := s.node.Execute(command)
		if err != nil {
			updater.Error()
			return err
		}
		if result.ExitCode != 0 {
			updater.Error()
			return fmt.Errorf("command failed with exit code %d: %s", result.ExitCode, result.Stderr)
		}
	}
	updater.Complete()
	return nil
}
