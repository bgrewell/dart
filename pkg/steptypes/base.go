package steptypes

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/bgrewell/dart/internal/config"
	"github.com/bgrewell/dart/pkg/ifaces"
)

const (
	TypeSimulated    = "simulated"
	TypeExecute      = "execute"
	TypeApt          = "apt"
	TypeFileCreate   = "file_create"
	TypeFileWrite    = "file_write"
	TypeFileDelete   = "file_delete"
	TypeFileEdit     = "file_edit"
	TypeFileExists   = "file_exists"
	TypeFileRead     = "file_read"
	TypeHTTPRequest  = "http_request"
	TypeDNSRequest   = "dns_request"
	TypeServiceCheck = "service_check"
)

// BaseStep provides a common structure for all step types.
// It intentionally does not provide a default Run: every step type must
// implement Run itself or it fails to satisfy ifaces.Step at compile time.
type BaseStep struct {
	title    string
	nodeName string
}

// Title returns the title of the step.
func (s *BaseStep) Title() string {
	return s.title
}

// NodeName returns the name of the node the step runs on.
func (s *BaseStep) NodeName() string {
	return s.nodeName
}

// baseFor builds the BaseStep for a step configuration.
func baseFor(c *config.StepConfig) BaseStep {
	// After expansion, each config has exactly one node
	return BaseStep{title: c.Name, nodeName: c.Node[0]}
}

// stepFactory constructs a step from its configuration and target node.
// Invalid options produce a config-time error rather than a runtime failure.
type stepFactory func(c *config.StepConfig, node ifaces.Node) (ifaces.Step, error)

// stepFactories maps step type names, as written in YAML, to their
// factories. New step types register here. file_write is an alias for
// file_create (its options are a strict subset).
var stepFactories = map[string]stepFactory{
	TypeSimulated:    newSimulatedStep,
	TypeExecute:      newExecuteStep,
	TypeApt:          newAptStep,
	TypeFileCreate:   newFileCreateStep,
	TypeFileWrite:    newFileCreateStep,
	TypeFileDelete:   newFileDeleteStep,
	TypeFileEdit:     newFileEditStep,
	TypeFileExists:   newFileExistsStep,
	TypeFileRead:     newFileReadStep,
	TypeHTTPRequest:  newHTTPRequestStep,
	TypeDNSRequest:   newDNSRequestStep,
	TypeServiceCheck: newServiceCheckStep,
}

// CreateSteps constructs a slice of executable Steps based on provided configuration.
//
// Each step configuration is resolved to its target node and handed to the
// factory registered for its type. Factories validate their options and
// return config errors with source locations for malformed input.
func CreateSteps(configs []*config.StepConfig, nodes map[string]ifaces.Node) ([]ifaces.Step, error) {
	var steps []ifaces.Step

	for _, c := range configs {
		// After expansion, each config has exactly one node
		nodeName := c.Node[0]
		node, ok := nodes[nodeName]
		if !ok {
			return nil, &config.ConfigError{
				Message:  fmt.Sprintf("node %q not found (referenced in step %q)", nodeName, c.Name),
				Location: c.NodeLoc,
			}
		}

		factory, ok := stepFactories[c.Step.Type]
		if !ok {
			return nil, &config.ConfigError{
				Message:  fmt.Sprintf("unknown step type %q", c.Step.Type),
				Location: c.Step.TypeLoc,
			}
		}

		step, err := factory(c, node)
		if err != nil {
			return nil, err
		}
		steps = append(steps, step)
	}

	return steps, nil
}

// optionError builds a ConfigError anchored at the step's location.
func optionError(c *config.StepConfig, format string, args ...interface{}) error {
	return &config.ConfigError{
		Message:  fmt.Sprintf(format, args...),
		Location: c.Loc,
	}
}

// The opt* helpers validate raw option values. A present-but-wrong-typed
// option is a config error, never a silent zero value.

func optString(c *config.StepConfig, key string) (value string, present bool, err error) {
	raw, ok := c.Step.Options[key]
	if !ok {
		return "", false, nil
	}
	s, ok := raw.(string)
	if !ok {
		return "", true, optionError(c, "%s must be a string in step %q (got %T)", key, c.Name, raw)
	}
	return s, true, nil
}

func requiredString(c *config.StepConfig, key, missingMsg string) (string, error) {
	value, present, err := optString(c, key)
	if err != nil {
		return "", err
	}
	if !present || value == "" {
		return "", optionError(c, "%s in step %q", missingMsg, c.Name)
	}
	return value, nil
}

func optBool(c *config.StepConfig, key string) (bool, error) {
	raw, ok := c.Step.Options[key]
	if !ok {
		return false, nil
	}
	b, ok := raw.(bool)
	if !ok {
		return false, optionError(c, "%s must be a boolean in step %q (got %T)", key, c.Name, raw)
	}
	return b, nil
}

func optInt(c *config.StepConfig, key string, def int) (int, error) {
	raw, ok := c.Step.Options[key]
	if !ok {
		return def, nil
	}
	switch v := raw.(type) {
	case int:
		return v, nil
	case int64:
		return int(v), nil
	case float64:
		if v != float64(int(v)) {
			return 0, optionError(c, "%s must be an integer in step %q (got %v)", key, c.Name, v)
		}
		return int(v), nil
	default:
		return 0, optionError(c, "%s must be an integer in step %q (got %T)", key, c.Name, raw)
	}
}

func optFloat(c *config.StepConfig, key string, def float64) (float64, error) {
	raw, ok := c.Step.Options[key]
	if !ok {
		return def, nil
	}
	switch v := raw.(type) {
	case int:
		return float64(v), nil
	case int64:
		return float64(v), nil
	case float64:
		return v, nil
	default:
		return 0, optionError(c, "%s must be a number in step %q (got %T)", key, c.Name, raw)
	}
}

func optStringList(c *config.StepConfig, key string) (values []string, present bool, err error) {
	raw, ok := c.Step.Options[key]
	if !ok {
		return nil, false, nil
	}
	list, ok := raw.([]interface{})
	if !ok {
		return nil, true, optionError(c, "%s must be an array of strings in step %q (got %T)", key, c.Name, raw)
	}
	values = make([]string, len(list))
	for i, item := range list {
		s, ok := item.(string)
		if !ok {
			return nil, true, optionError(c, "%s entry is not a string in step %q (got %T)", key, c.Name, item)
		}
		values[i] = s
	}
	return values, true, nil
}

// optFileMode parses a file mode option. Strings parse as octal ("644",
// "0644", "0o644") and are the recommended form. yaml.v3 parses
// leading-zero integer literals as octal already (`mode: 0644` arrives as
// 0o644), so integers are used directly — but only up to 0o777: anything
// larger is either a bare decimal like `mode: 644` (which would silently
// mean 0o1204) or a special-bit mode, both of which must be written as a
// string. Residual caveat: a bare decimal that happens to be <= 0o777
// (e.g. `mode: 444`) cannot be detected and is taken at face value.
// Returns 0 when absent.
func optFileMode(c *config.StepConfig, key string) (os.FileMode, error) {
	raw, ok := c.Step.Options[key]
	if !ok {
		return 0, nil
	}

	var value int64
	switch v := raw.(type) {
	case string:
		digits := strings.TrimPrefix(strings.TrimPrefix(v, "0o"), "0O")
		bits, err := strconv.ParseUint(digits, 8, 32)
		if err != nil || bits > 0o7777 {
			return 0, optionError(c, "invalid file mode %q in step %q (use octal, e.g. \"0644\")", v, c.Name)
		}
		return os.FileMode(bits), nil
	case int:
		value = int64(v)
	case int64:
		value = v
	default:
		return 0, optionError(c, "%s must be an octal string or integer in step %q (got %T)", key, c.Name, raw)
	}

	if value < 0 || value > 0o777 {
		return 0, optionError(c,
			"ambiguous or out-of-range file mode %v in step %q: write it with a leading zero (0644) or as a quoted octal string (\"0644\"); special-bit modes require the string form", raw, c.Name)
	}
	return os.FileMode(value), nil
}
