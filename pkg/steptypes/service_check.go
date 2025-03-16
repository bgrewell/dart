package steptypes

import (
	"fmt"
	"io"
	"strings"

	"github.com/bgrewell/dart/internal/formatters"
	"github.com/bgrewell/dart/pkg/ifaces"
)

var _ ifaces.Step = &ServiceCheckStep{}

// ServiceCheckStep verifies if a system service is active.
type ServiceCheckStep struct {
	BaseStep
	node    ifaces.Node
	service string
}

// Run checks if the specified service is active.
func (s *ServiceCheckStep) Run(updater formatters.TaskCompleter) error {
	cmd := fmt.Sprintf("systemctl is-active %s", s.service)
	result, err := s.node.Execute(cmd)
	if err != nil {
		updater.Error()
		return fmt.Errorf("failed to check service: %w", err)
	}

	// Read the command output
	output, err := io.ReadAll(result.Stdout)
	if err != nil {
		updater.Error()
		return fmt.Errorf("failed to read service check output: %w", err)
	}

	// Normalize and trim the output for comparison
	status := strings.TrimSpace(string(output))
	if status != "active" {
		updater.Error()
		return fmt.Errorf("service %s is not active (status: %s)", s.service, status)
	}

	updater.Complete()
	return nil
}
