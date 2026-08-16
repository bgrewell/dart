package nodetypes

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A relative path in a command must mean what it means in a file step's
// source: relative to the suite file. Previously the command inherited
// DART's working directory, so a suite that worked from the repository root
// broke when run from anywhere else — while its file_push steps kept working.
func TestLocalNodeRunsInSuiteDir(t *testing.T) {
	suite := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(suite, "marker.txt"), []byte("found\n"), 0o644))

	node := NewLocalNode("builder", nil, suite)
	result, err := node.Execute("cat marker.txt")
	require.NoError(t, err)

	stdout, err := result.StdoutBytes()
	require.NoError(t, err)
	assert.Equal(t, 0, result.ExitCode)
	assert.Contains(t, string(stdout), "found")
}

// pwd reports the suite directory, not wherever DART was invoked from.
func TestLocalNodeWorkingDirectoryIsSuiteDir(t *testing.T) {
	suite := t.TempDir()

	result, err := NewLocalNode("builder", nil, suite).Execute("pwd")
	require.NoError(t, err)
	stdout, err := result.StdoutBytes()
	require.NoError(t, err)

	// macOS reports /private/var for /var, so compare the resolved paths
	wantResolved, err := filepath.EvalSymlinks(suite)
	require.NoError(t, err)
	gotResolved, err := filepath.EvalSymlinks(strings.TrimSpace(string(stdout)))
	require.NoError(t, err)
	assert.Equal(t, wantResolved, gotResolved)
}

// A node built without a suite directory — in memory, or in a unit test —
// keeps the previous behaviour rather than being forced somewhere.
func TestLocalNodeWithoutSuiteDirInheritsCwd(t *testing.T) {
	cwd, err := os.Getwd()
	require.NoError(t, err)

	result, err := NewLocalNode("builder", nil, "").Execute("pwd")
	require.NoError(t, err)
	stdout, err := result.StdoutBytes()
	require.NoError(t, err)

	wantResolved, _ := filepath.EvalSymlinks(cwd)
	gotResolved, _ := filepath.EvalSymlinks(strings.TrimSpace(string(stdout)))
	assert.Equal(t, wantResolved, gotResolved)
}
