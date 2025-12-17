package steptypes

import (
	"fmt"
	"io"

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
	// When not ignoring errors, check if file exists first
	if !s.ignoreErrors {
		checkCmd := fmt.Sprintf("test -f '%s'", s.filePath)
		result, err := s.node.Execute(checkCmd)
		if err != nil {
			updater.Error()
			return fmt.Errorf("failed to check file existence: %w", err)
		}
		if result.ExitCode != 0 {
			updater.Error()
			return fmt.Errorf("failed to delete file: file does not exist: %s", s.filePath)
		}
	}

	// Delete the file using rm command
	rmCmd := fmt.Sprintf("rm -f '%s'", s.filePath)
	result, err := s.node.Execute(rmCmd)
	if err != nil {
		if s.ignoreErrors {
			updater.Complete()
			return nil
		}
		updater.Error()
		return fmt.Errorf("failed to execute delete command: %w", err)
	}
	if result.ExitCode != 0 {
		if s.ignoreErrors {
			updater.Complete()
			return nil
		}
		updater.Error()
		stderr, _ := io.ReadAll(result.Stderr)
		return fmt.Errorf("failed to delete file: %s", string(stderr))
	}

	updater.Complete()
	return nil
}
