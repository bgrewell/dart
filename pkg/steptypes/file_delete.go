package steptypes

import (
	"fmt"

	"github.com/bgrewell/dart/internal/formatters"
	"github.com/bgrewell/dart/pkg/ifaces"
)

var _ ifaces.Step = &FileDeleteStep{}

// FileDeleteStep deletes a specified file on the configured node.
type FileDeleteStep struct {
	BaseStep
	node         ifaces.Node
	filePath     string
	ignoreErrors bool
}

// Run deletes the file at the specified path.
func (s *FileDeleteStep) Run(updater formatters.TaskCompleter) error {
	if err := s.node.RemoveFile(s.filePath); err != nil {
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
