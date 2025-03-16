package steptypes

import (
	"fmt"
	"github.com/bgrewell/dart/pkg/ifaces"
	"os"

	"github.com/bgrewell/dart/internal/formatters"
)

var _ ifaces.Step = &FileExistsStep{}

// FileExistsStep checks if a file exists.
type FileExistsStep struct {
	BaseStep
	filePath string
}

// Run verifies the file's existence.
func (s *FileExistsStep) Run(updater formatters.TaskCompleter) error {
	_, err := os.Stat(s.filePath)
	if os.IsNotExist(err) {
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
