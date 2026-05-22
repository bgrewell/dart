package steptypes

import (
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"

	"github.com/bgrewell/dart/internal/formatters"
	"github.com/bgrewell/dart/pkg/ifaces"
)

var _ ifaces.Step = &FileCreateStep{}

// FileCreateStep creates a new file with specified content on the configured node.
type FileCreateStep struct {
	BaseStep
	node      ifaces.Node
	filePath  string
	contents  string
	overwrite bool
	mode      fs.FileMode
	createDir bool
}

// Run creates the file with the specified content.
func (s *FileCreateStep) Run(updater formatters.TaskCompleter) error {
	if s.createDir {
		dir := filepath.Dir(s.filePath)
		if err := s.node.MkdirAll(dir, 0755); err != nil {
			updater.Error()
			return fmt.Errorf("failed to create directories: %w", err)
		}
	}

	if !s.overwrite {
		if _, err := s.node.Stat(s.filePath); err == nil {
			updater.Error()
			return fmt.Errorf("failed to create file: %s already exists", s.filePath)
		} else if !errors.Is(err, fs.ErrNotExist) {
			updater.Error()
			return fmt.Errorf("failed to stat file: %w", err)
		}
	}

	mode := s.mode
	if mode == 0 {
		mode = 0644
	}

	if err := s.node.WriteFile(s.filePath, []byte(s.contents), mode); err != nil {
		updater.Error()
		return fmt.Errorf("failed to create file: %w", err)
	}

	updater.Complete()
	return nil
}
