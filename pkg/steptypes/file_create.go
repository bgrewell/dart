package steptypes

import (
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/bgrewell/dart/internal/formatters"
	"github.com/bgrewell/dart/pkg/ifaces"
)

var _ ifaces.Step = &FileCreateStep{}

// FileCreateStep creates a new file with specified content.
type FileCreateStep struct {
	BaseStep
	node      ifaces.Node
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
		mkdirCmd := fmt.Sprintf("mkdir -p '%s'", dir)
		result, err := s.node.Execute(mkdirCmd)
		if err != nil {
			updater.Error()
			return fmt.Errorf("failed to execute mkdir command: %w", err)
		}
		if result.ExitCode != 0 {
			updater.Error()
			stderr, _ := io.ReadAll(result.Stderr)
			return fmt.Errorf("failed to create directories: %s", string(stderr))
		}
	}

	// Check if file exists if overwrite is false
	if !s.overwrite {
		checkCmd := fmt.Sprintf("test -f '%s'", s.filePath)
		result, err := s.node.Execute(checkCmd)
		if err != nil {
			updater.Error()
			return fmt.Errorf("failed to check file existence: %w", err)
		}
		if result.ExitCode == 0 {
			updater.Error()
			return fmt.Errorf("file already exists and overwrite is false: %s", s.filePath)
		}
	}

	// Use base64 encoding to safely transfer the content
	encoded := base64.StdEncoding.EncodeToString([]byte(s.contents))
	
	// Set file permissions - convert os.FileMode to octal string
	mode := s.mode
	if mode == 0 {
		mode = 0644
	}
	modeStr := fmt.Sprintf("%04o", mode)

	// Write content using base64 decoding and set permissions
	writeCmd := fmt.Sprintf("echo '%s' | base64 -d > '%s' && chmod %s '%s'", encoded, s.filePath, modeStr, s.filePath)
	result, err := s.node.Execute(writeCmd)
	if err != nil {
		updater.Error()
		return fmt.Errorf("failed to execute write command: %w", err)
	}
	if result.ExitCode != 0 {
		updater.Error()
		stderr, _ := io.ReadAll(result.Stderr)
		return fmt.Errorf("failed to create file: %s", string(stderr))
	}

	updater.Complete()
	return nil
}

