package probe

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseVantageDefaultsToNode(t *testing.T) {
	vantage, ok := ParseVantage("")
	require.True(t, ok)
	assert.Equal(t, VantageNode, vantage)

	for _, raw := range []string{"node", "host"} {
		vantage, ok := ParseVantage(raw)
		require.True(t, ok, raw)
		assert.Equal(t, Vantage(raw), vantage)
	}

	for _, raw := range []string{"Node", "HOST", "controller", "local", " node"} {
		_, ok := ParseVantage(raw)
		assert.False(t, ok, raw)
	}
}

// Values reaching a probe come from facts, vars, and captures, so a hostile
// value must arrive at the tool as data. The check is behavioural: the
// commands run for real against stub tools, and the payload's side effect
// must never occur.
func TestCommandsQuoteUntrustedValues(t *testing.T) {
	shell, err := exec.LookPath("sh")
	if err != nil {
		t.Skip("a POSIX shell is required")
	}

	// Stubs stand in for the probe tools so the commands reach their
	// argument-handling instead of exiting at the missing-tool guard
	stubs := t.TempDir()
	for _, tool := range []string{"curl", "openssl", "getent", "dig", "host", "nslookup", "awk", "sort", "grep", "sed"} {
		require.NoError(t, os.WriteFile(filepath.Join(stubs, tool), []byte("#!/bin/sh\nexit 0\n"), 0o755))
	}

	marker := filepath.Join(t.TempDir(), "pwned")
	payload := `evil'; touch ` + marker + `; echo '`

	commands := map[string]string{
		"http": HTTPCommand("GET", "http://"+payload+"/", map[string]string{"X-Test": payload}, 5),
		"dns":  DNSCommand(payload),
		"tls":  TLSCommand(payload, 443, payload),
	}
	for name, command := range commands {
		cmd := exec.Command(shell, "-c", command)
		cmd.Env = append(os.Environ(), "PATH="+stubs)
		_ = cmd.Run()

		_, err := os.Stat(marker)
		assert.True(t, os.IsNotExist(err), "%s executed the payload: %s", name, command)
	}
}

// The generated commands must be valid shell, not merely well-quoted.
func TestCommandsParseAsShell(t *testing.T) {
	shell, err := exec.LookPath("sh")
	if err != nil {
		t.Skip("a POSIX shell is required")
	}
	commands := map[string]string{
		"http": HTTPCommand("GET", "http://example.invalid/", map[string]string{"A": "b"}, 5),
		"dns":  DNSCommand("example.invalid"),
		"tls":  TLSCommand("example.invalid", 443, "example.invalid"),
	}
	for name, command := range commands {
		out, err := exec.Command(shell, "-n", "-c", command).CombinedOutput()
		assert.NoError(t, err, "%s: %s", name, out)
	}
}

// Header order is stable so the same suite generates the same command,
// which keeps failures reproducible and diffs readable.
func TestHTTPCommandOrdersHeaders(t *testing.T) {
	headers := map[string]string{"Zeta": "1", "Alpha": "2", "Mid": "3"}
	first := HTTPCommand("GET", "http://example.invalid/", headers, 5)
	assert.Equal(t, first, HTTPCommand("GET", "http://example.invalid/", headers, 5))
	assert.Less(t, strings.Index(first, "Alpha"), strings.Index(first, "Mid"))
	assert.Less(t, strings.Index(first, "Mid"), strings.Index(first, "Zeta"))
}

// A fractional timeout still yields an integer --max-time curl accepts,
// rounded up so a sub-second budget is never truncated to zero (which curl
// reads as "no limit").
func TestHTTPCommandRoundsTimeoutUp(t *testing.T) {
	assert.Contains(t, HTTPCommand("GET", "http://x/", nil, 0.4), "--max-time 1 ")
	assert.Contains(t, HTTPCommand("GET", "http://x/", nil, 2.1), "--max-time 3 ")
}

func TestParseHTTPOutput(t *testing.T) {
	body, status, err := ParseHTTPOutput("hello world"+httpStatusSentinel+"200", "")
	require.NoError(t, err)
	assert.Equal(t, "hello world", body)
	assert.Equal(t, 200, status)

	// A body containing something sentinel-like must not confuse the split:
	// the last occurrence is the one curl wrote
	body, status, err = ParseHTTPOutput(httpStatusSentinel+"999"+httpStatusSentinel+"404", "")
	require.NoError(t, err)
	assert.Equal(t, httpStatusSentinel+"999", body)
	assert.Equal(t, 404, status)

	// No sentinel means curl never got a response; its message is the
	// useful part and the check must error rather than report a status
	_, _, err = ParseHTTPOutput("", "curl: (7) Failed to connect")
	assert.ErrorContains(t, err, "Failed to connect")

	// curl still writes the sentinel when no transfer completed, with
	// http_code 000. Reporting that as status 0 would bury the reason.
	_, _, err = ParseHTTPOutput(httpStatusSentinel+"000", "curl: (60) certificate problem")
	assert.ErrorContains(t, err, "certificate problem")
}

func TestParseAddresses(t *testing.T) {
	// getent ahosts repeats an address once per socket type
	assert.Equal(t, []string{"127.0.0.1", "::1"},
		ParseAddresses("127.0.0.1\n127.0.0.1\n127.0.0.1\n::1\n::1\n"))

	// nslookup labels its answers
	assert.Equal(t, []string{"93.184.216.34"},
		ParseAddresses("Address: 93.184.216.34\n"))

	assert.Empty(t, ParseAddresses("\n  \n"))
}

// The DNS lookup falls back across resolvers rather than assuming one is
// installed, and says so plainly when none is.
func TestDNSCommandCoversResolvers(t *testing.T) {
	command := DNSCommand("example.invalid")
	for _, tool := range []string{"getent", "dig", "host", "nslookup"} {
		assert.Contains(t, command, "command -v "+tool)
	}
	assert.Contains(t, command, "exit 127")
}

func TestRequireToolExits127(t *testing.T) {
	shell, err := exec.LookPath("sh")
	if err != nil {
		t.Skip("a POSIX shell is required")
	}
	command := RequireTool("dart-tool-that-does-not-exist") + "\necho reached"

	out, err := exec.Command(shell, "-c", command).CombinedOutput()
	require.Error(t, err)
	exitErr, ok := err.(*exec.ExitError)
	require.True(t, ok)
	assert.Equal(t, MissingToolExitCode, exitErr.ExitCode())
	assert.Contains(t, string(out), "is required on this node")
	assert.NotContains(t, string(out), "reached")
}
