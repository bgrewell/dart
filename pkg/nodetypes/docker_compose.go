package nodetypes

import (
	"encoding/json"
	"fmt"
	"github.com/bgrewell/dart/internal/docker"
	"github.com/bgrewell/dart/internal/execution"
	"github.com/bgrewell/dart/internal/helpers"
	"github.com/bgrewell/dart/pkg/ifaces"
	"strings"
)

var _ ifaces.Node = &DockerComposeNode{}

// DockerComposeNodeOpts represents the options for a docker-compose node
type DockerComposeNodeOpts struct {
	ComposeFile string                 `yaml:"compose_file,omitempty" json:"compose_file"`
	ProjectName string                 `yaml:"project_name,omitempty" json:"project_name"`
	Service     string                 `yaml:"service,omitempty" json:"service"`
	ExecOptions map[string]interface{} `yaml:"exec_opts,omitempty" json:"exec_opts"`
}

// NewDockerComposeNode creates a new docker-compose node
func NewDockerComposeNode(wrapper *docker.Wrapper, name string, opts ifaces.NodeOptions) (node ifaces.Node, err error) {
	jsonData, err := json.Marshal(opts)
	if err != nil {
		return nil, err
	}

	var nodeopts DockerComposeNodeOpts
	err = json.Unmarshal(jsonData, &nodeopts)
	if err != nil {
		return nil, err
	}

	// Validate required options
	if nodeopts.ComposeFile == "" {
		return nil, fmt.Errorf("compose_file is required for docker-compose node")
	}

	return &DockerComposeNode{
		name:    name,
		wrapper: wrapper,
		options: nodeopts,
	}, nil
}

// DockerComposeNode represents a node that manages a docker-compose stack
type DockerComposeNode struct {
	name     string
	wrapper  *docker.Wrapper
	options  DockerComposeNodeOpts
	stack    *docker.ComposeStack
	stackKey string
}

// Setup starts the docker-compose stack
func (d *DockerComposeNode) Setup() error {
	// Generate a unique key for this compose stack
	projectName := d.options.ProjectName
	if projectName == "" {
		projectName = d.name
	}
	d.stackKey = docker.GetStackKey(d.options.ComposeFile, projectName)

	// Get or create the stack from the registry
	registry := d.wrapper.GetComposeRegistry()
	stack, err := registry.GetOrCreateStack(d.stackKey, func() (*docker.ComposeStack, error) {
		// Create and start the compose stack
		stack := docker.NewComposeStack(
			d.wrapper.GetClient(),
			d.name,
			d.options.ComposeFile,
			projectName,
		)

		if err := stack.Up(); err != nil {
			return nil, fmt.Errorf("failed to start compose stack: %v", err)
		}

		return stack, nil
	})

	if err != nil {
		return err
	}

	d.stack = stack
	return nil
}

// Teardown stops and removes the docker-compose stack
func (d *DockerComposeNode) Teardown() error {
	if d.stack == nil {
		return nil
	}

	// Release the stack from the registry
	registry := d.wrapper.GetComposeRegistry()
	shouldTeardown := registry.ReleaseStack(d.stackKey)

	// Only tear down if this is the last node using the stack
	if shouldTeardown {
		if err := d.stack.Down(); err != nil {
			return fmt.Errorf("failed to stop compose stack: %v", err)
		}
	}

	d.stack = nil
	return nil
}

// Execute runs a command in the specified service of the compose stack
func (d *DockerComposeNode) Execute(command string, options ...execution.ExecutionOption) (result *execution.ExecutionResult, err error) {
	if d.stack == nil {
		return nil, fmt.Errorf("compose stack not initialized")
	}

	// Determine which service to execute in
	service := d.options.Service
	if service == "" {
		// If no default service is specified, we need to fail
		return nil, fmt.Errorf("no service specified for execution (set 'service' in node options)")
	}

	// Execute the command in the service
	code, stdout, stderr, err := d.stack.ExecInService(service, command)
	if err != nil {
		return nil, fmt.Errorf("failed to execute in service '%s': %v", service, err)
	}

	return &execution.ExecutionResult{
		ExecutionId: helpers.GetRandomId(),
		ExitCode:    code,
		Stdout:      strings.NewReader(stdout),
		Stderr:      strings.NewReader(stderr),
	}, nil
}

// Close cleans up any resources
func (d *DockerComposeNode) Close() error {
	// No specific cleanup needed beyond teardown
	return nil
}
