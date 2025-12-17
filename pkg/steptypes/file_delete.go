package steptypes

import (
	"fmt"
	"os"

	"github.com/bgrewell/dart/internal/formatters"
	"github.com/bgrewell/dart/pkg/ifaces"
)

var _ ifaces.Step = &FileDeleteStep{}

// FileDeleteStep deletes a specified file.
type FileDeleteStep struct {
	BaseStep
	node         ifaces.Node
	filePath     string
	ignoreErrors bool
}

// Run deletes the file at the specified path.
func (s *FileDeleteStep) Run(updater formatters.TaskCompleter) error {
	err := os.Remove(s.filePath)
	if err != nil {
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
