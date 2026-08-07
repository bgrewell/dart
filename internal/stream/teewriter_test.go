package stream

import (
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTeeWriterCapturesWithoutDebug(t *testing.T) {
	tw := NewTeeWriter(StreamStdout, "node", false)

	n, err := tw.Write([]byte("line one\nline two\n"))
	require.NoError(t, err)
	assert.Equal(t, 18, n)

	captured, err := io.ReadAll(tw.Reader())
	require.NoError(t, err)
	assert.Equal(t, "line one\nline two\n", string(captured))
}

func TestTeeWriterMultipleWrites(t *testing.T) {
	tw := NewTeeWriter(StreamStderr, "node", false)
	_, err := tw.Write([]byte("first "))
	require.NoError(t, err)
	_, err = tw.Write([]byte("second"))
	require.NoError(t, err)

	captured, err := io.ReadAll(tw.Reader())
	require.NoError(t, err)
	assert.Equal(t, "first second", string(captured))
}

func TestTeeWriterPrefixOnlyWhenEnabled(t *testing.T) {
	disabled := NewTeeWriter(StreamStdout, "node", false)
	assert.Empty(t, disabled.prefix)

	enabled := NewTeeWriter(StreamStdout, "node", true)
	assert.Contains(t, enabled.prefix, "node")
	assert.Contains(t, enabled.prefix, "stdout")

	stderrWriter := NewTeeWriter(StreamStderr, "node", true)
	assert.Contains(t, stderrWriter.prefix, "stderr")
}

func TestStreamCopy(t *testing.T) {
	src := strings.NewReader("copied content")
	reader, err := StreamCopy(src, StreamStdout, "node", false)
	require.NoError(t, err)

	data, err := io.ReadAll(reader)
	require.NoError(t, err)
	assert.Equal(t, "copied content", string(data))
}

func TestCoordinatorWriteWithoutSpinner(t *testing.T) {
	// The path with no active spinner must not panic and must not deadlock
	c := GetCoordinator()
	c.ClearActiveSpinner()
	c.WriteDebugLine("debug output with no spinner")
	c.WriteDebugLineStderr("stderr debug output with no spinner")
}
