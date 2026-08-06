package steptypes

import (
	"fmt"

	"github.com/bgrewell/dart/internal/config"
	"github.com/bgrewell/dart/internal/formatters"
	"github.com/bgrewell/dart/pkg/ifaces"
)

var _ ifaces.Step = &FileDeleteStep{}

// FileDeleteStep deletes a file on the step's target node.
type FileDeleteStep struct {
	BaseStep
	node         ifaces.Node
	filePath     string
	ignoreErrors bool
}

func newFileDeleteStep(c *config.StepConfig, node ifaces.Node) (ifaces.Step, error) {
	filePath, err := requiredString(c, "path", "file path is required")
	if err != nil {
		return nil, err
	}
	ignoreErrors, err := optBool(c, "ignore_errors")
	if err != nil {
		return nil, err
	}

	return &FileDeleteStep{
		BaseStep:     baseFor(c),
		node:         node,
		filePath:     filePath,
		ignoreErrors: ignoreErrors,
	}, nil
}

// Run deletes the file at the specified path on the target node.
func (s *FileDeleteStep) Run(updater formatters.TaskCompleter) error {
	ops := fileOpsFor(s.node)
	if err := ops.DeleteFile(s.filePath); err != nil {
		if s.ignoreErrors {
			updater.Complete()
			return nil
		}
		updater.Error()
		return fmt.Errorf("failed to delete file: %w", err)
	}

	updater.Complete()
	return nil
}
