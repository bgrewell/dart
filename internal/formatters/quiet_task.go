package formatters

import (
	"fmt"
	"io"
	"strings"
	"sync"
)

// QuietTaskCompleter renders a task without an animated spinner: the line is
// written once, when the task finishes.
//
// Rationale: the animated completer owns a terminal line and registers itself
// as the process's single active spinner. Two of those running at once
// overwrite each other and leave the coordinator tracking only one, so
// concurrent tasks need a completer that produces output only at the end.
type QuietTaskCompleter struct {
	out     io.Writer
	mu      *sync.Mutex
	message string
	done    bool
}

// NewQuietTaskCompleter builds a completer whose line is printed on
// completion. The mutex is shared across the concurrent group so lines never
// interleave mid-write.
func NewQuietTaskCompleter(out io.Writer, mu *sync.Mutex, message string) *QuietTaskCompleter {
	return &QuietTaskCompleter{out: out, mu: mu, message: message}
}

// Update is a no-op: there is no live line to update.
func (q *QuietTaskCompleter) Update(status string) {}

func (q *QuietTaskCompleter) Complete() { q.finish("done") }
func (q *QuietTaskCompleter) Fail()     { q.finish("failed") }
func (q *QuietTaskCompleter) Error()    { q.finish("error") }

// finish writes the task's line exactly once, so a completer that is both
// failed and errored (or completed twice) cannot double-print.
func (q *QuietTaskCompleter) finish(status string) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.done {
		return
	}
	q.done = true
	fmt.Fprintf(q.out, "%s%s\n", q.message, status)
}

// QuietTaskLine renders the same prefix the animated completer uses, so
// parallel output lines up with sequential output.
func (sf *StandardFormatter) QuietTaskLine(task, nodeName string) string {
	indent := strings.Repeat(" ", sf.indent)
	nodeBox := sf.formatNodeBox(nodeName)
	return fmt.Sprintf("%s%s%s", indent, nodeBox, padRightWithPeriods(task, sf.taskColumnWidth-len(task)+3))
}

// Out exposes the formatter's writer so a concurrent group can render through
// the same destination.
func (sf *StandardFormatter) Out() io.Writer { return sf.out }
