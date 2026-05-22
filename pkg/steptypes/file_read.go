package steptypes

import (
	"fmt"
	"strings"

	"github.com/bgrewell/dart/internal/formatters"
	"github.com/bgrewell/dart/pkg/ifaces"
)

var _ ifaces.Step = &FileReadStep{}

// FileReadStep reads a file from the configured node and validates its content.
type FileReadStep struct {
	BaseStep
	node     ifaces.Node
	filePath string
	contains string
}

// Run reads the file and verifies expected content.
func (s *FileReadStep) Run(updater formatters.TaskCompleter) error {
	data, err := s.node.ReadFile(s.filePath)
	if err != nil {
		updater.Error()
		return fmt.Errorf("failed to read file: %w", err)
	}

	if s.contains != "" && !strings.Contains(string(data), s.contains) {
		updater.Error()
		return fmt.Errorf("file content validation failed: expected content missing")
	}

	updater.Complete()
	return nil
}
