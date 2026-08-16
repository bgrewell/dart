package steptypes

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A misspelled option used to be dropped in silence, leaving the suite
// reading as though it were set.
func TestUnknownStepOptionIsRejected(t *testing.T) {
	_, err := makeStep(t, TypeExecute, map[string]interface{}{
		"command": "true",
		"timout":  30,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), `unknown option "timout"`)
	// The message names what the type does accept, including the spelling
	// the author meant
	assert.Contains(t, err.Error(), "timeout")
}

// An option that is valid on a different step type is still wrong here.
func TestStepOptionFromAnotherTypeIsRejected(t *testing.T) {
	_, err := makeStep(t, TypeExecute, map[string]interface{}{
		"command": "true",
		"url":     "http://localhost/",
	})
	assert.ErrorContains(t, err, `unknown option "url"`)
}

// Every documented option of every step type must survive the check, or the
// tracking has missed a read and valid suites break.
func TestKnownStepOptionsAccepted(t *testing.T) {
	cases := map[string]map[string]interface{}{
		TypeSimulated:    {"time": 1, "message": "standing in for a package install"},
		TypeExecute:      {"command": "true", "timeout": 5},
		TypeApt:          {"packages": []interface{}{"curl"}},
		TypeFileCreate:   {"path": "/tmp/x", "contents": "y", "mode": "0644", "overwrite": true, "create_dir": true},
		TypeFileDelete:   {"path": "/tmp/x"},
		TypeFileEdit:     {"path": "/tmp/x", "operation": "replace", "match": "a", "content": "b"},
		TypeFileExists:   {"path": "/tmp/x"},
		TypeFileRead:     {"path": "/tmp/x"},
		TypeHTTPRequest:  {"url": "http://localhost/", "method": "GET", "expected_status": 200, "expected_body": "ok", "timeout": 5, "from": "host", "headers": map[string]interface{}{"A": "b"}},
		TypeDNSRequest:   {"hostname": "localhost", "expected_ips": []interface{}{"127.0.0.1"}, "timeout": 5, "from": "host"},
		TypeServiceCheck: {"service": "nginx"},
		TypeWaitFor:      {"command": "true", "timeout": 5},
	}
	for stepType, options := range cases {
		_, err := makeStep(t, stepType, options)
		assert.NoError(t, err, stepType)
	}
}

// The simulated step's message is displayed while it waits — it was
// documented by example everywhere and read by nothing.
func TestSimulatedStepMessage(t *testing.T) {
	step, err := makeStep(t, TypeSimulated, map[string]interface{}{
		"time": 0, "message": "standing in for a package install",
	})
	require.NoError(t, err)
	assert.Equal(t, "standing in for a package install", step.(*SimulatedStep).message)
}

// workdir applies to any node type, since every node runs commands through a
// shell — so it is a prefix on the command rather than an executor setting.
func TestExecuteStepWorkdir(t *testing.T) {
	step, err := makeStep(t, TypeExecute, map[string]interface{}{
		"command": "cat app.txt",
		"workdir": "data",
	})
	require.NoError(t, err)
	assert.Equal(t, []string{"cd -- 'data' && cat app.txt"}, step.(*ExecuteStep).commands)
}

// A directory carrying shell metacharacters must reach cd as data. The check
// is behavioural: the generated command runs for real and the payload's side
// effect must not occur.
func TestExecuteStepWorkdirIsQuoted(t *testing.T) {
	shell, err := exec.LookPath("sh")
	if err != nil {
		t.Skip("a POSIX shell is required")
	}
	marker := filepath.Join(t.TempDir(), "pwned")

	step, err := makeStep(t, TypeExecute, map[string]interface{}{
		"command": "true",
		"workdir": "evil'; touch " + marker + "; echo '",
	})
	require.NoError(t, err)

	// The cd itself fails — there is no such directory — but nothing else
	// may run
	_ = exec.Command(shell, "-c", step.(*ExecuteStep).commands[0]).Run()

	_, statErr := os.Stat(marker)
	assert.True(t, os.IsNotExist(statErr), "the workdir escaped its quoting")
}

// Every command in a multi-command step gets the directory.
func TestExecuteStepWorkdirAppliesToEveryCommand(t *testing.T) {
	step, err := makeStep(t, TypeExecute, map[string]interface{}{
		"command": []interface{}{"one", "two"},
		"workdir": "data",
	})
	require.NoError(t, err)
	for _, cmd := range step.(*ExecuteStep).commands {
		assert.True(t, strings.HasPrefix(cmd, "cd -- 'data' && "), cmd)
	}
}
