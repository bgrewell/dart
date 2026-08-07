package formatters

import (
	"bytes"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCleanLogWriterCollapsesSpinnerFrames(t *testing.T) {
	var out bytes.Buffer
	w := NewCleanLogWriter(&out)
	w.Write([]byte("frame1\rframe2\rfinal line\n"))
	assert.Equal(t, "final line\n", out.String())
}

func TestCleanLogWriterStripsANSI(t *testing.T) {
	var out bytes.Buffer
	w := NewCleanLogWriter(&out)
	w.Write([]byte("\x1b[32mgreen\x1b[0m plain\n"))
	assert.Equal(t, "green plain\n", out.String())
}

func TestCleanLogWriterPartialLinesAcrossWrites(t *testing.T) {
	var out bytes.Buffer
	w := NewCleanLogWriter(&out)
	w.Write([]byte("part one "))
	w.Write([]byte("part two\nnext"))
	assert.Equal(t, "part one part two\n", out.String())

	// The trailing partial line lands on Flush
	w.Flush()
	assert.Equal(t, "part one part two\nnext\n", out.String())
}

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, errors.New("disk full") }

// A destination error must not propagate: through io.MultiWriter it would
// abort console output too.
func TestCleanLogWriterSwallowsDestinationErrors(t *testing.T) {
	w := NewCleanLogWriter(failingWriter{})
	n, err := w.Write([]byte("line\n"))
	assert.NoError(t, err)
	assert.Equal(t, 5, n)
}

// CSI private-mode sequences (cursor hide/show from the spinner) must be
// stripped even when they sit after the line's last carriage return —
// the regex itself, not the \r-collapse, must handle them.
func TestCleanLogWriterStripsCursorEscapesAfterCR(t *testing.T) {
	var out bytes.Buffer
	w := NewCleanLogWriter(&out)
	w.Write([]byte("spin1\rspin2\rdone\x1b[?25h\n"))
	assert.Equal(t, "done\n", out.String())

	out.Reset()
	w.Write([]byte("\x1b[?25lhidden cursor line\n"))
	assert.Equal(t, "hidden cursor line\n", out.String())
}
