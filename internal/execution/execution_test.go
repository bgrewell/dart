package execution

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The Stdout/Stderr streams are one-shot; the cached accessors must return
// the full output on every call.
func TestOutputBytesCached(t *testing.T) {
	result := &ExecutionResult{
		Stdout: strings.NewReader("out"),
		Stderr: strings.NewReader("err"),
	}

	for i := 0; i < 3; i++ {
		stdout, err := result.StdoutBytes()
		require.NoError(t, err)
		assert.Equal(t, "out", string(stdout))

		stderr, err := result.StderrBytes()
		require.NoError(t, err)
		assert.Equal(t, "err", string(stderr))
	}
}

func TestOutputBytesNilStreams(t *testing.T) {
	result := &ExecutionResult{}

	stdout, err := result.StdoutBytes()
	require.NoError(t, err)
	assert.Empty(t, stdout)

	stderr, err := result.StderrBytes()
	require.NoError(t, err)
	assert.Empty(t, stderr)
}
