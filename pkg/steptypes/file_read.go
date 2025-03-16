package steptypes

import (
	"fmt"
	"github.com/bgrewell/dart/pkg/ifaces"
	"os"
	"strings"

	"github.com/bgrewell/dart/internal/formatters"
)

var _ ifaces.Step = &FileReadStep{}

// FileReadStep reads a file and validates its content.
type FileReadStep struct {
	BaseStep
	filePath string
	contains string
}

// Run reads the file and verifies expected content.
func (s *FileReadStep) Run(updater formatters.TaskCompleter) error {
	data, err := os.ReadFile(s.filePath)
	if err != nil {
		updater.Error()
		return fmt.Errorf("failed to read file: %w", err)
	}

	content := string(data)
	if s.contains != "" && !strings.Contains(content, s.contains) {
		updater.Error()
		return fmt.Errorf("file content validation failed: expected content missing")
	}

	updater.Complete()
	return nil
}
