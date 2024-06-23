package main

import (
	"context"
	"fmt"
	"github.com/bgrewell/dart/internal/docker"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"log"
)

func main() {
	ctx := context.Background()

	// Create a Docker client
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		log.Fatalf("Could not create Docker client: %v", err)
	}

	// Pull an image
	err = docker.PullImage(ctx, cli, "alpine")
	if err != nil {
		log.Fatalf("Could not pull image: %v", err)
	}

	// List images
	images, err := docker.ListImages(ctx, cli)
	if err != nil {
		log.Fatalf("Could not list images: %v", err)
	}

	for _, image := range images {
		log.Printf("Image: %s\n", image.RepoTags)
	}

	// Create a container
	cc := &container.Config{
		Image: "alpine",
		Cmd:   []string{"tail", "-f", "/dev/null"},
	}
	id, err := docker.CreateContainer(ctx, cli, cc, nil, nil, "test_container")
	if err != nil {
		log.Fatalf("Could not create container: %v", err)
	}
	fmt.Println(id)

	// Start a container
	err = docker.StartContainer(ctx, cli, id)
	if err != nil {
		log.Fatalf("Could not start container: %v", err)
	}

	// Remove a container
	//err = cli.ContainerRemove(ctx, id, container.RemoveOptions{})
	//if err != nil {
	//	log.Fatalf("Could not remove container: %v", err)
	//}

	// Build Docker image
	//dockerfilePath := "/home/ben/repos/dart/examples/docker/dockerfiles/client.dockerfile"
	//imageName := "test_image"
	//err = docker.BuildImage(ctx, cli, dockerfilePath, imageName)
	//if err != nil {
	//	log.Fatalf("Could not build image: %v", err)
	//}
	//fmt.Println("Image built successfully")
}
