package ifaces

import (
	"testing"
)

// MockPlatformManager is a mock implementation of PlatformManager for testing
type MockPlatformManager struct {
	configured     bool
	setupCalled    bool
	teardownCalled bool
	setupErr       error
	teardownErr    error
	name           string
}

func (m *MockPlatformManager) Configured() bool {
	return m.configured
}

func (m *MockPlatformManager) Setup() error {
	m.setupCalled = true
	return m.setupErr
}

func (m *MockPlatformManager) Teardown() error {
	m.teardownCalled = true
	return m.teardownErr
}

func (m *MockPlatformManager) Name() string {
	return m.name
}

// NewMockPlatformManager creates a new mock platform manager for testing
func NewMockPlatformManager(name string, configured bool) *MockPlatformManager {
	return &MockPlatformManager{
		name:       name,
		configured: configured,
	}
}

func TestPlatformManagerInterface(t *testing.T) {
	// Test that MockPlatformManager implements PlatformManager
	var _ PlatformManager = &MockPlatformManager{}

	t.Run("configured platform", func(t *testing.T) {
		pm := NewMockPlatformManager("test", true)
		if !pm.Configured() {
			t.Error("Expected Configured() to return true")
		}
		if pm.Name() != "test" {
			t.Errorf("Expected Name() to return 'test', got %s", pm.Name())
		}
	})

	t.Run("unconfigured platform", func(t *testing.T) {
		pm := NewMockPlatformManager("empty", false)
		if pm.Configured() {
			t.Error("Expected Configured() to return false")
		}
	})

	t.Run("setup and teardown", func(t *testing.T) {
		pm := NewMockPlatformManager("docker", true)

		if err := pm.Setup(); err != nil {
			t.Errorf("Setup() returned unexpected error: %v", err)
		}
		if !pm.setupCalled {
			t.Error("Expected Setup() to be called")
		}

		if err := pm.Teardown(); err != nil {
			t.Errorf("Teardown() returned unexpected error: %v", err)
		}
		if !pm.teardownCalled {
			t.Error("Expected Teardown() to be called")
		}
	})
}
