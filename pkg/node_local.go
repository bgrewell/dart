package pkg

import (
	"github.com/bgrewell/dart/internal/execution"
	"github.com/bgrewell/dart/internal/helpers"
	"github.com/bgrewell/go-execute/v2"
	"os/exec"
	"syscall"
)

func NewLocalNode(defaultOptions ...execution.ExecutionOption) Node {
	return &LocalNode{
		defaultOptions: defaultOptions,
	}
}

type LocalNode struct {
	defaultOptions []execution.ExecutionOption
}

func (l *LocalNode) Close() error {
	// Nothing to do here
	return nil
}

func (l *LocalNode) Execute(command string, options ...execution.ExecutionOption) (result *execution.ExecutionResult, err error) {

	opts := append(l.defaultOptions, options...)

	// Create a new executor with any options that are passed in
	exe := execute.NewExecutor(execution.ToExecuteOptions(opts)...)
	ret, err := exe.ExecuteAsync(command)
	if err != nil {
		return nil, err
	}

	// Wait for the command to finish
	var exitCode int
	select {
	case err = <-ret.Finished:
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.Sys().(syscall.WaitStatus).ExitStatus()
		} else if err != nil {
			return nil, err
		}
	}

	// Return the result
	return &execution.ExecutionResult{
		ExecutionId: helpers.GetRandomId(),
		ExitCode:    exitCode,
		Stdout:      ret.Stdout,
		Stderr:      ret.Stderr,
	}, nil
}
