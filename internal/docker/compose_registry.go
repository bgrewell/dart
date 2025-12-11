package docker

import (
	"fmt"
	"sync"
)

// ComposeStackRegistry manages shared compose stacks to prevent duplicate up/down operations
type ComposeStackRegistry struct {
	stacks    map[string]*ComposeStack
	refCounts map[string]int
	mu        sync.Mutex
}

// NewComposeStackRegistry creates a new registry for managing compose stacks
func NewComposeStackRegistry() *ComposeStackRegistry {
	return &ComposeStackRegistry{
		stacks:    make(map[string]*ComposeStack),
		refCounts: make(map[string]int),
	}
}

// GetOrCreateStack returns an existing stack or creates a new one
// The key is typically the combination of compose file path and project name
func (r *ComposeStackRegistry) GetOrCreateStack(key string, createFn func() (*ComposeStack, error)) (*ComposeStack, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if stack, exists := r.stacks[key]; exists {
		r.refCounts[key]++
		return stack, nil
	}

	stack, err := createFn()
	if err != nil {
		return nil, err
	}

	r.stacks[key] = stack
	r.refCounts[key] = 1
	return stack, nil
}

// ReleaseStack decrements the reference count and returns true if it should be torn down
func (r *ComposeStackRegistry) ReleaseStack(key string) (shouldTeardown bool) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.refCounts[key]; exists {
		r.refCounts[key]--
		if r.refCounts[key] <= 0 {
			delete(r.stacks, key)
			delete(r.refCounts, key)
			return true
		}
	}
	return false
}

// GetStackKey generates a unique key for a compose stack
func GetStackKey(composeFile, projectName string) string {
	return fmt.Sprintf("%s::%s", composeFile, projectName)
}
