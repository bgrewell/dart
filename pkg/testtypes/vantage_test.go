package testtypes

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/bgrewell/dart/internal/config"
	"github.com/bgrewell/dart/internal/formatters"
	"github.com/bgrewell/dart/pkg/ifaces"
	"github.com/bgrewell/dart/pkg/nodetypes"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Network tests probe from the node they name. A suite that says
// `node: web-01` is asking what web-01 can see, and answering from the
// controller reports a different machine's reachability as if it were the
// node's.
func TestNetworkTestsDefaultToNode(t *testing.T) {
	node := nodetypes.NewMockNode()
	cases := map[string]map[string]interface{}{
		TypeHTTPRequest: {"url": "http://localhost/"},
		TypePortCheck:   {"host": "localhost", "port": 80},
		TypeTLSCert:     {"host": "localhost", "port": 443},
	}
	for testType, options := range cases {
		test, err := makeTest(t, node, testType, options)
		require.NoError(t, err, testType)
		assert.IsType(t, &commandTest{}, test,
			"%s must run on the node by default", testType)
	}
}

// from: host keeps the controller's viewpoint for the cases that want it.
func TestNetworkTestsAcceptHostVantage(t *testing.T) {
	node := nodetypes.NewMockNode()
	cases := map[string]map[string]interface{}{
		TypeHTTPRequest: {"url": "http://localhost/", "from": "host"},
		TypePortCheck:   {"host": "localhost", "port": 80, "from": "host"},
		TypeTLSCert:     {"host": "localhost", "port": 443, "from": "host"},
	}
	for testType, options := range cases {
		test, err := makeTest(t, node, testType, options)
		require.NoError(t, err, testType)
		_, onNode := test.(*commandTest)
		assert.False(t, onNode,
			"%s with from: host must not run on the node", testType)
	}

	for testType, options := range map[string]map[string]interface{}{
		TypeHTTPRequest: {"url": "http://localhost/", "from": "somewhere"},
		TypePortCheck:   {"host": "localhost", "port": 80, "from": "somewhere"},
		TypeTLSCert:     {"host": "localhost", "port": 443, "from": "somewhere"},
	} {
		_, err := makeTest(t, node, testType, options)
		assert.ErrorContains(t, err, `from must be "node" or "host"`, testType)
	}
}

// A captured value reaches a node-side probe as data. The probe's command
// is quoted when it is built, so substituting a capture into that quoted
// text afterwards would let a value containing a quote escape into the
// shell — the command is therefore built after substitution, not before.
func TestCaptureCannotInjectIntoNodeProbe(t *testing.T) {
	probeOptions := map[string]map[string]interface{}{
		TypePortCheck: {
			"host": "{{capture.payload}}", "port": 9, "timeout": 1,
			"evaluate": map[string]interface{}{"status": "closed"},
		},
		TypeHTTPRequest: {
			"url": "http://{{capture.payload}}/", "timeout": 2,
			"headers": map[string]interface{}{"X-Test": "{{capture.payload}}"},
		},
		TypeTLSCert: {
			"host": "{{capture.payload}}", "port": 9, "timeout": 2,
		},
	}

	for testType, options := range probeOptions {
		t.Run(testType, func(t *testing.T) {
			marker := filepath.Join(t.TempDir(), "pwned")
			payload := `evil'; touch ` + marker + `; echo '`

			nodes := map[string]ifaces.Node{"n": localNode(t)}
			configs := []*config.TestConfig{
				{
					Name: "capture the payload",
					Node: config.NodeReference{"n"},
					Type: TypeExecute,
					Options: map[string]interface{}{
						"command": "printf %s " + shellSingleQuote(payload),
						"capture": "payload",
					},
				},
				{
					Name:    "probe with the payload",
					Node:    config.NodeReference{"n"},
					Type:    testType,
					Options: options,
				},
			}

			tests, err := CreateTests(configs, nodes)
			require.NoError(t, err)
			for _, test := range tests {
				// The probe is expected to fail; only its side effects matter
				_, _ = test.Run(formatters.NewMockTestCompleter())
			}

			_, statErr := os.Stat(marker)
			assert.True(t, os.IsNotExist(statErr),
				"the captured value escaped %s's quoting", testType)
		})
	}
}

// shellSingleQuote wraps text for the capturing command, which is written
// by the test rather than by a probe builder.
func shellSingleQuote(text string) string {
	quoted := "'"
	for _, r := range text {
		if r == '\'' {
			quoted += `'\''`
			continue
		}
		quoted += string(r)
	}
	return quoted + "'"
}
