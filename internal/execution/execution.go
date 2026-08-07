package execution

import (
	"fmt"
	"io"
	"os"
	"sync"
	"time"

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

// OptionsToExecutionOptions is a helper function that converts a map of
// options to a list of ExecutionOptions. Values arrive from YAML, so lists
// are []interface{} and every assertion is checked — a wrong-typed option
// warns and is skipped rather than panicking.
func OptionsToExecutionOptions(options map[string]interface{}) []ExecutionOption {
	opts := make([]ExecutionOption, 0)
	for k, v := range options {
		switch k {
		case "env":
			env, ok := toStringSlice(v)
			if !ok {
				optionWarning("env", "a list of KEY=VALUE strings", v)
				continue
			}
			opts = append(opts, WithEnvironment(env))
		case "shell":
			shell, ok := v.(string)
			if !ok {
				optionWarning("shell", "a string", v)
				continue
			}
			opts = append(opts, WithShell(shell))
		case "sudo":
			sudo, ok := v.(map[string]interface{})
			if !ok {
				optionWarning("sudo", "a map with env_var or password", v)
				continue
			}
			if value, ok := sudo["env_var"]; ok {
				name, ok := value.(string)
				if !ok {
					optionWarning("sudo.env_var", "a string", value)
					continue
				}
				pass := os.Getenv(name)
				if pass == "" {
					fmt.Fprintf(os.Stderr, "Warning: sudo env_var %q is empty or unset\n", name)
				}
				opts = append(opts, WithSudo(pass))
			} else if value, ok := sudo["password"]; ok {
				pass, ok := value.(string)
				if !ok {
					optionWarning("sudo.password", "a string", value)
					continue
				}
				opts = append(opts, WithSudo(pass))
			}
		}
	}
	return opts
}

// toStringSlice accepts []string directly or a YAML-shaped []interface{} of
// strings.
func toStringSlice(v interface{}) ([]string, bool) {
	switch list := v.(type) {
	case []string:
		return list, true
	case []interface{}:
		result := make([]string, len(list))
		for i, item := range list {
			s, ok := item.(string)
			if !ok {
				return nil, false
			}
			result[i] = s
		}
		return result, true
	}
	return nil, false
}

func optionWarning(key, expected string, got interface{}) {
	fmt.Fprintf(os.Stderr, "Warning: exec option %q must be %s (got %T); ignored\n", key, expected, got)
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
	ExecutionId string        `json:"execution_id"`
	ExitCode    int           `json:"exit_code"`
	Stdout      io.Reader     `json:"stdout"`
	Stderr      io.Reader     `json:"stderr"`
	Duration    time.Duration `json:"duration"`

	stdoutOnce sync.Once
	stdoutData []byte
	stdoutErr  error
	stderrOnce sync.Once
	stderrData []byte
	stderrErr  error
}

// StdoutBytes drains Stdout once and caches the result, so multiple
// consumers (e.g. several evaluators on one test) all see the full output.
// Stdout is a one-shot stream; reading it directly and calling StdoutBytes
// on the same result must not be mixed.
func (r *ExecutionResult) StdoutBytes() ([]byte, error) {
	r.stdoutOnce.Do(func() {
		if r.Stdout == nil {
			return
		}
		r.stdoutData, r.stdoutErr = io.ReadAll(r.Stdout)
	})
	return r.stdoutData, r.stdoutErr
}

// StderrBytes drains Stderr once and caches the result. See StdoutBytes.
func (r *ExecutionResult) StderrBytes() ([]byte, error) {
	r.stderrOnce.Do(func() {
		if r.Stderr == nil {
			return
		}
		r.stderrData, r.stderrErr = io.ReadAll(r.Stderr)
	})
	return r.stderrData, r.stderrErr
}
