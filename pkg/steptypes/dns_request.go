package steptypes

import (
	"fmt"
	"github.com/bgrewell/dart/internal/formatters"
	"github.com/bgrewell/dart/pkg/ifaces"
	"net"
)

var _ ifaces.Step = &DNSRequestStep{}

// DNSRequestStep resolves a hostname and verifies its expected IP.
type DNSRequestStep struct {
	BaseStep
	hostname    string
	expectedIPs []string
}

// Run resolves the hostname and checks for expected IPs.
func (s *DNSRequestStep) Run(updater formatters.TaskCompleter) error {
	ips, err := net.LookupIP(s.hostname)
	if err != nil {
		updater.Error()
		return fmt.Errorf("DNS resolution failed: %w", err)
	}

	foundIPs := []string{}
	for _, ip := range ips {
		foundIPs = append(foundIPs, ip.String())
	}

	if len(s.expectedIPs) > 0 {
		for _, expectedIP := range s.expectedIPs {
			if !contains(foundIPs, expectedIP) {
				updater.Error()
				return fmt.Errorf("expected IP %s not found, got %v", expectedIP, foundIPs)
			}
		}
	}

	updater.Complete()
	return nil
}

// contains checks if a slice contains a specific string.
func contains(slice []string, value string) bool {
	for _, item := range slice {
		if item == value {
			return true
		}
	}
	return false
}
