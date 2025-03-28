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
	node    ifaces.Node
	command string
}

// Run executes the command and evaluates success.
func (s *ExecuteStep) Run(updater formatters.TaskCompleter) error {
	result, err := s.node.Execute(s.command)
	if err != nil {
		updater.Error()
		return err
	}
	if result.ExitCode != 0 {
		updater.Error()
		return fmt.Errorf("command failed with exit code %d: %s", result.ExitCode, result.Stderr)
	}
	updater.Complete()
	return nil
}
