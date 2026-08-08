package testtypes

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"math/big"
	"net"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/bgrewell/dart/internal/eval"
	"github.com/bgrewell/dart/internal/formatters"
	"github.com/bgrewell/dart/pkg/ifaces"
	"github.com/bgrewell/dart/pkg/nodetypes"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// startTLSServer serves a self-signed certificate with the given validity
// window and names, returning its host and port.
func startTLSServer(t *testing.T, notAfter time.Time, dnsNames []string, org string) (string, int) {
	t.Helper()

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: dnsNames[0], Organization: []string{org}},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     notAfter,
		DNSNames:     dnsNames,
		IPAddresses:  []net.IP{net.ParseIP("127.0.0.1")},
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	der, err := x509.CreateCertificate(rand.Reader, &template, &template, &key.PublicKey, key)
	require.NoError(t, err)

	listener, err := tls.Listen("tcp", "127.0.0.1:0", &tls.Config{
		Certificates: []tls.Certificate{{Certificate: [][]byte{der}, PrivateKey: key}},
	})
	require.NoError(t, err)
	t.Cleanup(func() { listener.Close() })

	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				if tlsConn, ok := c.(*tls.Conn); ok {
					_ = tlsConn.Handshake()
				}
				c.Close()
			}(conn)
		}
	}()

	addr := listener.Addr().(*net.TCPAddr)
	return "127.0.0.1", addr.Port
}

// tlsNode returns a node that really runs the probe, so the default
// vantage — the test's own node — is what the suite exercises.
func tlsNode(t *testing.T) ifaces.Node {
	t.Helper()
	skipWithoutTool(t, "openssl")
	return localNode(t)
}

func tlsTest(t *testing.T, host string, port int, evaluate map[string]interface{}) map[string]interface{} {
	t.Helper()
	options := map[string]interface{}{
		"host":        host,
		"port":        port,
		"server_name": "dart.test",
		"timeout":     5,
	}
	if evaluate != nil {
		options["evaluate"] = evaluate
	}
	return options
}

func TestTLSCertValidWindow(t *testing.T) {
	host, port := startTLSServer(t, time.Now().Add(90*24*time.Hour), []string{"dart.test", "alt.dart.test"}, "DART Org")

	test, err := makeTest(t, tlsNode(t), TypeTLSCert, tlsTest(t, host, port, map[string]interface{}{
		"min_days_remaining": 30,
		"dns_names":          []interface{}{"dart.test", "alt.dart.test"},
		// Self-signed: the issuer is the certificate's own subject
		"issuer_contains":  "DART Org",
		"subject_contains": "DART Org",
	}))
	require.NoError(t, err)
	allPassed(t, runTest(t, test))
}

// A certificate inside the expiry window fails the days check — the
// renewal alarm this test type exists for.
func TestTLSCertExpiringSoonFails(t *testing.T) {
	host, port := startTLSServer(t, time.Now().Add(5*24*time.Hour), []string{"dart.test"}, "DART Org")

	test, err := makeTest(t, tlsNode(t), TypeTLSCert, tlsTest(t, host, port, map[string]interface{}{
		"min_days_remaining": 30,
	}))
	require.NoError(t, err)
	results := runTest(t, test)
	assert.False(t, results["min_days_remaining"].Passed)
}

// An already-expired certificate is still inspectable (no chain
// verification during the handshake) and fails cleanly.
func TestTLSCertExpiredInspectable(t *testing.T) {
	host, port := startTLSServer(t, time.Now().Add(-24*time.Hour), []string{"dart.test"}, "DART Org")

	test, err := makeTest(t, tlsNode(t), TypeTLSCert, tlsTest(t, host, port, nil))
	require.NoError(t, err)
	results := runTest(t, test)
	assert.False(t, results["min_days_remaining"].Passed, "default check is not-expired")
}

func TestTLSCertMissingName(t *testing.T) {
	host, port := startTLSServer(t, time.Now().Add(90*24*time.Hour), []string{"dart.test"}, "DART Org")

	test, err := makeTest(t, tlsNode(t), TypeTLSCert, tlsTest(t, host, port, map[string]interface{}{
		"dns_names": []interface{}{"other.example"},
	}))
	require.NoError(t, err)
	results := runTest(t, test)
	assert.False(t, results["dns_names"].Passed)
}

// A self-signed certificate fails chain validation; the check can assert
// either polarity.
func TestTLSCertChainValidity(t *testing.T) {
	host, port := startTLSServer(t, time.Now().Add(90*24*time.Hour), []string{"dart.test"}, "DART Org")

	selfSigned, err := makeTest(t, tlsNode(t), TypeTLSCert, tlsTest(t, host, port, map[string]interface{}{
		"chain_valid": false,
	}))
	require.NoError(t, err)
	allPassed(t, runTest(t, selfSigned))

	expectValid, err := makeTest(t, tlsNode(t), TypeTLSCert, tlsTest(t, host, port, map[string]interface{}{
		"chain_valid": true,
	}))
	require.NoError(t, err)
	results := runTest(t, expectValid)
	assert.False(t, results["chain_valid"].Passed)
}

// The JSON facts are on stdout, so the standard evaluators work too.
func TestTLSCertStandardEvaluators(t *testing.T) {
	host, port := startTLSServer(t, time.Now().Add(90*24*time.Hour), []string{"dart.test"}, "DART Org")

	test, err := makeTest(t, tlsNode(t), TypeTLSCert, tlsTest(t, host, port, map[string]interface{}{
		"json_path": map[string]interface{}{"path": "chain_valid", "equals": false},
		"contains":  "dart.test",
	}))
	require.NoError(t, err)
	allPassed(t, runTest(t, test))
}

func TestTLSCertUnreachable(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	_, portStr, _ := net.SplitHostPort(listener.Addr().String())
	port, _ := strconv.Atoi(portStr)
	listener.Close()

	test, err := makeTest(t, tlsNode(t), TypeTLSCert, map[string]interface{}{
		"host": "127.0.0.1", "port": port, "timeout": 1,
	})
	require.NoError(t, err)
	_, runErr := test.Run(formatters.NewMockTestCompleter())
	require.Error(t, runErr)
	assert.Contains(t, runErr.Error(), "no certificate returned by the node")
}

func TestTLSCertValidation(t *testing.T) {
	node := tlsNode(t)
	_, err := makeTest(t, node, TypeTLSCert, map[string]interface{}{})
	assert.ErrorContains(t, err, "host is required")

	_, err = makeTest(t, node, TypeTLSCert, map[string]interface{}{"host": "x", "port": 99999})
	assert.ErrorContains(t, err, "port must be between")

	_, err = makeTest(t, node, TypeTLSCert, map[string]interface{}{
		"host": "x", "evaluate": map[string]interface{}{"min_days_remaining": "soon"}})
	assert.ErrorContains(t, err, "must be a number")

	_, err = makeTest(t, node, TypeTLSCert, map[string]interface{}{
		"host": "x", "evaluate": map[string]interface{}{"dns_names": "not-a-list"}})
	assert.ErrorContains(t, err, "non-empty list")
}

// Node-side port_check runs a real probe through a real shell: these
// exercise the generated script itself, not a hardcoded string.
func runNodeProbe(t *testing.T, host string, port int, evaluate map[string]interface{}) map[string]*eval.EvaluateResult {
	t.Helper()
	shellOpts := map[string]interface{}{"shell": "/bin/sh"}
	node := nodetypes.NewLocalNode("probe", ifaces.NodeOptions(&shellOpts))
	options := map[string]interface{}{
		"host": host, "port": port, "from": "node", "timeout": 2,
	}
	if evaluate != nil {
		options["evaluate"] = evaluate
	}
	test, err := makeTest(t, node, TypePortCheck, options)
	require.NoError(t, err)
	return runTest(t, test)
}

func TestPortCheckFromNodeOpen(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer listener.Close()
	_, portStr, _ := net.SplitHostPort(listener.Addr().String())
	port, _ := strconv.Atoi(portStr)

	allPassed(t, runNodeProbe(t, "127.0.0.1", port, nil))
}

func TestPortCheckFromNodeClosed(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	_, portStr, _ := net.SplitHostPort(listener.Addr().String())
	port, _ := strconv.Atoi(portStr)
	listener.Close()

	allPassed(t, runNodeProbe(t, "127.0.0.1", port, map[string]interface{}{"status": "closed"}))
}

// A host carrying shell metacharacters (which can arrive from a fact, var,
// or capture) must fail the connection, never execute.
func TestPortCheckFromNodeNoInjection(t *testing.T) {
	marker := filepath.Join(t.TempDir(), "pwned")
	hostile := "x'; touch " + marker + "; echo '"

	results := runNodeProbe(t, hostile, 9, map[string]interface{}{"status": "closed"})
	assert.True(t, results["status"].Passed, "hostile host resolves to a failed connection")
	assert.NoFileExists(t, marker, "injected command must never run")
}

// With no usable probe method the script reports "unsupported", so the
// status check fails loudly instead of guessing "closed".
func TestPortCheckFromNodeUnsupportedIsLoud(t *testing.T) {
	emptyDir := t.TempDir()
	shellOpts := map[string]interface{}{
		"shell": "/bin/sh",
		"env":   []interface{}{"PATH=" + emptyDir},
	}
	node := nodetypes.NewLocalNode("probe", ifaces.NodeOptions(&shellOpts))
	test, err := makeTest(t, node, TypePortCheck, map[string]interface{}{
		"host": "127.0.0.1", "port": 9, "from": "node", "timeout": 2,
	})
	require.NoError(t, err)

	results := runTest(t, test)
	assert.False(t, results["status"].Passed, "an unusable probe must fail, not report closed")
}

func TestPortCheckFromValidation(t *testing.T) {
	_, err := makeTest(t, tlsNode(t), TypePortCheck, map[string]interface{}{
		"host": "x", "port": 80, "from": "somewhere",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), `"node" or "host"`)
}

// The probe never inlines the host into an inner shell string.
func TestNodePortProbeQuoting(t *testing.T) {
	probe := nodePortProbe("evil'; rm -rf /; echo '", 80, 3)
	assert.NotContains(t, probe, "/dev/tcp/evil")
	assert.Contains(t, probe, `bash -c 'exec 3<>/dev/tcp/"$0"/"$1"' "$dart_h" "$dart_p"`)
	assert.Contains(t, probe, "echo unsupported")
	assert.Contains(t, probe, "invalid option")
}

// Certificates covering an IP rather than a name verify correctly.
func TestTLSCertIPSAN(t *testing.T) {
	host, port := startTLSServer(t, time.Now().Add(90*24*time.Hour), []string{"dart.test"}, "DART Org")

	options := map[string]interface{}{
		"host": host, "port": port, "server_name": "dart.test", "timeout": 5,
		"evaluate": map[string]interface{}{"dns_names": []interface{}{"127.0.0.1"}},
	}
	test, err := makeTest(t, tlsNode(t), TypeTLSCert, options)
	require.NoError(t, err)
	allPassed(t, runTest(t, test))
}

// Host-side inspection reaches the same conclusions as node-side, so a
// suite can switch vantage without rewriting its checks.
func TestTLSCertFromHost(t *testing.T) {
	host, port := startTLSServer(t, time.Now().Add(90*24*time.Hour), []string{"dart.test"}, "DART Org")

	options := tlsTest(t, host, port, map[string]interface{}{
		"min_days_remaining": 30,
		"dns_names":          []interface{}{"dart.test"},
		"issuer_contains":    "DART Org",
	})
	options["from"] = "host"

	test, err := makeTest(t, nodetypes.NewMockNode(), TypeTLSCert, options)
	require.NoError(t, err)
	allPassed(t, runTest(t, test))
}
