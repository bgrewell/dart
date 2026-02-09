package stream

import (
	"bytes"
	"io"
	"sync"

	"github.com/fatih/color"
)

// StreamType identifies the output stream for display purposes.
type StreamType int

const (
	StreamStdout StreamType = iota
	StreamStderr
)

// TeeWriter writes to both a buffer (for evaluation) and console (for debug output).
type TeeWriter struct {
	buf        *bytes.Buffer
	prefix     string
	streamType StreamType
	mu         sync.Mutex
	enabled    bool
}

// NewTeeWriter creates a writer that captures output and optionally streams to console.
func NewTeeWriter(streamType StreamType, nodeName string, enabled bool) *TeeWriter {
	var prefix string

	if enabled {
		if streamType == StreamStdout {
			prefix = color.HiGreenString("[%s:stdout] ", nodeName)
		} else {
			prefix = color.HiRedString("[%s:stderr] ", nodeName)
		}
	}

	return &TeeWriter{
		buf:        &bytes.Buffer{},
		prefix:     prefix,
		streamType: streamType,
		enabled:    enabled,
	}
}

// Write implements io.Writer, writing to both destinations.
func (tw *TeeWriter) Write(p []byte) (n int, err error) {
	tw.mu.Lock()
	defer tw.mu.Unlock()

	// Always write to buffer for evaluation
	n, err = tw.buf.Write(p)
	if err != nil {
		return n, err
	}

	// Stream to console if debug mode is enabled
	if tw.enabled {
		coordinator := GetCoordinator()

		// Prefix each line with stream identifier
		lines := bytes.Split(p, []byte("\n"))
		for i, line := range lines {
			if len(line) > 0 {
				debugLine := tw.prefix + string(line)
				if tw.streamType == StreamStderr {
					coordinator.WriteDebugLineStderr(debugLine)
				} else {
					coordinator.WriteDebugLine(debugLine)
				}
			} else if i < len(lines)-1 {
				// Empty line in the middle, still print
				if tw.streamType == StreamStderr {
					coordinator.WriteDebugLineStderr(tw.prefix)
				} else {
					coordinator.WriteDebugLine(tw.prefix)
				}
			}
		}
	}

	return n, nil
}

// Reader returns an io.Reader for the captured output.
func (tw *TeeWriter) Reader() io.Reader {
	return bytes.NewReader(tw.buf.Bytes())
}

// Buffer returns the underlying buffer.
func (tw *TeeWriter) Buffer() *bytes.Buffer {
	return tw.buf
}

// StreamCopy reads from src and writes to a TeeWriter, returning the buffer as a reader.
func StreamCopy(src io.Reader, streamType StreamType, nodeName string, debugEnabled bool) (io.Reader, error) {
	tw := NewTeeWriter(streamType, nodeName, debugEnabled)
	_, err := io.Copy(tw, src)
	if err != nil {
		return nil, err
	}
	return tw.Reader(), nil
}
