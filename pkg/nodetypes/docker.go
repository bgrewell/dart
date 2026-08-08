package nodetypes

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

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
	// Privileged grants the container full host capabilities. It is
	// opt-in: DART used to set it unconditionally, which quietly gave
	// every test container far more access than a test needs.
	Privileged bool     `yaml:"privileged,omitempty" json:"privileged"`
	Volumes    []string `yaml:"volumes,omitempty" json:"volumes"` // host:container[:ro]
	Env        []string `yaml:"env,omitempty" json:"env"`         // KEY=VALUE
	Ports      []string `yaml:"ports,omitempty" json:"ports"`     // host:container[/proto]
	// Capabilities adds specific Linux capabilities without going
	// privileged (e.g. NET_ADMIN for network tests).
	Capabilities []string `yaml:"capabilities,omitempty" json:"capabilities"`
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

// resolveVolumes turns relative host paths into absolute ones. The Engine
// API treats a non-absolute source as a NAMED VOLUME, so "./fixtures"
// would silently mount an empty volume instead of the directory —
// a passing-looking test against missing data.
func resolveVolumes(volumes []string) ([]string, error) {
	resolved := make([]string, 0, len(volumes))
	for _, spec := range volumes {
		parts := strings.SplitN(spec, ":", 2)
		if len(parts) != 2 || parts[0] == "" {
			return nil, fmt.Errorf("volume %q must be host:container[:options]", spec)
		}
		source := parts[0]
		// A bare name (no separator) is a named volume by intent; a path
		// is anything containing a separator or starting with . or ~
		if strings.HasPrefix(source, "~") {
			home, err := os.UserHomeDir()
			if err != nil {
				return nil, fmt.Errorf("volume %q: cannot resolve ~: %w", spec, err)
			}
			source = filepath.Join(home, strings.TrimPrefix(source, "~"))
		}
		if strings.ContainsAny(source, "/.") && !filepath.IsAbs(source) {
			absolute, err := filepath.Abs(source)
			if err != nil {
				return nil, fmt.Errorf("volume %q: cannot resolve host path: %w", spec, err)
			}
			source = absolute
		}
		resolved = append(resolved, source+":"+parts[1])
	}
	return resolved, nil
}

func (d *DockerNode) Setup() error {
	var opts []docker.ContainerOptions
	if d.options.Privileged {
		opts = append(opts, docker.WithPrivileged())
	}
	if len(d.options.Capabilities) > 0 {
		opts = append(opts, docker.WithCapabilities(d.options.Capabilities))
	}
	if len(d.options.Volumes) > 0 {
		volumes, err := resolveVolumes(d.options.Volumes)
		if err != nil {
			return err
		}
		opts = append(opts, docker.WithVolumes(volumes))
	}
	if len(d.options.Env) > 0 {
		opts = append(opts, docker.WithEnv(d.options.Env))
	}
	if len(d.options.Ports) > 0 {
		opts = append(opts, docker.WithPorts(d.options.Ports))
	}

	if err := d.wrapper.CreateContainer(d.name, d.name, d.options.Image, opts...); err != nil {
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

// Teardown stops and removes the container. A container that no longer
// exists (partial setup, previous cleanup, teardown-only run) counts as
// already removed.
func (d *DockerNode) Teardown() error {
	if err := d.wrapper.StopContainer(d.name); err != nil {
		if docker.IsNotFound(err) {
			return nil
		}
		return err
	}
	if err := d.wrapper.RemoveContainer(d.name); err != nil && !docker.IsNotFound(err) {
		return err
	}
	return nil
}

func (d *DockerNode) Execute(command string, options ...execution.ExecutionOption) (result *execution.ExecutionResult, err error) {
	code, stdout, stderr, err := d.wrapper.ExecuteInContainerStreaming(d.name, command, execution.IsDebugMode())
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

var _ ifaces.NetworkInspector = &DockerNode{}

// NetworkFacts reports the container's addresses from Docker's own
// inspection data, so suites can reference {{ fact "node" "ipv4" }}
// without a fact command. Each attached network also yields a
// per-network fact ("ipv4.test-net").
func (d *DockerNode) NetworkFacts() (map[string]string, error) {
	return d.wrapper.ContainerNetworkFacts(d.name)
}

// Close has nothing to release: the container lifecycle is handled by
// Setup/Teardown and the client belongs to the shared wrapper. Returning
// an error here would fail every run's final cleanup.
func (d *DockerNode) Close() error {
	return nil
}
