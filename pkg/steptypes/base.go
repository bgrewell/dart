package steptypes

import (
	"os"

	"github.com/bgrewell/dart/internal/config"
	"github.com/bgrewell/dart/internal/formatters"
	"github.com/bgrewell/dart/internal/helpers"
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
		node, ok := nodes[c.Node]
		if !ok {
			return nil, helpers.ErrNodeNotFound
		}

		switch c.Step.Type {
		case "simulated":
			steps = append(steps, &SimulatedStep{
				BaseStep:  BaseStep{title: c.Name, nodeName: c.Node},
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
						return nil, helpers.ErrCommandNotString
					}
					commands[i] = s
				}
			default:
				return nil, helpers.ErrInvalidCommandType
			}
			steps = append(steps, &ExecuteStep{
				BaseStep: BaseStep{title: c.Name, nodeName: c.Node},
				node:     node,
				commands: commands,
			})
		case "apt":
			rawPackages, ok := c.Step.Options["packages"].([]interface{})
			if !ok {
				return nil, helpers.ErrPackagesNotArray
			}

			packages := make([]string, len(rawPackages))
			for i, pkg := range rawPackages {
				packages[i], ok = pkg.(string)
				if !ok {
					return nil, helpers.ErrPackageNotString
				}
			}

			steps = append(steps, &AptStep{
				BaseStep: BaseStep{title: c.Name, nodeName: c.Node},
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
			return nil, helpers.ErrUnknownStepType
		}
	}

	return steps, nil
}

// createFileCreateStep creates a FileCreateStep from configuration
func createFileCreateStep(c *config.StepConfig, _ ifaces.Node) (*FileCreateStep, error) {
	filePath, _ := c.Step.Options["path"].(string)
	if filePath == "" {
		return nil, helpers.ErrMissingFilePath
	}

	contents, _ := c.Step.Options["contents"].(string)
	overwrite, _ := c.Step.Options["overwrite"].(bool)
	createDir, _ := c.Step.Options["create_dir"].(bool)

	var mode os.FileMode = 0644
	if modeVal, ok := c.Step.Options["mode"].(int); ok {
		mode = os.FileMode(modeVal)
	}

	return &FileCreateStep{
		BaseStep:  BaseStep{title: c.Name, nodeName: c.Node},
		filePath:  filePath,
		contents:  contents,
		overwrite: overwrite,
		mode:      mode,
		createDir: createDir,
	}, nil
}

// createFileDeleteStep creates a FileDeleteStep from configuration
func createFileDeleteStep(c *config.StepConfig, _ ifaces.Node) (*FileDeleteStep, error) {
	filePath, _ := c.Step.Options["path"].(string)
	if filePath == "" {
		return nil, helpers.ErrMissingFilePath
	}

	ignoreErrors, _ := c.Step.Options["ignore_errors"].(bool)

	return &FileDeleteStep{
		BaseStep:     BaseStep{title: c.Name, nodeName: c.Node},
		filePath:     filePath,
		ignoreErrors: ignoreErrors,
	}, nil
}

// createFileEditStep creates a FileEditStep from configuration
func createFileEditStep(c *config.StepConfig, _ ifaces.Node) (*FileEditStep, error) {
	filePath, _ := c.Step.Options["path"].(string)
	if filePath == "" {
		return nil, helpers.ErrMissingFilePath
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
		return nil, helpers.ErrInvalidEditOperation
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
		return nil, helpers.ErrInvalidInsertPosition
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
		return nil, helpers.ErrInvalidMatchType
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
		return nil, helpers.ErrMissingMatch
	}

	return &FileEditStep{
		BaseStep:    BaseStep{title: c.Name, nodeName: c.Node},
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
