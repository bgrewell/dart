package nodetypes

import (
	"io"
	"os/exec"
	"syscall"

	"github.com/bgrewell/dart/internal/execution"
	"github.com/bgrewell/dart/internal/helpers"
	"github.com/bgrewell/dart/internal/stream"
	"github.com/bgrewell/dart/pkg/ifaces"
	"github.com/bgrewell/go-execute/v2"
)

var _ ifaces.Node = &LocalNode{}

func NewLocalNode(name string, opts ifaces.NodeOptions) ifaces.Node {

	var options []execution.ExecutionOption
	if opts != nil {
		options = execution.OptionsToExecutionOptions(*opts)
	}

	return &LocalNode{
		name:           name,
		defaultOptions: options,
	}
}

type LocalNode struct {
	name           string
	defaultOptions []execution.ExecutionOption
}

func (l *LocalNode) Setup() error {
	return nil
}

func (l *LocalNode) Teardown() error {
	return nil
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

	// Stream output to console if debug mode is enabled, while capturing for evaluation
	var stdout, stderr io.Reader = ret.Stdout, ret.Stderr
	if execution.IsDebugMode() {
		stdout, err = stream.StreamCopy(ret.Stdout, stream.StreamStdout, l.name, true)
		if err != nil {
			return nil, err
		}
		stderr, err = stream.StreamCopy(ret.Stderr, stream.StreamStderr, l.name, true)
		if err != nil {
			return nil, err
		}
	}

	// Return the result
	return &execution.ExecutionResult{
		ExecutionId: helpers.GetRandomId(),
		ExitCode:    exitCode,
		Stdout:      stdout,
		Stderr:      stderr,
	}, nil
}
