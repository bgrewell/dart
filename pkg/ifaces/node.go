package ifaces

import (
	"io/fs"

	"github.com/bgrewell/dart/internal/execution"
)

// NodeOptions represents configuration options for a node
type NodeOptions *map[string]interface{}

// FileInfo describes a file on a node's filesystem.
type FileInfo struct {
	Size  int64
	Mode  fs.FileMode
	IsDir bool
}

// Node is an interface representing a computing entity (e.g., a server, VM, or container)
// that can be used as a target for test operations, such as executing commands or participating
// in distributed systems for testing purposes.
type Node interface {
	Setup() error
	Teardown() error
	Execute(command string, options ...execution.ExecutionOption) (result *execution.ExecutionResult, err error)
	Close() error

	// ReadFile returns the contents of the file at path on this node.
	ReadFile(path string) ([]byte, error)
	// WriteFile writes data to path on this node, creating or truncating the file.
	WriteFile(path string, data []byte, mode fs.FileMode) error
	// RemoveFile removes the file at path on this node.
	RemoveFile(path string) error
	// Stat returns information about path on this node. If the file does not
	// exist the returned error wraps fs.ErrNotExist.
	Stat(path string) (FileInfo, error)
	// MkdirAll creates the directory at path on this node, along with any
	// necessary parents.
	MkdirAll(path string, mode fs.FileMode) error
}
