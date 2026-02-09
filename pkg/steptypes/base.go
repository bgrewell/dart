package steptypes

import (
	"fmt"
	"os"

	"github.com/bgrewell/dart/internal/config"
	"github.com/bgrewell/dart/internal/formatters"
	"github.com/bgrewell/dart/pkg/ifaces"
)

// BaseStep provides a common structure for all step types.
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

// Run is defined by specific step implementations.
func (s *BaseStep) Run(updater formatters.TaskCompleter) error {
	return nil // Should be overridden
}

// CreateSteps constructs a slice of executable Steps based on provided configuration.
//
// This function processes a slice of step configurations and maps each step configuration
// to its corresponding concrete implementation (e.g., `ExecuteStep`, `AptStep`, `SimulateStep`).
//
// Parameters:
// - `configs`: Slice of step configurations specifying the type, node, and parameters for each step.
// - `nodes`: Map of node names to Node interfaces, used to associate a step with the correct execution context.
//
// Returns:
//   - A slice of initialized Step implementations ready for execution.
//   - An error if configuration parsing fails, if required parameters are missing or incorrectly typed,
//     or if an unknown step type is encountered.
//
// Example usage:
// ```go
// steps, err := CreateSteps(stepConfigs, availableNodes)
//
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	for _, step := range steps {
//	    step.Run()
//	}
//
// ```
//
// Errors:
// - `ErrUnknownStepType` if a configuration includes a type that is not supported.
// - `ErrPackageNotString` if package entries for an apt step are not strings.
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

		switch c.Step.Type {
		case "simulated":
			steps = append(steps, &SimulatedStep{
				BaseStep:  BaseStep{title: c.Name, nodeName: nodeName},
				sleepTime: c.Step.Options["time"].(int),
			})
		case "execute":
			var commands []string
			switch cmd := c.Step.Options["command"].(type) {
			case string:
				commands = []string{cmd}
			case []interface{}:
				commands = make([]string, len(cmd))
				for i, v := range cmd {
					s, ok := v.(string)
					if !ok {
						return nil, &config.ConfigError{
							Message:  fmt.Sprintf("command entry is not a string in step %q", c.Name),
							Location: c.Loc,
						}
					}
					commands[i] = s
				}
			default:
				return nil, &config.ConfigError{
					Message:  fmt.Sprintf("command must be a string or array of strings in step %q", c.Name),
					Location: c.Loc,
				}
			}
			steps = append(steps, &ExecuteStep{
				BaseStep: BaseStep{title: c.Name, nodeName: nodeName},
				node:     node,
				commands: commands,
			})
		case "apt":
			rawPackages, ok := c.Step.Options["packages"].([]interface{})
			if !ok {
				return nil, &config.ConfigError{
					Message:  fmt.Sprintf("packages field is not an array in step %q", c.Name),
					Location: c.Loc,
				}
			}

			packages := make([]string, len(rawPackages))
			for i, pkg := range rawPackages {
				packages[i], ok = pkg.(string)
				if !ok {
					return nil, &config.ConfigError{
						Message:  fmt.Sprintf("package entry is not a string in step %q", c.Name),
						Location: c.Loc,
					}
				}
			}

			steps = append(steps, &AptStep{
				BaseStep: BaseStep{title: c.Name, nodeName: nodeName},
				node:     node,
				packages: packages,
			})
		case "file_create":
			step, err := createFileCreateStep(c, node)
			if err != nil {
				return nil, err
			}
			steps = append(steps, step)
		case "file_delete":
			step, err := createFileDeleteStep(c, node)
			if err != nil {
				return nil, err
			}
			steps = append(steps, step)
		case "file_edit":
			step, err := createFileEditStep(c, node)
			if err != nil {
				return nil, err
			}
			steps = append(steps, step)
		default:
			return nil, &config.ConfigError{
				Message:  fmt.Sprintf("unknown step type %q", c.Step.Type),
				Location: c.Step.TypeLoc,
			}
		}
	}

	return steps, nil
}

// createFileCreateStep creates a FileCreateStep from configuration
func createFileCreateStep(c *config.StepConfig, _ ifaces.Node) (*FileCreateStep, error) {
	// After expansion, each config has exactly one node
	nodeName := c.Node[0]
	
	filePath, _ := c.Step.Options["path"].(string)
	if filePath == "" {
		return nil, &config.ConfigError{
			Message:  fmt.Sprintf("file path is required in step %q", c.Name),
			Location: c.Loc,
		}
	}

	contents, _ := c.Step.Options["contents"].(string)
	overwrite, _ := c.Step.Options["overwrite"].(bool)
	createDir, _ := c.Step.Options["create_dir"].(bool)

	var mode os.FileMode = 0644
	if modeVal, ok := c.Step.Options["mode"].(int); ok {
		mode = os.FileMode(modeVal)
	}

	return &FileCreateStep{
		BaseStep:  BaseStep{title: c.Name, nodeName: nodeName},
		filePath:  filePath,
		contents:  contents,
		overwrite: overwrite,
		mode:      mode,
		createDir: createDir,
	}, nil
}

// createFileDeleteStep creates a FileDeleteStep from configuration
func createFileDeleteStep(c *config.StepConfig, _ ifaces.Node) (*FileDeleteStep, error) {
	// After expansion, each config has exactly one node
	nodeName := c.Node[0]

	filePath, _ := c.Step.Options["path"].(string)
	if filePath == "" {
		return nil, &config.ConfigError{
			Message:  fmt.Sprintf("file path is required in step %q", c.Name),
			Location: c.Loc,
		}
	}

	ignoreErrors, _ := c.Step.Options["ignore_errors"].(bool)

	return &FileDeleteStep{
		BaseStep:     BaseStep{title: c.Name, nodeName: nodeName},
		filePath:     filePath,
		ignoreErrors: ignoreErrors,
	}, nil
}

// createFileEditStep creates a FileEditStep from configuration
func createFileEditStep(c *config.StepConfig, _ ifaces.Node) (*FileEditStep, error) {
	// After expansion, each config has exactly one node
	nodeName := c.Node[0]

	filePath, _ := c.Step.Options["path"].(string)
	if filePath == "" {
		return nil, &config.ConfigError{
			Message:  fmt.Sprintf("file path is required in step %q", c.Name),
			Location: c.Loc,
		}
	}

	operationStr, _ := c.Step.Options["operation"].(string)
	var operation EditOperation
	switch operationStr {
	case "insert":
		operation = EditInsert
	case "replace":
		operation = EditReplace
	case "remove":
		operation = EditRemove
	default:
		return nil, &config.ConfigError{
			Message:  fmt.Sprintf("invalid edit operation %q in step %q", operationStr, c.Name),
			Location: c.Loc,
		}
	}

	positionStr, _ := c.Step.Options["position"].(string)
	var position InsertPosition
	switch positionStr {
	case "before":
		position = InsertBefore
	case "after":
		position = InsertAfter
	case "":
		position = InsertAfter // default
	default:
		return nil, &config.ConfigError{
			Message:  fmt.Sprintf("invalid insert position %q in step %q", positionStr, c.Name),
			Location: c.Loc,
		}
	}

	matchTypeStr, _ := c.Step.Options["match_type"].(string)
	var matchType MatchType
	switch matchTypeStr {
	case "plain":
		matchType = MatchPlain
	case "regex":
		matchType = MatchRegex
	case "line":
		matchType = MatchLine
	case "":
		matchType = MatchPlain // default
	default:
		return nil, &config.ConfigError{
			Message:  fmt.Sprintf("invalid match type %q in step %q", matchTypeStr, c.Name),
			Location: c.Loc,
		}
	}

	match, _ := c.Step.Options["match"].(string)
	content, _ := c.Step.Options["content"].(string)
	useCaptures, _ := c.Step.Options["use_captures"].(bool)

	var lineNumber int
	if ln, ok := c.Step.Options["line_number"].(int); ok {
		lineNumber = ln
	}

	// Validate required fields based on match type
	if matchType != MatchLine && match == "" {
		return nil, &config.ConfigError{
			Message:  fmt.Sprintf("match pattern is required in step %q", c.Name),
			Location: c.Loc,
		}
	}

	return &FileEditStep{
		BaseStep:    BaseStep{title: c.Name, nodeName: nodeName},
		filePath:    filePath,
		operation:   operation,
		position:    position,
		matchType:   matchType,
		match:       match,
		lineNumber:  lineNumber,
		content:     content,
		useCaptures: useCaptures,
	}, nil
}
