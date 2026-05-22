package steptypes

import (
	"errors"
	"fmt"
	"io/fs"

	"github.com/bgrewell/dart/internal/formatters"
	"github.com/bgrewell/dart/pkg/ifaces"
)

var _ ifaces.Step = &FileWriteStep{}

// FileWriteStep writes content to a file on the configured node.
type FileWriteStep struct {
	BaseStep
	node      ifaces.Node
	filePath  string
	contents  string
	overwrite bool
}

// Run writes the content to the file.
func (s *FileWriteStep) Run(updater formatters.TaskCompleter) error {
	if !s.overwrite {
		if _, err := s.node.Stat(s.filePath); err == nil {
			updater.Error()
			return fmt.Errorf("failed to write file: %s already exists", s.filePath)
		} else if !errors.Is(err, fs.ErrNotExist) {
			updater.Error()
			return fmt.Errorf("failed to stat file: %w", err)
		}
	}

	if err := s.node.WriteFile(s.filePath, []byte(s.contents), 0644); err != nil {
		updater.Error()
		return fmt.Errorf("failed to write file: %w", err)
	}

	updater.Complete()
	return nil
}
