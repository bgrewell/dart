package nodetypes

import (
	"bytes"
	"errors"
	"github.com/bgrewell/dart/pkg/ifaces"
	"io"
	"sync"

	"github.com/bgrewell/dart/internal/execution"
	"github.com/stretchr/testify/mock"
)

var _ ifaces.Node = &MockNode{}

// MockNode is a mock implementation of the ifaces.Node interface for unit testing.
type MockNode struct {
	mock.Mock
	mu        sync.Mutex
	responses map[string]*execution.ExecutionResult
	errors    map[string]error
}

// NewMockNode creates a new instance of MockNode.
func NewMockNode() *MockNode {
	return &MockNode{
		responses: make(map[string]*execution.ExecutionResult),
		errors:    make(map[string]error),
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
