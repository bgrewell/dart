package docker

import (
	"context"
	"fmt"
	"github.com/bgrewell/go-execute/v2"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/client"
	"io"
)

// BuildImage builds an image from a Dockerfile.
func BuildImage(ctx context.Context, cli *client.Client, dockerfile, imageName string) error {
	cmd := fmt.Sprintf("docker build -t %s -f %s .", imageName, dockerfile)
	_, err := execute.Execute(cmd)
	if err != nil {
		return err
	}
	return nil
}

// PullImage pulls an image from a registry.
func PullImage(ctx context.Context, cli client.APIClient, imageName string) error {
	reader, err := cli.ImagePull(ctx, imageName, image.PullOptions{})
	if err != nil {
		return err
	}
	defer reader.Close()

	// ImagePull is asynchronous: the stream must be drained for the pull to
	// finish. The progress output is discarded rather than printed, because
	// node setup renders through a spinner that raw daemon output would
	// scribble over.
	if _, err := io.Copy(io.Discard, reader); err != nil {
		return fmt.Errorf("pulling %s: %w", imageName, err)
	}
	return nil
}

// ImageExists reports whether the daemon already holds the image.
func ImageExists(ctx context.Context, cli client.APIClient, imageName string) (bool, error) {
	_, err := cli.ImageInspect(ctx, imageName)
	if err == nil {
		return true, nil
	}
	if IsNotFound(err) {
		return false, nil
	}
	return false, err
}

// ListImages returns a list of images on the Docker host.
func ListImages(ctx context.Context, cli *client.Client) ([]image.Summary, error) {
	images, err := cli.ImageList(ctx, image.ListOptions{
		All: true,
	})
	if err != nil {
		return nil, err
	}

	return images, nil
}

func RemoveImage(ctx context.Context, cli client.APIClient, imageName string) error {
	_, err := cli.ImageRemove(ctx, imageName, image.RemoveOptions{})
	if err != nil {
		return err
	}

	return nil
}
