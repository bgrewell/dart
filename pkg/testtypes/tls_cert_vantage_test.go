package testtypes

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"math/big"
	"net"
	"testing"
	"time"

	"github.com/bgrewell/dart/internal/formatters"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The two vantages must reach identical conclusions about the same
// endpoint when the node IS the local machine. Any divergence here is a
// divergence a real suite would see when switching `from`.
func TestTLSCertNodeAndHostAgree(t *testing.T) {
	skipWithoutTool(t, "openssl")
	host, port := startTLSServer(t, time.Now().Add(90*24*time.Hour), []string{"dart.test", "alt.dart.test"}, "DART Org")

	run := func(from string) certFacts {
		t.Helper()
		options := tlsTest(t, host, port, nil)
		options["from"] = from
		node := localNode(t)
		test, err := makeTest(t, node, TypeTLSCert, options)
		require.NoError(t, err)

		var produced certFacts
		switch probe := test.(type) {
		case *commandTest:
			command, err := probe.resolveCommand()
			require.NoError(t, err)
			raw, err := node.Execute(command)
			require.NoError(t, err)
			shaped, err := probe.transform(raw)
			require.NoError(t, err)
			out, err := shaped.StdoutBytes()
			require.NoError(t, err)
			require.NoError(t, json.Unmarshal(out, &produced))
		case *TLSCertTest:
			result, err := probe.inspect()
			require.NoError(t, err)
			out, err := result.StdoutBytes()
			require.NoError(t, err)
			require.NoError(t, json.Unmarshal(out, &produced))
		default:
			t.Fatalf("unexpected test type %T for from=%s", test, from)
		}
		return produced
	}

	node, host2 := run("node"), run("host")

	require.Equal(t, host2.Subject, node.Subject, "subject")
	require.Equal(t, host2.Issuer, node.Issuer, "issuer")
	require.Equal(t, host2.DNSNames, node.DNSNames, "dns_names")
	require.Equal(t, host2.IPAddresses, node.IPAddresses, "ip_addresses")
	require.Equal(t, host2.NotBefore, node.NotBefore, "not_before")
	require.Equal(t, host2.NotAfter, node.NotAfter, "not_after")
	require.Equal(t, host2.ChainValid, node.ChainValid, "chain_valid")
	require.InDelta(t, host2.DaysRemaining, node.DaysRemaining, 0.01, "days_remaining")
}

// A served chain is leaf-first for Go and for `openssl -showcerts`. If the
// two ever disagreed, certFactsFrom would treat an intermediate as the leaf
// and report the wrong subject, expiry, and names.
func TestTLSCertChainOrderMatchesAcrossVantages(t *testing.T) {
	skipWithoutTool(t, "openssl")

	rootKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	rootTmpl := x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "DART Test Root", Organization: []string{"DART Root Org"}},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(365 * 24 * time.Hour),
		KeyUsage:              x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}
	rootDER, err := x509.CreateCertificate(rand.Reader, &rootTmpl, &rootTmpl, &rootKey.PublicKey, rootKey)
	require.NoError(t, err)
	rootCert, err := x509.ParseCertificate(rootDER)
	require.NoError(t, err)

	interKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	interTmpl := x509.Certificate{
		SerialNumber:          big.NewInt(2),
		Subject:               pkix.Name{CommonName: "DART Test Intermediate", Organization: []string{"DART Intermediate Org"}},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(180 * 24 * time.Hour),
		KeyUsage:              x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}
	interDER, err := x509.CreateCertificate(rand.Reader, &interTmpl, rootCert, &interKey.PublicKey, rootKey)
	require.NoError(t, err)
	interCert, err := x509.ParseCertificate(interDER)
	require.NoError(t, err)

	leafKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	leafTmpl := x509.Certificate{
		SerialNumber: big.NewInt(3),
		Subject:      pkix.Name{CommonName: "dart.test", Organization: []string{"DART Leaf Org"}},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(45 * 24 * time.Hour),
		DNSNames:     []string{"dart.test"},
		IPAddresses:  []net.IP{net.ParseIP("127.0.0.1")},
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	leafDER, err := x509.CreateCertificate(rand.Reader, &leafTmpl, interCert, &leafKey.PublicKey, interKey)
	require.NoError(t, err)

	// Served leaf-first with the intermediate following, as a real server does
	listener, err := tls.Listen("tcp", "127.0.0.1:0", &tls.Config{
		Certificates: []tls.Certificate{{
			Certificate: [][]byte{leafDER, interDER},
			PrivateKey:  leafKey,
		}},
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
				if tc, ok := c.(*tls.Conn); ok {
					_ = tc.Handshake()
				}
				c.Close()
			}(conn)
		}
	}()

	addr := listener.Addr().(*net.TCPAddr)
	options := tlsTest(t, "127.0.0.1", addr.Port, nil)
	node := localNode(t)

	facts := func(from string) certFacts {
		t.Helper()
		opts := map[string]interface{}{}
		for k, v := range options {
			opts[k] = v
		}
		opts["from"] = from
		test, err := makeTest(t, node, TypeTLSCert, opts)
		require.NoError(t, err)

		var out []byte
		switch probe := test.(type) {
		case *commandTest:
			command, err := probe.resolveCommand()
			require.NoError(t, err)
			raw, err := node.Execute(command)
			require.NoError(t, err)
			shaped, err := probe.transform(raw)
			require.NoError(t, err)
			out, err = shaped.StdoutBytes()
			require.NoError(t, err)
		case *TLSCertTest:
			result, err := probe.inspect()
			require.NoError(t, err)
			out, err = result.StdoutBytes()
			require.NoError(t, err)
		}
		var f certFacts
		require.NoError(t, json.Unmarshal(out, &f))
		return f
	}

	nodeFacts, hostFacts := facts("node"), facts("host")

	// The leaf, not the intermediate, must be what both report
	require.Contains(t, nodeFacts.Subject, "dart.test", "node path picked the wrong leaf")
	require.Contains(t, nodeFacts.Issuer, "DART Test Intermediate", "node path picked the wrong issuer")
	require.Equal(t, hostFacts.Subject, nodeFacts.Subject)
	require.Equal(t, hostFacts.Issuer, nodeFacts.Issuer)
	require.Equal(t, hostFacts.NotAfter, nodeFacts.NotAfter)
	require.Equal(t, hostFacts.DNSNames, nodeFacts.DNSNames)
	require.Equal(t, hostFacts.ChainValid, nodeFacts.ChainValid)
	require.InDelta(t, hostFacts.DaysRemaining, nodeFacts.DaysRemaining, 0.01)
	t.Logf("leaf subject=%q issuer=%q days=%.1f chain_valid=%v",
		nodeFacts.Subject, nodeFacts.Issuer, nodeFacts.DaysRemaining, nodeFacts.ChainValid)
}

// A server that accepts the TCP connection and then never speaks stalls
// openssl indefinitely — openssl's own timeout flags do not cover it. The
// suite-side command bound is the only thing that ends the run.
func TestTLSCertNodeProbeCannotHang(t *testing.T) {
	skipWithoutTool(t, "openssl")

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { listener.Close() })

	accepted := make(chan net.Conn, 4)
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			// Hold the connection open, send nothing
			accepted <- conn
		}
	}()

	port := listener.Addr().(*net.TCPAddr).Port
	options := tlsTest(t, "127.0.0.1", port, nil)
	options["timeout"] = 2

	test, err := makeTest(t, localNode(t), TypeTLSCert, options)
	require.NoError(t, err)

	done := make(chan struct{})
	start := time.Now()
	go func() {
		_, _ = test.Run(formatters.NewMockTestCompleter())
		close(done)
	}()

	select {
	case <-done:
		elapsed := time.Since(start)
		t.Logf("probe returned after %s", elapsed)
		// timeout(2) + 5s suite-side bound, with slack for process teardown
		assert.Less(t, elapsed, 20*time.Second, "probe outran its bound")
	case <-time.After(30 * time.Second):
		t.Fatal("node-side tls_cert probe hung past 30s — the command bound did not fire")
	}
}
