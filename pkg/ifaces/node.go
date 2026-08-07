package ifaces

import (
	"errors"
	"fmt"
	"time"

	"github.com/bgrewell/dart/internal/execution"
)

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

// ErrCommandTimeout marks a bounded execution that exceeded its wait; test
// flows convert it into a failing check rather than an infrastructure
// error, so timeouts respect stop-on-error and never skip teardown.
var ErrCommandTimeout = errors.New("command timed out")

type executeOutcome struct {
	result *execution.ExecutionResult
	err    error
}

// ExecuteWithTimeout runs a command on a node, bounding the wait. A zero or
// negative timeout runs unbounded. On timeout the suite-side wait ends with
// an error; the remote command itself may keep running (killing it requires
// per-node-type cancellation, which the clients do not all support), but a
// hung command no longer hangs the whole run. For repeated bounded
// executions of the same command use BoundedCommand, which never overlaps
// invocations.
func ExecuteWithTimeout(node Node, command string, timeout time.Duration) (*execution.ExecutionResult, error) {
	return BoundedCommand(node, command, timeout)()
}

// BoundedCommand returns a producer that executes command with a per-call
// wait bound. A call that times out leaves the command running and returns
// ErrCommandTimeout — but the next call waits on that same in-flight
// invocation instead of launching another, so retry loops never stack
// overlapping executions (or their side effects) against the node, and at
// most one goroutine is ever outstanding per command.
func BoundedCommand(node Node, command string, timeout time.Duration) func() (*execution.ExecutionResult, error) {
	var pending chan executeOutcome
	return func() (*execution.ExecutionResult, error) {
		if timeout <= 0 && pending == nil {
			return node.Execute(command)
		}
		if pending == nil {
			pending = make(chan executeOutcome, 1)
			done := pending
			go func() {
				result, err := node.Execute(command)
				done <- executeOutcome{result, err}
			}()
		}
		select {
		case out := <-pending:
			pending = nil
			return out.result, out.err
		case <-time.After(timeout):
			return nil, fmt.Errorf("%w after %s: %s", ErrCommandTimeout, timeout, command)
		}
	}
}

// NetworkInspector is implemented by node types that can report their own
// addresses without running a command, so suites can reference a node's IP
// ({{ fact "web" "ipv4" }}) without hand-rolling `hostname -I` facts.
type NetworkInspector interface {
	// NetworkFacts returns address facts such as "ipv4", "ipv6", and
	// per-interface entries ("ipv4.eth0").
	NetworkFacts() (map[string]string, error)
}

// Rebooter is implemented by node types that can restart their target and
// block until it accepts commands again. With force set the restart models
// a power cut (no clean shutdown). readyCommand overrides the node's
// default readiness check; a zero timeout uses the node's default.
type Rebooter interface {
	Reboot(force bool, readyCommand string, timeout time.Duration) error
}
