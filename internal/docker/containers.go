package docker

import (
	"context"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	"io"
)

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

func ContainerLogs(ctx context.Context, cli client.APIClient, containerID string) (io.ReadCloser, error) {
	return cli.ContainerLogs(ctx, containerID, container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Follow:     true,
	})
}
