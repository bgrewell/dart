package config

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A YAML syntax error carries its line inside the message. Recovering it is
// what lets a malformed suite get the same snippet a semantic error gets,
// instead of a bare one-line message.
func TestAsConfigErrorRecoversYAMLPosition(t *testing.T) {
	converted := AsConfigError(errors.New("yaml: line 2: did not find expected '-' indicator"), "/tmp/suite.yaml")

	var cfgErr *ConfigError
	require.ErrorAs(t, converted, &cfgErr)
	assert.Equal(t, 2, cfgErr.Location.Line)
	assert.Equal(t, "/tmp/suite.yaml", cfgErr.Location.File)
	// The position is carried in the location, not repeated in the message
	assert.Equal(t, "did not find expected '-' indicator", cfgErr.Message)
}

func TestAsConfigErrorRecoversUnmarshalPosition(t *testing.T) {
	converted := AsConfigError(errors.New("yaml: unmarshal errors:\n  line 7: cannot unmarshal !!str into int"), "/tmp/suite.yaml")

	var cfgErr *ConfigError
	require.ErrorAs(t, converted, &cfgErr)
	assert.Equal(t, 7, cfgErr.Location.Line)
}

// Errors with no position pass through untouched rather than acquiring a
// misleading location.
func TestAsConfigErrorLeavesUnlocatedErrorsAlone(t *testing.T) {
	original := errors.New("something else went wrong")
	assert.Equal(t, original, AsConfigError(original, "/tmp/suite.yaml"))
	assert.Nil(t, AsConfigError(nil, "/tmp/suite.yaml"))

	// An error that already has a location keeps it
	located := &ConfigError{Message: "m", Location: SourceLocation{File: "a", Line: 9}}
	assert.Equal(t, error(located), AsConfigError(located, "/tmp/other.yaml"))
}

// An option error must mark the option's own line, not the first line of the
// test or step that contains it.
func TestOptionLocationsPointAtTheKey(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "suite.yaml")
	require.NoError(t, os.WriteFile(path, []byte(`suite: locations
nodes:
  - name: local
    type: local
tests:
  - name: check the service
    node: local
    type: execute
    options:
      command: "true"
      evaluatte:
        exit_code: 0
`), 0o644))

	cfg, err := LoadConfiguration(path)
	require.NoError(t, err)
	require.Len(t, cfg.Tests, 1)

	// `evaluatte:` is on line 11; the test itself starts on line 6
	loc, ok := cfg.Tests[0].OptionLocs["evaluatte"]
	require.True(t, ok, "option key locations must be recorded")
	assert.Equal(t, 11, loc.Line)
	assert.Equal(t, 6, cfg.Tests[0].Loc.Line, "the test's own location is still the block start")
}

// A value error must mark the option it is about, not the first line of the
// enclosing test — the same rule that applies to an unknown option.
func TestValueErrorMarksItsOwnOption(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "suite.yaml")
	require.NoError(t, os.WriteFile(path, []byte(`suite: value errors
nodes:
  - name: web
    type: local
tests:
  - name: health endpoint answers
    node: web
    type: http_request
    options:
      url: http://localhost:8080/health
      timeout: -5
`), 0o644))

	cfg, err := LoadConfiguration(path)
	require.NoError(t, err)

	// `timeout:` is on line 11; the test block starts on line 6
	loc, ok := cfg.Tests[0].OptionLocs["timeout"]
	require.True(t, ok)
	assert.Equal(t, 11, loc.Line)
	assert.Equal(t, 6, cfg.Tests[0].Loc.Line)
}
