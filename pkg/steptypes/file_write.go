package steptypes

import (
	"fmt"
	"os"

	"github.com/bgrewell/dart/internal/formatters"
	"github.com/bgrewell/dart/pkg/ifaces"
)

var _ ifaces.Step = &FileWriteStep{}

// FileWriteStep writes content to a specified file.
type FileWriteStep struct {
	BaseStep
	node      ifaces.Node
	filePath  string
	contents  string
	overwrite bool
}

// Run writes the content to the file.
func (s *FileWriteStep) Run(updater formatters.TaskCompleter) error {
	flags := os.O_WRONLY | os.O_CREATE
	if s.overwrite {
		flags |= os.O_TRUNC
	} else {
		flags |= os.O_EXCL
	}

	file, err := os.OpenFile(s.filePath, flags, 0644)
	if err != nil {
		updater.Error()
		return fmt.Errorf("failed to write file: %w", err)
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
