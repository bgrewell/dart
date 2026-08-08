package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolveLocalPath(t *testing.T) {
	suite := filepath.Join(t.TempDir(), "suites")
	require.NoError(t, os.MkdirAll(suite, 0o755))

	// Relative paths belong to the suite, not to the working directory
	resolved, err := ResolveLocalPath(suite, "fixtures/app.conf")
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(suite, "fixtures/app.conf"), resolved)

	// ../ still works, relative to the suite
	resolved, err = ResolveLocalPath(suite, "../shared/app.conf")
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(filepath.Dir(suite), "shared/app.conf"), resolved)

	// Absolute stays put
	resolved, err = ResolveLocalPath(suite, "/etc/hosts")
	require.NoError(t, err)
	assert.Equal(t, "/etc/hosts", resolved)

	// ~ is the invoking user's home, never the suite
	home, err := os.UserHomeDir()
	require.NoError(t, err)
	resolved, err = ResolveLocalPath(suite, "~/.ssh/id_ed25519")
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(home, ".ssh/id_ed25519"), resolved)

	// An empty path is left alone: callers use "" to mean "not set"
	resolved, err = ResolveLocalPath(suite, "")
	require.NoError(t, err)
	assert.Equal(t, "", resolved)

	// With no suite directory — a configuration built in memory — relative
	// paths fall back to the working directory
	wd, err := os.Getwd()
	require.NoError(t, err)
	resolved, err = ResolveLocalPath("", "fixtures/app.conf")
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(wd, "fixtures/app.conf"), resolved)
}

// The suite directory reaches every record that can carry a local path, so
// step and node construction can apply the rule without global state.
func TestLoadStampsSuiteDir(t *testing.T) {
	dir := t.TempDir()
	suitePath := filepath.Join(dir, "suite.yaml")
	require.NoError(t, os.WriteFile(suitePath, []byte(`suite: paths
nodes:
  - name: local
    type: local
setup:
  - name: push a fixture
    node: local
    step:
      type: file_push
      options:
        source: fixtures/app.conf
        dest: /tmp/app.conf
teardown:
  - name: clean up
    node: local
    step:
      type: execute
      options:
        command: "true"
`), 0o644))

	config, err := LoadConfiguration(suitePath)
	require.NoError(t, err)

	assert.Equal(t, dir, config.SuiteDir)
	require.Len(t, config.Setup, 1)
	assert.Equal(t, dir, config.Setup[0].SuiteDir)
	require.Len(t, config.Teardown, 1)
	assert.Equal(t, dir, config.Teardown[0].SuiteDir)
	require.Len(t, config.Nodes, 1)
	assert.Equal(t, dir, config.Nodes[0].SuiteDir)
}
