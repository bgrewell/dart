package steptypes

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/bgrewell/dart/internal/config"
	"github.com/bgrewell/dart/internal/formatters"
	"github.com/bgrewell/dart/pkg/ifaces"
)

var _ ifaces.Step = &FilePushStep{}

// FilePushStep copies a file from the machine running DART onto the
// step's target node — deploying a config, binary, or fixture that lives
// in the repository rather than inline in the suite.
type FilePushStep struct {
	BaseStep
	node      ifaces.Node
	source    string
	dest      string
	mode      os.FileMode
	overwrite bool
	createDir bool
}

// localPath resolves a path the suite wrote for the machine running DART.
// Relative paths are relative to the suite file, so a suite behaves the same
// regardless of the directory DART is invoked from.
func localPath(c *config.StepConfig, raw string) (string, error) {
	resolved, err := config.ResolveLocalPath(c.SuiteDir, raw)
	if err != nil {
		return "", optionError(c, "%v in step %q", err, c.Name)
	}
	return resolved, nil
}

func newFilePushStep(c *config.StepConfig, node ifaces.Node) (ifaces.Step, error) {
	source, err := requiredString(c, "source", "source is required")
	if err != nil {
		return nil, err
	}
	if source, err = localPath(c, source); err != nil {
		return nil, err
	}
	dest, err := requiredString(c, "dest", "dest is required")
	if err != nil {
		return nil, err
	}
	mode, err := optFileMode(c, "mode")
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

	return &FilePushStep{
		BaseStep:  baseFor(c),
		node:      node,
		source:    source,
		dest:      dest,
		mode:      mode,
		overwrite: overwrite,
		createDir: createDir,
	}, nil
}

// Run reads the local source and writes it to the node.
func (s *FilePushStep) Run(updater formatters.TaskCompleter) error {
	contents, err := os.ReadFile(s.source)
	if err != nil {
		updater.Error()
		return fmt.Errorf("failed to read source %s: %w", s.source, err)
	}

	mode := s.mode
	if mode == 0 {
		// Carry the source's permissions when none were requested, so
		// pushed scripts stay executable
		if info, statErr := os.Stat(s.source); statErr == nil {
			mode = info.Mode().Perm()
		}
	}

	ops := fileOpsFor(s.node)
	if err := ops.WriteFile(s.dest, string(contents), mode, s.overwrite, s.createDir); err != nil {
		updater.Error()
		return fmt.Errorf("failed to push %s to %s: %w", s.source, s.dest, err)
	}

	updater.Complete()
	return nil
}

var _ ifaces.Step = &FileFetchStep{}

// FileFetchStep copies a file from the step's target node to the machine
// running DART — pulling logs or artifacts out for later analysis.
type FileFetchStep struct {
	BaseStep
	node      ifaces.Node
	source    string
	dest      string
	overwrite bool
	createDir bool
}

func newFileFetchStep(c *config.StepConfig, node ifaces.Node) (ifaces.Step, error) {
	source, err := requiredString(c, "source", "source is required")
	if err != nil {
		return nil, err
	}
	dest, err := requiredString(c, "dest", "dest is required")
	if err != nil {
		return nil, err
	}
	if dest, err = localPath(c, dest); err != nil {
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

	return &FileFetchStep{
		BaseStep:  baseFor(c),
		node:      node,
		source:    source,
		dest:      dest,
		overwrite: overwrite,
		createDir: createDir,
	}, nil
}

// Run reads the file from the node and writes it locally.
func (s *FileFetchStep) Run(updater formatters.TaskCompleter) error {
	ops := fileOpsFor(s.node)
	contents, err := ops.ReadFile(s.source)
	if err != nil {
		updater.Error()
		return fmt.Errorf("failed to fetch %s: %w", s.source, err)
	}

	if s.createDir {
		if err := os.MkdirAll(filepath.Dir(s.dest), 0755); err != nil {
			updater.Error()
			return fmt.Errorf("failed to create local directories: %w", err)
		}
	}

	// Consistent with the other file steps: an existing destination is an
	// error unless overwrite was requested, so a fetched artifact cannot
	// silently clobber a previous run's
	flags := os.O_WRONLY | os.O_CREATE
	if s.overwrite {
		flags |= os.O_TRUNC
	} else {
		flags |= os.O_EXCL
	}
	file, err := os.OpenFile(s.dest, flags, 0644)
	if err != nil {
		updater.Error()
		return fmt.Errorf("failed to write %s: %w", s.dest, err)
	}
	defer file.Close()
	if _, err := file.WriteString(contents); err != nil {
		updater.Error()
		return fmt.Errorf("failed to write %s: %w", s.dest, err)
	}

	updater.Complete()
	return nil
}

var _ ifaces.Step = &FileTemplateStep{}

// FileTemplateStep renders a local template file and writes the result to
// the node, so one template can configure many nodes with per-node values.
// Facts and suite variables are already substituted in the `values` map by
// the time the step runs; the template body itself uses Go template syntax
// against those values ({{ .name }}).
type FileTemplateStep struct {
	BaseStep
	node      ifaces.Node
	source    string
	dest      string
	values    map[string]interface{}
	mode      os.FileMode
	overwrite bool
	createDir bool
}

func newFileTemplateStep(c *config.StepConfig, node ifaces.Node) (ifaces.Step, error) {
	source, err := requiredString(c, "source", "source is required")
	if err != nil {
		return nil, err
	}
	if source, err = localPath(c, source); err != nil {
		return nil, err
	}
	dest, err := requiredString(c, "dest", "dest is required")
	if err != nil {
		return nil, err
	}
	mode, err := optFileMode(c, "mode")
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

	values := map[string]interface{}{}
	if raw, ok := c.Step.Options["values"]; ok {
		valueMap, ok := raw.(map[string]interface{})
		if !ok {
			return nil, optionError(c, "values must be a map in step %q (got %T)", c.Name, raw)
		}
		// missingkey=error only catches absent keys; a key present with a
		// null value renders the literal "<no value>" into a live config
		// with no error at all
		for key, value := range valueMap {
			if value == nil {
				return nil, optionError(c, "template value %q is null in step %q; give it a value or remove it", key, c.Name)
			}
		}
		values = valueMap
	}

	// Parse at config time so a broken template fails before anything runs
	body, err := os.ReadFile(source)
	if err != nil {
		return nil, optionError(c, "cannot read template %s in step %q: %v", source, c.Name, err)
	}
	if _, err := template.New(filepath.Base(source)).Option("missingkey=error").Parse(string(body)); err != nil {
		return nil, optionError(c, "template %s in step %q is invalid: %v", source, c.Name, err)
	}

	return &FileTemplateStep{
		BaseStep:  baseFor(c),
		node:      node,
		source:    source,
		dest:      dest,
		values:    values,
		mode:      mode,
		overwrite: overwrite,
		createDir: createDir,
	}, nil
}

// Run renders the template and writes the result to the node.
func (s *FileTemplateStep) Run(updater formatters.TaskCompleter) error {
	body, err := os.ReadFile(s.source)
	if err != nil {
		updater.Error()
		return fmt.Errorf("failed to read template %s: %w", s.source, err)
	}

	tmpl, err := template.New(filepath.Base(s.source)).Option("missingkey=error").Parse(string(body))
	if err != nil {
		updater.Error()
		return fmt.Errorf("template %s is invalid: %w", s.source, err)
	}

	var rendered strings.Builder
	if err := tmpl.Execute(&rendered, s.values); err != nil {
		updater.Error()
		return fmt.Errorf("failed to render %s: %w", s.source, err)
	}

	// Belt and braces for the nil paths text/template renders silently
	if strings.Contains(rendered.String(), "<no value>") {
		updater.Error()
		return fmt.Errorf("rendering %s produced \"<no value>\": a referenced value is null or missing", s.source)
	}

	ops := fileOpsFor(s.node)
	if err := ops.WriteFile(s.dest, rendered.String(), s.mode, s.overwrite, s.createDir); err != nil {
		updater.Error()
		return fmt.Errorf("failed to write rendered %s to %s: %w", s.source, s.dest, err)
	}

	updater.Complete()
	return nil
}
