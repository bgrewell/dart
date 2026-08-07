package testtypes

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/bgrewell/dart/internal/eval"
	"github.com/bgrewell/dart/internal/execution"
	"github.com/bgrewell/dart/internal/formatters"
	"github.com/bgrewell/dart/internal/results"
	"github.com/bgrewell/dart/pkg/ifaces"
)

var _ ifaces.Test = &TLSCertTest{}

// TLSCertTest connects to a TLS endpoint and inspects its leaf
// certificate: expiry window, name coverage, issuer. The handshake skips
// chain verification so expired or misissued certificates can still be
// inspected and asserted on. Note: the connection is made from the host
// running DART, not from the test's node.
type TLSCertTest struct {
	BaseTest
	host       string
	port       int
	serverName string
	timeout    time.Duration
}

// certFacts is the leaf-certificate summary the checks evaluate; it is
// also emitted as JSON on the result's stdout, so json_path and the
// standard evaluators work on it.
type certFacts struct {
	Subject       string   `json:"subject"`
	Issuer        string   `json:"issuer"`
	DNSNames      []string `json:"dns_names"`
	IPAddresses   []string `json:"ip_addresses"`
	NotBefore     string   `json:"not_before"`
	NotAfter      string   `json:"not_after"`
	DaysRemaining float64  `json:"days_remaining"`
	ChainValid    bool     `json:"chain_valid"`
}

// newTLSCertTest parses host (required), port (default 443), server_name
// (SNI, defaults to host), timeout seconds (default 10). Evaluate keys:
// min_days_remaining (number), dns_names (list — every name must be
// covered by the certificate), issuer_contains / subject_contains
// (strings), chain_valid (bool, verification against system roots);
// others fall through to the standard evaluators against the JSON facts.
// With no evaluate block, min_days_remaining: 0 (not expired) is checked.
func newTLSCertTest(base BaseTest, opts map[string]interface{}) (ifaces.Test, error) {
	host, err := requiredString(base.name, opts, "host")
	if err != nil {
		return nil, err
	}
	port, err := optInt(base.name, opts, "port", 443)
	if err != nil {
		return nil, err
	}
	if port < 1 || port > 65535 {
		return nil, fmt.Errorf("port must be between 1 and 65535 in test %q", base.name)
	}
	serverName, _, err := optString(base.name, opts, "server_name")
	if err != nil {
		return nil, err
	}
	if serverName == "" {
		serverName = host
	}
	timeoutSeconds, err := optFloat(base.name, opts, "timeout", 10)
	if err != nil {
		return nil, err
	}
	if timeoutSeconds <= 0 {
		return nil, fmt.Errorf("timeout must be positive in test %q", base.name)
	}

	spec, err := evaluateSpec(base.name, opts)
	if err != nil {
		return nil, err
	}

	evaluations := make(map[string]eval.Evaluate, len(spec))
	for name, value := range spec {
		switch name {
		case "min_days_remaining":
			minDays, ok := coerceFloat(value)
			if !ok {
				return nil, fmt.Errorf("min_days_remaining must be a number in test %q (got %v)", base.name, value)
			}
			evaluations[name] = &certDaysCheck{minDays: minDays}
		case "dns_names":
			names, ok := toStringList(value)
			if !ok || len(names) == 0 {
				return nil, fmt.Errorf("dns_names must be a non-empty list of strings in test %q", base.name)
			}
			evaluations[name] = &certNamesCheck{names: names}
		case "issuer_contains":
			text, ok := value.(string)
			if !ok {
				return nil, fmt.Errorf("issuer_contains must be a string in test %q", base.name)
			}
			evaluations[name] = &certFieldCheck{field: "issuer", contains: text}
		case "subject_contains":
			text, ok := value.(string)
			if !ok {
				return nil, fmt.Errorf("subject_contains must be a string in test %q", base.name)
			}
			evaluations[name] = &certFieldCheck{field: "subject", contains: text}
		case "chain_valid":
			expected, ok := value.(bool)
			if !ok {
				return nil, fmt.Errorf("chain_valid must be a boolean in test %q", base.name)
			}
			evaluations[name] = &certChainCheck{expected: expected}
		default:
			evaluator, err := eval.New(name, value)
			if err != nil {
				return nil, err
			}
			evaluations[name] = evaluator
		}
	}
	if len(evaluations) == 0 {
		evaluations["min_days_remaining"] = &certDaysCheck{minDays: 0}
	}

	base.evaluations = evaluations
	return &TLSCertTest{
		BaseTest:   base,
		host:       host,
		port:       port,
		serverName: serverName,
		timeout:    time.Duration(timeoutSeconds * float64(time.Second)),
	}, nil
}

func toStringList(v interface{}) ([]string, bool) {
	list, ok := v.([]interface{})
	if !ok {
		return nil, false
	}
	names := make([]string, len(list))
	for i, item := range list {
		s, ok := item.(string)
		if !ok {
			return nil, false
		}
		names[i] = s
	}
	return names, true
}

func (t *TLSCertTest) Run(updater formatters.TestCompleter) (map[string]*eval.EvaluateResult, error) {
	return t.runProducer(t.inspect, updater)
}

// inspect performs the handshake and shapes the leaf certificate's facts
// as an execution result: JSON on stdout, exit code 0.
func (t *TLSCertTest) inspect() (*execution.ExecutionResult, error) {
	address := net.JoinHostPort(t.host, strconv.Itoa(t.port))
	dialer := &net.Dialer{Timeout: t.timeout}
	conn, err := tls.DialWithDialer(dialer, "tcp", address, &tls.Config{
		ServerName:         t.serverName,
		InsecureSkipVerify: true, // inspection must work on invalid certs; chain_valid asserts validity explicitly
	})
	if err != nil {
		return nil, fmt.Errorf("tls handshake with %s failed: %w", address, err)
	}
	defer conn.Close()

	certs := conn.ConnectionState().PeerCertificates
	if len(certs) == 0 {
		return nil, fmt.Errorf("no peer certificates from %s", address)
	}
	leaf := certs[0]

	intermediates := x509.NewCertPool()
	for _, cert := range certs[1:] {
		intermediates.AddCert(cert)
	}
	_, verifyErr := leaf.Verify(x509.VerifyOptions{
		DNSName:       t.serverName,
		Intermediates: intermediates,
	})

	ips := make([]string, 0, len(leaf.IPAddresses))
	for _, ip := range leaf.IPAddresses {
		ips = append(ips, ip.String())
	}

	facts := certFacts{
		Subject:       leaf.Subject.String(),
		Issuer:        leaf.Issuer.String(),
		DNSNames:      leaf.DNSNames,
		IPAddresses:   ips,
		NotBefore:     leaf.NotBefore.UTC().Format(time.RFC3339),
		NotAfter:      leaf.NotAfter.UTC().Format(time.RFC3339),
		DaysRemaining: time.Until(leaf.NotAfter).Hours() / 24,
		ChainValid:    verifyErr == nil,
	}
	payload, err := json.Marshal(facts)
	if err != nil {
		return nil, err
	}

	return &execution.ExecutionResult{
		ExitCode: 0,
		Stdout:   strings.NewReader(string(payload)),
		Stderr:   strings.NewReader(""),
	}, nil
}

// parseCertFacts reloads the JSON facts from a result's stdout.
func parseCertFacts(execResult *execution.ExecutionResult) (*certFacts, *eval.EvaluateResult) {
	stdout, err := execResult.StdoutBytes()
	if err != nil {
		return nil, &eval.EvaluateResult{Passed: false, Err: err}
	}
	var facts certFacts
	if err := json.Unmarshal(stdout, &facts); err != nil {
		return nil, &eval.EvaluateResult{Passed: false, Err: err}
	}
	return &facts, nil
}

type certDaysCheck struct {
	minDays float64
}

func (c *certDaysCheck) Verify(execResult *execution.ExecutionResult) *eval.EvaluateResult {
	facts, fail := parseCertFacts(execResult)
	if fail != nil {
		return fail
	}
	passed := facts.DaysRemaining >= c.minDays
	var details interface{} = fmt.Sprintf("%.1f days remaining (expires %s)", facts.DaysRemaining, facts.NotAfter)
	if !passed {
		details = &results.ResultStringMatchFail{
			Expected: fmt.Sprintf(">= %.0f days remaining", c.minDays),
			Actual:   fmt.Sprintf("%.1f days (expires %s)", facts.DaysRemaining, facts.NotAfter),
		}
	}
	return &eval.EvaluateResult{Passed: passed, Details: details}
}

type certNamesCheck struct {
	names []string
}

func (c *certNamesCheck) Verify(execResult *execution.ExecutionResult) *eval.EvaluateResult {
	facts, fail := parseCertFacts(execResult)
	if fail != nil {
		return fail
	}
	// Reconstruct with IP SANs too: internal-service certificates commonly
	// cover an IP rather than a name, and dropping them here would fail
	// names a real client verifies successfully
	cert := &x509.Certificate{DNSNames: facts.DNSNames}
	for _, ip := range facts.IPAddresses {
		if parsed := net.ParseIP(ip); parsed != nil {
			cert.IPAddresses = append(cert.IPAddresses, parsed)
		}
	}
	var missing []string
	for _, name := range c.names {
		if cert.VerifyHostname(name) != nil {
			missing = append(missing, name)
		}
	}
	passed := len(missing) == 0
	var details interface{} = fmt.Sprintf("covers %v", c.names)
	if !passed {
		details = &results.ResultStringMatchFail{
			Expected: fmt.Sprintf("certificate covering %v", missing),
			Actual:   fmt.Sprintf("dns_names %v, ip_addresses %v", facts.DNSNames, facts.IPAddresses),
		}
	}
	return &eval.EvaluateResult{Passed: passed, Details: details}
}

type certFieldCheck struct {
	field    string
	contains string
}

func (c *certFieldCheck) Verify(execResult *execution.ExecutionResult) *eval.EvaluateResult {
	facts, fail := parseCertFacts(execResult)
	if fail != nil {
		return fail
	}
	actual := facts.Issuer
	if c.field == "subject" {
		actual = facts.Subject
	}
	passed := strings.Contains(actual, c.contains)
	var details interface{} = actual
	if !passed {
		details = &results.ResultStringMatchFail{
			Expected: fmt.Sprintf("%s containing %q", c.field, c.contains),
			Actual:   actual,
		}
	}
	return &eval.EvaluateResult{Passed: passed, Details: details}
}

type certChainCheck struct {
	expected bool
}

func (c *certChainCheck) Verify(execResult *execution.ExecutionResult) *eval.EvaluateResult {
	facts, fail := parseCertFacts(execResult)
	if fail != nil {
		return fail
	}
	passed := facts.ChainValid == c.expected
	var details interface{} = fmt.Sprintf("chain_valid=%v", facts.ChainValid)
	if !passed {
		details = &results.ResultStringMatchFail{
			Expected: fmt.Sprintf("chain_valid=%v", c.expected),
			Actual:   fmt.Sprintf("chain_valid=%v", facts.ChainValid),
		}
	}
	return &eval.EvaluateResult{Passed: passed, Details: details}
}
