package docker

import (
	"context"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
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
