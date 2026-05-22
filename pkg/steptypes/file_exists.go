package steptypes

import (
	"errors"
	"fmt"
	"io/fs"

	"github.com/bgrewell/dart/internal/formatters"
	"github.com/bgrewell/dart/pkg/ifaces"
)

var _ ifaces.Step = &FileExistsStep{}

// FileExistsStep checks if a file exists on the configured node.
type FileExistsStep struct {
	BaseStep
	node     ifaces.Node
	filePath string
}

// Run verifies the file's existence.
func (s *FileExistsStep) Run(updater formatters.TaskCompleter) error {
	_, err := s.node.Stat(s.filePath)
	if errors.Is(err, fs.ErrNotExist) {
		updater.Error()
		return fmt.Errorf("file does not exist: %s", s.filePath)
	}
	if err != nil {
		updater.Error()
		return fmt.Errorf("error checking file: %w", err)
	}

	updater.Complete()
	return nil
}
