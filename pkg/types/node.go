package types

import "github.com/bgrewell/dart/internal/execution"

// NodeOptions represents configuration options for a node
type NodeOptions *map[string]interface{}

// Node is an interface representing a computing entity (e.g., a server, VM, or container)
// that can be used as a target for test operations, such as executing commands or participating
// in distributed systems for testing purposes.
type Node interface {
	Setup() error
	Teardown() error
	Execute(command string, options ...execution.ExecutionOption) (result *execution.ExecutionResult, err error)
	Close() error
}
