package steptypes

import (
	"fmt"
	"net"
	"slices"
	"strings"
	"time"

	"github.com/bgrewell/dart/internal/config"
	"github.com/bgrewell/dart/internal/formatters"
	"github.com/bgrewell/dart/internal/probe"
	"github.com/bgrewell/dart/pkg/ifaces"
)

var _ ifaces.Step = &DNSRequestStep{}

// DNSRequestStep resolves a hostname and verifies its expected IPs.
// Resolution happens on the step's node by default, using that node's
// resolver and hosts file — which is the answer a suite is usually asking
// for, since the controller's resolver may see a different view entirely.
// from: host resolves using the machine running DART instead.
type DNSRequestStep struct {
	BaseStep
	node        ifaces.Node
	from        Vantage
	hostname    string
	expectedIPs []string
	timeout     time.Duration
}

// newDNSRequestStep parses hostname (required), expected_ips (optional
// list; when present, every listed IP must appear in the answers), timeout
// seconds (default 10), and from (node|host, default node).
func newDNSRequestStep(c *config.StepConfig, node ifaces.Node) (ifaces.Step, error) {
	hostname, err := requiredString(c, "hostname", "hostname is required")
	if err != nil {
		return nil, err
	}
	expectedIPs, _, err := optStringList(c, "expected_ips")
	if err != nil {
		return nil, err
	}
	timeoutSeconds, err := optFloat(c, "timeout", 10)
	if err != nil {
		return nil, err
	}
	if timeoutSeconds <= 0 {
		return nil, optionError(c, "timeout must be positive in step %q", c.Name)
	}
	from, err := parseVantage(c)
	if err != nil {
		return nil, err
	}

	return &DNSRequestStep{
		BaseStep:    baseFor(c),
		node:        node,
		from:        from,
		hostname:    hostname,
		expectedIPs: expectedIPs,
		timeout:     time.Duration(timeoutSeconds * float64(time.Second)),
	}, nil
}

// Run resolves the hostname and checks for expected IPs.
func (s *DNSRequestStep) Run(updater formatters.TaskCompleter) error {
	foundIPs, err := s.resolve()
	if err != nil {
		updater.Error()
		return err
	}

	for _, expectedIP := range s.expectedIPs {
		if !slices.Contains(foundIPs, expectedIP) {
			updater.Error()
			return fmt.Errorf("expected IP %s not found, got %v", expectedIP, foundIPs)
		}
	}

	updater.Complete()
	return nil
}

// resolve returns the addresses for the hostname as seen from the
// configured vantage.
func (s *DNSRequestStep) resolve() ([]string, error) {
	if s.from == VantageNode {
		return s.resolveOnNode()
	}
	return s.resolveOnHost()
}

func (s *DNSRequestStep) resolveOnNode() ([]string, error) {
	result, err := ifaces.ExecuteWithTimeout(s.node, probe.DNSCommand(s.hostname), s.timeout)
	if err != nil {
		return nil, fmt.Errorf("DNS resolution failed: %w", err)
	}
	stdout, err := result.StdoutBytes()
	if err != nil {
		return nil, fmt.Errorf("failed to read resolver output: %w", err)
	}
	stderr, _ := result.StderrBytes()
	if result.ExitCode == probe.MissingToolExitCode {
		return nil, fmt.Errorf("%s", strings.TrimSpace(string(stderr)))
	}

	addresses := probe.ParseAddresses(string(stdout))
	if len(addresses) == 0 {
		// Every supported resolver exits non-zero or prints nothing for an
		// unresolvable name; either way there is no answer
		message := strings.TrimSpace(string(stderr))
		if message == "" {
			message = fmt.Sprintf("%s did not resolve on the node", s.hostname)
		}
		return nil, fmt.Errorf("DNS resolution failed: %s", message)
	}
	return addresses, nil
}

func (s *DNSRequestStep) resolveOnHost() ([]string, error) {
	ips, err := net.LookupIP(s.hostname)
	if err != nil {
		return nil, fmt.Errorf("DNS resolution failed: %w", err)
	}
	addresses := make([]string, 0, len(ips))
	for _, ip := range ips {
		addresses = append(addresses, ip.String())
	}
	return addresses, nil
}
