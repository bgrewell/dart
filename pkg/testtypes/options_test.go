package testtypes

import (
	"strings"
	"testing"

	"github.com/bgrewell/dart/pkg/nodetypes"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The most damaging typo in the whole configuration format: a misspelled
// `evaluate` left the test with zero checks, which is reported as a pass.
func TestMisspelledEvaluateIsRejected(t *testing.T) {
	_, err := makeTest(t, nodetypes.NewMockNode(), TypeExecute, map[string]interface{}{
		"command":   "true",
		"evaluatte": map[string]interface{}{"exit_code": 0},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), `unknown option "evaluatte"`)
	assert.Contains(t, err.Error(), "evaluate")
}

// An evaluator written at the option level instead of inside evaluate is
// the same failure wearing a different hat.
func TestMisplacedEvaluatorIsRejected(t *testing.T) {
	_, err := makeTest(t, nodetypes.NewMockNode(), TypeExecute, map[string]interface{}{
		"command":   "true",
		"exit_code": 0,
	})
	assert.ErrorContains(t, err, `unknown option "exit_code"`)
}

// capture and extract are execute-only. Silently ignoring them elsewhere
// meant a later {{capture.x}} failed with a confusing "no captured value".
func TestCaptureOnWrongTestTypeIsRejected(t *testing.T) {
	_, err := makeTest(t, nodetypes.NewMockNode(), TypePing, map[string]interface{}{
		"target":  "localhost",
		"capture": "addr",
	})
	assert.ErrorContains(t, err, `unknown option "capture"`)
}

// Every documented option of every test type must survive the check, or the
// tracking has missed a read and valid suites break.
func TestKnownTestOptionsAccepted(t *testing.T) {
	node := nodetypes.NewMockNode()
	cases := map[string]map[string]interface{}{
		TypeExecute:       {"command": "true", "timeout": 5, "capture": "out", "evaluate": map[string]interface{}{"exit_code": 0}},
		TypeExists:        {"path": "/tmp/x", "evaluate": map[string]interface{}{"exists": true}},
		TypeFileContent:   {"filename": "/tmp/x", "evaluate": map[string]interface{}{"contains": "y"}},
		TypeFileHash:      {"filename": "/tmp/x", "evaluate": map[string]interface{}{"sha256": strings.Repeat("a", 64)}},
		TypeHTTPRequest:   {"url": "http://localhost/", "method": "GET", "timeout": 5, "from": "host", "headers": map[string]interface{}{"A": "b"}},
		TypePing:          {"target": "localhost", "count": 3, "evaluate": map[string]interface{}{"packet_loss": 0}},
		TypePortCheck:     {"host": "localhost", "port": 80, "timeout": 5, "from": "host"},
		TypeServiceStatus: {"service": "nginx", "evaluate": map[string]interface{}{"status": "active"}},
		TypeTLSCert:       {"host": "localhost", "port": 443, "server_name": "x", "timeout": 5, "from": "host"},
	}
	for testType, options := range cases {
		_, err := makeTest(t, node, testType, options)
		assert.NoError(t, err, testType)
	}
}

// Documented aliases must both be accepted, and naming one must not make
// the other look unread.
func TestOptionAliasesAccepted(t *testing.T) {
	node := nodetypes.NewMockNode()
	for _, key := range []string{"filename", "path"} {
		_, err := makeTest(t, node, TypeFileContent, map[string]interface{}{
			key: "/tmp/x", "evaluate": map[string]interface{}{"contains": "y"},
		})
		assert.NoError(t, err, key)
	}
}
