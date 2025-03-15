package internal

import (
	"fmt"
	"github.com/bgrewell/dart/internal/formatters"
	"io"
	"strings"
	"time"
)

type AptStep struct {
	title    string
	node     Node
	packages []string
}

func (s *AptStep) Title() string {
	return s.title
}

func (s *AptStep) Run(updater formatters.TaskCompleter) error {

	// Test to see if the user can run sudo commands without a password

	// Check to see if /var/lib/apt/periodic/update-success-stamp exists and if so what the timestamp is. If it is older
	// than 24 hours or doesn't exist, run apt-get update.
	if s.AptUpdateNeeded() {
		result, err := s.node.Execute("sudo -n apt-get update")
		if err != nil {
			updater.Error()
			return err
		}
		if result.ExitCode != 0 {
			updater.Error()
			errorDetails, _ := io.ReadAll(result.Stderr)
			return fmt.Errorf("apt-get update failed with exit code %d: %s (hint: try running `sudo true` to cache credentials before running this)", result.ExitCode, errorDetails)
		}
	}

	// Build a command to install the list of packages
	packages := strings.Join(s.packages, " ")
	command := fmt.Sprintf("sudo -n apt-get install -y %s", packages)

	result, err := s.node.Execute(command)
	if err != nil {
		updater.Error()
		return err
	}
	if result.ExitCode != 0 {
		updater.Error()
		errorDetails, _ := io.ReadAll(result.Stderr)
		return fmt.Errorf("command failed with exit code %d: %s", result.ExitCode, errorDetails)
	}
	updater.Complete()
	return nil
}

// AptUpdateNeeded checks if apt update is needed on the node
func (s *AptStep) AptUpdateNeeded() bool {
	const filePath = "/var/lib/apt/periodic/update-success-stamp"

	// Check if the file exists
	result, err := s.node.Execute(fmt.Sprintf("stat %s", filePath))
	if err != nil {
		return true
	}
	if result.ExitCode != 0 {
		return true
	}

	// Parse the output to get the modification time
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

	// Check if the last modified time is older than 24 hours
	if time.Since(modTime) > 24*time.Hour {
		return true
	}

	return false
}
