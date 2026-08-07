package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const varsSuite = `
suite: vars demo
vars:
  target: 10.0.0.5
  port: "8080"
nodes:
  - name: local
    type: local
tests:
  - name: reach target
    node: local
    type: execute
    options:
      command: curl http://{{var.target}}:{{var.port}}/health
      evaluate:
        exit_code: 0
`

func TestVarsSubstitution(t *testing.T) {
	cfg, err := ParseConfiguration([]byte(varsSuite), ".")
	require.NoError(t, err)
	assert.Equal(t, "curl http://10.0.0.5:8080/health", cfg.Tests[0].Options["command"])
}

func TestVarsCLIOverride(t *testing.T) {
	cfg, err := ParseConfigurationWithVars([]byte(varsSuite), ".",
		map[string]string{"target": "192.168.1.1"})
	require.NoError(t, err)
	assert.Equal(t, "curl http://192.168.1.1:8080/health", cfg.Tests[0].Options["command"])
}

// A numeric var in a numeric position lands as a native YAML number.
func TestVarsNativeTypes(t *testing.T) {
	yamlData := `
suite: typed
vars:
  port: "8080"
nodes:
  - name: local
    type: local
tests:
  - name: t
    node: local
    type: port_check
    options:
      host: localhost
      port: {{var.port}}
`
	cfg, err := ParseConfiguration([]byte(yamlData), ".")
	require.NoError(t, err)
	assert.Equal(t, 8080, cfg.Tests[0].Options["port"])
}

func TestEnvSubstitution(t *testing.T) {
	t.Setenv("DART_TEST_TARGET", "envhost")
	yamlData := `
suite: env
nodes:
  - name: local
    type: local
tests:
  - name: t
    node: local
    type: execute
    options:
      command: ping {{env.DART_TEST_TARGET}}
`
	cfg, err := ParseConfiguration([]byte(yamlData), ".")
	require.NoError(t, err)
	assert.Equal(t, "ping envhost", cfg.Tests[0].Options["command"])
}

// Var values may reference env; resolution happens before substitution.
func TestEnvInsideVarValue(t *testing.T) {
	t.Setenv("DART_TEST_BASE", "/srv/data")
	yamlData := `
suite: nested
vars:
  datadir: "{{env.DART_TEST_BASE}}/run1"
nodes:
  - name: local
    type: local
tests:
  - name: t
    node: local
    type: execute
    options:
      command: ls {{var.datadir}}
`
	cfg, err := ParseConfiguration([]byte(yamlData), ".")
	require.NoError(t, err)
	assert.Equal(t, "ls /srv/data/run1", cfg.Tests[0].Options["command"])
}

func TestUnresolvedReferencesError(t *testing.T) {
	yamlData := "suite: s\nnodes:\n  - name: l\n    type: local\ntests:\n  - name: t\n    node: l\n    type: execute\n    options:\n      command: echo {{var.nope}} {{env.DART_DEFINITELY_UNSET_VAR}}\n"
	_, err := ParseConfiguration([]byte(yamlData), ".")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "var.nope")
	assert.Contains(t, err.Error(), "env.DART_DEFINITELY_UNSET_VAR")
}

func TestVarNewlineRejected(t *testing.T) {
	_, err := ParseConfigurationWithVars([]byte(varsSuite), ".",
		map[string]string{"target": "evil\ninjection: true"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "newline")
}

// Capture references and fact templates pass through untouched.
func TestVarsLeaveOtherTemplatesAlone(t *testing.T) {
	yamlData := `
suite: mixed
nodes:
  - name: local
    type: local
tests:
  - name: t
    node: local
    type: execute
    options:
      command: echo {{capture.pre}} {{ fact "self" "ip" }}
`
	cfg, err := ParseConfiguration([]byte(yamlData), ".")
	require.NoError(t, err)
	assert.Contains(t, cfg.Tests[0].Options["command"], "{{capture.pre}}")
	assert.Contains(t, cfg.Tests[0].Options["command"], `{{ fact "self" "ip" }}`)
}

// Retry and Tags must survive multi-node expansion.
func TestExpansionPropagatesRetryAndTags(t *testing.T) {
	yamlData := `
suite: expand
nodes:
  - name: a
    type: local
  - name: b
    type: local
tests:
  - name: shared
    node: [a, b]
    type: execute
    tags: [network, smoke]
    retry:
      timeout: 30
    options:
      command: echo hi
`
	cfg, err := ParseConfiguration([]byte(yamlData), ".")
	require.NoError(t, err)
	require.Len(t, cfg.Tests, 2)
	for _, test := range cfg.Tests {
		require.NotNil(t, test.Retry, "retry must survive expansion")
		assert.Equal(t, 30.0, test.Retry.Timeout)
		assert.Equal(t, []string{"network", "smoke"}, test.Tags)
	}
	// Cloned, not shared
	cfg.Tests[0].Tags[0] = "mutated"
	assert.Equal(t, "network", cfg.Tests[1].Tags[0])
}
