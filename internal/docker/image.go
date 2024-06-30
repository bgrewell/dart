package docker

import (
	"context"
	"fmt"
	"github.com/bgrewell/go-execute/v2"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/client"
	"io"
	"os"
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
func PullImage(ctx context.Context, cli *client.Client, imageName string) error {
	reader, err := cli.ImagePull(ctx, imageName, image.PullOptions{})
	if err != nil {
		return err
	}

	defer reader.Close()
	// cli.ImagePull is asynchronous.
	// The reader needs to be read completely for the pull operation to complete.
	// If stdout is not required, consider using io.Discard instead of os.Stdout.
	io.Copy(os.Stdout, reader)

	return nil
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

func RemoveImage(ctx context.Context, cli *client.Client, imageName string) error {
	_, err := cli.ImageRemove(ctx, imageName, image.RemoveOptions{})
	if err != nil {
		return err
	}

	return nil
}
