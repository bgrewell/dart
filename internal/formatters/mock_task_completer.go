package formatters

import "sync"

// MockTaskCompleter is a lightweight mock implementation of TaskCompleter for unit testing.
type MockTaskCompleter struct {
	mu        sync.Mutex
	status    string
	completed bool
	failed    bool
	errored   bool
}

// NewMockTaskCompleter creates a new instance of MockTaskCompleter.
func NewMockTaskCompleter() *MockTaskCompleter {
	return &MockTaskCompleter{}
}

// Update updates the status of the task.
func (m *MockTaskCompleter) Update(status string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.status = status
}

// Complete marks the task as completed.
func (m *MockTaskCompleter) Complete() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.completed = true
}

// Fail marks the task as failed.
func (m *MockTaskCompleter) Fail() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.failed = true
}

// Error marks the task as having encountered an error.
func (m *MockTaskCompleter) Error() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.errored = true
}

// IsCompleted returns whether the task has completed successfully.
func (m *MockTaskCompleter) IsCompleted() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.completed
}

// IsFailed returns whether the task has failed.
func (m *MockTaskCompleter) IsFailed() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.failed
}

// IsErrored returns whether the task encountered an error.
func (m *MockTaskCompleter) IsErrored() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.errored
}

// Status returns the current status message.
func (m *MockTaskCompleter) Status() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.status
}
