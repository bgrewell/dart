package nodetypes

import (
	"bytes"
	"errors"
	"io"
	"io/fs"
	"path"
	"strings"
	"sync"

	"github.com/bgrewell/dart/internal/execution"
	"github.com/bgrewell/dart/pkg/ifaces"
	"github.com/stretchr/testify/mock"
)

var _ ifaces.Node = &MockNode{}

// mockFile is an entry in the MockNode's in-memory filesystem.
type mockFile struct {
	data  []byte
	mode  fs.FileMode
	isDir bool
}

// MockNode is a mock implementation of the ifaces.Node interface for unit testing.
type MockNode struct {
	mock.Mock
	mu        sync.Mutex
	responses map[string]*execution.ExecutionResult
	errors    map[string]error
	files     map[string]*mockFile
}

// NewMockNode creates a new instance of MockNode.
func NewMockNode() *MockNode {
	return &MockNode{
		responses: make(map[string]*execution.ExecutionResult),
		errors:    make(map[string]error),
		files:     make(map[string]*mockFile),
	}
}

// Setup is a no-op for the mock.
func (m *MockNode) Setup() error {
	return nil
}

// Teardown is a no-op for the mock.
func (m *MockNode) Teardown() error {
	return nil
}

// Close is a no-op for the mock.
func (m *MockNode) Close() error {
	return nil
}

// Execute simulates executing a command.
func (m *MockNode) Execute(command string, options ...execution.ExecutionOption) (*execution.ExecutionResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if err, exists := m.errors[command]; exists {
		return nil, err
	}

	if result, exists := m.responses[command]; exists {
		return result, nil
	}

	return nil, errors.New("mock node has no response for command")
}

// SetResponse configures a mock response for a given command.
func (m *MockNode) SetResponse(command string, exitCode int, stdout, stderr string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.responses[command] = &execution.ExecutionResult{
		ExecutionId: "mock-id",
		ExitCode:    exitCode,
		Stdout:      io.NopCloser(bytes.NewBufferString(stdout)),
		Stderr:      io.NopCloser(bytes.NewBufferString(stderr)),
	}
}

// SetError configures a mock error for a given command.
func (m *MockNode) SetError(command string, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.errors[command] = err
}

// SeedFile pre-populates a file in the mock filesystem.
func (m *MockNode) SeedFile(path string, data []byte, mode fs.FileMode) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.files[path] = &mockFile{data: data, mode: mode}
}

// GetFile returns the current contents of a mock file, if present.
func (m *MockNode) GetFile(path string) ([]byte, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	f, ok := m.files[path]
	if !ok || f.isDir {
		return nil, false
	}
	out := make([]byte, len(f.data))
	copy(out, f.data)
	return out, true
}

func (m *MockNode) ReadFile(p string) ([]byte, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	f, ok := m.files[p]
	if !ok {
		return nil, &fs.PathError{Op: "open", Path: p, Err: fs.ErrNotExist}
	}
	if f.isDir {
		return nil, &fs.PathError{Op: "read", Path: p, Err: errors.New("is a directory")}
	}
	out := make([]byte, len(f.data))
	copy(out, f.data)
	return out, nil
}

func (m *MockNode) WriteFile(p string, data []byte, mode fs.FileMode) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	buf := make([]byte, len(data))
	copy(buf, data)
	m.files[p] = &mockFile{data: buf, mode: mode}
	return nil
}

func (m *MockNode) RemoveFile(p string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.files[p]; !ok {
		return &fs.PathError{Op: "remove", Path: p, Err: fs.ErrNotExist}
	}
	delete(m.files, p)
	return nil
}

func (m *MockNode) Stat(p string) (ifaces.FileInfo, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	f, ok := m.files[p]
	if !ok {
		return ifaces.FileInfo{}, &fs.PathError{Op: "stat", Path: p, Err: fs.ErrNotExist}
	}
	return ifaces.FileInfo{Size: int64(len(f.data)), Mode: f.mode, IsDir: f.isDir}, nil
}

func (m *MockNode) MkdirAll(p string, mode fs.FileMode) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	// Walk components and create any missing directories.
	parts := strings.Split(strings.Trim(p, "/"), "/")
	cur := ""
	for _, part := range parts {
		if part == "" {
			continue
		}
		cur = path.Join(cur, part)
		key := "/" + cur
		if existing, ok := m.files[key]; ok {
			if !existing.isDir {
				return &fs.PathError{Op: "mkdir", Path: key, Err: errors.New("not a directory")}
			}
			continue
		}
		m.files[key] = &mockFile{mode: mode | fs.ModeDir, isDir: true}
	}
	return nil
}
