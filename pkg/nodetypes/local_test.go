package nodetypes

import (
	"runtime"
	"strings"
	"testing"

	"github.com/bgrewell/dart/pkg/ifaces"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func execStdout(t *testing.T, node ifaces.Node, command string) string {
	t.Helper()
	result, err := node.Execute(command)
	require.NoError(t, err)
	stdout, err := result.StdoutBytes()
	require.NoError(t, err)
	return strings.TrimSpace(string(stdout))
}

// Without a configured shell, commands still run through the platform
// default shell, so pipes and compound commands work out of the box.
func TestLocalNodeDefaultShell(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX shell expectations")
	}
	node := NewLocalNode("default-shell", nil)
	assert.Equal(t, "HELLO", execStdout(t, node, "echo hello | tr a-z A-Z"))
	assert.Equal(t, "yes", execStdout(t, node, "if true; then echo yes; fi"))
}

// A top-level shell option is honored.
func TestLocalNodeTopLevelShell(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX shell expectations")
	}
	opts := map[string]interface{}{"shell": "/bin/bash"}
	node := NewLocalNode("top-level", ifaces.NodeOptions(&opts))
	assert.NotEmpty(t, execStdout(t, node, "echo $BASH_VERSION"))
}

// The exec_opts.shell shape used by the container node types also works.
func TestLocalNodeExecOptsShell(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX shell expectations")
	}
	opts := map[string]interface{}{
		"exec_opts": map[string]interface{}{"shell": "/bin/bash"},
	}
	node := NewLocalNode("exec-opts", ifaces.NodeOptions(&opts))
	assert.NotEmpty(t, execStdout(t, node, "echo $BASH_VERSION"))
}
