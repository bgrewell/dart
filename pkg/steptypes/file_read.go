package steptypes

import (
	"fmt"
	"strings"

	"github.com/bgrewell/dart/internal/config"
	"github.com/bgrewell/dart/internal/formatters"
	"github.com/bgrewell/dart/pkg/ifaces"
)

var _ ifaces.Step = &FileReadStep{}

// FileReadStep reads a file on the step's target node and validates its
// content.
type FileReadStep struct {
	BaseStep
	node     ifaces.Node
	filePath string
	contains string
}

func newFileReadStep(c *config.StepConfig, node ifaces.Node) (ifaces.Step, error) {
	filePath, err := requiredString(c, "path", "file path is required")
	if err != nil {
		return nil, err
	}
	contains, _, err := optString(c, "contains")
	if err != nil {
		return nil, err
	}

	return &FileReadStep{
		BaseStep: baseFor(c),
		node:     node,
		filePath: filePath,
		contains: contains,
	}, nil
}

// Run reads the file and verifies expected content.
func (s *FileReadStep) Run(updater formatters.TaskCompleter) error {
	ops := fileOpsFor(s.node)
	content, err := ops.ReadFile(s.filePath)
	if err != nil {
		updater.Error()
		return fmt.Errorf("failed to read file: %w", err)
	}

	if s.contains != "" && !strings.Contains(content, s.contains) {
		updater.Error()
		return fmt.Errorf("file content validation failed: expected content missing")
	}

	updater.Complete()
	return nil
}
