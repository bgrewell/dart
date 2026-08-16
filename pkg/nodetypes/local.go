package nodetypes

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"slices"
	"strings"
	"syscall"

	"github.com/bgrewell/dart/internal/execution"
	"github.com/bgrewell/dart/internal/helpers"
	"github.com/bgrewell/dart/internal/stream"
	"github.com/bgrewell/dart/pkg/ifaces"
	"github.com/bgrewell/go-execute/v2"
)

var _ ifaces.Node = &LocalNode{}

// NewLocalNode accepts exec options either at the top level of the node's
// options (shell, env, sudo) or nested under exec_opts, matching the shape
// the container node types use. Without a configured shell, commands run
// through the platform default shell — every other node type runs commands
// through a shell (docker `sh -c`, lxd `bash -c`, ssh's remote shell), and
// a shell-less local node made pipes, conditionals, and multi-line
// commands fail with a confusing exec error.
func NewLocalNode(name string, opts ifaces.NodeOptions, suiteDir string) ifaces.Node {

	var options []execution.ExecutionOption
	shellConfigured := false
	if opts != nil {
		o := *opts
		execSrc := o
		if execOpts, ok := o["exec_opts"].(map[string]interface{}); ok {
			execSrc = execOpts
			// With exec_opts present, top-level exec keys are ignored by
			// the option parser; misplaced ones deserve a warning, not a
			// silent no-op
			warnIgnoredOptions(name, o, "exec_opts")
		}
		warnIgnoredOptions(name, execSrc, "env", "shell", "sudo", "exec_opts")
		if _, ok := execSrc["shell"]; ok {
			shellConfigured = true
		}
		options = execution.OptionsToExecutionOptions(execSrc)
	}
	if !shellConfigured {
		options = append(options, execution.WithShell(defaultLocalShell()))
	}

	// Commands run from the suite's directory, so a relative path means the
	// same thing in a command as it does in a file step's source. Without
	// this the command inherited DART's working directory, and a suite that
	// worked from the repository root broke when run from anywhere else.
	if suiteDir != "" {
		options = append(options, execution.WithWorkingDir(suiteDir))
	}

	return &LocalNode{
		name:           name,
		defaultOptions: options,
	}
}

// defaultLocalShell returns the platform's standard shell.
func defaultLocalShell() string {
	if runtime.GOOS == "windows" {
		return "cmd"
	}
	return "/bin/sh"
}

// warnIgnoredOptions reports option keys that nothing consumes. A
// misspelled or misplaced key (e.g. shell at the top level next to
// exec_opts) previously no-opd silently.
func warnIgnoredOptions(nodeName string, options map[string]interface{}, known ...string) {
	for key := range options {
		if !slices.Contains(known, key) {
			fmt.Fprintf(os.Stderr, "Warning: node %q: option %q is not recognized and was ignored (known options: %s)\n",
				nodeName, key, strings.Join(known, ", "))
		}
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
