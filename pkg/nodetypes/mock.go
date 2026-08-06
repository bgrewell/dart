package nodetypes

import (
	"fmt"
	"strings"
	"sync"

	"github.com/bgrewell/dart/internal/execution"
	"github.com/bgrewell/dart/pkg/ifaces"
)

var _ ifaces.Node = &MockNode{}

// mockResponse holds the canned output for a command as strings, so each
// Execute call can mint fresh readers; the streams in an ExecutionResult are
// one-shot and must not be shared between calls.
type mockResponse struct {
	exitCode int
	stdout   string
	stderr   string
}

// MockNode is a mock implementation of the ifaces.Node interface for unit
// testing. Configure it with SetResponse/SetError; unconfigured commands
// return an error naming the command.
type MockNode struct {
	mu        sync.Mutex
	responses map[string]mockResponse
	errors    map[string]error
}

// NewMockNode creates a new instance of MockNode.
func NewMockNode() *MockNode {
	return &MockNode{
		responses: make(map[string]mockResponse),
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

	if response, exists := m.responses[command]; exists {
		return &execution.ExecutionResult{
			ExecutionId: "mock-id",
			ExitCode:    response.exitCode,
			Stdout:      strings.NewReader(response.stdout),
			Stderr:      strings.NewReader(response.stderr),
		}, nil
	}

	return nil, fmt.Errorf("mock node has no response for command %q", command)
}

// SetResponse configures a mock response for a given command.
func (m *MockNode) SetResponse(command string, exitCode int, stdout, stderr string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.responses[command] = mockResponse{
		exitCode: exitCode,
		stdout:   stdout,
		stderr:   stderr,
	}
}

// SetError configures a mock error for a given command.
func (m *MockNode) SetError(command string, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.errors[command] = err
}
