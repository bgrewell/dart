package stream

import (
	"fmt"
	"os"
	"sync"

	"github.com/theckman/yacspin"
)

// OutputCoordinator manages coordination between debug output and spinners.
// When a spinner is active, debug output pauses the spinner, prints, then resumes.
type OutputCoordinator struct {
	mu      sync.Mutex
	spinner *yacspin.Spinner
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
		fmt.Fprintln(os.Stdout, line)

		// Resume spinner (redraws the spinner line)
		c.spinner.Unpause()
	} else {
		// No active spinner, just print
		fmt.Fprintln(os.Stdout, line)
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
		fmt.Fprintln(os.Stderr, line)

		// Resume spinner (redraws the spinner line)
		c.spinner.Unpause()
	} else {
		// No active spinner, just print
		fmt.Fprintln(os.Stderr, line)
	}
}
