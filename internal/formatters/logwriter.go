package formatters

import (
	"bytes"
	"io"
	"regexp"
	"sync"
)

// ansiRe covers CSI sequences including private-mode parameters (e.g. the
// \x1b[?25l / \x1b[?25h cursor hide/show that the spinner emits) and OSC
// sequences with either BEL or ST terminators.
var ansiRe = regexp.MustCompile(`\x1b\[[0-9;?]*[ -/]*[@-~]|\x1b\][^\x07\x1b]*(?:\x07|\x1b\\)`)

// cleanLogWriter turns terminal output into a readable log file: spinner
// redraws collapse to their final state (content after the last carriage
// return per line) and ANSI escape sequences are stripped.
type cleanLogWriter struct {
	mu  sync.Mutex
	dst io.Writer
	buf bytes.Buffer
}

// FlushableWriter is a writer whose buffered tail can be flushed at exit.
type FlushableWriter interface {
	io.Writer
	Flush()
}

// NewCleanLogWriter wraps dst so terminal-oriented output written through
// it lands as plain final-state lines. Call Flush before exit to emit any
// final partial line.
func NewCleanLogWriter(dst io.Writer) FlushableWriter {
	return &cleanLogWriter{dst: dst}
}

func (w *cleanLogWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.buf.Write(p)
	for {
		line, err := w.buf.ReadString('\n')
		if err != nil {
			// Partial line: keep buffered for the next write
			w.buf.WriteString(line)
			break
		}
		if idx := bytes.LastIndexByte([]byte(line), '\r'); idx >= 0 {
			line = line[idx+1:]
		}
		clean := ansiRe.ReplaceAllString(line, "")
		// Best-effort: a log-file write error must not propagate — through
		// io.MultiWriter it would abort console output too
		_, _ = io.WriteString(w.dst, clean)
	}
	return len(p), nil
}

// Flush writes any buffered partial line (a final line without a trailing
// newline would otherwise be lost when the process exits).
func (w *cleanLogWriter) Flush() {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.buf.Len() == 0 {
		return
	}
	line := w.buf.String()
	w.buf.Reset()
	if idx := bytes.LastIndexByte([]byte(line), '\r'); idx >= 0 {
		line = line[idx+1:]
	}
	_, _ = io.WriteString(w.dst, ansiRe.ReplaceAllString(line, "")+"\n")
}
