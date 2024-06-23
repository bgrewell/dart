package docker

import (
	"context"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
)

// CreateNetwork creates a new container network
func CreateNetwork(ctx context.Context, cli client.APIClient, name string) (string, error) {
	resp, err := cli.NetworkCreate(ctx, name, network.CreateOptions{})
	if err != nil {
		return "", err
	}

	return resp.ID, nil
}

// RemoveNetwork removes a container network
func RemoveNetwork(ctx context.Context, cli client.APIClient, id string) error {
	err := cli.NetworkRemove(ctx, id)
	if err != nil {
		return err
	}

	return nil
}

// AttachNetwork attaches a container to a network
func AttachNetwork(ctx context.Context, cli client.APIClient, networkID string, containerID string) error {
	err := cli.NetworkConnect(ctx, networkID, containerID, nil)
	if err != nil {
		return err
	}

	return nil
}

// DetachNetwork detaches a container from a network
func DetachNetwork(ctx context.Context, cli client.APIClient, networkID string, containerID string) error {
	err := cli.NetworkDisconnect(ctx, networkID, containerID, false)
	if err != nil {
		return err
	}

	return nil
}
