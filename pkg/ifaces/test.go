package ifaces

import (
	"github.com/bgrewell/dart/internal/eval"
	"github.com/bgrewell/dart/internal/formatters"
)

// Test represents a test that can be executed against a node
type Test interface {
	Name() string
	NodeName() string
	// ShouldSkip evaluates the test's skip conditions (if any) on its node.
	// A returned reason explains which condition triggered the skip.
	ShouldSkip() (skip bool, reason string, err error)
	Run(updater formatters.TestCompleter) (results map[string]*eval.EvaluateResult, err error)
}
