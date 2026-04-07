package docker

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

// TestNewComposeStackRegistry tests creating a new registry
func TestNewComposeStackRegistry(t *testing.T) {
	registry := NewComposeStackRegistry()
	assert.NotNil(t, registry)
	assert.NotNil(t, registry.stacks)
	assert.NotNil(t, registry.refCounts)
}

// TestGetOrCreateStack tests getting or creating a stack
func TestGetOrCreateStack(t *testing.T) {
	registry := NewComposeStackRegistry()

	callCount := 0
	createFn := func() (*ComposeStack, error) {
		callCount++
		return &ComposeStack{
			Name:         "test-stack",
			ComposeFile:  "/path/to/compose.yml",
			ProjectName:  "test-project",
			containerIDs: make(map[string]string),
		}, nil
	}

	key := "test-key"

	// First call should create the stack
	stack1, err := registry.GetOrCreateStack(key, createFn)
	assert.Nil(t, err)
	assert.NotNil(t, stack1)
	assert.Equal(t, 1, callCount)
	assert.Equal(t, 1, registry.refCounts[key])

	// Second call should return the same stack without calling createFn
	stack2, err := registry.GetOrCreateStack(key, createFn)
	assert.Nil(t, err)
	assert.Equal(t, stack1, stack2)
	assert.Equal(t, 1, callCount) // createFn not called again
	assert.Equal(t, 2, registry.refCounts[key])
}

// TestReleaseStack tests releasing a stack
func TestReleaseStack(t *testing.T) {
	registry := NewComposeStackRegistry()

	createFn := func() (*ComposeStack, error) {
		return &ComposeStack{
			Name:         "test-stack",
			containerIDs: make(map[string]string),
		}, nil
	}

	key := "test-key"

	// Create a stack with 2 references
	registry.GetOrCreateStack(key, createFn)
	registry.GetOrCreateStack(key, createFn)
	assert.Equal(t, 2, registry.refCounts[key])

	// First release should not teardown
	shouldTeardown := registry.ReleaseStack(key)
	assert.False(t, shouldTeardown)
	assert.Equal(t, 1, registry.refCounts[key])

	// Second release should teardown
	shouldTeardown = registry.ReleaseStack(key)
	assert.True(t, shouldTeardown)
	assert.Equal(t, 0, len(registry.refCounts))
	assert.Equal(t, 0, len(registry.stacks))
}

// TestGetStackKey tests the stack key generation
func TestGetStackKey(t *testing.T) {
	key1 := GetStackKey("/path/to/compose.yml", "project1")
	key2 := GetStackKey("/path/to/compose.yml", "project2")
	key3 := GetStackKey("/path/to/other.yml", "project1")

	assert.NotEqual(t, key1, key2)
	assert.NotEqual(t, key1, key3)
	assert.NotEqual(t, key2, key3)

	// Same inputs should produce same key
	key4 := GetStackKey("/path/to/compose.yml", "project1")
	assert.Equal(t, key1, key4)
}
