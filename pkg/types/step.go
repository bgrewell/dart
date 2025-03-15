package types

import "github.com/bgrewell/dart/internal/formatters"

// Step represents a single operation or task that can be executed as part of a test
type Step interface {
	Run(updater formatters.TaskCompleter) error
	TitleLen() int
	Title() string
}
