package steptypes

import (
	"fmt"
	"strings"

	"github.com/bgrewell/dart/internal/config"
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

// newExecuteStep accepts a single command string or an array of commands.
func newExecuteStep(c *config.StepConfig, node ifaces.Node) (ifaces.Step, error) {
	var commands []string
	switch cmd := c.Step.Options["command"].(type) {
	case string:
		commands = []string{cmd}
	case []interface{}:
		commands = make([]string, len(cmd))
		for i, v := range cmd {
			s, ok := v.(string)
			if !ok {
				return nil, optionError(c, "command entry is not a string in step %q", c.Name)
			}
			commands[i] = s
		}
	default:
		return nil, optionError(c, "command must be a string or array of strings in step %q", c.Name)
	}

	return &ExecuteStep{
		BaseStep: baseFor(c),
		node:     node,
		commands: commands,
	}, nil
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
			stderr, _ := result.StderrBytes()
			updater.Error()
			return fmt.Errorf("command failed with exit code %d: %s",
				result.ExitCode, strings.TrimSpace(string(stderr)))
		}
	}
	updater.Complete()
	return nil
}
