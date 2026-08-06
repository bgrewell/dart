package steptypes

import (
	"fmt"
	"strings"

	"github.com/bgrewell/dart/internal/config"
	"github.com/bgrewell/dart/internal/formatters"
	"github.com/bgrewell/dart/pkg/ifaces"
)

var _ ifaces.Step = &ServiceCheckStep{}

// ServiceCheckStep verifies that a systemd service is active on the step's
// target node.
type ServiceCheckStep struct {
	BaseStep
	node    ifaces.Node
	service string
}

func newServiceCheckStep(c *config.StepConfig, node ifaces.Node) (ifaces.Step, error) {
	service, err := requiredString(c, "service", "service is required")
	if err != nil {
		return nil, err
	}

	return &ServiceCheckStep{
		BaseStep: baseFor(c),
		node:     node,
		service:  service,
	}, nil
}

// Run checks if the specified service is active.
func (s *ServiceCheckStep) Run(updater formatters.TaskCompleter) error {
	cmd := fmt.Sprintf("systemctl is-active %s", shellQuote(s.service))
	result, err := s.node.Execute(cmd)
	if err != nil {
		updater.Error()
		return fmt.Errorf("failed to check service: %w", err)
	}

	output, err := result.StdoutBytes()
	if err != nil {
		updater.Error()
		return fmt.Errorf("failed to read service check output: %w", err)
	}

	// systemctl is-active exits non-zero for inactive states; the trimmed
	// output ("inactive", "failed", ...) is the useful detail either way
	status := strings.TrimSpace(string(output))
	if status != "active" {
		updater.Error()
		return fmt.Errorf("service %s is not active (status: %s)", s.service, status)
	}

	updater.Complete()
	return nil
}
