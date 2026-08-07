package execution

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// YAML delivers lists as []interface{}; the env option must accept that
// shape instead of panicking on a []string assertion.
func TestOptionsEnvFromYAMLShape(t *testing.T) {
	opts := OptionsToExecutionOptions(map[string]interface{}{
		"env": []interface{}{"A=1", "B=2"},
	})
	assert.Len(t, opts, 1)
}

func TestOptionsEnvNative(t *testing.T) {
	opts := OptionsToExecutionOptions(map[string]interface{}{
		"env": []string{"A=1"},
	})
	assert.Len(t, opts, 1)
}

// Wrong-typed options are skipped with a warning, never a panic.
func TestOptionsWrongTypesDoNotPanic(t *testing.T) {
	opts := OptionsToExecutionOptions(map[string]interface{}{
		"env":   "not-a-list",
		"shell": 42,
		"sudo":  "not-a-map",
	})
	assert.Empty(t, opts)

	opts = OptionsToExecutionOptions(map[string]interface{}{
		"env": []interface{}{"A=1", 7},
	})
	assert.Empty(t, opts)
}

func TestOptionsShellAndSudo(t *testing.T) {
	opts := OptionsToExecutionOptions(map[string]interface{}{
		"shell": "/bin/bash",
		"sudo":  map[string]interface{}{"password": "hunter2"},
	})
	assert.Len(t, opts, 2)
}

func TestOptionsUnknownKeysIgnored(t *testing.T) {
	opts := OptionsToExecutionOptions(map[string]interface{}{
		"bogus": true,
	})
	assert.Empty(t, opts)
}
