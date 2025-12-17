package ifaces

import (
	"github.com/bgrewell/dart/internal/eval"
	"github.com/bgrewell/dart/internal/formatters"
)

// Test represents a test that can be executed against a node
type Test interface {
	Name() string
	NodeName() string
	Run(updater formatters.TestCompleter) (results map[string]*eval.EvaluateResult, err error)
}
