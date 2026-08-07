package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidationDuplicateNodeNames(t *testing.T) {
	yamlData := `
suite: dup
nodes:
  - name: same
    type: local
  - name: same
    type: local
`
	_, err := ParseConfiguration([]byte(yamlData), ".")
	require.Error(t, err)
	assert.Contains(t, err.Error(), `duplicate node name "same"`)
}

func TestValidationUnnamedNode(t *testing.T) {
	yamlData := `
suite: unnamed
nodes:
  - type: local
`
	_, err := ParseConfiguration([]byte(yamlData), ".")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "node has no name")
}

func TestValidationTestWithoutNode(t *testing.T) {
	yamlData := `
suite: missing node
nodes:
  - name: local
    type: local
tests:
  - name: forgot the node key
    type: execute
    options:
      command: echo hi
`
	_, err := ParseConfiguration([]byte(yamlData), ".")
	require.Error(t, err)
	assert.Contains(t, err.Error(), `test "forgot the node key" references no node`)
}

// Expanded multi-node tests must not share Setup/Teardown backing arrays:
// fact rendering rewrites entries in place per node, and a shared array
// would leak one node's rendered values into its siblings.
func TestExpandedTestsDoNotShareCommandSlices(t *testing.T) {
	yamlData := `
suite: expansion isolation
nodes:
  - name: a
    type: local
  - name: b
    type: local
tests:
  - name: shared
    node: [a, b]
    type: execute
    setup:
      - "echo {{ fact }}"
    options:
      command: echo hi
`
	cfg, err := ParseConfiguration([]byte(yamlData), ".")
	require.NoError(t, err)
	require.Len(t, cfg.Tests, 2)

	cfg.Tests[0].Setup[0] = "mutated by node a"
	assert.Equal(t, "echo {{ fact }}", cfg.Tests[1].Setup[0],
		"sibling test's setup commands must be unaffected")
}

func TestMalformedLoadFromDirective(t *testing.T) {
	yamlData := "suite: bad\ntests: !!load_from(missing-paren\n"
	_, err := ParseConfiguration([]byte(yamlData), ".")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing closing parenthesis")
	assert.Contains(t, err.Error(), "line 2")
}

func TestLoadFromDirectory(t *testing.T) {
	dir := t.TempDir()
	testsDir := filepath.Join(dir, "tests")
	require.NoError(t, os.Mkdir(testsDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(testsDir, "a.yaml"), []byte(
		"- name: from file a\n  node: local\n  type: execute\n  options:\n    command: echo a\n"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(testsDir, "b.yaml"), []byte(
		"- name: from file b\n  node: local\n  type: execute\n  options:\n    command: echo b\n"), 0644))

	yamlData := "suite: loaded\nnodes:\n  - name: local\n    type: local\ntests: !!load_from(tests)\n"
	cfg, err := ParseConfiguration([]byte(yamlData), dir)
	require.NoError(t, err)
	require.Len(t, cfg.Tests, 2)
	assert.Equal(t, "from file a", cfg.Tests[0].Name)
	assert.Equal(t, "from file b", cfg.Tests[1].Name)
}

// Locations point at real file lines only when the parsed buffer matches
// the file on disk; load_from shifts lines, so locations are skipped.
func TestLocationsSkippedWithLoadFrom(t *testing.T) {
	dir := t.TempDir()
	testsDir := filepath.Join(dir, "tests")
	require.NoError(t, os.Mkdir(testsDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(testsDir, "a.yaml"), []byte(
		"- name: t\n  node: local\n  type: execute\n  options:\n    command: echo a\n"), 0644))

	cfgFile := filepath.Join(dir, "suite.yaml")
	yamlData := "suite: loaded\nnodes:\n  - name: local\n    type: local\ntests: !!load_from(tests)\n"
	require.NoError(t, os.WriteFile(cfgFile, []byte(yamlData), 0644))

	cfg, err := LoadConfiguration(cfgFile)
	require.NoError(t, err)
	require.Len(t, cfg.Tests, 1)
	assert.Zero(t, cfg.Tests[0].Loc.Line, "locations must not point at shifted lines")
}

func TestLocationsPopulatedWithoutLoadFrom(t *testing.T) {
	dir := t.TempDir()
	cfgFile := filepath.Join(dir, "suite.yaml")
	yamlData := `suite: located
nodes:
  - name: local
    type: local
tests:
  - name: t
    node: local
    type: execute
    options:
      command: echo hi
`
	require.NoError(t, os.WriteFile(cfgFile, []byte(yamlData), 0644))

	cfg, err := LoadConfiguration(cfgFile)
	require.NoError(t, err)
	require.Len(t, cfg.Tests, 1)
	assert.Equal(t, 6, cfg.Tests[0].Loc.Line)
	assert.Equal(t, cfgFile, cfg.Tests[0].Loc.File)
}
