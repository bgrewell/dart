package steptypes

import (
	"fmt"

	"github.com/bgrewell/dart/internal/config"
	"github.com/bgrewell/dart/internal/formatters"
	"github.com/bgrewell/dart/pkg/ifaces"
)

var _ ifaces.Step = &FileExistsStep{}

// FileExistsStep checks if a file exists on the step's target node.
type FileExistsStep struct {
	BaseStep
	node     ifaces.Node
	filePath string
}

func newFileExistsStep(c *config.StepConfig, node ifaces.Node) (ifaces.Step, error) {
	filePath, err := requiredString(c, "path", "file path is required")
	if err != nil {
		return nil, err
	}

	return &FileExistsStep{
		BaseStep: baseFor(c),
		node:     node,
		filePath: filePath,
	}, nil
}

// Run verifies the file's existence on the target node.
func (s *FileExistsStep) Run(updater formatters.TaskCompleter) error {
	ops := fileOpsFor(s.node)
	exists, err := ops.Exists(s.filePath)
	if err != nil {
		updater.Error()
		return fmt.Errorf("error checking file: %w", err)
	}
	if !exists {
		updater.Error()
		return fmt.Errorf("file does not exist: %s", s.filePath)
	}

	updater.Complete()
	return nil
}
