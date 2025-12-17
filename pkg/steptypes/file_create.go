package steptypes

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/bgrewell/dart/internal/formatters"
	"github.com/bgrewell/dart/pkg/ifaces"
)

var _ ifaces.Step = &FileCreateStep{}

// FileCreateStep creates a new file with specified content.
type FileCreateStep struct {
	BaseStep
	filePath  string
	contents  string
	overwrite bool
	mode      os.FileMode
	createDir bool
}

// Run creates the file with the specified content.
func (s *FileCreateStep) Run(updater formatters.TaskCompleter) error {
	// Create parent directories if requested
	if s.createDir {
		dir := filepath.Dir(s.filePath)
		if err := os.MkdirAll(dir, 0755); err != nil {
			updater.Error()
			return fmt.Errorf("failed to create directories: %w", err)
		}
	}

	flags := os.O_WRONLY | os.O_CREATE
	if s.overwrite {
		flags |= os.O_TRUNC
	} else {
		flags |= os.O_EXCL
	}

	mode := s.mode
	if mode == 0 {
		mode = 0644
	}

	file, err := os.OpenFile(s.filePath, flags, mode)
	if err != nil {
		updater.Error()
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer file.Close()

	_, err = file.WriteString(s.contents)
	if err != nil {
		updater.Error()
		return fmt.Errorf("failed to write file contents: %w", err)
	}

	updater.Complete()
	return nil
}
