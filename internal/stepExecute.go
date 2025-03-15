package internal

import (
	"fmt"
	"github.com/bgrewell/dart/internal/formatters"
)

type ExecuteStep struct {
	title   string
	node    Node
	command string
}

func (s *ExecuteStep) Title() string {
	return s.title
}

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
