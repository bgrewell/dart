package steptypes

import (
	"encoding/base64"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/bgrewell/dart/internal/execution"
	"github.com/bgrewell/dart/pkg/ifaces"
	"github.com/bgrewell/dart/pkg/nodetypes"
)

// fileOps abstracts the file operations used by the file_* steps so they act
// on the step's target node: local nodes use the native filesystem, remote
// nodes (docker, LXD, SSH, ...) go through shell commands over node.Execute.
// Note: the shell-based implementation requires a POSIX shell with cat,
// test, rm, mkdir, stat, chmod, printf, and base64 available on the node.
type fileOps interface {
	ReadFile(path string) (string, error)
	// WriteFile creates or overwrites a file. With overwrite false the write
	// fails if the file already exists. A zero mode leaves new files at the
	// default 0644 and existing files untouched.
	WriteFile(path, contents string, mode os.FileMode, overwrite, createDir bool) error
	DeleteFile(path string) error
	Exists(path string) (bool, error)
	// FileMode returns the permission bits of an existing file.
	FileMode(path string) (os.FileMode, error)
}

// fileOpsFor selects the implementation for a node. A nil node (steps
// constructed directly, e.g. in tests) and local nodes use the native
// filesystem; everything else operates through the node's shell.
func fileOpsFor(node ifaces.Node) fileOps {
	if node == nil {
		return localFileOps{}
	}
	if _, ok := node.(*nodetypes.LocalNode); ok {
		return localFileOps{}
	}
	return execFileOps{node: node}
}

// localFileOps implements fileOps against the local filesystem.
type localFileOps struct{}

func (localFileOps) ReadFile(p string) (string, error) {
	data, err := os.ReadFile(p)
	return string(data), err
}

func (localFileOps) WriteFile(p, contents string, mode os.FileMode, overwrite, createDir bool) error {
	if createDir {
		if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
			return fmt.Errorf("failed to create directories: %w", err)
		}
	}

	flags := os.O_WRONLY | os.O_CREATE
	if overwrite {
		flags |= os.O_TRUNC
	} else {
		flags |= os.O_EXCL
	}

	createMode := mode
	if createMode == 0 {
		createMode = 0644
	}

	file, err := os.OpenFile(p, flags, createMode)
	if err != nil {
		return err
	}
	defer file.Close()

	if _, err = file.WriteString(contents); err != nil {
		return err
	}

	// The OpenFile mode is masked by umask and only applies to newly created
	// files; an explicit chmod makes a requested mode exact in both cases.
	if mode != 0 {
		if err := os.Chmod(p, mode); err != nil {
			return fmt.Errorf("failed to set mode: %w", err)
		}
	}
	return nil
}

func (localFileOps) DeleteFile(p string) error {
	return os.Remove(p)
}

func (localFileOps) Exists(p string) (bool, error) {
	_, err := os.Stat(p)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

func (localFileOps) FileMode(p string) (os.FileMode, error) {
	info, err := os.Stat(p)
	if err != nil {
		return 0, err
	}
	return info.Mode().Perm(), nil
}

// execFileOps implements fileOps through shell commands on a node.
type execFileOps struct {
	node ifaces.Node
}

// shellQuote wraps s in single quotes, escaping embedded single quotes, so
// arbitrary paths survive shell interpolation.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// execChecked runs a command on a node and converts a non-zero exit code
// into an error carrying the command's stderr.
func execChecked(node ifaces.Node, command string) (*execution.ExecutionResult, error) {
	result, err := node.Execute(command)
	if err != nil {
		return nil, err
	}
	if result.ExitCode != 0 {
		stderr, _ := result.StderrBytes()
		return result, fmt.Errorf("command failed with exit code %d: %s",
			result.ExitCode, strings.TrimSpace(string(stderr)))
	}
	return result, nil
}

func (o execFileOps) ReadFile(p string) (string, error) {
	result, err := execChecked(o.node, "cat "+shellQuote(p))
	if err != nil {
		return "", fmt.Errorf("failed to read %s: %w", p, err)
	}
	data, err := result.StdoutBytes()
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func (o execFileOps) WriteFile(p, contents string, mode os.FileMode, overwrite, createDir bool) error {
	if !overwrite {
		exists, err := o.Exists(p)
		if err != nil {
			return err
		}
		if exists {
			return fmt.Errorf("file already exists: %s", p)
		}
	}

	if createDir {
		if _, err := execChecked(o.node, "mkdir -p "+shellQuote(path.Dir(p))); err != nil {
			return fmt.Errorf("failed to create directories: %w", err)
		}
	}

	// Content travels base64-encoded so arbitrary bytes survive the shell.
	encoded := base64.StdEncoding.EncodeToString([]byte(contents))
	cmd := fmt.Sprintf("printf '%%s' %s | base64 -d > %s", encoded, shellQuote(p))
	if _, err := execChecked(o.node, cmd); err != nil {
		return fmt.Errorf("failed to write %s: %w", p, err)
	}

	if mode != 0 {
		if _, err := execChecked(o.node, fmt.Sprintf("chmod %o %s", mode, shellQuote(p))); err != nil {
			return fmt.Errorf("failed to set mode on %s: %w", p, err)
		}
	}
	return nil
}

func (o execFileOps) DeleteFile(p string) error {
	_, err := execChecked(o.node, "rm -- "+shellQuote(p))
	return err
}

func (o execFileOps) Exists(p string) (bool, error) {
	result, err := o.node.Execute("test -e " + shellQuote(p))
	if err != nil {
		return false, err
	}
	return result.ExitCode == 0, nil
}

func (o execFileOps) FileMode(p string) (os.FileMode, error) {
	result, err := execChecked(o.node, "stat -c %a "+shellQuote(p))
	if err != nil {
		return 0, err
	}
	out, err := result.StdoutBytes()
	if err != nil {
		return 0, err
	}
	bits, err := strconv.ParseUint(strings.TrimSpace(string(out)), 8, 32)
	if err != nil {
		return 0, fmt.Errorf("unexpected stat output for %s: %q", p, string(out))
	}
	return os.FileMode(bits), nil
}
