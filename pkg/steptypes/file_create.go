package steptypes

import (
	"fmt"
	"os"

	"github.com/bgrewell/dart/internal/config"
	"github.com/bgrewell/dart/internal/formatters"
	"github.com/bgrewell/dart/pkg/ifaces"
)

var _ ifaces.Step = &FileCreateStep{}

// FileCreateStep creates a new file with specified content on the step's
// target node. Also registered as file_write.
type FileCreateStep struct {
	BaseStep
	node      ifaces.Node
	filePath  string
	contents  string
	overwrite bool
	mode      os.FileMode
	createDir bool
}

func newFileCreateStep(c *config.StepConfig, node ifaces.Node) (ifaces.Step, error) {
	filePath, err := requiredString(c, "path", "file path is required")
	if err != nil {
		return nil, err
	}
	contents, _, err := optString(c, "contents")
	if err != nil {
		return nil, err
	}
	overwrite, err := optBool(c, "overwrite")
	if err != nil {
		return nil, err
	}
	createDir, err := optBool(c, "create_dir")
	if err != nil {
		return nil, err
	}
	mode, err := optFileMode(c, "mode")
	if err != nil {
		return nil, err
	}

	return &FileCreateStep{
		BaseStep:  baseFor(c),
		node:      node,
		filePath:  filePath,
		contents:  contents,
		overwrite: overwrite,
		mode:      mode,
		createDir: createDir,
	}, nil
}

// Run creates the file with the specified content on the target node.
func (s *FileCreateStep) Run(updater formatters.TaskCompleter) error {
	ops := fileOpsFor(s.node)
	if err := ops.WriteFile(s.filePath, s.contents, s.mode, s.overwrite, s.createDir); err != nil {
		updater.Error()
		return fmt.Errorf("failed to create file: %w", err)
	}

	updater.Complete()
	return nil
}
