package docker

import (
	"context"
	"fmt"
	"github.com/bgrewell/go-execute/v2"
	"github.com/docker/docker/client"
	"io"
	"path/filepath"
	"strings"
)

// ComposeStack represents a Docker Compose stack
type ComposeStack struct {
	Name         string
	ComposeFile  string
	ProjectName  string
	cli          client.APIClient
	containerIDs map[string]string // service name -> container ID
}

// NewComposeStack creates a new compose stack instance
func NewComposeStack(cli client.APIClient, name, composeFile, projectName string) *ComposeStack {
	if projectName == "" {
		projectName = name
	}
	return &ComposeStack{
		Name:         name,
		ComposeFile:  composeFile,
		ProjectName:  projectName,
		cli:          cli,
		containerIDs: make(map[string]string),
	}
}

// Up starts the compose stack
func (cs *ComposeStack) Up() error {
	dir := filepath.Dir(cs.ComposeFile)
	file := filepath.Base(cs.ComposeFile)

	executor := execute.NewExecutor(
		execute.WithDefaultShell(),
		execute.WithWorkingDir(dir),
	)

	cmd := fmt.Sprintf("docker compose -f %s -p %s up -d", file, cs.ProjectName)
	_, eout, err := executor.ExecuteSeparate(cmd)
	if err != nil {
		if eout != "" {
			return fmt.Errorf("could not start compose stack: %v (%s)", err, eout)
		}
		return fmt.Errorf("could not start compose stack: %v", err)
	}

	// Discover container IDs for services
	if err := cs.discoverContainers(); err != nil {
		return fmt.Errorf("could not discover containers: %v", err)
	}

	return nil
}

// Down stops and removes the compose stack
func (cs *ComposeStack) Down() error {
	dir := filepath.Dir(cs.ComposeFile)
	file := filepath.Base(cs.ComposeFile)

	executor := execute.NewExecutor(
		execute.WithDefaultShell(),
		execute.WithWorkingDir(dir),
	)

	cmd := fmt.Sprintf("docker compose -f %s -p %s down", file, cs.ProjectName)
	_, eout, err := executor.ExecuteSeparate(cmd)
	if err != nil {
		if eout != "" {
			return fmt.Errorf("could not stop compose stack: %v (%s)", err, eout)
		}
		return fmt.Errorf("could not stop compose stack: %v", err)
	}

	cs.containerIDs = make(map[string]string)
	return nil
}

// discoverContainers finds all containers in the compose stack
func (cs *ComposeStack) discoverContainers() error {
	ctx := context.Background()

	// List containers with the project label
	filter := &ContainerFilter{
		Label: fmt.Sprintf("com.docker.compose.project=%s", cs.ProjectName),
	}

	containers, err := ListContainers(ctx, cs.cli, false, filter)
	if err != nil {
		return err
	}

	// Map service names to container IDs
	for _, container := range containers {
		// Extract service name from labels
		if serviceName, ok := container.Labels["com.docker.compose.service"]; ok {
			cs.containerIDs[serviceName] = container.ID
		}
	}

	return nil
}

// GetServiceContainerID returns the container ID for a given service
func (cs *ComposeStack) GetServiceContainerID(service string) (string, error) {
	if id, ok := cs.containerIDs[service]; ok {
		return id, nil
	}
	return "", fmt.Errorf("service '%s' not found in compose stack", service)
}

// ExecInService executes a command in a specific service container
func (cs *ComposeStack) ExecInService(service, command string) (exitCode int, stdout, stderr string, err error) {
	containerID, err := cs.GetServiceContainerID(service)
	if err != nil {
		return -1, "", "", err
	}

	code, stdoutReader, stderrReader, err := RunCommandInContainer(cs.cli, containerID, command)
	if err != nil {
		return -1, "", "", err
	}

	// Read stdout and stderr to strings
	stdoutBuf := new(strings.Builder)
	stderrBuf := new(strings.Builder)

	if _, err := io.Copy(stdoutBuf, stdoutReader); err != nil {
		return code, "", "", fmt.Errorf("could not read stdout: %v", err)
	}
	if _, err := io.Copy(stderrBuf, stderrReader); err != nil {
		return code, stdoutBuf.String(), "", fmt.Errorf("could not read stderr: %v", err)
	}

	return code, stdoutBuf.String(), stderrBuf.String(), nil
}

// ListServices returns the names of all services in the stack
func (cs *ComposeStack) ListServices() []string {
	services := make([]string, 0, len(cs.containerIDs))
	for service := range cs.containerIDs {
		services = append(services, service)
	}
	return services
}
