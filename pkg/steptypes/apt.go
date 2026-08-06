package steptypes

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/bgrewell/dart/internal/config"
	"github.com/bgrewell/dart/internal/formatters"
	"github.com/bgrewell/dart/pkg/ifaces"
)

var _ ifaces.Step = &AptStep{}

// AptStep installs packages using APT.
type AptStep struct {
	BaseStep
	node     ifaces.Node
	packages []string
}

func newAptStep(c *config.StepConfig, node ifaces.Node) (ifaces.Step, error) {
	packages, present, err := optStringList(c, "packages")
	if err != nil {
		return nil, err
	}
	if !present || len(packages) == 0 {
		return nil, optionError(c, "packages field is required in step %q", c.Name)
	}

	return &AptStep{
		BaseStep: baseFor(c),
		node:     node,
		packages: packages,
	}, nil
}

// Run installs required packages using APT.
func (s *AptStep) Run(updater formatters.TaskCompleter) error {
	if s.AptUpdateNeeded() {
		result, err := s.node.Execute("sudo -n apt-get update")
		if err != nil {
			updater.Error()
			return err
		}
		if result.ExitCode != 0 {
			updater.Error()
			errorDetails, _ := result.StderrBytes()
			return fmt.Errorf("apt-get update failed: %s", errorDetails)
		}
	}

	command := fmt.Sprintf("sudo -n apt-get install -y %s", strings.Join(s.packages, " "))
	result, err := s.node.Execute(command)
	if err != nil {
		updater.Error()
		return err
	}
	if result.ExitCode != 0 {
		updater.Error()
		errorDetails, _ := result.StderrBytes()
		return fmt.Errorf("apt-get install failed: %s", errorDetails)
	}
	updater.Complete()
	return nil
}

// AptUpdateNeeded reports whether the package index is older than 24 hours,
// based on the epoch mtime of apt's update-success stamp. Any failure to
// read the stamp counts as stale.
func (s *AptStep) AptUpdateNeeded() bool {
	const stampPath = "/var/lib/apt/periodic/update-success-stamp"

	result, err := s.node.Execute(fmt.Sprintf("stat -c %%Y %s", stampPath))
	if err != nil || result.ExitCode != 0 {
		return true
	}

	output, err := result.StdoutBytes()
	if err != nil {
		return true
	}

	epoch, err := strconv.ParseInt(strings.TrimSpace(string(output)), 10, 64)
	if err != nil {
		return true
	}

	return time.Since(time.Unix(epoch, 0)) > 24*time.Hour
}
