package main

import (
	"context"
	"github.com/bgrewell/dart/internal/docker"
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

	// Build Docker image
	//dockerfilePath := "/home/ben/repos/dart/examples/docker/dockerfiles/client.dockerfile"
	//imageName := "test_image"
	//err = docker.BuildImage(ctx, cli, dockerfilePath, imageName)
	//if err != nil {
	//	log.Fatalf("Could not build image: %v", err)
	//}
	//fmt.Println("Image built successfully")
}
