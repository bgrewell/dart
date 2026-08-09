package docker

import (
	"context"
	"fmt"
	"github.com/bgrewell/dart/internal/config"
	"github.com/bgrewell/dart/internal/helpers"
	"github.com/bgrewell/dart/pkg/ifaces"
	"github.com/bgrewell/go-execute/v2"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
	"io"
	"path/filepath"
	"sort"
)

// Ensure Wrapper implements the PlatformManager interface
var _ ifaces.PlatformManager = &Wrapper{}

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
		composeRegistry:    NewComposeStackRegistry(),
	}, nil
}

type Wrapper struct {
	// cli is the interface rather than the concrete client so the wrapper's
	// call sequences can be asserted without a daemon
	cli                client.APIClient
	cfg                *config.DockerConfig
	networkNamesToId   map[string]string
	containerNamesToId map[string]string
	composeRegistry    *ComposeStackRegistry
}

// Configured returns true if the wrapper has been configured
func (w *Wrapper) Configured() bool {
	return w.cfg != nil
}

// Name returns the name of this platform manager
func (w *Wrapper) Name() string {
	return "docker"
}

// GetClient returns the Docker client
func (w *Wrapper) GetClient() client.APIClient {
	return w.cli
}

// GetComposeRegistry returns the compose stack registry
func (w *Wrapper) GetComposeRegistry() *ComposeStackRegistry {
	return w.composeRegistry
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

// Teardown removes the networks and images created by the wrapper.
// Resources that no longer exist — a partial setup, a previous run's
// cleanup, or a teardown-only run in a fresh process — count as already
// removed rather than failing the remaining teardown.
func (w *Wrapper) Teardown() error {
	// Remove the networks
	for _, net := range w.cfg.Networks {
		if err := w.RemoveNetwork(net.Name); err != nil && !IsNotFound(err) {
			return err
		}
	}

	// Remove the images
	for _, image := range w.cfg.Images {
		if err := w.RemoveImage(image.Name); err != nil && !IsNotFound(err) {
			return err
		}
	}

	return nil
}

// containerRef resolves a container name to the ID recorded at creation,
// falling back to the name itself: the map only holds containers created by
// this process, and a --teardown-only run starts with it empty. The Docker
// API accepts names wherever it accepts IDs.
func (w *Wrapper) containerRef(name string) string {
	if id, ok := w.containerNamesToId[name]; ok && id != "" {
		return id
	}
	return name
}

// networkRef resolves a network name to its recorded ID, falling back to
// the name. See containerRef.
func (w *Wrapper) networkRef(name string) string {
	if id, ok := w.networkNamesToId[name]; ok && id != "" {
		return id
	}
	return name
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
		Image:      image,
		Hostname:   hostname,
		Env:        c.env,
		Cmd:        c.command,
		Entrypoint: c.entrypoint,
	}
	hostCfg := &container.HostConfig{
		Privileged:  c.priviliged,
		CapAdd:      c.capabilities,
		NetworkMode: container.NetworkMode(c.networkMode),
		Binds:       c.volumes,
	}

	if len(c.ports) > 0 {
		exposed, bindings, err := nat.ParsePortSpecs(c.ports)
		if err != nil {
			return fmt.Errorf("invalid port specification: %w", err)
		}
		containerCfg.ExposedPorts = exposed
		hostCfg.PortBindings = bindings
	}
	// Docker accepts a single endpoint at creation; the rest are connected
	// once the container exists
	networkCfg := &network.NetworkingConfig{}
	if len(c.networks) > 0 {
		networkCfg.EndpointsConfig = map[string]*network.EndpointSettings{
			c.networks[0].Name: endpointSettings(c.networks[0]),
		}
	}

	id, err := CreateContainer(ctx, w.cli, containerCfg, hostCfg, networkCfg, name)
	if err != nil {
		return fmt.Errorf("could not create container: %v", err)
	}
	w.containerNamesToId[name] = id

	for _, attachment := range c.networks[min(1, len(c.networks)):] {
		if err := AttachNetwork(ctx, w.cli, attachment.Name, id, endpointSettings(attachment)); err != nil {
			return fmt.Errorf("could not attach container %s to network %s: %w", name, attachment.Name, err)
		}
	}
	return nil
}

// endpointSettings renders one attachment, carrying a fixed address when the
// suite asked for one.
func endpointSettings(attachment NetworkAttachment) *network.EndpointSettings {
	settings := &network.EndpointSettings{}
	if attachment.IPv4 != "" {
		settings.IPAMConfig = &network.EndpointIPAMConfig{IPv4Address: attachment.IPv4}
	}
	return settings
}

func (w *Wrapper) StartContainer(name string) error {
	ctx := context.Background()
	if err := StartContainer(ctx, w.cli, w.containerRef(name)); err != nil {
		return fmt.Errorf("could not start container: %v", err)
	}
	return nil
}

func (w *Wrapper) WaitForContainerReady(name string) error {
	ctx := context.Background()
	if err := WaitForContainerReady(ctx, w.cli, w.containerRef(name), nil); err != nil {
		return fmt.Errorf("container %s not ready: %v", name, err)
	}
	return nil
}

func (w *Wrapper) StopContainer(name string) error {
	ctx := context.Background()
	if err := StopContainer(ctx, w.cli, w.containerRef(name)); err != nil {
		return fmt.Errorf("could not stop container: %v", err)
	}
	return nil
}

func (w *Wrapper) RemoveContainer(name string) error {
	ctx := context.Background()
	if err := RemoveContainer(ctx, w.cli, w.containerRef(name)); err != nil {
		return fmt.Errorf("could not remove container: %v", err)
	}
	return nil
}

// ContainerNetworkFacts returns the container's global addresses keyed as
// "ipv4"/"ipv6" plus per-network entries ("ipv4.<network>").
func (w *Wrapper) ContainerNetworkFacts(name string) (map[string]string, error) {
	ctx := context.Background()
	inspect, err := w.cli.ContainerInspect(ctx, w.containerRef(name))
	if err != nil {
		return nil, err
	}
	facts := make(map[string]string)
	if inspect.NetworkSettings == nil {
		return facts, nil
	}

	networkNames := make([]string, 0, len(inspect.NetworkSettings.Networks))
	for network := range inspect.NetworkSettings.Networks {
		networkNames = append(networkNames, network)
	}
	sort.Strings(networkNames)

	for _, network := range networkNames {
		settings := inspect.NetworkSettings.Networks[network]
		if settings.IPAddress != "" {
			if _, exists := facts["ipv4"]; !exists {
				facts["ipv4"] = settings.IPAddress
			}
			facts["ipv4."+network] = settings.IPAddress
		}
		if settings.GlobalIPv6Address != "" {
			if _, exists := facts["ipv6"]; !exists {
				facts["ipv6"] = settings.GlobalIPv6Address
			}
			facts["ipv6."+network] = settings.GlobalIPv6Address
		}
	}
	return facts, nil
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
	if err := RemoveNetwork(ctx, w.cli, w.networkRef(name)); err != nil {
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
	return RunCommandInContainer(w.cli, w.containerRef(containerName), command)
}

func (w *Wrapper) ExecuteInContainerStreaming(containerName, command string, debugEnabled bool) (exitCode int, stdout, stderr io.Reader, err error) {
	return RunCommandInContainerStreaming(w.cli, w.containerRef(containerName), containerName, command, debugEnabled)
}
