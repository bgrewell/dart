package steptypes

import (
	"time"

	"fmt"
	"strings"

	"github.com/bgrewell/dart/internal/config"
	"github.com/bgrewell/dart/internal/formatters"
	"github.com/bgrewell/dart/internal/helpers"
	"github.com/bgrewell/dart/pkg/ifaces"
)

var _ ifaces.Step = &ExecuteStep{}

// ExecuteStep runs shell commands on a node.
type ExecuteStep struct {
	BaseStep
	node     ifaces.Node
	commands []string
	timeout  time.Duration
}

// newExecuteStep accepts a single command string or an array of commands.
func newExecuteStep(c *config.StepConfig, node ifaces.Node) (ifaces.Step, error) {
	var commands []string
	noteOption("command")
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

	workdir, _, err := optString(c, "workdir")
	if err != nil {
		return nil, err
	}

	timeoutSeconds, err := optFloat(c, "timeout", 0)
	if err != nil {
		return nil, err
	}
	if timeoutSeconds < 0 {
		return nil, optionError(c, "timeout must be non-negative in step %q", c.Name)
	}

	if workdir != "" {
		// Applied as a shell prefix rather than an executor setting so it
		// works the same on every node type: the directory belongs to the
		// node, and every node runs commands through a shell.
		for i, command := range commands {
			commands[i] = fmt.Sprintf("cd -- %s && %s", helpers.ShellQuote(workdir), command)
		}
	}

	return &ExecuteStep{
		BaseStep: baseFor(c),
		node:     node,
		commands: commands,
		timeout:  time.Duration(timeoutSeconds * float64(time.Second)),
	}, nil
}

// Run executes the commands sequentially and evaluates success.
func (s *ExecuteStep) Run(updater formatters.TaskCompleter) error {
	for _, command := range s.commands {
		result, err := ifaces.ExecuteWithTimeout(s.node, command, s.timeout)
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
