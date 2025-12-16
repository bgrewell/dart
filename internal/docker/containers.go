package docker

import (
	"bytes"
	"context"
	"fmt"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/api/types/strslice"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
	"io"
)

// ContainerOptions is a function type that sets options for creating a container.
type ContainerOptions func(option *containerOptions)

// containerOptions is a struct that contains options for creating a container.
type containerOptions struct {
	detach       bool
	capabilities []string
	networkMode  string
	priviliged   bool
}

// WithDetach is a function that sets the detach option for creating a container.
func WithDetach(detach bool) ContainerOptions {
	return func(o *containerOptions) {
		o.detach = detach
	}
}

// WithCapabilities is a function that sets the capabilities option for creating a container.
func WithCapabilities(capabilities []string) ContainerOptions {
	return func(o *containerOptions) {
		o.capabilities = capabilities
	}
}

// WithPrivileged is a function that sets the privileged option for creating a container.
func WithPrivileged() ContainerOptions {
	return func(o *containerOptions) {
		o.priviliged = true
	}
}

// WithNetworkMode is a function that sets the network mode option for creating a container.
func WithNetworkMode(networkMode string) ContainerOptions {
	return func(o *containerOptions) {
		o.networkMode = networkMode
	}
}

// ListContainers returns a list of running containers that match the optional filter criteria.
func ListContainers(ctx context.Context, cli client.APIClient, showAll bool, filter *ContainerFilter) ([]types.Container, error) {

	f := filters.NewArgs()
	if filter != nil {
		if filter.Name != "" {
			f.Add("name", filter.Name)
		}
		if filter.Status != "" {
			f.Add("status", filter.Status)
		}
		if filter.Image != "" {
			f.Add("ancestor", filter.Image)
		}
		if filter.Label != "" {
			f.Add("label", filter.Label)
		}
	}

	options := container.ListOptions{
		Size:    false,
		All:     showAll,
		Latest:  false,
		Since:   "",
		Before:  "",
		Limit:   0,
		Filters: f,
	}

	containers, err := cli.ContainerList(ctx, options)
	if err != nil {
		return nil, err
	}

	return containers, nil
}

// CreateContainer creates a new container with the provided configuration.
func CreateContainer(ctx context.Context, cli client.APIClient, config *container.Config, hostConfig *container.HostConfig, networkingConfig *network.NetworkingConfig, name string) (string, error) {
	resp, err := cli.ContainerCreate(ctx, config, hostConfig, networkingConfig, nil, name)
	if err != nil {
		return "", err
	}

	return resp.ID, nil
}

// StartContainer starts a container with the provided ID.
func StartContainer(ctx context.Context, cli client.APIClient, containerID string) error {
	return cli.ContainerStart(ctx, containerID, container.StartOptions{})
}

// StopContainer stops a container with the provided ID.
func StopContainer(ctx context.Context, cli client.APIClient, containerID string) error {
	return cli.ContainerStop(ctx, containerID, container.StopOptions{})
}

// RemoveContainer removes a container with the provided ID.
func RemoveContainer(ctx context.Context, cli client.APIClient, containerID string) error {
	return cli.ContainerRemove(ctx, containerID, container.RemoveOptions{})
}

// RunCommandInContainer runs a command in a specified Docker container
func RunCommandInContainer(cli client.APIClient, containerID string, command string) (exitCode int, stdout, stderr io.Reader, err error) {
	ctx := context.Background()

	// Create an exec instance
	execConfig := types.ExecConfig{
		Cmd:          strslice.StrSlice{"sh", "-c", command},
		AttachStdout: true,
		AttachStderr: true,
	}
	execIDResp, err := cli.ContainerExecCreate(ctx, containerID, execConfig)
	if err != nil {
		return -1, nil, nil, fmt.Errorf("could not create exec instance: %v", err)
	}
	execID := execIDResp.ID

	// Start the exec instance
	resp, err := cli.ContainerExecAttach(ctx, execID, types.ExecStartCheck{})
	if err != nil {
		return -1, nil, nil, fmt.Errorf("could not attach to exec instance: %v", err)
	}
	defer resp.Close()

	// Read the output
	var sout, serr bytes.Buffer
	_, err = stdcopy.StdCopy(&sout, &serr, resp.Reader)
	if err != nil {
		return -1, &sout, &serr, fmt.Errorf("could not copy exec output: %v", err)
	}

	// Inspect the exec instance to get the exit code
	inspectResp, err := cli.ContainerExecInspect(ctx, execID)
	if err != nil {
		return -1, &sout, &serr, fmt.Errorf("could not inspect exec instance: %v", err)
	}

	return inspectResp.ExitCode, &sout, &serr, nil
}

func ContainerLogs(ctx context.Context, cli client.APIClient, containerID string) (io.ReadCloser, error) {
	return cli.ContainerLogs(ctx, containerID, container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Follow:     true,
	})
}

// ContainerReadinessConfig holds configuration for waiting on container readiness
type ContainerReadinessConfig struct {
	// Timeout is the maximum time to wait for the container to become ready
	Timeout time.Duration
	// PollInterval is how often to check the container state
	PollInterval time.Duration
}

// DefaultContainerReadinessConfig returns sensible defaults for readiness checking
func DefaultContainerReadinessConfig() *ContainerReadinessConfig {
	return &ContainerReadinessConfig{
		Timeout:      2 * time.Minute,
		PollInterval: 1 * time.Second,
	}
}

// WaitForContainerReady waits for a container to be fully ready to accept commands.
// This checks that:
// 1. The container state is "running"
// 2. A simple command can be executed successfully (indicating the container is responsive)
func WaitForContainerReady(ctx context.Context, cli client.APIClient, containerID string, config *ContainerReadinessConfig) error {
	if config == nil {
		config = DefaultContainerReadinessConfig()
	}

	// Create a context with timeout
	ctx, cancel := context.WithTimeout(ctx, config.Timeout)
	defer cancel()

	ticker := time.NewTicker(config.PollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("timeout waiting for container %s to become ready: %w", containerID, ctx.Err())
		case <-ticker.C:
			ready, err := isContainerReady(ctx, cli, containerID)
			if err != nil {
				// Log but continue - the container may still be initializing
				continue
			}
			if ready {
				return nil
			}
		}
	}
}

// isContainerReady checks if a container is fully ready to accept commands
func isContainerReady(ctx context.Context, cli client.APIClient, containerID string) (bool, error) {
	// Check container state
	inspect, err := cli.ContainerInspect(ctx, containerID)
	if err != nil {
		return false, fmt.Errorf("failed to inspect container: %w", err)
	}

	// Container must be running
	if inspect.State == nil || !inspect.State.Running {
		return false, nil
	}

	// Try to execute a simple command to verify the container is responsive
	exitCode, _, _, err := RunCommandInContainer(cli, containerID, "true")
	if err != nil {
		return false, nil // Container not ready yet
	}

	return exitCode == 0, nil
}
