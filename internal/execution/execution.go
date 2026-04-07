package execution

import (
	"io"
	"os"
	"sync"

	"github.com/bgrewell/go-execute/v2"
)

var (
	debugMu      sync.RWMutex
	debugEnabled bool
)

// SetDebugMode enables or disables debug output streaming globally.
func SetDebugMode(enabled bool) {
	debugMu.Lock()
	defer debugMu.Unlock()
	debugEnabled = enabled
}

// IsDebugMode returns whether debug mode is enabled.
func IsDebugMode() bool {
	debugMu.RLock()
	defer debugMu.RUnlock()
	return debugEnabled
}

// ExecutionOption is a wrapper that allows for the passing of options to the Execute method
type ExecutionOption struct {
	apply func(exec execute.Executor)
}

// Apply is a method that applies the option to the executor
func (e ExecutionOption) Apply(exec execute.Executor) {
	e.apply(exec)
}

// ToInternal is a method that converts the ExecutionOption to an internal option
func (o ExecutionOption) ToInternal() execute.Option {
	return func(exec execute.Executor) {
		o.Apply(exec)
	}
}

// ToExecuteOptions is a helper function that converts a list of ExecutionOptions to a list of execute.Options
func ToExecuteOptions(options []ExecutionOption) []execute.Option {
	opts := make([]execute.Option, 0)
	for _, o := range options {
		opts = append(opts, o.ToInternal())
	}
	return opts
}

// OptionsToExecutionOptions is a helper function that converts a map of options to a list of ExecutionOptions
func OptionsToExecutionOptions(options map[string]interface{}) []ExecutionOption {
	opts := make([]ExecutionOption, 0)
	for k, v := range options {
		switch k {
		case "env":
			opts = append(opts, WithEnvironment(v.([]string)))
		case "shell":
			opts = append(opts, WithShell(v.(string)))
		case "sudo":
			sudo := v.(map[string]interface{})
			if value, ok := sudo["env_var"]; ok {
				pass := os.Getenv(value.(string))
				opts = append(opts, WithSudo(pass))
			} else if value, ok = sudo["password"]; ok {
				opts = append(opts, WithSudo(value.(string)))
			}
		}
	}
	return opts
}

func WithEnvironment(env []string) ExecutionOption {
	return ExecutionOption{
		apply: func(exec execute.Executor) {
			exec.SetEnvironment(env)
		},
	}
}

func WithShell(shell string) ExecutionOption {
	return ExecutionOption{
		apply: func(exec execute.Executor) {
			exec.SetShell(shell)
		},
	}
}

func WithSudo(pass string) ExecutionOption {
	return ExecutionOption{
		apply: func(exec execute.Executor) {
			exec.SetSudoCredentials(pass)
		},
	}
}

// ExecutionResult is a struct that contains the results of an execution
type ExecutionResult struct {
	ExecutionId string    `json:"execution_id"`
	ExitCode    int       `json:"exit_code"`
	Stdout      io.Reader `json:"stdout"`
	Stderr      io.Reader `json:"stderr"`
}
