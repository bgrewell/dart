package docker

import (
	"context"
	"fmt"
	"github.com/bgrewell/dart/internal/config"
	"github.com/bgrewell/dart/internal/helpers"
	"github.com/bgrewell/go-execute/v2"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	"io"
	"path/filepath"
)

func NewWrapper(cfg *config.Configuration) (wrapper *Wrapper, err error) {

	// Create a Docker client
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, fmt.Errorf("Could not create Docker client: %v", err)
	}

	// Return a wrapper instance
	return &Wrapper{
		cli:                cli,
		cfg:                cfg.Docker,
		networkNamesToId:   make(map[string]string),
		containerNamesToId: make(map[string]string),
	}, nil
}

type Wrapper struct {
	cli                *client.Client
	cfg                *config.DockerConfig
	networkNamesToId   map[string]string
	containerNamesToId map[string]string
}

// Configured returns true if the wrapper has been configured
func (w *Wrapper) Configured() bool {
	return w.cfg != nil
}

// Setup configures the Docker wrapper by creating networks and building images
func (w *Wrapper) Setup() error {
	// Create the networks
	for _, net := range w.cfg.Networks {
		if err := w.CreateNetwork(net.Name, net.Subnet, net.Gateway); err != nil {
			return err
		}

	}

	// Build the images
	for _, image := range w.cfg.Images {
		if err := w.BuildImage(image.Name, image.Tag, image.Dockerfile); err != nil {
			return err
		}
	}

	return nil
}

// Teardown removes the networks and images created by the wrapper
func (w *Wrapper) Teardown() error {
	// Remove the networks
	for _, net := range w.cfg.Networks {
		if err := w.RemoveNetwork(net.Name); err != nil {
			return err
		}
	}

	// Remove the images
	for _, image := range w.cfg.Images {
		if err := w.RemoveImage(image.Name); err != nil {
			return err
		}
	}

	return nil
}

func (w *Wrapper) BuildImage(name string, tag string, dockerFilePath string) error {

	dir, filename := filepath.Split(dockerFilePath)

	executor := execute.NewExecutor(
		execute.WithDefaultShell(),
		execute.WithWorkingDir(dir),
	)
	cmd := fmt.Sprintf("docker build -t %s:%s -f %s .", name, tag, filename)

	_, eout, err := executor.ExecuteSeparate(cmd)
	if err != nil {
		if eout != "" {
			return fmt.Errorf("could not build image: %v (%s)", err, eout)
		}
		return fmt.Errorf("could not build image: %v", err)
	}

	return nil
}

func (w *Wrapper) CreateContainer(name, hostname, image string, options ...ContainerOptions) error {
	ctx := context.Background()

	c := &containerOptions{}

	for _, opt := range options {
		opt(c)
	}

	containerCfg := &container.Config{
		Image:    image,
		Hostname: hostname,
	}
	hostCfg := &container.HostConfig{
		Privileged:  c.priviliged,
		CapAdd:      c.capabilities,
		NetworkMode: container.NetworkMode(c.networkMode),
	}
	networkCfg := &network.NetworkingConfig{}

	id, err := CreateContainer(ctx, w.cli, containerCfg, hostCfg, networkCfg, name)
	if err != nil {
		return fmt.Errorf("could not create container: %v", err)
	}
	w.containerNamesToId[name] = id
	return nil
}

func (w *Wrapper) StartContainer(name string) error {
	ctx := context.Background()
	if err := StartContainer(ctx, w.cli, w.containerNamesToId[name]); err != nil {
		return fmt.Errorf("could not start container: %v", err)
	}
	return nil
}

func (w *Wrapper) StopContainer(name string) error {
	ctx := context.Background()
	if err := StopContainer(ctx, w.cli, w.containerNamesToId[name]); err != nil {
		return fmt.Errorf("could not stop container: %v", err)
	}
	return nil
}

func (w *Wrapper) RemoveContainer(name string) error {
	ctx := context.Background()
	if err := RemoveContainer(ctx, w.cli, w.containerNamesToId[name]); err != nil {
		return fmt.Errorf("could not remove container: %v", err)
	}
	return nil
}

func (w *Wrapper) RemoveImage(name string) error {
	ctx := context.Background()
	if err := RemoveImage(ctx, w.cli, name); err != nil {
		return fmt.Errorf("could not remove image: %v", err)
	}
	return nil
}

func (w *Wrapper) CreateNetwork(name string, subnet string, gateway string) error {
	ctx := context.Background()
	id, err := CreateNetwork(ctx, w.cli, name, network.CreateOptions{
		IPAM: &network.IPAM{
			Config: []network.IPAMConfig{
				{
					Subnet:  subnet,
					Gateway: gateway,
				},
			},
		},
	})
	if err != nil {
		return fmt.Errorf("could not create network: %v", err)
	}
	w.networkNamesToId[name] = id
	return nil
}

func (w *Wrapper) RemoveNetwork(name string) error {
	ctx := context.Background()
	if err := RemoveNetwork(ctx, w.cli, w.networkNamesToId[name]); err != nil {
		return fmt.Errorf("could not remove network: %v", err)
	}
	return nil
}

func (w *Wrapper) ConnectContainerToNetwork(containerName, networkName string) error {
	return helpers.WrapError("not implemented")
}

func (w *Wrapper) DisconnectContainerFromNetwork(containerName, networkName string) error {
	return helpers.WrapError("not implemented")
}

func (w *Wrapper) ModifyContainerGateway(containerName, gateway string) error {
	return helpers.WrapError("not implemented")
}

func (w *Wrapper) RemoveNetworkIsolationRules() error {
	return helpers.WrapError("not implemented")
}

func (w *Wrapper) ExecuteInContainer(containerName, command string) (exitCode int, stdout, stderr io.Reader, err error) {
	id := w.containerNamesToId[containerName]
	return RunCommandInContainer(w.cli, id, command)
}
