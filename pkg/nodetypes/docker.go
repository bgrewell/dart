package nodetypes

import (
	"encoding/json"
	"github.com/bgrewell/dart/internal/docker"
	"github.com/bgrewell/dart/internal/execution"
	"github.com/bgrewell/dart/internal/helpers"
	"github.com/bgrewell/dart/pkg/ifaces"
)

var _ ifaces.Node = &DockerNode{}

type DockerNetworkOpts struct {
	Name   string `yaml:"name,omitempty" json:"name"`
	Subnet string `yaml:"subnet,omitempty" json:"subnet"`
	Ip     string `yaml:"ip,omitempty" json:"ip"`
}

type DockerNodeOpts struct {
	Image       string                 `yaml:"image,omitempty" json:"image"`
	ExecOptions map[string]interface{} `yaml:"exec_opts,omitempty" json:"exec_opts"`
	Networks    []DockerNetworkOpts    `yaml:"networks,omitempty" json:"networks"`
}

func NewDockerNode(wrapper *docker.Wrapper, name string, opts ifaces.NodeOptions) (node ifaces.Node, err error) {

	jsonData, err := json.Marshal(opts)
	if err != nil {
		return nil, err
	}

	var nodeopts DockerNodeOpts
	err = json.Unmarshal(jsonData, &nodeopts)
	if err != nil {
		return nil, err
	}

	return &DockerNode{
		name:    name,
		wrapper: wrapper,
		options: nodeopts,
	}, nil
}

type DockerNode struct {
	name    string
	wrapper *docker.Wrapper
	options DockerNodeOpts
}

func (d *DockerNode) Setup() error {
	priv := docker.WithPrivileged()
	if err := d.wrapper.CreateContainer(d.name, d.name, d.options.Image, priv); err != nil {
		return err
	}
	if err := d.wrapper.StartContainer(d.name); err != nil {
		return err
	}
	// Wait for the container to be fully ready (running and responsive)
	if err := d.wrapper.WaitForContainerReady(d.name); err != nil {
		return err
	}
	return nil
}

func (d *DockerNode) Teardown() error {
	if err := d.wrapper.StopContainer(d.name); err != nil {
		return err
	}
	return d.wrapper.RemoveContainer(d.name)
}

func (d *DockerNode) Execute(command string, options ...execution.ExecutionOption) (result *execution.ExecutionResult, err error) {
	code, stdout, stderr, err := d.wrapper.ExecuteInContainer(d.name, command)
	if err != nil {
		return nil, err
	}

	return &execution.ExecutionResult{
		ExecutionId: helpers.GetRandomId(),
		ExitCode:    code,
		Stdout:      stdout,
		Stderr:      stderr,
	}, nil
}

func (d *DockerNode) Close() error {
	//TODO implement me
	return helpers.WrapError("not implemented")
}
