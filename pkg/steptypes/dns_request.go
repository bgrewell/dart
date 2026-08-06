package steptypes

import (
	"fmt"
	"net"
	"slices"

	"github.com/bgrewell/dart/internal/config"
	"github.com/bgrewell/dart/internal/formatters"
	"github.com/bgrewell/dart/pkg/ifaces"
)

var _ ifaces.Step = &DNSRequestStep{}

// DNSRequestStep resolves a hostname and verifies its expected IPs.
// Note: resolution happens on the host running DART, using its resolver —
// not on the step's node.
type DNSRequestStep struct {
	BaseStep
	hostname    string
	expectedIPs []string
}

// newDNSRequestStep parses hostname (required) and expected_ips (optional
// list; when present, every listed IP must appear in the answers).
func newDNSRequestStep(c *config.StepConfig, _ ifaces.Node) (ifaces.Step, error) {
	hostname, err := requiredString(c, "hostname", "hostname is required")
	if err != nil {
		return nil, err
	}
	expectedIPs, _, err := optStringList(c, "expected_ips")
	if err != nil {
		return nil, err
	}

	return &DNSRequestStep{
		BaseStep:    baseFor(c),
		hostname:    hostname,
		expectedIPs: expectedIPs,
	}, nil
}

// Run resolves the hostname and checks for expected IPs.
func (s *DNSRequestStep) Run(updater formatters.TaskCompleter) error {
	ips, err := net.LookupIP(s.hostname)
	if err != nil {
		updater.Error()
		return fmt.Errorf("DNS resolution failed: %w", err)
	}

	foundIPs := make([]string, 0, len(ips))
	for _, ip := range ips {
		foundIPs = append(foundIPs, ip.String())
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
