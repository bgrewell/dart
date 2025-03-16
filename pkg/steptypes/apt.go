package steptypes

import (
	"fmt"
	"github.com/bgrewell/dart/internal/formatters"
	"github.com/bgrewell/dart/pkg/ifaces"
	"io"
	"strings"
	"time"
)

var _ ifaces.Step = &AptStep{}

// AptStep installs packages using APT.
type AptStep struct {
	BaseStep
	node     ifaces.Node
	packages []string
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
			errorDetails, _ := io.ReadAll(result.Stderr)
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
		errorDetails, _ := io.ReadAll(result.Stderr)
		return fmt.Errorf("apt-get install failed: %s", errorDetails)
	}
	updater.Complete()
	return nil
}

// AptUpdateNeeded checks if apt-get update is necessary.
func (s *AptStep) AptUpdateNeeded() bool {
	const filePath = "/var/lib/apt/periodic/update-success-stamp"

	result, err := s.node.Execute(fmt.Sprintf("stat %s", filePath))
	if err != nil || result.ExitCode != 0 {
		return true
	}

	output, err := io.ReadAll(result.Stdout)
	if err != nil {
		return true
	}
	lines := strings.Split(string(output), "\n")
	var modTime time.Time
	for _, line := range lines {
		if strings.HasPrefix(line, "Modify:") {
			parts := strings.Split(line, " ")
			if len(parts) >= 2 {
				modTime, err = time.Parse("2006-01-02 15:04:05", parts[1])
				if err != nil {
					return true
				}
			}
		}
	}

	return time.Since(modTime) > 24*time.Hour
}
