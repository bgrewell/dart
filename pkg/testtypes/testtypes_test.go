package testtypes

import (
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"strconv"
	"strings"
	"testing"

	"github.com/bgrewell/dart/internal/config"
	"github.com/bgrewell/dart/internal/eval"
	"github.com/bgrewell/dart/internal/formatters"
	"github.com/bgrewell/dart/pkg/ifaces"
	"github.com/bgrewell/dart/pkg/nodetypes"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func makeTest(t *testing.T, node ifaces.Node, testType string, options map[string]interface{}) (ifaces.Test, error) {
	t.Helper()
	nodes := map[string]ifaces.Node{"test-node": node}
	configs := []*config.TestConfig{
		{
			Name:    "factory test",
			Node:    config.NodeReference{"test-node"},
			Type:    testType,
			Options: options,
		},
	}
	tests, err := CreateTests(configs, nodes)
	if err != nil {
		return nil, err
	}
	require.Len(t, tests, 1)
	return tests[0], nil
}

// localNode returns a node that really executes commands, so node-side
// probes are exercised end to end rather than mocked.
func localNode(t *testing.T) ifaces.Node {
	t.Helper()
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("a POSIX shell is required for node-side probes")
	}
	return nodetypes.NewLocalNode("test-node", nil, "")
}

func skipWithoutTool(t *testing.T, tool string) {
	t.Helper()
	if _, err := exec.LookPath(tool); err != nil {
		t.Skipf("%s is required for this test", tool)
	}
}

func runTest(t *testing.T, test ifaces.Test) map[string]*eval.EvaluateResult {
	t.Helper()
	results, err := test.Run(formatters.NewMockTestCompleter())
	require.NoError(t, err)
	return results
}

func allPassed(t *testing.T, results map[string]*eval.EvaluateResult) {
	t.Helper()
	require.NotEmpty(t, results)
	for name, result := range results {
		assert.NoError(t, result.Err, name)
		assert.True(t, result.Passed, name)
	}
}

func TestAllTestTypesWired(t *testing.T) {
	node := nodetypes.NewMockNode()
	cases := map[string]map[string]interface{}{
		TypeExecute:       {"command": "true"},
		TypeExists:        {"path": "/tmp/x"},
		TypeFileContent:   {"filename": "/tmp/x"},
		TypeFileHash:      {"filename": "/tmp/x", "evaluate": map[string]interface{}{"sha256": strings.Repeat("a", 64)}},
		TypeHTTPRequest:   {"url": "http://localhost/health"},
		TypePing:          {"target": "localhost"},
		TypePortCheck:     {"host": "localhost", "port": 80},
		TypeServiceStatus: {"service": "nginx"},
	}
	for testType, options := range cases {
		_, err := makeTest(t, node, testType, options)
		assert.NoError(t, err, testType)
	}
}

func TestUnknownTestType(t *testing.T) {
	_, err := makeTest(t, nodetypes.NewMockNode(), "resource", map[string]interface{}{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown test type")
}

func TestExistsTest(t *testing.T) {
	node := nodetypes.NewMockNode()
	node.SetResponse("test -e '/tmp/present'", 0, "", "")
	node.SetResponse("test -e '/tmp/absent'", 1, "", "")

	present, err := makeTest(t, node, TypeExists, map[string]interface{}{"path": "/tmp/present"})
	require.NoError(t, err)
	allPassed(t, runTest(t, present))

	absent, err := makeTest(t, node, TypeExists, map[string]interface{}{
		"path":     "/tmp/absent",
		"evaluate": map[string]interface{}{"exists": false},
	})
	require.NoError(t, err)
	allPassed(t, runTest(t, absent))

	wrong, err := makeTest(t, node, TypeExists, map[string]interface{}{"path": "/tmp/absent"})
	require.NoError(t, err)
	results := runTest(t, wrong)
	assert.False(t, results["exists"].Passed)
}

func TestFileContentTest(t *testing.T) {
	node := nodetypes.NewMockNode()
	node.SetResponse("cat '/etc/app.conf'", 0, "port = 8080\nhost = db\n", "")

	test, err := makeTest(t, node, TypeFileContent, map[string]interface{}{
		"filename": "/etc/app.conf",
		"evaluate": map[string]interface{}{
			"contains": "port = 8080",
			"regex":    "host = \\w+",
		},
	})
	require.NoError(t, err)
	allPassed(t, runTest(t, test))

	// No evaluate block asserts readability via the exit code
	node.SetResponse("cat '/missing'", 1, "", "cat: /missing: No such file or directory")
	missing, err := makeTest(t, node, TypeFileContent, map[string]interface{}{"path": "/missing"})
	require.NoError(t, err)
	results := runTest(t, missing)
	assert.False(t, results["readable"].Passed)
}

func TestFileHashTest(t *testing.T) {
	node := nodetypes.NewMockNode()
	md5hex := "098f6bcd4621d373cade4e832627b4f6"
	sha256hex := "9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08"
	node.SetResponse("md5sum -- '/tmp/f' && sha256sum -- '/tmp/f'", 0,
		fmt.Sprintf("%s  /tmp/f\n%s  /tmp/f\n", md5hex, sha256hex), "")

	test, err := makeTest(t, node, TypeFileHash, map[string]interface{}{
		"filename": "/tmp/f",
		"evaluate": map[string]interface{}{"md5": md5hex, "sha256": strings.ToUpper(sha256hex)},
	})
	require.NoError(t, err)
	allPassed(t, runTest(t, test))

	// Wrong digest fails
	bad, err := makeTest(t, node, TypeFileHash, map[string]interface{}{
		"filename": "/tmp/f",
		"evaluate": map[string]interface{}{"md5": strings.Repeat("0", 32), "sha256": sha256hex},
	})
	require.NoError(t, err)
	results := runTest(t, bad)
	assert.False(t, results["md5"].Passed)
	assert.True(t, results["sha256"].Passed)

	// Validation errors
	_, err = makeTest(t, node, TypeFileHash, map[string]interface{}{"filename": "/tmp/f"})
	assert.ErrorContains(t, err, "at least one of")
	_, err = makeTest(t, node, TypeFileHash, map[string]interface{}{
		"filename": "/tmp/f", "evaluate": map[string]interface{}{"md5": "nothex"}})
	assert.ErrorContains(t, err, "hex digest")
	_, err = makeTest(t, node, TypeFileHash, map[string]interface{}{
		"filename": "/tmp/f", "evaluate": map[string]interface{}{"crc32": "abcd"}})
	assert.ErrorContains(t, err, "is not available in a file_hash test")
}

func TestServiceStatusTest(t *testing.T) {
	node := nodetypes.NewMockNode()
	node.SetResponse("systemctl is-active 'nginx'", 0, "active\n", "")
	node.SetResponse("systemctl is-active 'stopped-svc'", 3, "inactive\n", "")

	active, err := makeTest(t, node, TypeServiceStatus, map[string]interface{}{"service": "nginx"})
	require.NoError(t, err)
	allPassed(t, runTest(t, active))

	// Asserting a non-active state passes when the output matches
	inactive, err := makeTest(t, node, TypeServiceStatus, map[string]interface{}{
		"service":  "stopped-svc",
		"evaluate": map[string]interface{}{"status": "inactive"},
	})
	require.NoError(t, err)
	allPassed(t, runTest(t, inactive))

	wrong, err := makeTest(t, node, TypeServiceStatus, map[string]interface{}{"service": "stopped-svc"})
	require.NoError(t, err)
	results := runTest(t, wrong)
	assert.False(t, results["status"].Passed)
}

const iputilsPing = `PING localhost (127.0.0.1) 56(84) bytes of data.

--- localhost ping statistics ---
5 packets transmitted, 5 received, 0% packet loss, time 4005ms
rtt min/avg/max/mdev = 0.031/0.042/0.058/0.010 ms
`

const busyboxPing = `PING localhost (127.0.0.1): 56 data bytes

--- localhost ping statistics ---
5 packets transmitted, 4 packets received, 20% packet loss
round-trip min/avg/max = 0.5/1.2/3.4 ms
`

func TestPingTest(t *testing.T) {
	node := nodetypes.NewMockNode()
	node.SetResponse("ping -q -c 5 'localhost'", 0, iputilsPing, "")

	test, err := makeTest(t, node, TypePing, map[string]interface{}{
		"target": "localhost",
		"evaluate": map[string]interface{}{
			"packet_loss": 0,
			"rtt_min":     0,
			"rtt_avg":     1,
			"rtt_max":     10,
		},
	})
	require.NoError(t, err)
	allPassed(t, runTest(t, test))

	// busybox format, lossy: default packet_loss 0 check fails
	node.SetResponse("ping -q -c 5 'flaky-host'", 1, busyboxPing, "")
	lossy, err := makeTest(t, node, TypePing, map[string]interface{}{"target": "flaky-host"})
	require.NoError(t, err)
	results := runTest(t, lossy)
	assert.False(t, results["packet_loss"].Passed)

	// but passes when the bound allows it, and busybox rtt parses
	tolerant, err := makeTest(t, node, TypePing, map[string]interface{}{
		"target": "flaky-host",
		"evaluate": map[string]interface{}{
			"packet_loss": 25,
			"rtt_max":     5,
		},
	})
	require.NoError(t, err)
	allPassed(t, runTest(t, tolerant))
}

func TestHTTPRequestTest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "test-agent", r.Header.Get("User-Agent"))
		if r.URL.Path == "/health" {
			fmt.Fprint(w, `{"status": "healthy"}`)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	// The default vantage is the node; from: host must produce an
	// identical result when the node is the local machine
	for _, from := range []string{"", "node", "host"} {
		t.Run("from="+from, func(t *testing.T) {
			node := nodetypes.NewMockNode()
			if from != "host" {
				skipWithoutTool(t, "curl")
				node = nil
			}
			options := map[string]interface{}{
				"url":     server.URL + "/health",
				"headers": map[string]interface{}{"User-Agent": "test-agent"},
				"evaluate": map[string]interface{}{
					"status_code": 200,
					"contains":    "healthy",
					"json_path":   map[string]interface{}{"path": "status", "equals": "healthy"},
				},
			}
			if from != "" {
				options["from"] = from
			}

			target := ifaces.Node(node)
			if node == nil {
				target = localNode(t)
			}
			test, err := makeTest(t, target, TypeHTTPRequest, options)
			require.NoError(t, err)
			allPassed(t, runTest(t, test))

			// Default evaluate asserts status 200; a 404 fails it
			missingOptions := map[string]interface{}{
				"url":     server.URL + "/nope",
				"headers": map[string]interface{}{"User-Agent": "test-agent"},
			}
			if from != "" {
				missingOptions["from"] = from
			}
			missing, err := makeTest(t, target, TypeHTTPRequest, missingOptions)
			require.NoError(t, err)
			results := runTest(t, missing)
			assert.False(t, results["status_code"].Passed)
		})
	}

	_, err := makeTest(t, nodetypes.NewMockNode(), TypeHTTPRequest, map[string]interface{}{
		"url":  "http://localhost/",
		"from": "elsewhere",
	})
	assert.ErrorContains(t, err, `from must be "node" or "host"`)
}

func TestPortCheckTest(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer listener.Close()
	_, portStr, err := net.SplitHostPort(listener.Addr().String())
	require.NoError(t, err)
	openPort, err := strconv.Atoi(portStr)
	require.NoError(t, err)

	node := localNode(t)
	open, err := makeTest(t, node, TypePortCheck, map[string]interface{}{
		"host": "127.0.0.1",
		"port": openPort,
	})
	require.NoError(t, err)
	allPassed(t, runTest(t, open))

	// A closed port with status: closed passes; default open check fails
	closedListener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	_, closedPortStr, _ := net.SplitHostPort(closedListener.Addr().String())
	closedPort, _ := strconv.Atoi(closedPortStr)
	closedListener.Close()

	closed, err := makeTest(t, node, TypePortCheck, map[string]interface{}{
		"host":     "127.0.0.1",
		"port":     closedPort,
		"timeout":  1,
		"evaluate": map[string]interface{}{"status": "closed"},
	})
	require.NoError(t, err)
	allPassed(t, runTest(t, closed))

	_, err = makeTest(t, node, TypePortCheck, map[string]interface{}{"host": "x", "port": 99999})
	assert.ErrorContains(t, err, "port must be between")
}

func TestFactoryValidation(t *testing.T) {
	node := nodetypes.NewMockNode()
	cases := map[string]struct {
		testType string
		options  map[string]interface{}
		contains string
	}{
		"execute missing command": {TypeExecute, map[string]interface{}{}, "command is required"},
		"exists missing path":     {TypeExists, map[string]interface{}{}, "path is required"},
		"exists bad exists type":  {TypeExists, map[string]interface{}{"path": "/x", "evaluate": map[string]interface{}{"exists": "yes"}}, "must be a boolean"},
		"ping missing target":     {TypePing, map[string]interface{}{}, "target is required"},
		"ping bad count":          {TypePing, map[string]interface{}{"target": "x", "count": 0}, "at least 1"},
		"http missing url":        {TypeHTTPRequest, map[string]interface{}{}, "url is required"},
		"http bad status_code":    {TypeHTTPRequest, map[string]interface{}{"url": "http://x", "evaluate": map[string]interface{}{"status_code": "ok"}}, "must be an integer"},
		"port bad status":         {TypePortCheck, map[string]interface{}{"host": "x", "port": 80, "evaluate": map[string]interface{}{"status": "ajar"}}, "open"},
		"service missing service": {TypeServiceStatus, map[string]interface{}{}, "service is required"},
		"unknown evaluate key":    {TypeExecute, map[string]interface{}{"command": "true", "evaluate": map[string]interface{}{"bogus": 1}}, "unknown evaluation type"},
		"evaluate not a map":      {TypeExecute, map[string]interface{}{"command": "true", "evaluate": "exit_code"}, "evaluate must be a map"},
	}
	for name, tc := range cases {
		_, err := makeTest(t, node, tc.testType, tc.options)
		require.Error(t, err, name)
		assert.Contains(t, err.Error(), tc.contains, name)
	}
}

// Native YAML integer values (no JSON round-trip) parse in evaluate blocks.
func TestExecuteNativeIntEvaluate(t *testing.T) {
	node := nodetypes.NewMockNode()
	node.SetResponse("exit-check", 2, "", "")

	test, err := makeTest(t, node, TypeExecute, map[string]interface{}{
		"command":  "exit-check",
		"evaluate": map[string]interface{}{"exit_code": 2},
	})
	require.NoError(t, err)
	allPassed(t, runTest(t, test))
}
