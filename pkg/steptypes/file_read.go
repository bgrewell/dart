package steptypes

import (
	"fmt"
	"io"
	"strings"

	"github.com/bgrewell/dart/internal/formatters"
	"github.com/bgrewell/dart/pkg/ifaces"
)

var _ ifaces.Step = &FileReadStep{}

// FileReadStep reads a file and validates its content.
type FileReadStep struct {
	BaseStep
	node     ifaces.Node
	filePath string
	contains string
}

// Run reads the file and verifies expected content.
func (s *FileReadStep) Run(updater formatters.TaskCompleter) error {
	// Read the file from the node using cat
	readCmd := fmt.Sprintf("cat '%s'", s.filePath)
	result, err := s.node.Execute(readCmd)
	if err != nil {
		updater.Error()
		return fmt.Errorf("failed to execute read command: %w", err)
	}
	if result.ExitCode != 0 {
		updater.Error()
		stderr, _ := io.ReadAll(result.Stderr)
		return fmt.Errorf("failed to read file: %s", string(stderr))
	}

	// Read the content from stdout
	data, err := io.ReadAll(result.Stdout)
	if err != nil {
		updater.Error()
		return fmt.Errorf("failed to read command output: %w", err)
	}

	content := string(data)
	if s.contains != "" && !strings.Contains(content, s.contains) {
		updater.Error()
		return fmt.Errorf("file content validation failed: expected content missing")
	}

	updater.Complete()
	return nil
}
