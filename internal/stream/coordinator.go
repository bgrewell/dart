package stream

import (
	"fmt"
	"io"
	"os"
	"sync"

	"github.com/theckman/yacspin"
)

// OutputCoordinator manages coordination between debug output and spinners.
// When a spinner is active, debug output pauses the spinner, prints, then resumes.
type OutputCoordinator struct {
	mu      sync.Mutex
	spinner *yacspin.Spinner
	out     io.Writer
	errOut  io.Writer
}

// SetWriters redirects debug output (default os.Stdout/os.Stderr) — wired
// by --log so streamed command output reaches the transcript too.
func (c *OutputCoordinator) SetWriters(out, errOut io.Writer) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.out = out
	c.errOut = errOut
}

func (c *OutputCoordinator) stdout() io.Writer {
	if c.out != nil {
		return c.out
	}
	return os.Stdout
}

func (c *OutputCoordinator) stderr() io.Writer {
	if c.errOut != nil {
		return c.errOut
	}
	return os.Stderr
}

var (
	globalCoordinator     *OutputCoordinator
	globalCoordinatorOnce sync.Once
)

// GetCoordinator returns the global output coordinator singleton.
func GetCoordinator() *OutputCoordinator {
	globalCoordinatorOnce.Do(func() {
		globalCoordinator = &OutputCoordinator{}
	})
	return globalCoordinator
}

// SetActiveSpinner registers the currently active spinner.
// Call with nil when the spinner completes.
func (c *OutputCoordinator) SetActiveSpinner(s *yacspin.Spinner) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.spinner = s
}

// ClearActiveSpinner removes the active spinner reference.
func (c *OutputCoordinator) ClearActiveSpinner() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.spinner = nil
}

// WriteDebugLine writes a debug line to the console, coordinating with any active spinner.
// If a spinner is active, it pauses the spinner, writes the line, then resumes.
func (c *OutputCoordinator) WriteDebugLine(line string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.spinner != nil && c.spinner.Status() == yacspin.SpinnerRunning {
		// Pause spinner (clears the spinner line)
		c.spinner.Pause()

		// Write the debug line
		fmt.Fprintln(c.stdout(), line)

		// Resume spinner (redraws the spinner line)
		c.spinner.Unpause()
	} else {
		// No active spinner, just print
		fmt.Fprintln(c.stdout(), line)
	}
}

// WriteDebugLineStderr writes a debug line to stderr, coordinating with any active spinner.
func (c *OutputCoordinator) WriteDebugLineStderr(line string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.spinner != nil && c.spinner.Status() == yacspin.SpinnerRunning {
		// Pause spinner (clears the spinner line)
		c.spinner.Pause()

		// Write the debug line
		fmt.Fprintln(c.stderr(), line)

		// Resume spinner (redraws the spinner line)
		c.spinner.Unpause()
	} else {
		// No active spinner, just print
		fmt.Fprintln(c.stderr(), line)
	}
}
