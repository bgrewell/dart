package lxd

import (
	"bytes"
	"context"
	"fmt"
	"io"

	"github.com/bgrewell/dart/internal/config"
	lxd "github.com/canonical/lxd/client"
)

// ConnectionOptions defines options for connecting to an LXD server
type ConnectionOptions struct {
	// UnixSocket is the path to the Unix socket (default: uses system default)
	UnixSocket string
	// HTTPSAddress is the HTTPS address for remote connections
	HTTPSAddress string
	// ClientCert is the path to the client certificate for HTTPS connections
	ClientCert string
	// ClientKey is the path to the client key for HTTPS connections
	ClientKey string
	// ServerCert is the path to the server certificate for HTTPS connections
	ServerCert string
}

// NewWrapper creates a new LXD wrapper instance
func NewWrapper(cfg *config.LxdConfig) (*Wrapper, error) {
	w := &Wrapper{
		cfg:               cfg,
		networkNamesToId:  make(map[string]string),
		instanceNamesToId: make(map[string]string),
	}

	// Connect to the LXD server
	if err := w.Connect(nil); err != nil {
		return nil, err
	}

	return w, nil
}

// NewWrapperWithOptions creates a new LXD wrapper instance with custom connection options
func NewWrapperWithOptions(cfg *config.LxdConfig, opts *ConnectionOptions) (*Wrapper, error) {
	w := &Wrapper{
		cfg:               cfg,
		networkNamesToId:  make(map[string]string),
		instanceNamesToId: make(map[string]string),
	}

	// Connect to the LXD server with options
	if err := w.Connect(opts); err != nil {
		return nil, err
	}

	return w, nil
}

// Wrapper provides high-level operations for LXD management
type Wrapper struct {
	server            lxd.InstanceServer
	cfg               *config.LxdConfig
	networkNamesToId  map[string]string
	instanceNamesToId map[string]string
}

// Connect establishes a connection to the LXD server
func (w *Wrapper) Connect(opts *ConnectionOptions) error {
	var server lxd.InstanceServer
	var err error

	if opts == nil {
		// Use default Unix socket connection
		server, err = lxd.ConnectLXDUnix("", nil)
	} else if opts.UnixSocket != "" {
		// Use specified Unix socket
		server, err = lxd.ConnectLXDUnix(opts.UnixSocket, nil)
	} else if opts.HTTPSAddress != "" {
		// Use HTTPS connection
		args := &lxd.ConnectionArgs{}
		if opts.ClientCert != "" && opts.ClientKey != "" {
			args.TLSClientCert = opts.ClientCert
			args.TLSClientKey = opts.ClientKey
		}
		if opts.ServerCert != "" {
			args.TLSServerCert = opts.ServerCert
		}
		server, err = lxd.ConnectLXD(opts.HTTPSAddress, args)
	} else {
		// Default to Unix socket
		server, err = lxd.ConnectLXDUnix("", nil)
	}

	if err != nil {
		return fmt.Errorf("failed to connect to LXD server: %w", err)
	}

	w.server = server
	return nil
}

// Configured returns true if the wrapper has been configured
func (w *Wrapper) Configured() bool {
	return w.cfg != nil
}

// GetServer returns the underlying LXD instance server
func (w *Wrapper) GetServer() lxd.InstanceServer {
	return w.server
}

// Setup configures the LXD wrapper by creating networks and profiles
func (w *Wrapper) Setup() error {
	if w.cfg == nil {
		return nil
	}

	ctx := context.Background()

	// Create the networks
	for _, net := range w.cfg.Networks {
		if err := w.CreateNetwork(net.Name, net.Subnet, net.Gateway); err != nil {
			return err
		}
	}

	// Create the profiles
	for _, profileCfg := range w.cfg.Profiles {
		profile := configToProfile(profileCfg)
		if err := CreateProfile(ctx, w.server, profile); err != nil {
			return err
		}
	}

	return nil
}

// Teardown removes the networks and profiles created by the wrapper
func (w *Wrapper) Teardown() error {
	if w.cfg == nil {
		return nil
	}

	ctx := context.Background()

	// Remove the networks
	for _, net := range w.cfg.Networks {
		if err := w.RemoveNetwork(net.Name); err != nil {
			return err
		}
	}

	// Remove the profiles (skip default profiles)
	for _, profile := range w.cfg.Profiles {
		if profile.Name != "default" {
			if err := DeleteProfile(ctx, w.server, profile.Name); err != nil {
				return err
			}
		}
	}

	return nil
}

// CreateInstance creates a new instance (container or VM)
func (w *Wrapper) CreateInstance(name, image string, instanceType InstanceType, options ...InstanceOption) error {
	ctx := context.Background()

	cfg := &InstanceConfig{
		Name:        name,
		Image:       image,
		Type:        instanceType,
		ImageServer: "https://images.linuxcontainers.org",
		Protocol:    "simplestreams",
	}

	// Apply options
	for _, opt := range options {
		opt(cfg)
	}

	if err := CreateInstance(ctx, w.server, cfg); err != nil {
		return fmt.Errorf("could not create instance: %w", err)
	}

	w.instanceNamesToId[name] = name
	return nil
}

// StartInstance starts an instance
func (w *Wrapper) StartInstance(name string) error {
	ctx := context.Background()
	if err := StartInstance(ctx, w.server, name); err != nil {
		return fmt.Errorf("could not start instance: %w", err)
	}
	return nil
}

// StopInstance stops an instance
func (w *Wrapper) StopInstance(name string, force bool) error {
	ctx := context.Background()
	if err := StopInstance(ctx, w.server, name, force); err != nil {
		return fmt.Errorf("could not stop instance: %w", err)
	}
	return nil
}

// RestartInstance restarts an instance
func (w *Wrapper) RestartInstance(name string, force bool) error {
	ctx := context.Background()
	if err := RestartInstance(ctx, w.server, name, force); err != nil {
		return fmt.Errorf("could not restart instance: %w", err)
	}
	return nil
}

// RemoveInstance removes an instance
func (w *Wrapper) RemoveInstance(name string) error {
	ctx := context.Background()
	if err := DeleteInstance(ctx, w.server, name); err != nil {
		return fmt.Errorf("could not remove instance: %w", err)
	}
	delete(w.instanceNamesToId, name)
	return nil
}

// CreateNetwork creates a new network
func (w *Wrapper) CreateNetwork(name string, subnet string, gateway string) error {
	ctx := context.Background()
	if err := CreateBridgeNetwork(ctx, w.server, name, subnet, gateway); err != nil {
		return fmt.Errorf("could not create network: %w", err)
	}
	w.networkNamesToId[name] = name
	return nil
}

// RemoveNetwork removes a network
func (w *Wrapper) RemoveNetwork(name string) error {
	ctx := context.Background()
	if err := DeleteNetwork(ctx, w.server, name); err != nil {
		return fmt.Errorf("could not remove network: %w", err)
	}
	delete(w.networkNamesToId, name)
	return nil
}

// ConnectInstanceToNetwork connects an instance to a network
func (w *Wrapper) ConnectInstanceToNetwork(instanceName, networkName, deviceName string) error {
	ctx := context.Background()
	if deviceName == "" {
		deviceName = "eth-" + networkName
	}
	return AttachNetworkToInstance(ctx, w.server, instanceName, networkName, deviceName)
}

// DisconnectInstanceFromNetwork disconnects an instance from a network
func (w *Wrapper) DisconnectInstanceFromNetwork(instanceName, deviceName string) error {
	ctx := context.Background()
	return DetachNetworkFromInstance(ctx, w.server, instanceName, deviceName)
}

// ExecuteInInstance runs a command in an instance
func (w *Wrapper) ExecuteInInstance(instanceName, command string) (exitCode int, stdout, stderr io.Reader, err error) {
	var sout, serr bytes.Buffer

	// Use bash to execute the command
	cmd := []string{"/bin/bash", "-c", command}

	exitCode, err = ExecInInstance(w.server, instanceName, cmd, &sout, &serr)
	if err != nil {
		return -1, nil, nil, err
	}

	return exitCode, &sout, &serr, nil
}

// GetInstanceState returns the current state of an instance
func (w *Wrapper) GetInstanceState(name string) (string, error) {
	ctx := context.Background()
	state, _, err := GetInstanceState(ctx, w.server, name)
	if err != nil {
		return "", err
	}
	return state.Status, nil
}

// ListInstances returns a list of all instances
func (w *Wrapper) ListInstances() ([]string, error) {
	ctx := context.Background()
	instances, err := ListInstances(ctx, w.server, nil)
	if err != nil {
		return nil, err
	}

	names := make([]string, len(instances))
	for i, inst := range instances {
		names[i] = inst.Name
	}
	return names, nil
}

// CreateSnapshot creates a snapshot of an instance
func (w *Wrapper) CreateSnapshot(instanceName, snapshotName string, stateful bool) error {
	ctx := context.Background()
	return CreateInstanceSnapshot(ctx, w.server, instanceName, snapshotName, stateful)
}

// DeleteSnapshot deletes a snapshot
func (w *Wrapper) DeleteSnapshot(instanceName, snapshotName string) error {
	ctx := context.Background()
	return DeleteInstanceSnapshot(ctx, w.server, instanceName, snapshotName)
}

// InstanceOption is a function type that sets options for creating an instance
type InstanceOption func(*InstanceConfig)

// WithImageServer sets the image server URL
func WithImageServer(server string) InstanceOption {
	return func(c *InstanceConfig) {
		c.ImageServer = server
	}
}

// WithProtocol sets the protocol (lxd or simplestreams)
func WithProtocol(protocol string) InstanceOption {
	return func(c *InstanceConfig) {
		c.Protocol = protocol
	}
}

// WithProfiles sets the profiles to apply to the instance
func WithProfiles(profiles []string) InstanceOption {
	return func(c *InstanceConfig) {
		c.Profiles = profiles
	}
}

// WithConfig sets the instance configuration
func WithConfig(config map[string]string) InstanceOption {
	return func(c *InstanceConfig) {
		c.Config = config
	}
}

// WithDevices sets the devices for the instance
func WithDevices(devices map[string]Device) InstanceOption {
	return func(c *InstanceConfig) {
		c.Devices = devices
	}
}

// WithEphemeral sets whether the instance is ephemeral
func WithEphemeral(ephemeral bool) InstanceOption {
	return func(c *InstanceConfig) {
		c.Ephemeral = ephemeral
	}
}

// configToProfile converts a config.LxdProfileConfig to an lxd.Profile
func configToProfile(cfg *config.LxdProfileConfig) *Profile {
	devices := make(map[string]Device)
	for name, devCfg := range cfg.Devices {
		devices[name] = Device{
			Type: devCfg.Type,
			Path: devCfg.Path,
			Pool: devCfg.Pool,
			Name: devCfg.Name,
			Opts: devCfg.Opts,
		}
	}

	return &Profile{
		Name:        cfg.Name,
		Description: cfg.Description,
		Config:      cfg.Config,
		Devices:     devices,
	}
}
