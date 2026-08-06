package formatters

import "sync"

// MockTestCompleter is a lightweight mock implementation of TestCompleter
// for unit testing.
type MockTestCompleter struct {
	mu        sync.Mutex
	status    string
	completed bool
	passed    []bool
	skipped   bool
	failed    bool
	errored   bool
}

// NewMockTestCompleter creates a new instance of MockTestCompleter.
func NewMockTestCompleter() *MockTestCompleter {
	return &MockTestCompleter{}
}

// Update updates the status of the test.
func (m *MockTestCompleter) Update(status string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.status = status
}

// Complete marks the test as completed with per-check outcomes.
func (m *MockTestCompleter) Complete(passed []bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.completed = true
	m.passed = passed
}

// Passed marks the test as passed.
func (m *MockTestCompleter) Passed() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.completed = true
}

// Skip marks the test as skipped.
func (m *MockTestCompleter) Skip() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.skipped = true
}

// IsSkipped returns whether the test was skipped.
func (m *MockTestCompleter) IsSkipped() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.skipped
}

// Fail marks the test as failed.
func (m *MockTestCompleter) Fail() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.failed = true
}

// Error marks the test as having encountered an error.
func (m *MockTestCompleter) Error() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.errored = true
}

// IsCompleted returns whether the test completed.
func (m *MockTestCompleter) IsCompleted() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.completed
}

// IsErrored returns whether the test encountered an error.
func (m *MockTestCompleter) IsErrored() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.errored
}

// PassedChecks returns the per-check outcomes recorded at completion.
func (m *MockTestCompleter) PassedChecks() []bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.passed
}
